BIN_DIR := bin
BINS    := mpcium mpcium-cli

.PHONY: all build clean

# Default target
all: build

# Build both binaries
build: $(BIN_DIR) $(BINS:%=$(BIN_DIR)/%)

# Ensure bin directory exists
$(BIN_DIR):
	mkdir -p $@

# Build mpcium
$(BIN_DIR)/mpcium: | $(BIN_DIR)
	go build -o $@ ./cmd/mpcium

# Build mpcium-cli
$(BIN_DIR)/mpcium-cli: | $(BIN_DIR)
	go build -o $@ ./cmd/mpcium-cli

# Wipe out built binaries
clean:
	rm -rf $(BIN_DIR)
