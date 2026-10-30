// Command fetchdelta downloads the pinned delta release for every platform rh
// embeds it on, verifies its checksum, and writes third_party/delta/<plat>/delta.gz.
// Run via `go generate` from the repo root.
package main

import (
	"archive/tar"
	"bytes"
	"compress/gzip"
	"crypto/sha256"
	"encoding/hex"
	"fmt"
	"io"
	"net/http"
	"os"
	"path/filepath"
	"strings"
	"time"
)

const (
	version = "0.20.1"
	baseURL = "https://github.com/dandavison/delta/releases/download/" + version
	outDir  = "third_party/delta"
)

type target struct {
	plat, triple, sha256 string
}

// Checksums match the "digest" GitHub reports for each release asset.
var targets = []target{
	{"darwin_arm64", "aarch64-apple-darwin", "bc1839cea69288d4673a24faffbb825d408ca907e0f7d33b1678c9e6cdd5e58e"},
	{"linux_amd64", "x86_64-unknown-linux-musl", "69dcb4043f55c29118f408c6efd30115e0a4e0a6709253343b8c8fa814ab4a31"},
	{"linux_arm64", "aarch64-unknown-linux-gnu", "da7f4338f593572ff426ae153e0870e2fdc72729416eff551b40cdeb67940db8"},
}

func main() {
	client := &http.Client{Timeout: 2 * time.Minute}
	for _, t := range targets {
		if err := fetch(client, t); err != nil {
			fmt.Fprintf(os.Stderr, "fetchdelta %s: %v\n", t.plat, err)
			os.Exit(1)
		}
	}
}

func fetch(client *http.Client, t target) error {
	dir := filepath.Join(outDir, t.plat)
	out := filepath.Join(dir, "delta.gz")
	if v, err := os.ReadFile(filepath.Join(dir, "VERSION")); err == nil && strings.TrimSpace(string(v)) == version {
		if _, err := os.Stat(out); err == nil {
			return nil
		}
	}
	name := fmt.Sprintf("delta-%s-%s", version, t.triple)
	resp, err := client.Get(baseURL + "/" + name + ".tar.gz")
	if err != nil {
		return err
	}
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusOK {
		return fmt.Errorf("download: %s", resp.Status)
	}
	archive, err := io.ReadAll(resp.Body)
	if err != nil {
		return err
	}
	sum := sha256.Sum256(archive)
	if got := hex.EncodeToString(sum[:]); got != t.sha256 {
		return fmt.Errorf("checksum mismatch: got %s want %s", got, t.sha256)
	}
	bin, err := extract(archive, name+"/delta")
	if err != nil {
		return err
	}
	var gz bytes.Buffer
	zw, _ := gzip.NewWriterLevel(&gz, gzip.BestCompression)
	zw.Write(bin)
	if err := zw.Close(); err != nil {
		return err
	}
	if err := os.MkdirAll(dir, 0o755); err != nil {
		return err
	}
	if err := os.WriteFile(filepath.Join(dir, "VERSION"), []byte(version+"\n"), 0o644); err != nil {
		return err
	}
	fmt.Printf("fetchdelta: %s → %s (%d KB)\n", name, out, gz.Len()/1024)
	return os.WriteFile(out, gz.Bytes(), 0o644)
}

func extract(archive []byte, member string) ([]byte, error) {
	zr, err := gzip.NewReader(bytes.NewReader(archive))
	if err != nil {
		return nil, err
	}
	tr := tar.NewReader(zr)
	for {
		h, err := tr.Next()
		if err == io.EOF {
			return nil, fmt.Errorf("%s not in archive", member)
		}
		if err != nil {
			return nil, err
		}
		if h.Name == member {
			return io.ReadAll(tr)
		}
	}
}
