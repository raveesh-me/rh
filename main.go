// Command rh runs quick workspace chores and falls back to make for anything else.
package main

import (
	"fmt"
	"io"
	"os"
	"strings"
)

type builtin struct {
	name    string
	summary string
	run     func(args []string) int
}

var builtins = []builtin{
	{"init", "create or update CLAUDE.md, AGENTS.md, Makefile to latest + workdir/", cmdInit},
	{"add", "git clone one or more repo URLs into this folder, in parallel", cmdAdd},
	{"pull", "make pull, or checkout default branch + pull in every repo below here", cmdPull},
	{"update", "replace this rh binary with the latest GitHub release", cmdUpdate},
	{"mon", "live view of ls . and ls workdir", cmdMon},
	{"diff", "live diff explorer (diffnav + delta) of files changed below here", cmdDiff},
}

type routeKind int

const (
	routeHelp routeKind = iota
	routeVersion
	routeBuiltin
	routeMake
	routeClaude
	routeError
)

type route struct {
	kind    routeKind
	builtin *builtin
	args    []string
	err     string
}

// version is set at release time via -ldflags "-X main.version=...".
var version = "dev"

func main() {
	os.Exit(dispatch(os.Args[1:]))
}

func dispatch(args []string) int {
	r := resolve(args)
	switch r.kind {
	case routeHelp:
		usage(os.Stdout)
		return 0
	case routeVersion:
		fmt.Println("rh", version)
		return 0
	case routeBuiltin:
		return r.builtin.run(r.args)
	case routeMake:
		return runMake(r.args)
	case routeClaude:
		return runInteractive("claude", r.args...)
	default:
		fmt.Fprintln(os.Stderr, "rh:", r.err)
		usage(os.Stderr)
		return 2
	}
}

// resolve decides what a command line means without running anything.
func resolve(args []string) route {
	if len(args) == 0 {
		return route{kind: routeHelp}
	}
	switch args[0] {
	case "-h", "--help", "help":
		return route{kind: routeHelp}
	case "-v", "--version", "version":
		return route{kind: routeVersion}
	case "-m":
		return route{kind: routeMake, args: args[1:]}
	case "-c":
		prompt := strings.TrimSpace(strings.Join(args[1:], " "))
		if prompt == "" {
			return route{kind: routeError, err: "-c needs a prompt"}
		}
		return route{kind: routeClaude, args: []string{"-p", prompt}}
	}
	for i := range builtins {
		if builtins[i].name == args[0] {
			return route{kind: routeBuiltin, builtin: &builtins[i], args: args[1:]}
		}
	}
	return route{kind: routeMake, args: args}
}

func runMake(args []string) int {
	if findMakefile(".") == "" {
		fmt.Fprintf(os.Stderr, "rh: no Makefile here and %q is not an rh command\n", strings.Join(args, " "))
		return 2
	}
	return runInteractive("make", args...)
}

func usage(w io.Writer) {
	fmt.Fprintln(w, "usage: rh <command> [args]")
	fmt.Fprintln(w)
	for _, b := range builtins {
		fmt.Fprintf(w, "  rh %-8s %s\n", b.name, b.summary)
	}
	fmt.Fprintf(w, "  rh %-8s %s\n", "<target>", "anything else runs make <target>")
	fmt.Fprintf(w, "  rh %-8s %s\n", "-m ...", "always run make ... (even for rh command names)")
	fmt.Fprintf(w, "  rh %-8s %s\n", "-c ...", `run claude -p "..."`)
	fmt.Fprintf(w, "  rh %-8s %s\n", "-v", "print version")
}
