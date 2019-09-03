all: build

build:
	make -C server

test:
	make -C server test
