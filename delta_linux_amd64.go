package main

import "embed"

// Populated by go generate; holds VERSION and, after generate, delta.gz.
//
//go:embed third_party/delta/linux_amd64
var deltaFS embed.FS

const deltaFSDir = "third_party/delta/linux_amd64"
