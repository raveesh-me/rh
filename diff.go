package main

import (
	"bytes"
	"context"
	"encoding/gob"
	"errors"
	"fmt"
	"io"
	"io/fs"
	"os"
	"os/exec"
	"os/signal"
	"path/filepath"
	"sort"
	"strings"
	"syscall"
	"time"

	tea "charm.land/bubbletea/v2"
	"charm.land/log/v2"
	"github.com/dlvhdr/diffnav/pkg/config"
	"github.com/dlvhdr/diffnav/pkg/ui"
	zone "github.com/lrstanley/bubblezone/v2"
)

const (
	diffInterval    = time.Second
	maxWatchedBytes = 1 << 20
	binarySniffLen  = 8000
	// baselineEnv, when set, makes `rh diff` print the diff against that
	// baseline instead of opening the explorer. The explorer's watch loop uses it.
	baselineEnv = "RH_DIFF_BASELINE"
)

var skippedDirs = map[string]bool{
	".git": true, "node_modules": true, "vendor": true, "dist": true, "build": true,
	".next": true, ".venv": true, "venv": true, "__pycache__": true, "target": true,
	".idea": true, ".vscode": true, ".cache": true, "coverage": true,
}

type fileState struct {
	ModTime time.Time
	Size    int64
	Content []byte
}

type snapshot map[string]fileState

// baseline is what rh diff compares against: every text file as it was at start.
type baseline struct {
	Root  string
	Files snapshot
}

// cmdDiff snapshots the tree, then opens diffnav (file tree + delta-rendered
// diff) on a watch loop that re-runs `rh diff` with baselineEnv set every tick.
func cmdDiff(args []string) int {
	if base := os.Getenv(baselineEnv); base != "" {
		return printDiffAgainst(base, os.Stdout)
	}
	if len(args) > 0 {
		fmt.Fprintln(os.Stderr, "usage: rh diff")
		return 2
	}
	if err := runExplorer(); err != nil {
		fmt.Fprintln(os.Stderr, "rh diff:", err)
		return 1
	}
	return 0
}

func runExplorer() error {
	root, err := os.Getwd()
	if err != nil {
		return err
	}
	exe, err := currentExecutable()
	if err != nil {
		return err
	}
	deltaDir, err := ensureDelta()
	if err != nil {
		return err
	}
	tmp, err := os.MkdirTemp("", "rh-diff-")
	if err != nil {
		return err
	}
	defer os.RemoveAll(tmp)

	fmt.Fprintf(os.Stderr, "rh diff: snapshotting %s ...\n", root)
	base := filepath.Join(tmp, "baseline.gob")
	if err := writeBaseline(base, baseline{Root: root, Files: scan(root, nil)}); err != nil {
		return err
	}
	// The watch command is a shell string diffnav also shows in its footer;
	// a private rh symlink on PATH keeps it a plain `rh diff`.
	if err := os.Symlink(exe, filepath.Join(tmp, "rh")); err != nil {
		return err
	}
	path := []string{tmp}
	if deltaDir != "" {
		path = append(path, deltaDir)
	}
	os.Setenv("PATH", strings.Join(append(path, os.Getenv("PATH")), string(os.PathListSeparator)))
	os.Setenv(baselineEnv, base)

	cfg := config.Load()
	cfg.Watch = config.WatchConfig{
		Enabled:  true,
		Cmd:      "rh diff",
		Interval: diffInterval,
	}
	// diffnav logs every delta invocation; keep that off the TUI.
	log.SetOutput(io.Discard)
	log.SetLevel(log.FatalLevel)
	zone.NewGlobal()

	tty, _, err := tea.OpenTTY()
	if err != nil {
		return fmt.Errorf("needs a terminal: %w", err)
	}
	defer tty.Close()
	// Closing the terminal tab must still run the deferred cleanup of tmp.
	ctx, stop := signal.NotifyContext(context.Background(), syscall.SIGHUP, syscall.SIGTERM)
	defer stop()
	_, err = tea.NewProgram(ui.New(ui.ModelOpts{}, cfg), tea.WithInput(tty), tea.WithContext(ctx)).Run()
	if errors.Is(err, tea.ErrProgramKilled) && ctx.Err() != nil {
		return nil
	}
	return err
}

