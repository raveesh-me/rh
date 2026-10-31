package main

import (
	"os"
	"os/exec"
	"path/filepath"
	"reflect"
	"sort"
	"strings"
	"testing"

	"github.com/bluekeyes/go-gitdiff/gitdiff"
)

// newWorkspace builds root/api (a reference clone of a bare origin) with
// worktrees workdir/feat/api and workdir/idle/api branched from origin/main.
func newWorkspace(t *testing.T) string {
	t.Helper()
	root := t.TempDir()
	origin := filepath.Join(t.TempDir(), "origin.git")
	ref := filepath.Join(root, "api")
	run := func(dir string, args ...string) {
		t.Helper()
		cmd := exec.Command("git", append([]string{"-c", "user.name=t", "-c", "user.email=t@t"}, args...)...)
		cmd.Dir = dir
		if out, err := cmd.CombinedOutput(); err != nil {
			t.Fatalf("git %v: %v\n%s", args, err, out)
		}
	}
	run(root, "init", "-q", "--bare", "-b", "main", origin)
	run(root, "clone", "-q", origin, ref)
	os.WriteFile(filepath.Join(ref, "app.go"), []byte("package app\n"), 0o644)
	run(ref, "add", ".")
	run(ref, "commit", "-q", "-m", "base")
	run(ref, "push", "-q", "origin", "main")
	run(ref, "fetch", "-q")
	for _, task := range []string{"feat", "idle"} {
		run(ref, "worktree", "add", "-q", "-b", task, filepath.Join(root, "workdir", task, "api"), "origin/main")
	}

	feat := filepath.Join(root, "workdir", "feat", "api")
	os.WriteFile(filepath.Join(feat, "committed.go"), []byte("package app\n"), 0o644)
	run(feat, "add", ".")
	run(feat, "commit", "-q", "-m", "on branch")
	os.WriteFile(filepath.Join(feat, "app.go"), []byte("package app\n\nvar X = 1\n"), 0o644)
	os.WriteFile(filepath.Join(feat, "untracked.txt"), []byte("no newline"), 0o644)
	os.WriteFile(filepath.Join(feat, ".gitignore"), []byte("ignored.log\n"), 0o644)
	os.WriteFile(filepath.Join(feat, "ignored.log"), []byte("noise\n"), 0o644)

	// main moves on after feat branched; that must not show in feat's diff.
	os.WriteFile(filepath.Join(ref, "upstream.go"), []byte("package app\n"), 0o644)
	run(ref, "add", ".")
	run(ref, "commit", "-q", "-m", "upstream")
	run(ref, "push", "-q", "origin", "main")
	// The reference checkout itself is never shown, even when dirty.
	os.WriteFile(filepath.Join(ref, "dirty-ref.go"), []byte("package app\n"), 0o644)
	return root
}

func TestFindWorktrees(t *testing.T) {
	root := newWorkspace(t)
	var got []string
	for _, wt := range findWorktrees(root) {
		got = append(got, wt.prefix)
	}
	if want := []string{"workdir/feat/api/", "workdir/idle/api/"}; !reflect.DeepEqual(got, want) {
		t.Errorf("from workspace root = %v, want %v", got, want)
	}
	inside := findWorktrees(filepath.Join(root, "workdir", "feat", "api"))
	if len(inside) != 1 || inside[0].prefix != "" {
		t.Errorf("from inside a worktree = %+v, want just it, unprefixed", inside)
	}
	if got := findWorktrees(filepath.Join(root, "workdir")); len(got) != 2 || got[0].prefix != "feat/api/" {
		t.Errorf("from workdir = %+v", got)
	}
}

func TestPrintWorktreeDiffs(t *testing.T) {
	root := newWorkspace(t)
	var buf strings.Builder
	if code := printWorktreeDiffs(root, &buf); code != 0 {
		t.Fatalf("exit %d", code)
	}
	files, _, err := gitdiff.Parse(strings.NewReader(buf.String()))
	if err != nil {
		t.Fatalf("diffnav's parser rejects the output: %v\n%s", err, buf.String())
	}
	var got []string
	for _, f := range files {
		got = append(got, f.NewName)
	}
	sort.Strings(got)
	want := []string{
		"workdir/feat/api/.gitignore",
		"workdir/feat/api/app.go",
		"workdir/feat/api/committed.go",
		"workdir/feat/api/untracked.txt",
	}
	if !reflect.DeepEqual(got, want) {
		t.Errorf("files = %v, want %v\n%s", got, want, buf.String())
	}
	for _, f := range files {
		if f.NewName == "workdir/feat/api/untracked.txt" && !f.IsNew {
			t.Errorf("untracked file not marked new")
		}
	}
}

func TestNewFileDiff(t *testing.T) {
	tests := []struct {
		content string
		want    []string
	}{
		{"a\nb\n", []string{"@@ -0,0 +1,2 @@", "+a\n+b\n"}},
		{"a", []string{"@@ -0,0 +1,1 @@", "+a\n\\ No newline at end of file\n"}},
		{"", []string{"new file mode 100644\n"}},
	}
	for _, tt := range tests {
		got := newFileDiff("x/f.txt", []byte(tt.content))
		for _, w := range tt.want {
			if !strings.Contains(got, w) {
				t.Errorf("newFileDiff(%q) missing %q:\n%s", tt.content, w, got)
			}
		}
		if _, _, err := gitdiff.Parse(strings.NewReader(got)); err != nil {
			t.Errorf("newFileDiff(%q) unparseable: %v", tt.content, err)
		}
	}
}
