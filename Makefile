TARGET=pwndrop
BUILD_DIR=./build

# Host arch detection so `make build` / `make install` on the box where you
# happen to be sitting produces the right binary by default. Override with
# ARCH=arm64 (or amd64) to cross-compile explicitly.
HOST_ARCH := $(shell uname -m | sed 's/x86_64/amd64/;s/aarch64/arm64/')
ARCH ?= $(HOST_ARCH)
BIN=$(BUILD_DIR)/$(TARGET)-linux-$(ARCH)

.PHONY: all frontend build build-amd64 build-arm64 build-all clean install
all: build

frontend:
	@echo "*** building front-end..."
	@cd frontend && npm ci && npm run build

# `build` produces ONE binary, for $(ARCH). Defaults to the host arch.
build: frontend
	@echo "*** building $(TARGET) for linux/$(ARCH)..."
	@GOOS=linux GOARCH=$(ARCH) go build -ldflags="-s -w" -o $(BIN) ./
	@chmod 700 $(BIN)
	@ln -sf $(notdir $(BIN)) $(BUILD_DIR)/$(TARGET)

build-amd64:
	@$(MAKE) build ARCH=amd64

build-arm64:
	@$(MAKE) build ARCH=arm64

# `build-all` produces both binaries in a single front-end build, suitable
# for release artifact assembly.
build-all: frontend
	@echo "*** building $(TARGET) for linux/amd64..."
	@GOOS=linux GOARCH=amd64 go build -ldflags="-s -w" -o $(BUILD_DIR)/$(TARGET)-linux-amd64 ./
	@chmod 700 $(BUILD_DIR)/$(TARGET)-linux-amd64
	@echo "*** building $(TARGET) for linux/arm64..."
	@GOOS=linux GOARCH=arm64 go build -ldflags="-s -w" -o $(BUILD_DIR)/$(TARGET)-linux-arm64 ./
	@chmod 700 $(BUILD_DIR)/$(TARGET)-linux-arm64

clean:
	@go clean
	@rm -rf ./build/ ./frontend/dist/ ./frontend/node_modules/

install:
	@echo "*** stopping $(TARGET)"
	-@$(BIN) stop
	@echo "*** installing and starting $(TARGET) (linux/$(ARCH))"
	@$(BIN) install && $(BIN) start
	@$(BIN) status
