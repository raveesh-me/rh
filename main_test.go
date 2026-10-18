package main

import (
	"bytes"
	"os"
	"os/exec"
	"path/filepath"
	"reflect"
	"sort"
	"strings"
	"testing"
	"time"
)

func TestResolve(t *testing.T) {
	tests := []struct {
		name    string
		args    []string
		kind    routeKind
		builtin string
		rest    []string
	}{
		{"no args", nil, routeHelp, "", nil},
		{"help", []string{"help"}, routeHelp, "", nil},
		{"version", []string{"--version"}, routeVersion, "", nil},
		{"builtin", []string{"pull", "x"}, routeBuiltin, "pull", []string{"x"}},
		{"unknown goes to make", []string{"sync", "V=1"}, routeMake, "", []string{"sync", "V=1"}},
		{"-m forces make", []string{"-m", "update"}, routeMake, "", []string{"update"}},
		{"-m alone runs default target", []string{"-m"}, routeMake, "", []string{}},
		{"-c joins prompt", []string{"-c", "fix", "the", "tests"}, routeClaude, "", []string{"-p", "fix the tests"}},
		{"-c needs prompt", []string{"-c"}, routeError, "", nil},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			r := resolve(tt.args)
			if r.kind != tt.kind {
				t.Fatalf("kind = %v, want %v", r.kind, tt.kind)
			}
			if tt.builtin != "" && r.builtin.name != tt.builtin {
				t.Errorf("builtin = %s, want %s", r.builtin.name, tt.builtin)
			}
			if tt.rest != nil && !reflect.DeepEqual(r.args, tt.rest) {
				t.Errorf("args = %q, want %q", r.args, tt.rest)
			}
		})
	}
}

func TestMakefileDefines(t *testing.T) {
	tests := []struct {
		src  string
		want bool
	}{
		{"update:\n\tgit pull", true},
		{"update: deps\n", true},
		{"sync update: ## both\n", true},
		{"update::\n", true},
		{"update := 1\n", false},
		{"update ::= 1\n", false},
		{".PHONY: update\n", false},
		{"updates:\n", false},
		{"pre-update:\n", false},
		{"# update:\n", false},
	}
	for _, tt := range tests {
		if got := makefileDefines(tt.src, "update"); got != tt.want {
			t.Errorf("makefileDefines(%q) = %v, want %v", tt.src, got, tt.want)
		}
	}
}

func TestInitIsIdempotent(t *testing.T) {
	dir := t.TempDir()
	t.Chdir(dir)
	if err := os.WriteFile("CLAUDE.md", []byte("mine"), 0o644); err != nil {
		t.Fatal(err)
	}
	for range 2 {
		if code := cmdInit(nil); code != 0 {
			t.Fatalf("cmdInit = %d", code)
		}
	}
	if got, _ := os.ReadFile("CLAUDE.md"); string(got) != "mine" {
		t.Errorf("existing CLAUDE.md overwritten: %q", got)
	}
	agents, err := os.ReadFile("AGENTS.md")
	if err != nil || !strings.Contains(string(agents), filepath.Base(dir)) {
		t.Errorf("AGENTS.md missing project name: %v %q", err, agents)
	}
	makefile, _ := os.ReadFile("Makefile")
	if want, _ := templates.ReadFile("templates/makefile"); !bytes.Equal(makefile, want) {
		t.Error("Makefile differs from template")
	}
	if !makefileDefines(string(makefile), "new") {
		t.Error("Makefile missing worktree targets")
	}
	if info, err := os.Stat("workdir"); err != nil || !info.IsDir() {
		t.Errorf("workdir not created: %v", err)
	}
}

func TestScanAndChangedPaths(t *testing.T) {
	root := t.TempDir()
	write := func(rel, body string) {
		t.Helper()
		p := filepath.Join(root, rel)
		os.MkdirAll(filepath.Dir(p), 0o755)
		if err := os.WriteFile(p, []byte(body), 0o644); err != nil {
			t.Fatal(err)
		}
	}
	write("a.txt", "one\n")
	write("gone.txt", "bye\n")
	write("node_modules/x.js", "skip\n")
	write(".git/HEAD", "skip\n")
	write("bin.dat", "a\x00b")

	before := scan(root, nil)
	if got := sortedKeys(before); !reflect.DeepEqual(got, []string{"a.txt", "gone.txt"}) {
		t.Fatalf("scan = %v", got)
	}

	write("a.txt", "two\n")
	os.Chtimes(filepath.Join(root, "a.txt"), time.Now().Add(time.Minute), time.Now().Add(time.Minute))
	os.Remove(filepath.Join(root, "gone.txt"))
	write("new.txt", "hi\n")

	after := scan(root, before)
	want := []string{"a.txt", "gone.txt", "new.txt"}
	if got := changedPaths(before, after); !reflect.DeepEqual(got, want) {
		t.Errorf("changedPaths = %v, want %v", got, want)
	}
}

