BINARY=backup-agent
RESTORE_BIN=restore-agent
PEERCTL=peerctl
BUILD_FLAGS=-ldflags="-s -w"

.PHONY: all build test clean docker

all: build

build:
	@echo "Building binaries..."
	go build $(BUILD_FLAGS) -o bin/$(BINARY) ./cmd/backup-agent
	go build $(BUILD_FLAGS) -o bin/$(RESTORE_BIN) ./cmd/backup-agent-restore
	go build $(BUILD_FLAGS) -o bin/$(PEERCTL) ./cmd/peerctl

test:
	@echo "Running unit tests..."
	go test ./... -v

docker:
	docker build -t backupagent .

clean:
	@echo "Cleaning..."
	rm -rf bin

fmt:
	./format.sh

check-fmt:
	./check-format.sh

