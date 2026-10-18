package main

import (
	"fmt"
	"io"
	"os"
	"sync"
	"time"
	"unicode/utf8"
)

const (
	frameInterval = 100 * time.Millisecond
	defaultWidth  = 80
)

var spinnerFrames = []rune("⠋⠙⠹⠸⠼⠴⠦⠧⠇⠏")

type taskState int

const (
	taskQueued taskState = iota
	taskRunning
	taskOK
	taskFailed
)

type task struct {
	label string
	state taskState
	msg   string
}

// progress renders one status line per task. On a terminal the block is
// redrawn in place; otherwise a plain line is printed as each task finishes.
type progress struct {
	mu    sync.Mutex
	w     io.Writer
	live  bool
	width int
	verb  string
	tasks []task
	frame int
	drawn int
}

func newProgress(w io.Writer, verb string, items []string, label func(string) string) *progress {
	p := &progress{w: w, verb: verb, width: defaultWidth}
	if f, ok := w.(*os.File); ok && isTerminal(f) {
		p.live = true
		if cols := termWidth(f); cols > 0 {
			p.width = cols
		}
	}
	for _, it := range items {
		p.tasks = append(p.tasks, task{label: label(it)})
	}
	return p
}

func (p *progress) start(i int) {
	p.mu.Lock()
	defer p.mu.Unlock()
	p.tasks[i].state = taskRunning
}

func (p *progress) report(i int, msg string) {
	p.mu.Lock()
	defer p.mu.Unlock()
	p.tasks[i].msg = msg
}

func (p *progress) finish(i int, msg string, err error) {
	p.mu.Lock()
	defer p.mu.Unlock()
	t := &p.tasks[i]
	t.state, t.msg = taskOK, msg
	if err != nil {
		t.state, t.msg = taskFailed, err.Error()
	}
	if !p.live {
		fmt.Fprintln(p.w, p.line(*t, false))
	}
}

// animate redraws the live block until the returned stop func is called.
func (p *progress) animate() (stop func()) {
	if !p.live {
		return func() {}
	}
	quit, exited := make(chan struct{}), make(chan struct{})
	go func() {
		defer close(exited)
		ticker := time.NewTicker(frameInterval)
		defer ticker.Stop()
		for {
			p.mu.Lock()
			p.frame++
			p.redraw(true)
			p.mu.Unlock()
			select {
			case <-quit:
				return
			case <-ticker.C:
			}
		}
	}()
	return func() { close(quit); <-exited }
}

// done draws the final state untruncated and prints a summary.
func (p *progress) done() int {
	p.mu.Lock()
	defer p.mu.Unlock()
	if p.live {
		p.redraw(false)
	}
	failed := 0
	for _, t := range p.tasks {
		if t.state == taskFailed {
			failed++
		}
	}
	if failed > 0 {
		fmt.Fprintf(p.w, "done, %d of %d failed.\n", failed, len(p.tasks))
		return 1
	}
	fmt.Fprintln(p.w, "done.")
	return 0
}

// redraw moves the cursor back over the previous block and repaints it.
// Truncated lines never wrap, so the cursor math stays right.
func (p *progress) redraw(truncate bool) {
	if p.drawn > 0 {
		fmt.Fprintf(p.w, "\033[%dA", p.drawn)
	}
	for _, t := range p.tasks {
		fmt.Fprintf(p.w, "\r\033[2K%s\n", p.line(t, truncate))
	}
	p.drawn = len(p.tasks)
}

func (p *progress) line(t task, truncate bool) string {
	text := fmt.Sprintf(" %s %s...", p.verb, t.label)
	if t.msg != "" {
		text += " " + t.msg
	}
	if truncate {
		text = truncateRunes(text, p.width-4)
	}
	return p.mark(t.state) + text
}

func (p *progress) mark(s taskState) string {
	switch s {
	case taskRunning:
		return "[" + string(spinnerFrames[p.frame%len(spinnerFrames)]) + "]"
	case taskOK:
		return p.color("32", "[✓]")
	case taskFailed:
		return p.color("31", "[✗]")
	}
	return "[ ]"
}

func (p *progress) color(code, s string) string {
	if !p.live {
		return s
	}
	return "\033[" + code + "m" + s + "\033[0m"
}

func truncateRunes(s string, n int) string {
	if n <= 1 || utf8.RuneCountInString(s) <= n {
		return s
	}
	r := []rune(s)
	return string(r[:n-1]) + "…"
}

func isTerminal(f *os.File) bool {
	info, err := f.Stat()
	return err == nil && info.Mode()&os.ModeCharDevice != 0
}
