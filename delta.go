package main

//go:generate go run ./tools/fetchdelta

import (
	"bytes"
	"compress/gzip"
	"errors"
	"fmt"
	"io"
	"io/fs"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
)

// ensureDelta makes a delta binary available for diffnav and returns the
// directory to prepend to PATH, or "" when delta is already on PATH.
// The embedded copy is unpacked once per version into the user cache dir.
func ensureDelta() (string, error) {
	gz, version, err := embeddedDelta()
	if err != nil {
		if _, lookErr := exec.LookPath("delta"); lookErr == nil {
			return "", nil
		}
		return "", fmt.Errorf("%w; install delta (brew install git-delta) and retry", err)
	}
	cache, err := os.UserCacheDir()
	if err != nil {
		return "", err
	}
	dir := filepath.Join(cache, "rh", "delta-"+version)
	if _, err := os.Stat(filepath.Join(dir, "delta")); err == nil {
		return dir, nil
	}
	if err := unpackDelta(gz, dir); err != nil {
		return "", fmt.Errorf("unpack delta: %w", err)
	}
	return dir, nil
}

func embeddedDelta() (gz []byte, version string, err error) {
	if deltaFSDir == "" {
		return nil, "", errors.New("no embedded delta for this platform")
	}
	v, err := fs.ReadFile(deltaFS, deltaFSDir+"/VERSION")
	if err != nil {
		return nil, "", err
	}
	gz, err = fs.ReadFile(deltaFS, deltaFSDir+"/delta.gz")
	if err != nil {
		return nil, "", errors.New("this rh build has no embedded delta (run go generate before go build)")
	}
	return gz, strings.TrimSpace(string(v)), nil
}

// unpackDelta writes the binary via a temp file + rename so a concurrent
// rh diff never sees a half-written executable.
func unpackDelta(gz []byte, dir string) error {
	zr, err := gzip.NewReader(bytes.NewReader(gz))
	if err != nil {
		return err
	}
	if err := os.MkdirAll(dir, 0o755); err != nil {
		return err
	}
	tmp, err := os.CreateTemp(dir, ".delta-*")
	if err != nil {
		return err
	}
	defer os.Remove(tmp.Name())
	if _, err := io.Copy(tmp, zr); err != nil {
		tmp.Close()
		return err
	}
	if err := tmp.Chmod(0o755); err != nil {
		tmp.Close()
		return err
	}
	if err := tmp.Close(); err != nil {
		return err
	}
	return os.Rename(tmp.Name(), filepath.Join(dir, "delta"))
}
