mkdir -p bin

echo "[build] - Building evm-pmpc-node"
go build -o bin/evm-pmpc-node cmd/node/main.go
echo "[build] - Done building evm-pmpc-node"

echo "[build] - Building evm-pmpc-bootstrap"
go build -o bin/evm-pmpc-bootstrap cmd/bootstrapnode/main.go
echo "[build] - Done building evm-pmpc-bootstrap"

chmod +x bin/evm-pmpc-node
chmod +x bin/evm-pmpc-bootstrap
