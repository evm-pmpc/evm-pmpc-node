@echo off

mkdir \bin

echo [build] - Building evm-pmpc-node
go build -o \bin\evm-pmpc-node.exe cmd\node\main.go
echo [build] - Done building evm-pmpc-node

echo [build] - Building evm-pmpc-bootstrap
go build -o \bin\evm-pmpc-bootstrap.exe cmd\bootstrapnode\main.go
echo [build] - Done building evm-pmpc-bootstrap
