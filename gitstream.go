package main

import (
	"bufio"
	"bytes"
	"fmt"
	"os"
	"os/exec"
	"strings"
)

// gitStream runs git, passing each progress line ("Receiving objects: 45% ...")
// to report as it arrives. On failure the error carries git's most useful line.
func gitStream(dir string, report func(string), args ...string) error {
	cmd := exec.Command("git", args...)
	cmd.Dir = dir
	cmd.Env = append(os.Environ(), "GIT_TERMINAL_PROMPT=0")
	stderr, err := cmd.StderrPipe()
	if err != nil {
		return err
	}
	if err := cmd.Start(); err != nil {
		return fmt.Errorf("git %s: %w", args[0], err)
	}
	var messages []string
	sc := bufio.NewScanner(stderr)
	sc.Split(scanCROrLF)
	for sc.Scan() {
		line := strings.TrimSpace(sc.Text())
		switch {
		case line == "":
		case strings.Contains(line, "%"):
			report(collapseSpaces(line))
		default:
			messages = append(messages, line)
		}
	}
	if err := cmd.Wait(); err != nil {
		if msg := gitErrorLine(messages); msg != "" {
			return fmt.Errorf("git %s: %s", args[0], msg)
		}
		return fmt.Errorf("git %s: %w", args[0], err)
	}
	return nil
}

// gitErrorLine prefers git's fatal/error line over chatter like "Cloning into".
func gitErrorLine(lines []string) string {
	for _, l := range lines {
		if strings.HasPrefix(l, "fatal:") || strings.HasPrefix(l, "ERROR:") || strings.HasPrefix(l, "error:") {
			return l
		}
	}
	if len(lines) > 0 {
		return lines[len(lines)-1]
	}
	return ""
}

// scanCROrLF splits on \r or \n, since git redraws progress with \r.
func scanCROrLF(data []byte, atEOF bool) (int, []byte, error) {
	if i := bytes.IndexAny(data, "\r\n"); i >= 0 {
		return i + 1, data[:i], nil
	}
	if atEOF && len(data) > 0 {
		return len(data), data, nil
	}
	return 0, nil, nil
}

func collapseSpaces(s string) string {
	return strings.Join(strings.Fields(s), " ")
}
