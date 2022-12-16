GOARCH=amd64
GOOS=linux
NAME=unspammer

# application repository path
REPOSITORY=github.com/oposs/${NAME}

# version from git
VERSION ?= $(shell git rev-parse --abbrev-ref HEAD | sed -e "s/.*\\///")
COMMIT ?= $(shell git rev-parse HEAD | cut -c 1-7)

# build ldflags
LDFLAGS ?= \
	-X ${REPOSITORY}/pkg/params.Name=${NAME} \
	-X ${REPOSITORY}/pkg/params.Version=${VERSION} \
	-X ${REPOSITORY}/pkg/params.Commit=${COMMIT}


all: build
.PHONY: all

build: build-linux-amd64
.PHONY: build

build-linux-amd64:
	env CGO_ENABLED=0 GOOS=linux GOARCH=amd64 go build -ldflags "$(LDFLAGS)" -o bin/$(NAME) ./cmd
.PHONY: build-amd64

clean:
	rm -r ./bin
.PHONY: clean
