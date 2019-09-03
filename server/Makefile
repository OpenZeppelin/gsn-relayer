pwd=$(shell pwd)
buildpath=$(pwd)/../build/server
server_exe=$(buildpath)/bin/RelayHttpServer
platforms=linux/amd64 windows/amd64 darwin/amd64 linux/386 windows/386
export GOPATH=$(pwd):$(buildpath)

server: $(server_exe)

ETHDIR=./src/github.com/ethereum/go-ethereum
ETHFILE=${ETHDIR}/Makefile
ETHREPO="https://github.com/ethereum/go-ethereum.git"
ETHVERSION=v1.8.21

GEN_FILE=$(buildpath)/src/gen/librelay/relay_hub_sol.go
RELAYHUB_JSON=$(buildpath)/../contracts/IRelayHub.json
RELAYHUB_ABI=$(buildpath)/../contracts/IRelayHub.abi

$(server_exe): $(GEN_FILE) $(GEN_FILE_REC) go-get $(ETHFILE) $() $(shell find . -maxdepth 3 -name '*.go') Makefile
	echo "Using GOPATH=$(GOPATH)"
	mkdir -p $(buildpath)/bin
	go build -o $(server_exe) relay
	strip $(server_exe)

go-get: $(GEN_FILE) $(ETHFILE)
	go get -v code.cloudfoundry.org/clock github.com/syndtr/goleveldb/leveldb

$(ETHFILE): Makefile
	@echo "Downloading the ethereum library. Might take a few minutes."
	@if [ ! -d ${ETHDIR} ]; then \
	  git clone ${ETHREPO} --depth=1 --branch=${ETHVERSION} ${ETHDIR} ;\
	  go get -v -d ./...;\
	fi
	touch $(ETHFILE)

gen-file: $(GEN_FILE) Makefile

$(RELAYHUB_JSON): ../contracts/IRelayHub.sol
	cd ../ && npx oz compile

$(RELAYHUB_ABI): $(RELAYHUB_JSON)
	./scripts/get_abi_bin.js IRelayHub

$(GEN_FILE): $(RELAYHUB_ABI)
	mkdir -p $(buildpath)/src/gen/librelay
	abigen --solc ../node_modules/.bin/solcjs --abi $(RELAYHUB_ABI) --pkg librelay --type IRelayHub --out $@

test: server
	@scripts/test.sh

go-get-xgo:
	go get github.com/karalabe/xgo

release-binaries: $(server_exe) go-get-xgo
	mkdir -p ./build/xgo
	go run github.com/karalabe/xgo -out='build/xgo/gsn-relay' --targets='$(platforms)' -v ./src/relay
	mv -f ./build/xgo/* $(buildpath)/bin/
	rm -rf ./build/xgo
