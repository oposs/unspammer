GOARCH=amd64
GOOS=linux
BIN=unspammer

all: $(BIN)-$(GOARCH)-$(GOOS)

%-$(GOARCH)-$(GOOS): unspammer.go
	GOARCH=$(GOARCH) GOOS=$(GOOS) go build -o $@ $<
