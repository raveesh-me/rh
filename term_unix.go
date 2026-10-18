//go:build unix

package main

import (
	"os"
	"syscall"
	"unsafe"
)

// termWidth returns the terminal's column count, or 0 if unknown.
func termWidth(f *os.File) int {
	var ws struct{ rows, cols, x, y uint16 }
	_, _, errno := syscall.Syscall(syscall.SYS_IOCTL, f.Fd(), uintptr(syscall.TIOCGWINSZ), uintptr(unsafe.Pointer(&ws)))
	if errno != 0 {
		return 0
	}
	return int(ws.cols)
}
