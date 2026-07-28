BINARY_ENCODE  ?= ystore-encode
BINARY_DECODE  ?= ystore-decode
GO             ?= go
GOFLAGS        ?= -ldflags="-s -w"
DIST_DIR       ?= dist

.PHONY: all build test lint clean release

all: build

build:
	$(GO) build $(GOFLAGS) -o $(BINARY_ENCODE) ./cmd/encode/
	$(GO) build $(GOFLAGS) -o $(BINARY_DECODE) ./cmd/decode/

test:
	$(GO) test ./... -timeout 180s -v

lint:
	$(GO) vet ./...

clean:
	rm -f $(BINARY_ENCODE) $(BINARY_DECODE)
	rm -rf $(DIST_DIR)

release: clean
	mkdir -p $(DIST_DIR)
	GOOS=linux   GOARCH=amd64 $(GO) build $(GOFLAGS) -o $(DIST_DIR)/ystore-encode-linux-amd64   ./cmd/encode/
	GOOS=linux   GOARCH=amd64 $(GO) build $(GOFLAGS) -o $(DIST_DIR)/ystore-decode-linux-amd64   ./cmd/decode/
	GOOS=linux   GOARCH=arm64 $(GO) build $(GOFLAGS) -o $(DIST_DIR)/ystore-encode-linux-arm64   ./cmd/encode/
	GOOS=linux   GOARCH=arm64 $(GO) build $(GOFLAGS) -o $(DIST_DIR)/ystore-decode-linux-arm64   ./cmd/decode/
	GOOS=darwin  GOARCH=amd64 $(GO) build $(GOFLAGS) -o $(DIST_DIR)/ystore-encode-darwin-amd64  ./cmd/encode/
	GOOS=darwin  GOARCH=amd64 $(GO) build $(GOFLAGS) -o $(DIST_DIR)/ystore-decode-darwin-amd64  ./cmd/decode/
	GOOS=darwin  GOARCH=arm64 $(GO) build $(GOFLAGS) -o $(DIST_DIR)/ystore-encode-darwin-arm64  ./cmd/encode/
	GOOS=darwin  GOARCH=arm64 $(GO) build $(GOFLAGS) -o $(DIST_DIR)/ystore-decode-darwin-arm64  ./cmd/decode/
	GOOS=windows GOARCH=amd64 $(GO) build $(GOFLAGS) -o $(DIST_DIR)/ystore-encode-windows-amd64.exe ./cmd/encode/
	GOOS=windows GOARCH=amd64 $(GO) build $(GOFLAGS) -o $(DIST_DIR)/ystore-decode-windows-amd64.exe ./cmd/decode/
	cd $(DIST_DIR) && for f in *; do sha256sum "$$f" > "$$f.sha256"; done
