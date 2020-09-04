package main

import (
	"encoding/json"
	"flag"
	"fmt"
	"io/ioutil"
	"librelay"
	"librelay/txstore"
	"log"
	"math/big"
	"net/http"
	"net/url"
	"os"
	"path/filepath"
	"time"

	"github.com/ethereum/go-ethereum/common"
	"github.com/ethereum/go-ethereum/crypto"
	"github.com/ethereum/go-ethereum/params"
)

const VERSION = "0.4.6"

var KeystoreDir = filepath.Join(os.Getenv("PWD"), "data/keystore")
var RegistrationBlockRate uint64
var devMode bool // Whether we wait after calls to blockchain or return (almost) immediately. Usually when testing...

var ready = false
var removed = false

var relay librelay.IRelay
var server *http.Server
var stopKeepAlive chan bool
var stopRefreshBlockchainView chan bool
var stopUpdatingPendingTxs chan bool
var stopListeningToRelayRemoved chan bool

var timeUnit time.Duration

var firebaseClient *firestore.Client
var firebaseContext context.Context

var minimumRelayBalance = big.NewInt(1e17) // 0.1 eth

var relayerID = os.Getenv("RELAYER_ID")

func main() {
	log.SetFlags(log.LstdFlags | log.Lshortfile)
	log.Println("RelayHttpServer starting. version:", VERSION)

	configRelay(parseCommandLine())

	firebaseSetup()
	defer firebaseClient.Close()

	server = &http.Server{Addr: ":" + relay.GetPort(), Handler: nil}

	http.HandleFunc("/relay", assureRelayReady(relayHandler))
	http.HandleFunc("/getaddr", getEthAddrHandler)

	timeUnit = time.Minute
	if devMode {
		timeUnit = time.Second
	}
	stopKeepAlive = schedule(keepAlive, 10*timeUnit, 0)
	stopRefreshBlockchainView = schedule(refreshBlockchainView, 1*timeUnit, 0)
	stopUpdatingPendingTxs = schedule(updatePendingTxs, 1*timeUnit, 0)
	stopListeningToRelayRemoved = schedule(stopServingOnRelayRemoved, 1*timeUnit, 0)

	log.Println("RelayHttpServer started. Listening on port: ", relay.GetPort())
	err := server.ListenAndServe()
	if err != nil {
		log.Fatalln(err)
	}
}

func firebaseSetup() {
	log.Println("configuring firebase")

	// Use the application default credentials
	firebaseContext = context.Background()

	conf := &firebase.Config{ProjectID: "gsn-relayer"}

	app, err := firebase.NewApp(firebaseContext, conf)
	if err != nil {
			log.Println(err)
			return
	}

	firebaseClient, err = app.Firestore(firebaseContext)
	if err != nil {
		log.Println(err)
		return
	}

	log.Println("Set up Firebase to report for relayerID: ", relayerID)
}

func firebaseStore(userAgent string) {
	if firebaseClient == nil || firebaseContext == nil {
		log.Println("Firebase not set up, skipping store")
		return
	}

	if len(userAgent) == 0 {
		userAgent = "unknown-agent"
	}
	log.Println("FirebaseStore request from: ", userAgent)

	_, _, err := firebaseClient.Collection("requests").Add(firebaseContext, map[string]interface{}{
		"relayerID": relayerID,
		"userAgent":  userAgent,
		"timestamp": time.Now(),
	})

	if err != nil {
		log.Println("Firebase store failed: ", err)
	}
}

// http.HandlerFunc wrapper to assure we have enough balance to operate, and server already has stake and registered
func assureRelayReady(fn http.HandlerFunc) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {

		w.Header()["Access-Control-Allow-Origin"] = []string{"*"}
		w.Header()["Access-Control-Allow-Headers"] = []string{"Content-Type, Authorization, Content-Length, X-Requested-With"}
		w.Header()["Access-Control-Allow-Methods"] = []string{"GET, POST, OPTIONS"}

		if !shouldHandleRelayRequests() {
			err := fmt.Errorf("Relay not staked and registered yet")
			log.Println(err)
			w.Write([]byte("{\"error\":\"" + err.Error() + "\"}"))
			return
		}

		// wait for funding
		balance, err := relay.Balance()
		if err != nil {
			log.Println(err)
			w.Write([]byte("{\"error\":\"" + err.Error() + "\"}"))
			return
		}
		if balance.Cmp(big.NewInt(0)) == 0 {
			err = fmt.Errorf("Waiting for funding...")
			log.Println(err)
			w.Write([]byte("{\"error\":\"" + err.Error() + "\"}"))
			return
		}
		log.Println("Relay balance:", balance.String())

		gasPrice := relay.GasPrice()
		if gasPrice.Uint64() == 0 {
			err = fmt.Errorf("Waiting for gasPrice...")
			log.Println(err)
			w.Write([]byte("{\"error\":\"" + err.Error() + "\"}"))
			return
		}
		log.Println("Relay received gasPrice:", gasPrice.Uint64())
		fn(w, r)
	}

}