func writeBaseline(path string, b baseline) error {
	var buf bytes.Buffer
	if err := gob.NewEncoder(&buf).Encode(b); err != nil {
		return fmt.Errorf("encode baseline: %w", err)
	}
	return os.WriteFile(path, buf.Bytes(), 0o600)
}

func readBaseline(path string) (baseline, error) {
	var b baseline
	f, err := os.Open(path)
	if err != nil {
		return b, err
	}
	defer f.Close()
	if err := gob.NewDecoder(f).Decode(&b); err != nil {
		return b, fmt.Errorf("decode baseline: %w", err)
	}
	return b, nil
}

// printDiffAgainst writes a git-style diff of every file changed since the
// baseline was taken.
func printDiffAgainst(basePath string, w io.Writer) int {
	base, err := readBaseline(basePath)
	if err != nil {
		fmt.Fprintln(os.Stderr, "rh diff:", err)
		return 1
	}
	cur := scan(base.Root, base.Files)
	for _, path := range changedPaths(base.Files, cur) {
		old, hadOld := base.Files[path]
		now, hasNow := cur[path]
		out, err := gitStyleDiff(path, old.Content, now.Content, hadOld, hasNow)
		if err != nil {
			fmt.Fprintln(os.Stderr, "rh diff:", err)
			return 1
		}
		io.WriteString(w, out)
	}
	return 0
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
		if old, ok := prev[rel]; ok && old.ModTime.Equal(info.ModTime()) && old.Size == info.Size() {
			out[rel] = old
			return nil
		}
		content, err := os.ReadFile(path)
		if err != nil || isBinary(content) {
			return nil
		}
		out[rel] = fileState{ModTime: info.ModTime(), Size: info.Size(), Content: content}
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
		if o, ok := old[p]; !ok || !bytes.Equal(o.Content, s.Content) {
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

// gitStyleDiff renders one file's change with the `diff --git` headers that
// diffnav's parser expects. Added files diff against /dev/null and vice versa.
func gitStyleDiff(path string, old, cur []byte, hadOld, hasNow bool) (string, error) {
	var head strings.Builder
	fmt.Fprintf(&head, "diff --git a/%s b/%s\n", path, path)
	oldLabel, newLabel := "a/"+path, "b/"+path
	switch {
	case !hadOld:
		head.WriteString("new file mode 100644\n")
		oldLabel = "/dev/null"
	case !hasNow:
		head.WriteString("deleted file mode 100644\n")
		newLabel = "/dev/null"
	}
	body, err := unifiedDiff(oldLabel, newLabel, old, cur)
	if err != nil {
		return "", err
	}
	return head.String() + body, nil
}

// unifiedDiff runs diff -u with old on stdin and cur on fd 3, so nothing is
// written to disk.
func unifiedDiff(oldLabel, newLabel string, old, cur []byte) (string, error) {
	r, w, err := os.Pipe()
	if err != nil {
		return "", err
	}
	defer r.Close()
	cmd := exec.Command("diff", "-u", "--label", oldLabel, "--label", newLabel, "-", "/dev/fd/3")
	cmd.Stdin = bytes.NewReader(old)
	cmd.ExtraFiles = []*os.File{r}
	go func() {
		w.Write(cur)
		w.Close()
	}()
	out, err := cmd.Output()
	// diff exits 1 when files differ; only >1 is a real failure.
	if exitErr, ok := err.(*exec.ExitError); ok && exitErr.ExitCode() == 1 {
		err = nil
	}
	return string(out), err
}
