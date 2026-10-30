package main

import (
	"fmt"
	"os"
	"path/filepath"
)

const workdirName = "workdir"

// cmdClean removes every task worktree under ./workdir like `make rm` does:
// branches are kept, and dirty worktrees are left alone unless -f is given.
func cmdClean(args []string) int {
	force := false
	for _, a := range args {
		switch a {
		case "-f", "--force":
			force = true
		default:
			fmt.Fprintln(os.Stderr, "usage: rh clean [-f]")
			return 2
		}
	}
	if hasMakeTarget(".", "clean") {
		return runMake(append([]string{"clean"}, args...))
	}
	if info, err := os.Stat(workdirName); err != nil || !info.IsDir() {
		fmt.Fprintln(os.Stderr, "rh clean: no workdir/ here; run it from the workspace root")
		return 1
	}
	kept, err := cleanWorkdir(workdirName, force)
	if err != nil {
		fmt.Fprintln(os.Stderr, "rh clean:", err)
		return 1
	}
	if kept > 0 {
		fmt.Printf("rh clean: kept %d entr(ies), see above; -f also removes dirty worktrees\n", kept)
		return 1
	}
	return 0
}

// cleanWorkdir removes workdir/<task>/<repo> worktrees and the task dirs they
// empty, then prunes stale worktree metadata. It returns how many it skipped.
func cleanWorkdir(workdir string, force bool) (int, error) {
	tasks, err := os.ReadDir(workdir)
	if err != nil {
		return 0, err
	}
	kept := 0
	repos := map[string]bool{}
	for _, t := range tasks {
		if !t.IsDir() {
			continue
		}
		taskDir := filepath.Join(workdir, t.Name())
		entries, err := os.ReadDir(taskDir)
		if err != nil {
			return kept, err
		}
		for _, e := range entries {
			wt := filepath.Join(taskDir, e.Name())
			repo, err := removeWorktree(wt, force)
			if err != nil {
				fmt.Printf("==> kept %s: %v\n", wt, err)
				kept++
				continue
			}
			repos[repo] = true
			fmt.Printf("==> removed %s\n", wt)
		}
		if os.Remove(taskDir) == nil {
			fmt.Printf("==> removed task %s\n", t.Name())
		}
	}
	for repo := range repos {
		git(repo, "worktree", "prune")
	}
	return kept, nil
}

// removeWorktree removes one linked worktree via its main repo and returns
// that repo. Anything that is not a linked worktree is left untouched.
func removeWorktree(wt string, force bool) (string, error) {
	if info, err := os.Stat(filepath.Join(wt, ".git")); err != nil || info.IsDir() {
		return "", fmt.Errorf("not a git worktree")
	}
	common, err := git(wt, "rev-parse", "--path-format=absolute", "--git-common-dir")
	if err != nil {
		return "", err
	}
	repo := filepath.Dir(common)
	abs, err := filepath.Abs(wt)
	if err != nil {
		return "", err
	}
	args := []string{"worktree", "remove"}
	if force {
		args = append(args, "--force")
	}
	if _, err := git(repo, append(args, abs)...); err != nil {
		return "", err
	}
	return repo, nil
}
