all: run

build:
	make -C server build

test:
	make -C server test

run:
	make -C server run

clean:
	rm -rf build
