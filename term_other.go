//go:build !unix

package main

import "os"

func termWidth(*os.File) int { return 0 }

func resizeSignals() []os.Signal { return nil }
