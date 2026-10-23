GOARCH=amd64
GOOS=linux
NAME=unspammer

all: build
.PHONY: all

build: build-linux-amd64
.PHONY: build

build-linux-amd64:
	env CGO_ENABLED=0 GOOS=linux GOARCH=amd64 go build -o bin/$(NAME) ./cmd
.PHONY: build-amd64

clean:
	rm -r ./bin
.PHONY: clean

MAN_NAME = unspammer
include build/man.mk
