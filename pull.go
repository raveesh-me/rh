package main

import (
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"sort"
	"strings"
	"sync"
)

const maxParallelPulls = 8

type updateResult struct {
	repo string
	msg  string
	err  error
}

func cmdPull(args []string) int {
	if hasMakeTarget(".", "pull") {
		return runMake(append([]string{"pull"}, args...))
	}
	repos, err := findRepos(".")
	if err != nil {
		fmt.Fprintln(os.Stderr, "rh pull:", err)
		return 1
	}
	if len(repos) == 0 {
		fmt.Println("rh pull: no git repos here")
		return 0
	}
	return reportUpdates(updateAll(repos))
}

// findRepos returns dir itself if it is a repo, otherwise its direct child repos.
// Worktrees (where .git is a file) are skipped so task branches are never switched.
func findRepos(dir string) ([]string, error) {
	if isRepoRoot(dir) {
		return []string{dir}, nil
	}
	entries, err := os.ReadDir(dir)
	if err != nil {
		return nil, err
	}
	var repos []string
	for _, e := range entries {
		p := filepath.Join(dir, e.Name())
		if e.IsDir() && isRepoRoot(p) {
			repos = append(repos, p)
		}
	}
	return repos, nil
}

func isRepoRoot(dir string) bool {
	info, err := os.Stat(filepath.Join(dir, ".git"))
	return err == nil && info.IsDir()
}

func updateAll(repos []string) []updateResult {
	results := make([]updateResult, len(repos))
	sem := make(chan struct{}, maxParallelPulls)
	var wg sync.WaitGroup
	for i, repo := range repos {
		wg.Go(func() {
			sem <- struct{}{}
			defer func() { <-sem }()
			msg, err := updateRepo(repo)
			results[i] = updateResult{repo: repoLabel(repo), msg: msg, err: err}
		})
	}
	wg.Wait()
	sort.Slice(results, func(a, b int) bool { return results[a].repo < results[b].repo })
	return results
}

func repoLabel(repo string) string {
	if abs, err := filepath.Abs(repo); err == nil {
		return filepath.Base(abs)
	}
	return repo
}

func updateRepo(dir string) (string, error) {
	status, err := git(dir, "status", "--porcelain")
	if err != nil {
		return "", err
	}
	if status != "" {
		return "", fmt.Errorf("dirty working tree, skipped")
	}
	branch := defaultBranch(dir)
	if _, err := git(dir, "checkout", "--quiet", branch); err != nil {
		return "", err
	}
	before, _ := git(dir, "rev-parse", "--short", "HEAD")
	if _, err := git(dir, "pull", "--ff-only", "--quiet"); err != nil {
		return "", err
	}
	after, _ := git(dir, "rev-parse", "--short", "HEAD")
	if before == after {
		return fmt.Sprintf("%s up to date", branch), nil
	}
	return fmt.Sprintf("%s %s..%s", branch, before, after), nil
}

// defaultBranch reads origin/HEAD, falling back to main or master.
func defaultBranch(dir string) string {
	if ref, err := git(dir, "symbolic-ref", "--short", "refs/remotes/origin/HEAD"); err == nil {
		return strings.TrimPrefix(ref, "origin/")
	}
	for _, b := range []string{"main", "master"} {
		if _, err := git(dir, "rev-parse", "--verify", "--quiet", "refs/heads/"+b); err == nil {
			return b
		}
	}
	return "main"
}

func reportUpdates(results []updateResult) int {
	width := 0
	for _, r := range results {
		width = max(width, len(r.repo))
	}
	code := 0
	for _, r := range results {
		if r.err != nil {
			fmt.Printf("  \033[31m✗\033[0m %-*s  %v\n", width, r.repo, r.err)
			code = 1
			continue
		}
		fmt.Printf("  \033[32m✓\033[0m %-*s  %s\n", width, r.repo, r.msg)
	}
	return code
}

func git(dir string, args ...string) (string, error) {
	cmd := exec.Command("git", args...)
	cmd.Dir = dir
	cmd.Env = append(os.Environ(), "GIT_TERMINAL_PROMPT=0")
	out, err := cmd.CombinedOutput()
	text := strings.TrimSpace(string(out))
	if err != nil {
		if text == "" {
			return "", fmt.Errorf("git %s: %w", args[0], err)
		}
		return "", fmt.Errorf("git %s: %s", args[0], firstLine(text))
	}
	return text, nil
}

func firstLine(s string) string {
	line, _, _ := strings.Cut(s, "\n")
	return line
}
