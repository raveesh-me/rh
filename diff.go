package main

import (
	"bytes"
	"context"
	"fmt"
	"io/fs"
	"os"
	"os/exec"
	"os/signal"
	"path/filepath"
	"sort"
	"strings"
	"syscall"
	"time"
)

const (
	diffInterval    = time.Second
	maxWatchedBytes = 1 << 20
	binarySniffLen  = 8000
)

var skippedDirs = map[string]bool{
	".git": true, "node_modules": true, "vendor": true, "dist": true, "build": true,
	".next": true, ".venv": true, "venv": true, "__pycache__": true, "target": true,
	".idea": true, ".vscode": true, ".cache": true, "coverage": true,
}

type fileState struct {
	modTime time.Time
	size    int64
	content []byte
}

type snapshot map[string]fileState

func cmdDiff(args []string) int {
	root, err := os.Getwd()
	if err != nil {
		fmt.Fprintln(os.Stderr, "rh diff:", err)
		return 1
	}
	tmp, err := os.MkdirTemp("", "rh-diff-")
	if err != nil {
		fmt.Fprintln(os.Stderr, "rh diff:", err)
		return 1
	}
	defer os.RemoveAll(tmp)

	ctx, stop := signal.NotifyContext(context.Background(), os.Interrupt, syscall.SIGTERM)
	defer stop()

	snap := scan(root, nil)
	fmt.Printf("\033[1mrh diff\033[0m  watching %d files in %s  \033[2m(ctrl-c to quit)\033[0m\n", len(snap), root)

	ticker := time.NewTicker(diffInterval)
	defer ticker.Stop()
	for {
		select {
		case <-ctx.Done():
			return 0
		case <-ticker.C:
		}
		next := scan(root, snap)
		for _, path := range changedPaths(snap, next) {
			printChange(tmp, path, snap[path], next[path])
		}
		snap = next
	}
}

// scan walks root and returns the text files in it. Files whose mtime and size
// match prev are reused without being read again.
func scan(root string, prev snapshot) snapshot {
	out := snapshot{}
	filepath.WalkDir(root, func(path string, d fs.DirEntry, err error) error {
		if err != nil {
			return nil
		}
		if d.IsDir() {
			if path != root && skippedDirs[d.Name()] {
				return filepath.SkipDir
			}
			return nil
		}
		if !d.Type().IsRegular() {
			return nil
		}
		info, err := d.Info()
		if err != nil || info.Size() > maxWatchedBytes {
			return nil
		}
		rel, _ := filepath.Rel(root, path)
		if old, ok := prev[rel]; ok && old.modTime.Equal(info.ModTime()) && old.size == info.Size() {
			out[rel] = old
			return nil
		}
		content, err := os.ReadFile(path)
		if err != nil || isBinary(content) {
			return nil
		}
		out[rel] = fileState{modTime: info.ModTime(), size: info.Size(), content: content}
		return nil
	})
	return out
}

func isBinary(b []byte) bool {
	return bytes.IndexByte(b[:min(len(b), binarySniffLen)], 0) >= 0
}

// changedPaths lists paths added, removed, or with different content, sorted.
func changedPaths(old, cur snapshot) []string {
	var paths []string
	for p, s := range cur {
		if o, ok := old[p]; !ok || !bytes.Equal(o.content, s.content) {
			paths = append(paths, p)
		}
	}
	for p := range old {
		if _, ok := cur[p]; !ok {
			paths = append(paths, p)
		}
	}
	sort.Strings(paths)
	return paths
}

func printChange(tmp, path string, old, cur fileState) {
	stamp := time.Now().Format("15:04:05")
	fmt.Printf("\n\033[1;33m── %s  %s\033[0m\n", stamp, path)
	out, err := unifiedDiff(tmp, path, old.content, cur.content)
	if err != nil {
		fmt.Fprintln(os.Stderr, "rh diff:", err)
		return
	}
	fmt.Print(colorize(out))
}

func unifiedDiff(tmp, path string, old, cur []byte) (string, error) {
	oldFile := filepath.Join(tmp, "old")
	curFile := filepath.Join(tmp, "new")
	if err := os.WriteFile(oldFile, old, 0o600); err != nil {
		return "", err
	}
	if err := os.WriteFile(curFile, cur, 0o600); err != nil {
		return "", err
	}
	cmd := exec.Command("diff", "-u", "--label", "a/"+path, "--label", "b/"+path, oldFile, curFile)
	out, err := cmd.Output()
	// diff exits 1 when files differ; only >1 is a real failure.
	if exitErr, ok := err.(*exec.ExitError); ok && exitErr.ExitCode() == 1 {
		err = nil
	}
	return string(out), err
}

func colorize(diff string) string {
	var b strings.Builder
	for line := range strings.Lines(diff) {
		fmt.Fprintf(&b, "%s%s\033[0m\n", lineColor(line), strings.TrimSuffix(line, "\n"))
	}
	return b.String()
}

func lineColor(line string) string {
	switch {
	case strings.HasPrefix(line, "+++"), strings.HasPrefix(line, "---"):
		return "\033[1m"
	case strings.HasPrefix(line, "@@"):
		return "\033[36m"
	case strings.HasPrefix(line, "+"):
		return "\033[32m"
	case strings.HasPrefix(line, "-"):
		return "\033[31m"
	}
	return ""
}