func getEthAddrHandler(w http.ResponseWriter, _ *http.Request) {

	w.Header()["Access-Control-Allow-Origin"] = []string{"*"}
	w.Header()["Access-Control-Allow-Headers"] = []string{"Content-Type, Authorization, Content-Length, X-Requested-With"}
	w.Header()["Access-Control-Allow-Methods"] = []string{"GET, OPTIONS"}

	getEthAddrResponse := &librelay.GetEthAddrResponse{
		RelayServerAddress: relay.Address(),
		MinGasPrice:        relay.GasPrice(),
		Ready:              shouldHandleRelayRequests(),
		Version:            VERSION,
	}
	resp, err := json.Marshal(getEthAddrResponse)
	if err != nil {
		log.Println(err)
		w.Write([]byte("{\"error\":\"" + err.Error() + "\"}"))
		return
	}
	log.Printf("address %s sent\n", relay.Address().Hex())

	w.Write(resp)
}

func relayHandler(w http.ResponseWriter, r *http.Request) {

	log.Println("Handling relay request...")
	if r.Method != http.MethodPost {
		w.WriteHeader(http.StatusOK)
		return
	}
	body, err := ioutil.ReadAll(r.Body)

	if err != nil {
		log.Println("Could not read request body", body, err)
		w.Write([]byte("{\"error\":\"" + err.Error() + "\"}"))
		return
	}
	var request = &librelay.RelayTransactionRequest{}
	err = json.Unmarshal(body, request)
	if err != nil {
		log.Println("Invalid json", body, err)
		w.Write([]byte("{\"error\":\"" + err.Error() + "\"}"))
		return
	}

	firebaseStore(request.UserAgent)

	signedTx, err := relay.CreateRelayTransaction(*request)
	if err != nil {
		log.Println("Failed to relay")
		w.Write([]byte("{\"error\":\"" + err.Error() + "\"}"))

		return
	}
	resp, err := signedTx.MarshalJSON()
	if err != nil {
		log.Println(err)
		w.Write([]byte("{\"error\":\"" + err.Error() + "\"}"))
		return
	}
	w.Write(resp)
}

func parseCommandLine() (relayParams librelay.RelayParams) {
	ownerAddress := flag.String("OwnerAddress", common.HexToAddress("0").Hex(), "Relay's owner address")
	fee := flag.Int64("Fee", 70, "Relay's per transaction fee")
	urlStr := flag.String("Url", "http://localhost:8090", "Relay server's url ")
	port := flag.String("Port", "", "Relay server's port")
	relayHubAddress := flag.String("RelayHubAddress", "0xD216153c06E857cD7f72665E0aF1d7D82172F494", "RelayHub address")
	defaultGasPrice := flag.Int64("DefaultGasPrice", int64(params.GWei), "Relay's default gasPrice per (non-relayed) transaction in wei")
	gasPricePercent := flag.Int64("GasPricePercent", 10, "Relay's gas price increase as percentage from current average. GasPrice = (100+GasPricePercent)/100 * eth_gasPrice() ")
	RegistrationBlockRate = *flag.Uint64("RegistrationBlockRate", 6000-200, "Relay registeration rate (in blocks)")
	ethereumNodeUrl := flag.String("EthereumNodeUrl", "http://localhost:8545", "The relay's ethereum node")
	workdir := flag.String("Workdir", filepath.Join(os.Getenv("PWD"), "data"), "The relay server's workdir")
	flag.BoolVar(&devMode, "DevMode", false, "Enable developer mode (do not retry unconfirmed txs, do not cache account nonce, do not wait after calls to the chain, faster polling)")

	flag.Parse()

	relayParams.OwnerAddress = common.HexToAddress(*ownerAddress)
	relayParams.Fee = big.NewInt(*fee)
	relayParams.Url = *urlStr
	u, err := url.Parse(*urlStr)
	if err != nil {
		log.Fatalln("Could not parse url")
	}
	if *port == "" && u.Port() != "" {
		log.Println("Using default published port given in url:", *port)
		*port = u.Port()
	}

	relayParams.Port = *port
	relayParams.RelayHubAddress = common.HexToAddress(*relayHubAddress)
	relayParams.DefaultGasPrice = *defaultGasPrice
	relayParams.GasPricePercent = big.NewInt(*gasPricePercent)
	relayParams.RegistrationBlockRate = RegistrationBlockRate
	relayParams.EthereumNodeURL = *ethereumNodeUrl
	relayParams.DBFile = filepath.Join(*workdir, "db")
	relayParams.DevMode = devMode

	KeystoreDir = filepath.Join(*workdir, "keystore")

	log.Println("Using RelayHub address: " + relayParams.RelayHubAddress.String())
	log.Println("Using workdir: " + *workdir)
	if devMode {
		log.Println("Using dev mode")
	}

	return relayParams

}

