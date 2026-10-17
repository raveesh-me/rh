package main

import (
	"archive/tar"
	"bytes"
	"compress/gzip"
	"crypto/sha256"
	"encoding/hex"
	"fmt"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"runtime"
	"testing"
)

func makeArchive(t *testing.T, name string, body []byte) []byte {
	t.Helper()
	var buf bytes.Buffer
	gz := gzip.NewWriter(&buf)
	tw := tar.NewWriter(gz)
	tw.WriteHeader(&tar.Header{Name: "README.md", Mode: 0o644, Size: 2, Typeflag: tar.TypeReg})
	tw.Write([]byte("hi"))
	tw.WriteHeader(&tar.Header{Name: name, Mode: 0o755, Size: int64(len(body)), Typeflag: tar.TypeReg})
	tw.Write(body)
	tw.Close()
	gz.Close()
	return buf.Bytes()
}

func sha(b []byte) string {
	s := sha256.Sum256(b)
	return hex.EncodeToString(s[:])
}

func TestVerifyChecksum(t *testing.T) {
	data := []byte("payload")
	sums := []byte(sha([]byte("other")) + "  x.tar.gz\n" + sha(data) + "  rh_a_b.tar.gz\n")
	if err := verifyChecksum(sums, "rh_a_b.tar.gz", data); err != nil {
		t.Errorf("valid checksum: %v", err)
	}
	if err := verifyChecksum(sums, "rh_a_b.tar.gz", []byte("tampered")); err == nil {
		t.Error("tampered data accepted")
	}
	if err := verifyChecksum(sums, "missing.tar.gz", data); err == nil {
		t.Error("missing entry accepted")
	}
}

func TestExtractBinary(t *testing.T) {
	got, err := extractBinary(makeArchive(t, "rh", []byte("BIN")), "rh")
	if err != nil || string(got) != "BIN" {
		t.Fatalf("extract = %q, %v", got, err)
	}
	if _, err := extractBinary(makeArchive(t, "other", nil), "rh"); err == nil {
		t.Error("expected error when binary missing")
	}
}

func TestInstallReleaseReplacesBinary(t *testing.T) {
	asset := fmt.Sprintf("rh_%s_%s.tar.gz", runtime.GOOS, runtime.GOARCH)
	archive := makeArchive(t, "rh", []byte("NEW"))
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		switch r.URL.Path {
		case "/latest":
			fmt.Fprint(w, `{"tag_name":"v9.9.9"}`)
		case "/dl/v9.9.9/" + asset:
			w.Write(archive)
		case "/dl/v9.9.9/checksums.txt":
			fmt.Fprintf(w, "%s  %s\n", sha(archive), asset)
		default:
			http.NotFound(w, r)
		}
	}))
	defer srv.Close()
	oldAPI, oldDL := releaseAPI, releaseDownload
	releaseAPI, releaseDownload = srv.URL+"/latest", srv.URL+"/dl"
	t.Cleanup(func() { releaseAPI, releaseDownload = oldAPI, oldDL })

	tag, err := latestTag(srv.Client())
	if err != nil || tag != "v9.9.9" {
		t.Fatalf("latestTag = %q, %v", tag, err)
	}
	exe := filepath.Join(t.TempDir(), "rh")
	os.WriteFile(exe, []byte("OLD"), 0o755)
	if err := installRelease(srv.Client(), tag, exe); err != nil {
		t.Fatal(err)
	}
	got, _ := os.ReadFile(exe)
	info, _ := os.Stat(exe)
	if string(got) != "NEW" || info.Mode().Perm() != 0o755 {
		t.Errorf("binary = %q mode %v", got, info.Mode())
	}
	if err := installRelease(srv.Client(), "v0.0.0", exe); err == nil {
		t.Error("expected error for missing release")
	}
}
