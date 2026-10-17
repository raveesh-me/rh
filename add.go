package main

import (
	"fmt"
	"os"
	"strings"
)

// cmdAdd clones every repo URL into the current folder in parallel:
// rh add <url> [url...]. Repos whose folder already exists are skipped.
func cmdAdd(args []string) int {
	if len(args) == 0 {
		fmt.Fprintln(os.Stderr, "usage: rh add <repo-url> [repo-url...]")
		return 2
	}
	urls, err := uniqueByDir(args)
	if err != nil {
		fmt.Fprintln(os.Stderr, "rh add:", err)
		return 2
	}
	return reportResults(forEachParallel(urls, repoDirName, cloneRepo))
}

// uniqueByDir drops repeated URLs and rejects different URLs that would clone
// into the same folder.
func uniqueByDir(urls []string) ([]string, error) {
	seen := map[string]string{}
	var out []string
	for _, u := range urls {
		dir := repoDirName(u)
		if dir == "" {
			return nil, fmt.Errorf("can't tell the folder name for %q", u)
		}
		if prev, ok := seen[dir]; ok {
			if prev != u {
				return nil, fmt.Errorf("%s and %s would both clone into %s/", prev, u, dir)
			}
			continue
		}
		seen[dir] = u
		out = append(out, u)
	}
	return out, nil
}

// repoDirName is the folder git clone would create: the last path segment without .git.
func repoDirName(url string) string {
	url = strings.TrimRight(url, "/")
	if i := strings.LastIndexAny(url, "/:"); i >= 0 {
		url = url[i+1:]
	}
	return strings.TrimSuffix(url, ".git")
}

func cloneRepo(url string) (string, error) {
	dir := repoDirName(url)
	if _, err := os.Stat(dir); err == nil {
		return "already exists, skipped", nil
	}
	if _, err := git(".", "clone", "--quiet", url, dir); err != nil {
		return "", err
	}
	return "cloned", nil
}
