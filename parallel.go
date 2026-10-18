package main

import (
	"os"
	"sync"
)

const maxParallel = 8

// job does the work for one item. report may be called with short progress
// text (e.g. git's "Receiving objects: 45%") while it runs.
type job func(item string, report func(string)) (string, error)

// runParallel runs fn on every item (at most maxParallel at once) with live
// per-item progress on stdout, and returns 1 if any item failed.
func runParallel(verb string, items []string, label func(string) string, fn job) int {
	p := newProgress(os.Stdout, verb, items, label)
	stop := p.animate()
	sem := make(chan struct{}, maxParallel)
	var wg sync.WaitGroup
	for i, item := range items {
		wg.Go(func() {
			sem <- struct{}{}
			defer func() { <-sem }()
			p.start(i)
			msg, err := fn(item, func(s string) { p.report(i, s) })
			p.finish(i, msg, err)
		})
	}
	wg.Wait()
	stop()
	return p.done()
}
