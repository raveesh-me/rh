//go:build !(darwin && arm64) && !(linux && amd64) && !(linux && arm64)

package main

import "embed"

// delta publishes no build for this platform; rh diff uses delta from PATH.
var deltaFS embed.FS

const deltaFSDir = ""
