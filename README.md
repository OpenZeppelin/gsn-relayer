# GSN Relayer server

_Forked from tabookey-gasless_.

## Build

### Setup

Install [yarn](https://yarnpkg.com/lang/en/docs/install) and other dependencies:

```bash
$ sudo add-apt-repository -y ppa:ethereum/ethereum
$ sudo apt update
$ sudo apt install build-essential golang abigen
```

Install Node packages:
```bash
$ yarn
```

### Compile

```bash
$ make build
```

### Test

```bash
$ make test
```

## Run

```bash
$ make run
```
