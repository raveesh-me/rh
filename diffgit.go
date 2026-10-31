package main

import (
	"bytes"
	"fmt"
	"io"
	"os"
	"os/exec"
	"path/filepath"
	"sort"
	"strings"
	"sync"
)

// rh diff: every git worktree below here vs its merge-base with
// origin/<default>, i.e. what its PR would contain, plus uncommitted and
// untracked files. Paths are prefixed with the worktree's location so the
// explorer's tree groups them as workdir/<task>/<repo>/...

const (
	// gitRootEnv, when set, makes `rh diff` print the worktree diffs below
	// that directory instead of opening the explorer.
	gitRootEnv = "RH_DIFF_ROOT"
	// worktreeSearchDepth reaches workdir/<task>/<repo> from a workspace root.
	worktreeSearchDepth = 3
	// maxUntrackedFiles bounds per-tick work when a worktree has a large
	// un-ignored output dir.
	maxUntrackedFiles = 500
)

type worktree struct {
	dir    string // absolute path
	prefix string // slash path relative to the diff root, "" or ending in "/"
}

func runGitExplorer() error {
	root, err := os.Getwd()
	if err != nil {
		return err
	}
	if len(findWorktrees(root)) == 0 && !isDir(filepath.Join(root, workdirName)) {
		return fmt.Errorf("no git worktrees below %s (rh diff --live watches plain files)", root)
	}
	e, err := newExplorer()
	if err != nil {
		return err
	}
	defer e.close()
	os.Setenv(gitRootEnv, root)
	return e.run("rh diff")
}

// findWorktrees returns the repo root contains if it is inside one; otherwise
// the linked worktrees (.git is a file) up to worktreeSearchDepth below it.
// Main checkouts (.git is a dir), i.e. the reference repos, are skipped.
// It runs on every tick, so worktrees added or removed meanwhile show up.
func findWorktrees(root string) []worktree {
	if top, err := gitOutput(root, "rev-parse", "--show-toplevel"); err == nil {
		return []worktree{{dir: strings.TrimSpace(string(top))}}
	}
	var out []worktree
	var walk func(dir string, depth int)
	walk = func(dir string, depth int) {
		entries, err := os.ReadDir(dir)
		if err != nil {
			return
		}
		for _, e := range entries {
			if !e.IsDir() || skippedDirs[e.Name()] {
				continue
			}
			p := filepath.Join(dir, e.Name())
			if info, err := os.Stat(filepath.Join(p, ".git")); err == nil {
				if !info.IsDir() {
					rel, _ := filepath.Rel(root, p)
					out = append(out, worktree{dir: p, prefix: filepath.ToSlash(rel) + "/"})
				}
				continue
			}
			if depth < worktreeSearchDepth {
				walk(p, depth+1)
			}
		}
	}
	walk(root, 1)
	sort.Slice(out, func(i, j int) bool { return out[i].prefix < out[j].prefix })
	return out
}

// printWorktreeDiffs writes every worktree's diff below root, in path order.
// A worktree that fails is reported on stderr and left out.
func printWorktreeDiffs(root string, w io.Writer) int {
	wts := findWorktrees(root)
	diffs := make([][]byte, len(wts))
	var wg sync.WaitGroup
	for i, wt := range wts {
		wg.Go(func() {
			d, err := worktreeDiff(wt)
			if err != nil {
				fmt.Fprintf(os.Stderr, "rh diff: %s: %v\n", wt.dir, err)
			}
			diffs[i] = d
		})
	}
	wg.Wait()
	for _, d := range diffs {
		w.Write(d)
	}
	return 0
}

// worktreeDiff is the working tree vs the merge-base, plus untracked files.
func worktreeDiff(wt worktree) ([]byte, error) {
	out, err := gitOutput(wt.dir, "diff", "--no-color", "--no-ext-diff",
		"--src-prefix=a/"+wt.prefix, "--dst-prefix=b/"+wt.prefix, mergeBase(wt.dir), "--")
	if err != nil {
		return nil, err
	}
	untracked, err := untrackedDiffs(wt)
	return append(out, untracked...), err
}

// mergeBase is where HEAD left the default branch, preferring the remote
// copy; HEAD when there is none (then only uncommitted work shows).
func mergeBase(dir string) string {
	branch := defaultBranch(dir)
	for _, ref := range []string{"origin/" + branch, branch} {
		if out, err := gitOutput(dir, "merge-base", "HEAD", ref); err == nil {
			return strings.TrimSpace(string(out))
		}
	}
	return "HEAD"
}

// untrackedDiffs renders un-ignored untracked text files as new-file diffs,
// built in Go to avoid a process per file every tick.
func untrackedDiffs(wt worktree) ([]byte, error) {
	out, err := gitOutput(wt.dir, "ls-files", "--others", "--exclude-standard", "-z")
	if err != nil {
		return nil, err
	}
	var b bytes.Buffer
	for i, rel := range strings.Split(strings.TrimRight(string(out), "\x00"), "\x00") {
		if rel == "" || i >= maxUntrackedFiles {
			break
		}
		path := filepath.Join(wt.dir, rel)
		info, err := os.Lstat(path)
		if err != nil || !info.Mode().IsRegular() || info.Size() > maxWatchedBytes {
			continue
		}
		content, err := os.ReadFile(path)
		if err != nil || isBinary(content) {
			continue
		}
		b.WriteString(newFileDiff(wt.prefix+rel, content))
	}
	return b.Bytes(), nil
}

// newFileDiff is what `git diff` prints for a file added with this content.
func newFileDiff(path string, content []byte) string {
	var b strings.Builder
	fmt.Fprintf(&b, "diff --git a/%s b/%s\nnew file mode 100644\n", path, path)
	if len(content) == 0 {
		return b.String()
	}
	text := string(content)
	noEOL := !strings.HasSuffix(text, "\n")
	lines := strings.Split(strings.TrimSuffix(text, "\n"), "\n")
	fmt.Fprintf(&b, "--- /dev/null\n+++ b/%s\n@@ -0,0 +1,%d @@\n", path, len(lines))
	for _, l := range lines {
		b.WriteByte('+')
		b.WriteString(l)
		b.WriteByte('\n')
	}
	if noEOL {
		b.WriteString("\\ No newline at end of file\n")
	}
	return b.String()
}

// gitOutput runs git without taking optional locks, so polling every second
// never makes the user's own git commands fail on index.lock.
func gitOutput(dir string, args ...string) ([]byte, error) {
	cmd := exec.Command("git", args...)
	cmd.Dir = dir
	cmd.Env = append(os.Environ(), "GIT_OPTIONAL_LOCKS=0", "GIT_TERMINAL_PROMPT=0")
	var stderr bytes.Buffer
	cmd.Stderr = &stderr
	out, err := cmd.Output()
	if err != nil {
		if msg := firstLine(strings.TrimSpace(stderr.String())); msg != "" {
			return nil, fmt.Errorf("git %s: %s", args[0], msg)
		}
		return nil, fmt.Errorf("git %s: %w", args[0], err)
	}
	return out, nil
}

func isDir(path string) bool {
	info, err := os.Stat(path)
	return err == nil && info.IsDir()
}
