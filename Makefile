BINARY        ?= ystore
BINARY_ENCODE ?= ystore-encode
BINARY_DECODE ?= ystore-decode
GO            ?= go
GOFLAGS       ?= -ldflags="-s -w"
DIST_DIR      ?= dist

.PHONY: all build test lint clean release

all: build

build:
	$(GO) build $(GOFLAGS) -o $(BINARY) ./cmd/ystore/
	$(GO) build $(GOFLAGS) -o $(BINARY_ENCODE) ./cmd/encode/
	$(GO) build $(GOFLAGS) -o $(BINARY_DECODE) ./cmd/decode/

test:
	$(GO) test ./... -timeout 180s -v

lint:
	$(GO) vet ./...

clean:
	rm -f $(BINARY) $(BINARY_ENCODE) $(BINARY_DECODE)
	rm -rf $(DIST_DIR)

release: clean
	mkdir -p $(DIST_DIR)
	for osarch in "linux/amd64" "linux/arm64" "darwin/amd64" "darwin/arm64"; do \
		os=$${osarch%/*}; arch=$${osarch#*/}; \
		GOOS=$$os GOARCH=$$arch $(GO) build $(GOFLAGS) -o $(DIST_DIR)/ystore-$$os-$$arch ./cmd/ystore/; \
	done
	GOOS=windows GOARCH=amd64 $(GO) build $(GOFLAGS) -o $(DIST_DIR)/ystore-windows-amd64.exe ./cmd/ystore/
	cd $(DIST_DIR) && for f in *; do sha256sum "$$f" > "$$f.sha256"; done
