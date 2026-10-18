package main

import (
	"bytes"
	"errors"
	"strings"
	"testing"
)

func TestProgressPlainOutput(t *testing.T) {
	var buf bytes.Buffer
	p := newProgress(&buf, "cloning", []string{"a", "b"}, func(s string) string { return s })
	p.start(0)
	p.report(0, "Receiving objects: 50%")
	p.finish(0, "cloned", nil)
	p.finish(1, "", errors.New("git clone: ERROR: Repository not found."))
	if code := p.done(); code != 1 {
		t.Errorf("done = %d, want 1", code)
	}
	want := "[✓] cloning a... cloned\n[✗] cloning b... git clone: ERROR: Repository not found.\ndone, 1 of 2 failed.\n"
	if buf.String() != want {
		t.Errorf("output:\n%s\nwant:\n%s", buf.String(), want)
	}
}

func TestProgressLiveRedrawTruncates(t *testing.T) {
	var buf bytes.Buffer
	p := &progress{w: &buf, live: true, width: 30, verb: "cloning", tasks: []task{
		{label: "leadgen-marketplace-backend", state: taskRunning, msg: "Receiving objects: 12%"},
	}}
	p.redraw(true)
	p.redraw(true)
	out := buf.String()
	if !strings.Contains(out, "\033[1A") {
		t.Error("second redraw should move cursor up over the block")
	}
	for _, line := range strings.Split(strings.TrimSpace(out), "\n") {
		visible := line[strings.LastIndex(line, "\033[2K")+len("\033[2K"):]
		if n := len([]rune(visible)); n > 30 {
			t.Errorf("line wider than terminal (%d): %q", n, visible)
		}
	}
}

func TestGitErrorLine(t *testing.T) {
	lines := []string{"Cloning into 'x'...", "ERROR: Repository not found.", "fatal: Could not read from remote repository."}
	if got := gitErrorLine(lines); got != "ERROR: Repository not found." {
		t.Errorf("gitErrorLine = %q", got)
	}
}
