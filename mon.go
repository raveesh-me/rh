package main

import (
	"context"
	"fmt"
	"os"
	"os/signal"
	"path/filepath"
	"sort"
	"strings"
	"syscall"
	"time"
)

const (
	monInterval = 2 * time.Second
	clearScreen = "\033[H\033[2J"
)

func cmdMon(args []string) int {
	dir, err := os.Getwd()
	if err != nil {
		fmt.Fprintln(os.Stderr, "rh mon:", err)
		return 1
	}
	ctx, stop := signal.NotifyContext(context.Background(), os.Interrupt, syscall.SIGTERM)
	defer stop()

	ticker := time.NewTicker(monInterval)
	defer ticker.Stop()
	last := ""
	for {
		// Only redraw on change so the screen doesn't flicker.
		if view := renderMon(dir); view != last {
			fmt.Print(clearScreen + view)
			last = view
		}
		select {
		case <-ctx.Done():
			fmt.Println()
			return 0
		case <-ticker.C:
		}
	}
}

func renderMon(dir string) string {
	var b strings.Builder
	fmt.Fprintf(&b, "\033[1mrh mon\033[0m  %s  \033[2m(ctrl-c to quit)\033[0m\n\n", dir)

	fmt.Fprintf(&b, "\033[1m── .\033[0m\n")
	for _, name := range listVisible(dir) {
		fmt.Fprintf(&b, "  %s\n", name)
	}

	workdir := filepath.Join(dir, "workdir")
	fmt.Fprintf(&b, "\n\033[1m── workdir\033[0m\n")
	tasks := listVisible(workdir)
	if len(tasks) == 0 {
		fmt.Fprintf(&b, "  \033[2m(empty)\033[0m\n")
	}
	for _, task := range tasks {
		fmt.Fprintf(&b, "  %s\n", task)
		if !strings.HasSuffix(task, "/") {
			continue
		}
		taskDir := filepath.Join(workdir, task)
		for _, repo := range listVisible(taskDir) {
			fmt.Fprintf(&b, "    %s%s\n", repo, branchSuffix(filepath.Join(taskDir, repo)))
		}
	}
	return b.String()
}

// listVisible lists non-hidden entries sorted by name, with "/" after directories.
func listVisible(dir string) []string {
	entries, err := os.ReadDir(dir)
	if err != nil {
		return nil
	}
	var names []string
	for _, e := range entries {
		if strings.HasPrefix(e.Name(), ".") {
			continue
		}
		name := e.Name()
		if isDirEntry(dir, e) {
			name += "/"
		}
		names = append(names, name)
	}
	sort.Strings(names)
	return names
}

func isDirEntry(dir string, e os.DirEntry) bool {
	if e.IsDir() {
		return true
	}
	info, err := os.Stat(filepath.Join(dir, e.Name()))
	return err == nil && info.IsDir()
}

func branchSuffix(repo string) string {
	if b := currentBranch(repo); b != "" {
		return "  \033[36m(" + b + ")\033[0m"
	}
	return ""
}

// currentBranch reads HEAD directly so mon doesn't spawn git every tick.
func currentBranch(repo string) string {
	gitDir := filepath.Join(repo, ".git")
	info, err := os.Stat(gitDir)
	if err != nil {
		return ""
	}
	if !info.IsDir() {
		// Worktree: .git is a file containing "gitdir: <path>".
		data, err := os.ReadFile(gitDir)
		if err != nil {
			return ""
		}
		path, ok := strings.CutPrefix(strings.TrimSpace(string(data)), "gitdir: ")
		if !ok {
			return ""
		}
		if !filepath.IsAbs(path) {
			path = filepath.Join(repo, path)
		}
		gitDir = path
	}
	head, err := os.ReadFile(filepath.Join(gitDir, "HEAD"))
	if err != nil {
		return ""
	}
	ref := strings.TrimSpace(string(head))
	if branch, ok := strings.CutPrefix(ref, "ref: refs/heads/"); ok {
		return branch
	}
	if len(ref) >= 7 {
		return "detached " + ref[:7]
	}
	return ""
}
