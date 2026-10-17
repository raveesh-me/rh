package main

import (
	"errors"
	"fmt"
	"os"
	"os/exec"
	"os/signal"
	"path/filepath"
	"regexp"
	"syscall"
)

var makefileNames = []string{"GNUmakefile", "makefile", "Makefile"}

// runInteractive runs a command attached to the terminal and returns its exit code.
func runInteractive(name string, args ...string) int {
	cmd := exec.Command(name, args...)
	cmd.Stdin, cmd.Stdout, cmd.Stderr = os.Stdin, os.Stdout, os.Stderr

	// The child gets ctrl-c from the terminal itself; rh just waits for it.
	sigs := make(chan os.Signal, 1)
	signal.Notify(sigs, os.Interrupt, syscall.SIGTERM)
	defer signal.Stop(sigs)

	err := cmd.Run()
	var exitErr *exec.ExitError
	switch {
	case err == nil:
		return 0
	case errors.As(err, &exitErr):
		return exitErr.ExitCode()
	default:
		fmt.Fprintf(os.Stderr, "rh: %s: %v\n", name, err)
		return 127
	}
}

// findMakefile returns the makefile make would pick in dir, or "" if none.
func findMakefile(dir string) string {
	for _, n := range makefileNames {
		p := filepath.Join(dir, n)
		if info, err := os.Stat(p); err == nil && !info.IsDir() {
			return p
		}
	}
	return ""
}

func hasMakeTarget(dir, target string) bool {
	path := findMakefile(dir)
	if path == "" {
		return false
	}
	data, err := os.ReadFile(path)
	if err != nil {
		return false
	}
	return makefileDefines(string(data), target)
}

// makefileDefines reports whether src has a rule for target (not a := / ::= assignment).
func makefileDefines(src, target string) bool {
	re := regexp.MustCompile(`(?m)^(?:[^\s:#=][^:#=]*\s)?` + regexp.QuoteMeta(target) + `(?:\s[^:#=]*)?:(?:$|[^:=]|:(?:$|[^=]))`)
	return re.MatchString(src)
}