func TestUnifiedDiff(t *testing.T) {
	out, err := unifiedDiff(t.TempDir(), "f.txt", []byte("a\nb\n"), []byte("a\nc\n"))
	if err != nil {
		t.Fatal(err)
	}
	for _, want := range []string{"--- a/f.txt", "+++ b/f.txt", "-b", "+c"} {
		if !strings.Contains(out, want) {
			t.Errorf("diff missing %q:\n%s", want, out)
		}
	}
}

func TestFindReposSkipsWorktrees(t *testing.T) {
	root := t.TempDir()
	os.MkdirAll(filepath.Join(root, "repo", ".git"), 0o755)
	os.MkdirAll(filepath.Join(root, "plain"), 0o755)
	os.MkdirAll(filepath.Join(root, "wt"), 0o755)
	os.WriteFile(filepath.Join(root, "wt", ".git"), []byte("gitdir: /x"), 0o644)

	repos, err := findRepos(root)
	if err != nil {
		t.Fatal(err)
	}
	if len(repos) != 1 || filepath.Base(repos[0]) != "repo" {
		t.Errorf("findRepos = %v", repos)
	}
}

func TestUpdateRepoPullsDefaultBranch(t *testing.T) {
	if _, err := exec.LookPath("git"); err != nil {
		t.Skip("git not installed")
	}
	root := t.TempDir()
	origin := filepath.Join(root, "origin")
	clone := filepath.Join(root, "clone")
	mustGit(t, root, "init", "-q", "-b", "main", origin)
	mustGit(t, origin, "commit", "-q", "--allow-empty", "-m", "one")
	mustGit(t, root, "clone", "-q", origin, clone)
	mustGit(t, clone, "checkout", "-q", "-b", "feature")
	mustGit(t, origin, "commit", "-q", "--allow-empty", "-m", "two")

	msg, err := updateRepo(clone, func(string) {})
	if err != nil {
		t.Fatal(err)
	}
	if !strings.HasPrefix(msg, "main ") || strings.Contains(msg, "up to date") {
		t.Errorf("msg = %q, want main fast-forwarded", msg)
	}
	if b := currentBranch(clone); b != "main" {
		t.Errorf("branch = %q, want main", b)
	}

	os.WriteFile(filepath.Join(clone, "dirty"), []byte("x"), 0o644)
	if _, err := updateRepo(clone, func(string) {}); err == nil || !strings.Contains(err.Error(), "dirty") {
		t.Errorf("dirty repo err = %v", err)
	}
}

func mustGit(t *testing.T, dir string, args ...string) {
	t.Helper()
	cmd := exec.Command("git", append([]string{"-c", "user.name=t", "-c", "user.email=t@t"}, args...)...)
	cmd.Dir = dir
	if out, err := cmd.CombinedOutput(); err != nil {
		t.Fatalf("git %v: %v\n%s", args, err, out)
	}
}

func sortedKeys(s snapshot) []string {
	var keys []string
	for k := range s {
		keys = append(keys, k)
	}
	sort.Strings(keys)
	return keys
}

func TestRepoDirName(t *testing.T) {
	tests := map[string]string{
		"git@github.com:GoHighLevel/leadgen-marketplace-backend.git": "leadgen-marketplace-backend",
		"https://github.com/raveesh-me/rh":                           "rh",
		"https://github.com/raveesh-me/rh.git/":                      "rh",
		"/tmp/src.git":                                               "src",
	}
	for in, want := range tests {
		if got := repoDirName(in); got != want {
			t.Errorf("repoDirName(%q) = %q, want %q", in, got, want)
		}
	}
}

func TestUniqueByDir(t *testing.T) {
	got, err := uniqueByDir([]string{"git@h:a/x.git", "git@h:a/y.git", "git@h:a/x.git"})
	if err != nil || !reflect.DeepEqual(got, []string{"git@h:a/x.git", "git@h:a/y.git"}) {
		t.Errorf("uniqueByDir = %v, %v", got, err)
	}
	if _, err := uniqueByDir([]string{"git@h:a/x.git", "git@h:b/x.git"}); err == nil {
		t.Error("expected conflict for two repos named x")
	}
}

func TestAddClonesReposInParallel(t *testing.T) {
	root := t.TempDir()
	var origins []string
	for _, name := range []string{"one", "two", "three"} {
		origin := filepath.Join(root, "src", name+".git")
		mustGit(t, root, "init", "-q", "--bare", origin)
		origins = append(origins, origin)
	}
	dest := filepath.Join(root, "ws")
	os.MkdirAll(filepath.Join(dest, "three"), 0o755) // pre-existing: skipped, not an error
	t.Chdir(dest)
	if code := cmdAdd(append(origins, origins[0])); code != 0 {
		t.Fatalf("cmdAdd = %d", code)
	}
	for _, name := range []string{"one", "two"} {
		if !isRepoRoot(filepath.Join(dest, name)) {
			t.Errorf("%s not cloned", name)
		}
	}
	if isRepoRoot(filepath.Join(dest, "three")) {
		t.Error("existing folder three was cloned into")
	}
	if code := cmdAdd([]string{filepath.Join(root, "missing.git")}); code != 1 {
		t.Errorf("failed clone exit = %d, want 1", code)
	}
}
