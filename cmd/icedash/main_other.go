//go:build !linux

// Command icedash only runs on ZimaOS. This stub lets `go vet ./...` and the
// other packages' tests run on a Windows or macOS PC.
package main

import "log"

func main() { log.Fatal("icedash runs on ZimaOS (Linux) only") }
