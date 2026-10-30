package main

import (
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"
)

func TestCleanWorkdir(t *testing.T) {
	root := t.TempDir()
	repo := filepath.Join(root, "api")
	run := func(dir string, args ...string) {
		t.Helper()
		cmd := exec.Command("git", args...)
		cmd.Dir = dir
		if out, err := cmd.CombinedOutput(); err != nil {
			t.Fatalf("git %v: %v\n%s", args, err, out)
		}
	}
	os.MkdirAll(repo, 0o755)
	run(repo, "init", "-q", "-b", "main")
	run(repo, "-c", "user.name=t", "-c", "user.email=t@t", "commit", "-q", "--allow-empty", "-m", "init")

	workdir := filepath.Join(root, "workdir")
	cleanWt := filepath.Join(workdir, "done", "api")
	dirtyWt := filepath.Join(workdir, "wip", "api")
	run(repo, "worktree", "add", "-q", "-b", "done", cleanWt)
	run(repo, "worktree", "add", "-q", "-b", "wip", dirtyWt)
	os.WriteFile(filepath.Join(dirtyWt, "scratch.txt"), []byte("unsaved\n"), 0o644)

	kept, err := cleanWorkdir(workdir, false)
	if err != nil {
		t.Fatal(err)
	}
	if kept != 1 {
		t.Errorf("kept = %d, want 1 (the dirty worktree)", kept)
	}
	if _, err := os.Stat(filepath.Join(workdir, "done")); !os.IsNotExist(err) {
		t.Errorf("clean task dir not removed: %v", err)
	}
	if _, err := os.Stat(filepath.Join(dirtyWt, "scratch.txt")); err != nil {
		t.Errorf("dirty worktree was touched without -f: %v", err)
	}

	if kept, err := cleanWorkdir(workdir, true); err != nil || kept != 0 {
		t.Fatalf("force clean: kept=%d err=%v", kept, err)
	}
	if entries, _ := os.ReadDir(workdir); len(entries) != 0 {
		t.Errorf("workdir not empty: %v", entries)
	}
	branches, _ := git(repo, "branch", "--format=%(refname:short)")
	for _, b := range []string{"done", "wip"} {
		if !strings.Contains(branches, b) {
			t.Errorf("branch %s deleted; clean must keep branches (have %q)", b, branches)
		}
	}
	if list, _ := git(repo, "worktree", "list"); strings.Count(list, "\n") != 0 {
		t.Errorf("stale worktrees left:\n%s", list)
	}
}
