.PHONY: all build clean mpcium mpc

BIN_DIR := bin

# Default target
all: build

# Build both binaries
build: mpcium mpc

# Install mpcium (builds and places it in $GOBIN or $GOPATH/bin)
mpcium:
	go install ./cmd/mpcium

# Install mpcium-cli
mpc:
	go install ./cmd/mpcium-cli

# Wipe out manually built binaries if needed (not required by go install)
clean:
	rm -rf $(BIN_DIR)
