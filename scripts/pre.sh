#!/usr/bin/env bash

! command -v golangci-lint >/dev/null 2>&1 && echo "[!] golangci-lint not found, aborting" && exit 1

echo "[*] running fmt & lint"
golangci-lint fmt
golangci-lint run --disable errcheck # ignores missing return value checks

echo "[*] running go mod tidy"
go mod tidy

# tests can be skipped with "-s" flag
if [[ "$1" != "-s" ]]; then
    echo "[*] running tests"
    go test -v ./...
    go test -race -v ./...
fi

