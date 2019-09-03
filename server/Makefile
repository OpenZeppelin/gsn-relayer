pwd=$(shell pwd)
buildpath=$(pwd)/../build/server
server_exe=$(buildpath)/bin/RelayHttpServer
platforms=linux/amd64 windows/amd64 darwin/amd64 linux/386 windows/386
export GOPATH=$(pwd):$(buildpath)

ETHDIR=./src/github.com/ethereum/go-ethereum
ETHFILE=${ETHDIR}/Makefile
ETHREPO="https://github.com/ethereum/go-ethereum.git"
ETHVERSION=v1.8.21

RELAYHUB_BINDING=$(buildpath)/src/gen/librelay/relay_hub_sol.go
RELAYHUB_JSON=$(buildpath)/../contracts/RelayHubMock.json
RELAYHUB_ABI=$(buildpath)/../contracts/RelayHubMock.abi

MOCK_RECIPIENT_BINDING=$(buildpath)/src/gen/samplerec/sample_rec_sol.go
MOCK_RECIPIENT_JSON=$(buildpath)/../contracts/MockRecipient.json
MOCK_RECIPIENT_ABI=$(buildpath)/../contracts/MockRecipient.abi
MOCK_RECIPIENT_BIN=$(buildpath)/../contracts/MockRecipient.bin

$(server_exe): $(RELAYHUB_BINDING) $(ETHFILE) go-get-deps $() $(shell find . -maxdepth 3 -name '*.go') Makefile
	echo "Using GOPATH=$(GOPATH)"
	mkdir -p $(buildpath)/bin
	go build -o $(server_exe) relay
	strip $(server_exe)

$(RELAYHUB_BINDING): $(RELAYHUB_ABI)
	mkdir -p $(buildpath)/src/gen/librelay
	abigen --solc ../node_modules/.bin/solcjs --abi $(RELAYHUB_ABI) --pkg librelay --type IRelayHub --out $@

$(RELAYHUB_ABI): $(RELAYHUB_JSON)
	./scripts/get_abi_bin.js RelayHubMock

$(RELAYHUB_JSON): ../contracts/RelayHubMock.sol
	cd ../ && npx oz compile

$(MOCK_RECIPIENT_BINDING): $(MOCK_RECIPIENT_ABI)
	mkdir -p $(buildpath)/src/gen/samplerec
	abigen --abi $(MOCK_RECIPIENT_ABI) --bin $(MOCK_RECIPIENT_BIN) --pkg samplerec --type SampleRecipient --out $@

$(MOCK_RECIPIENT_ABI): $(MOCK_RECIPIENT_JSON)
	./scripts/get_abi_bin.js MockRecipient

$(MOCK_RECIPIENT_BIN): $(MOCK_RECIPIENT_ABI)

$(MOCK_RECIPIENT_JSON): ../contracts/MockRecipient.sol
	cd ../ && npx oz compile

$(ETHFILE): Makefile
	@echo "Downloading the ethereum library. Might take a few minutes."
	@if [ ! -d ${ETHDIR} ]; then \
	  git clone ${ETHREPO} --depth=1 --branch=${ETHVERSION} ${ETHDIR} ;\
	  go get -v -d ./...;\
	fi
	touch $(ETHFILE)

go-get-deps: $(RELAYHUB_BINDING) $(ETHFILE)
	go get -v code.cloudfoundry.org/clock github.com/syndtr/goleveldb/leveldb

go-get-xgo:
	go get github.com/karalabe/xgo

build: $(server_exe)

run: build
	$(server_exe)

release-binaries: build go-get-xgo
	mkdir -p ./build/xgo
	./bin/xgo -out='build/xgo/gsn-relay' --targets='$(platforms)' -v ./src/relay
	mv -f ./build/xgo/* $(buildpath)/bin/
	rm -rf ./build/xgo

test: build $(MOCK_RECIPIENT_BINDING)
	@scripts/test.sh
