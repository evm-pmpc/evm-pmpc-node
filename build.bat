@echo off

mkdir bin

go build -o .\bin\evm-pmpc-node.exe cmd\node\main.go
go build -o .\bin\evm-pmpc-bootstrap.exe cmd\bootstrapnode\main.go
