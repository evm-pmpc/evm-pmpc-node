.PHONY: all clean build build-node build-bootstrap docker-node docker-bootstrap run-bootstrap run-worker

# Directories
BIN_DIR = bin
BUILD_DIR = build
DEPLOY_DIR = deployments

# Binaries
NODE_BIN = $(BIN_DIR)/evm-pmpc-node
BOOTSTRAP_BIN = $(BIN_DIR)/evm-pmpc-bootstrap

all: build

clean:
	@echo "[clean] - Removing binaries"
	rm -rf $(BIN_DIR)

build: build-node build-bootstrap

build-node:
	@echo "[build] - Building evm-pmpc-node"
	go build -o $(NODE_BIN) ./cmd/node/main.go
	@echo "[build] - Done building evm-pmpc-node"

build-bootstrap:
	@echo "[build] - Building evm-pmpc-bootstrap"
	go build -o $(BOOTSTRAP_BIN) ./cmd/bootstrapnode/main.go
	@echo "[build] - Done building evm-pmpc-bootstrap"

# Docker image building commands
docker-node:
	@echo "[docker] - Building worker node image"
	docker build -t evm-pmpc-node:latest -f $(BUILD_DIR)/Dockerfile.node .

docker-bootstrap:
	@echo "[docker] - Building bootstrap node image"
	docker build -t evm-pmpc-bootstrap:latest -f $(BUILD_DIR)/Dockerfile.bootstrapnode .

# Compose commands for running infrastructure
run-worker:
	@echo "[docker-compose] - Starting worker node"
	docker-compose -f $(DEPLOY_DIR)/docker-compose.worker.yaml up -d --build

run-bootstrap:
	@echo "[docker-compose] - Starting bootstrap node"
	docker-compose -f $(DEPLOY_DIR)/docker-compose.bootstrap.yaml up -d --build
