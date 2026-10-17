package main

import (
	"archive/tar"
	"bufio"
	"bytes"
	"compress/gzip"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net/http"
	"os"
	"path/filepath"
	"runtime"
	"strings"
	"time"
)

const (
	releaseRepo    = "raveesh-me/rh"
	httpTimeout    = 2 * time.Minute
	maxBinaryBytes = 100 << 20
)

// Overridable in tests.
var (
	releaseAPI      = "https://api.github.com/repos/" + releaseRepo + "/releases/latest"
	releaseDownload = "https://github.com/" + releaseRepo + "/releases/download"
)

func cmdUpdate(args []string) int {
	exe, err := currentExecutable()
	if err != nil {
		fmt.Fprintln(os.Stderr, "rh update:", err)
		return 1
	}
	client := &http.Client{Timeout: httpTimeout}
	tag, err := latestTag(client)
	if err != nil {
		fmt.Fprintln(os.Stderr, "rh update:", err)
		return 1
	}
	if strings.TrimPrefix(tag, "v") == strings.TrimPrefix(version, "v") {
		fmt.Printf("rh %s is already the latest\n", version)
		return 0
	}
	fmt.Printf("updating rh %s → %s (%s)\n", version, tag, exe)
	if err := installRelease(client, tag, exe); err != nil {
		fmt.Fprintln(os.Stderr, "rh update:", err)
		if errors.Is(err, os.ErrPermission) {
			fmt.Fprintln(os.Stderr, "rh update: try: sudo rh update")
		}
		return 1
	}
	fmt.Printf("rh %s installed\n", tag)
	return 0
}

func currentExecutable() (string, error) {
	exe, err := os.Executable()
	if err != nil {
		return "", fmt.Errorf("locate rh binary: %w", err)
	}
	return filepath.EvalSymlinks(exe)
}

func latestTag(client *http.Client) (string, error) {
	body, err := httpGet(client, releaseAPI)
	if err != nil {
		return "", fmt.Errorf("find latest release: %w", err)
	}
	var rel struct {
		TagName string `json:"tag_name"`
	}
	if err := json.Unmarshal(body, &rel); err != nil || rel.TagName == "" {
		return "", fmt.Errorf("find latest release: unexpected response")
	}
	return rel.TagName, nil
}

// installRelease downloads the archive for this platform, verifies its checksum
// and atomically replaces exe with the binary inside it.
func installRelease(client *http.Client, tag, exe string) error {
	asset := fmt.Sprintf("rh_%s_%s.tar.gz", runtime.GOOS, runtime.GOARCH)
	base := releaseDownload + "/" + tag + "/"
	archive, err := httpGet(client, base+asset)
	if err != nil {
		return fmt.Errorf("download %s: %w", asset, err)
	}
	sums, err := httpGet(client, base+"checksums.txt")
	if err != nil {
		return fmt.Errorf("download checksums: %w", err)
	}
	if err := verifyChecksum(sums, asset, archive); err != nil {
		return err
	}
	bin, err := extractBinary(archive, "rh")
	if err != nil {
		return err
	}
	return replaceFile(exe, bin)
}

func httpGet(client *http.Client, url string) ([]byte, error) {
	resp, err := client.Get(url)
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusOK {
		return nil, fmt.Errorf("GET %s: %s", url, resp.Status)
	}
	return io.ReadAll(io.LimitReader(resp.Body, maxBinaryBytes))
}

// verifyChecksum checks data against its line in a sha256sum-style file.
func verifyChecksum(sums []byte, name string, data []byte) error {
	sc := bufio.NewScanner(bytes.NewReader(sums))
	for sc.Scan() {
		fields := strings.Fields(sc.Text())
		if len(fields) != 2 || fields[1] != name {
			continue
		}
		got := sha256.Sum256(data)
		if hex.EncodeToString(got[:]) != fields[0] {
			return fmt.Errorf("checksum mismatch for %s", name)
		}
		return nil
	}
	return fmt.Errorf("no checksum for %s", name)
}

func extractBinary(archive []byte, name string) ([]byte, error) {
	gz, err := gzip.NewReader(bytes.NewReader(archive))
	if err != nil {
		return nil, fmt.Errorf("read archive: %w", err)
	}
	tr := tar.NewReader(gz)
	for {
		hdr, err := tr.Next()
		if err == io.EOF {
			return nil, fmt.Errorf("%s not found in archive", name)
		}
		if err != nil {
			return nil, fmt.Errorf("read archive: %w", err)
		}
		if hdr.Typeflag == tar.TypeReg && filepath.Base(hdr.Name) == name {
			return io.ReadAll(io.LimitReader(tr, maxBinaryBytes))
		}
	}
}

// replaceFile writes data next to path and renames it over path, so a failed
// update never leaves a half-written binary.
func replaceFile(path string, data []byte) error {
	tmp, err := os.CreateTemp(filepath.Dir(path), ".rh-update-*")
	if err != nil {
		return err
	}
	defer os.Remove(tmp.Name())
	if _, err := tmp.Write(data); err != nil {
		tmp.Close()
		return err
	}
	if err := tmp.Close(); err != nil {
		return err
	}
	if err := os.Chmod(tmp.Name(), 0o755); err != nil {
		return err
	}
	return os.Rename(tmp.Name(), path)
}
