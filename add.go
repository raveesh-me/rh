package main

import (
	"regexp"
	"strings"
)

// scpLikeURL matches git@host:owner/repo style remotes.
var scpLikeURL = regexp.MustCompile(`^[\w.-]+@[\w.-]+:`)

// cmdAdd clones a repo into the current folder. Anything that isn't a repo URL
// falls through to `make add`, so the workspace Makefile's add TASK=... still works.
func cmdAdd(args []string) int {
	if len(args) == 0 || !isRepoURL(args[0]) {
		return runMake(append([]string{"add"}, args...))
	}
	return runInteractive("git", append([]string{"clone"}, args...)...)
}

func isRepoURL(s string) bool {
	return strings.Contains(s, "://") || scpLikeURL.MatchString(s) || strings.HasSuffix(s, ".git")
}
