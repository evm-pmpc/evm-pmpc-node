mkdir -p bin

go build -o ./bin/evm-pmpc-node cmd/node/main.go
go build -o ./bin/evm-pmpc-bootstrap cmd/bootstrapnode/main.go

chmod +x ./bin/evm-pmpc-node
chmod +x ./bin/evm-pmpc-bootstrap