func configRelay(relayParams librelay.RelayParams) {
	log.Println("Constructing relay server in url ", relayParams.Url)
	privateKey := loadPrivateKey(KeystoreDir)
	log.Println("relay server address: ", crypto.PubkeyToAddress(privateKey.PublicKey).Hex())
	client, err := librelay.NewEthClient(relayParams.EthereumNodeURL, relayParams.DefaultGasPrice)
	if err != nil {
		log.Println("Could not connect to ethereum node", err)
		return
	}
	txStore, err := txstore.NewLevelDbTxStore(relayParams.DBFile, nil)
	if err != nil {
		log.Println("Could not create local transactions database", err)
		return
	}
	relay, err = librelay.NewRelayServer(
		relayParams.OwnerAddress, relayParams.Fee, relayParams.Url, relayParams.Port,
		relayParams.RelayHubAddress, relayParams.DefaultGasPrice, relayParams.GasPricePercent,
		privateKey, relayParams.RegistrationBlockRate, relayParams.EthereumNodeURL,
		client, txStore, nil, relayParams.DevMode)
	if err != nil {
		log.Println("Could not create Relay Server", err)
		return
	}
}

// Wait for server to be staked & funded by owner, then try and register on RelayHub
func refreshBlockchainView() {
	if removed {
		log.Println("Relay removed. No need to wait for owner actions")
		return
	}
	waitForOwnerActions()
	_, err := relay.BlockCountSinceRegistration()
	for ; err != nil; _, err = relay.BlockCountSinceRegistration() {
		if err != nil {
			log.Println(err)
		}
		ready = false
		sleep(15*time.Second, devMode)
	}

	for err := relay.RefreshGasPrice(); err != nil; err = relay.RefreshGasPrice() {
		if err != nil {
			log.Println(err)
		}
		ready = false
		sleep(10*time.Second, devMode)

	}
	gasPrice := relay.GasPrice()
	log.Println("GasPrice:", gasPrice.Uint64())

	ready = true
}

func updatePendingTxs() {
	if removed {
		log.Println("Relay removed. No need to wait for owner actions")
		return
	}
	waitForOwnerActions()

	log.Println("Updating unconfirmed txs...")
	_, err := relay.UpdateUnconfirmedTransactions()
	if err != nil {
		log.Println("Error updating unconfirmed txs", err)
	}
}

func waitForOwnerActions() {
	if removed {
		log.Println("Relay removed. No need to wait for owner actions")
		return
	}
	staked, err := relay.IsStaked()
	for ; err != nil || !staked; staked, err = relay.IsStaked() {
		if err != nil {
			log.Println(err)
		}
		ready = false
		log.Println("Waiting for stake...")
		sleep(5*time.Second, devMode)
	}

	// wait for funding
	balance, err := relay.Balance()
	if err != nil {
		log.Println(err)
		return
	}
	for ; err != nil || balance.Cmp(minimumRelayBalance) <= 0; balance, err = relay.Balance() {
		ready = false
		log.Printf("Server's balance too low (%s, required %s). Waiting for funding...", balance.String(), minimumRelayBalance.String())
		sleep(10*time.Second, devMode)
	}
}

func keepAlive() {
	if removed {
		log.Println("Relay removed. No need to reregister")
		return
	}
	waitForOwnerActions()
	count, err := relay.BlockCountSinceRegistration()
	if err != nil {
		log.Println(err)
	} else if count < (RegistrationBlockRate - 500) {
		log.Println("Relay registered lately: ", count, " blocks ago")
		return
	}
	log.Println("Registering relay...")
	for {
		err := relay.RegisterRelay()
		if err == nil {
			break
		}
		log.Println(err)
		log.Println("Trying to register again...")
		sleep(1*time.Minute, devMode)
	}
	log.Println("Done registering")
}

func stopServingOnRelayRemoved() {
	var err error
	removed, err = relay.IsRemoved()
	if err != nil {
		log.Println(err)
		return
	}
	if removed {
		log.Println("Relay removed. Listening to Unstaked event")
		schedule(shutdownOnRelayUnstaked, 1*timeUnit, 0)
		stopListeningToRelayRemoved <- true
	}

}

func shutdownOnRelayUnstaked() {
	var err error
	removed, err = relay.IsUnstaked()
	if err != nil {
		log.Println(err)
		return
	}
	if removed {
		log.Println("Relay removed. Listening to Unstaked event")
		log.Println("Relay unstaked. Sending balance back to owner")
		sleep(2*time.Minute, devMode)
		for {
			err = relay.SendBalanceToOwner()
			if err == nil {
				break
			}
			sleep(5*time.Second, devMode)
		}
		server.Close()
	}

}

func shouldHandleRelayRequests() bool {
	return ready && !removed
}
