package main

import (
	"fmt"
	"os"
)

// cmdAdd clones a repo into the current folder: rh add <url> [dir].
func cmdAdd(args []string) int {
	if len(args) == 0 {
		fmt.Fprintln(os.Stderr, "usage: rh add <repo-url> [dir]")
		return 2
	}
	return runInteractive("git", append([]string{"clone"}, args...)...)
}
