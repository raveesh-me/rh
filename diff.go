package main

import (
	"context"
	"errors"
	"fmt"
	"io"
	"os"
	"os/signal"
	"path/filepath"
	"strings"
	"syscall"
	"time"

	tea "charm.land/bubbletea/v2"
	"charm.land/log/v2"
	"github.com/dlvhdr/diffnav/pkg/config"
	"github.com/dlvhdr/diffnav/pkg/ui"
	zone "github.com/lrstanley/bubblezone/v2"
)

const diffInterval = time.Second

// cmdDiff opens diffnav (file tree + delta-rendered diff) on a watch loop.
// Each tick diffnav re-runs `rh diff`; the env var the explorer set makes
// that child print the current diff instead of opening another explorer.
//
//	rh diff         every git worktree below here vs its merge-base with origin/<default>
//	rh diff --live  every text file below here vs a snapshot taken at start
func cmdDiff(args []string) int {
	if root := os.Getenv(gitRootEnv); root != "" {
		return printWorktreeDiffs(root, os.Stdout)
	}
	if base := os.Getenv(baselineEnv); base != "" {
		return printDiffAgainst(base, os.Stdout)
	}
	var err error
	switch {
	case len(args) == 0:
		err = runGitExplorer()
	case len(args) == 1 && args[0] == "--live":
		err = runLiveExplorer()
	default:
		fmt.Fprintln(os.Stderr, "usage: rh diff [--live]")
		return 2
	}
	if err != nil {
		fmt.Fprintln(os.Stderr, "rh diff:", err)
		return 1
	}
	return 0
}

// explorer owns a temp dir holding a private `rh` symlink, put first on PATH
// so the watch command diffnav shows in its footer is a plain `rh diff`.
type explorer struct {
	tmp string
}

func newExplorer() (*explorer, error) {
	exe, err := currentExecutable()
	if err != nil {
		return nil, err
	}
	deltaDir, err := ensureDelta()
	if err != nil {
		return nil, err
	}
	tmp, err := os.MkdirTemp("", "rh-diff-")
	if err != nil {
		return nil, err
	}
	if err := os.Symlink(exe, filepath.Join(tmp, "rh")); err != nil {
		os.RemoveAll(tmp)
		return nil, err
	}
	path := []string{tmp}
	if deltaDir != "" {
		path = append(path, deltaDir)
	}
	os.Setenv("PATH", strings.Join(append(path, os.Getenv("PATH")), string(os.PathListSeparator)))
	return &explorer{tmp: tmp}, nil
}

func (e *explorer) close() { os.RemoveAll(e.tmp) }

// run shows the explorer until the user quits. The caller has already set the
// env var that makes watchCmd print the diff.
func (e *explorer) run(watchCmd string) error {
	cfg := config.Load()
	cfg.Watch = config.WatchConfig{Enabled: true, Cmd: watchCmd, Interval: diffInterval}
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
