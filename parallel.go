package main

import (
	"fmt"
	"sort"
	"sync"
)

const maxParallel = 8

type repoResult struct {
	repo string
	msg  string
	err  error
}

// forEachParallel runs fn on every item with bounded concurrency and returns
// the results sorted by label.
func forEachParallel(items []string, label func(string) string, fn func(string) (string, error)) []repoResult {
	results := make([]repoResult, len(items))
	sem := make(chan struct{}, maxParallel)
	var wg sync.WaitGroup
	for i, item := range items {
		wg.Go(func() {
			sem <- struct{}{}
			defer func() { <-sem }()
			msg, err := fn(item)
			results[i] = repoResult{repo: label(item), msg: msg, err: err}
		})
	}
	wg.Wait()
	sort.Slice(results, func(a, b int) bool { return results[a].repo < results[b].repo })
	return results
}

// reportResults prints one ✓/✗ line per repo and returns 1 if any failed.
func reportResults(results []repoResult) int {
	width := 0
	for _, r := range results {
		width = max(width, len(r.repo))
	}
	code := 0
	for _, r := range results {
		if r.err != nil {
			fmt.Printf("  \033[31m✗\033[0m %-*s  %v\n", width, r.repo, r.err)
			code = 1
			continue
		}
		fmt.Printf("  \033[32m✓\033[0m %-*s  %s\n", width, r.repo, r.msg)
	}
	return code
}
