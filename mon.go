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
	// Home, clear screen, and clear scrollback; used on resize when text reflows.
	clearScreen = "\033[H\033[2J\033[3J"
	// Synchronized output: the terminal paints the whole frame at once.
	// Terminals without support ignore these sequences.
	beginSync = "\033[?2026h"
	endSync   = "\033[?2026l"
	// Erase to end of line / to end of screen.
	eraseLine = "\033[K"
	eraseDown = "\033[J"
	// Alternate screen buffer (like top/less): frames never land in the shell's
	// scrollback, and the original screen is restored on exit.
	enterAltScreen = "\033[?1049h\033[?25l"
	leaveAltScreen = "\033[?25h\033[?1049l"
)

func cmdMon(args []string) int {
	dir, err := os.Getwd()
	if err != nil {
		fmt.Fprintln(os.Stderr, "rh mon:", err)
		return 1
	}
	ctx, stop := signal.NotifyContext(context.Background(), os.Interrupt, syscall.SIGTERM)
	defer stop()

	resize := make(chan os.Signal, 1)
	if sigs := resizeSignals(); len(sigs) > 0 {
		signal.Notify(resize, sigs...)
		defer signal.Stop(resize)
	}

	fmt.Print(enterAltScreen)
	defer fmt.Print(leaveAltScreen)

	ticker := time.NewTicker(monInterval)
	defer ticker.Stop()
	last, full := "", true
	for {
		if view := renderMon(dir); full || view != last {
			fmt.Print(monFrame(view, full))
			last, full = view, false
		}
		select {
		case <-ctx.Done():
			return 0
		case <-ticker.C:
		case <-resize:
			full = true
		}
	}
}

// monFrame overwrites the previous frame in place instead of clearing first,
// so ticks don't flash a blank screen. Each line erases its own leftover tail
// and eraseDown removes rows the previous frame had beyond this one. A full
// clear is only done when the terminal was resized and old text has reflowed.
func monFrame(view string, full bool) string {
	var b strings.Builder
	b.WriteString(beginSync)
	if full {
		b.WriteString(clearScreen)
	} else {
		b.WriteString("\033[H")
	}
	b.WriteString(strings.ReplaceAll(view, "\n", eraseLine+"\n"))
	b.WriteString(eraseLine + eraseDown + endSync)
	return b.String()
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
