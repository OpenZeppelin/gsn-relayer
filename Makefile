all: run

build:
	make -C server build

binaries:
	make -C server release-binaries

.PHONY: test
test:
	make -C server test

run:
	make -C server run

clean:
	rm -rf build
