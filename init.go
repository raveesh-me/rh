package main

import (
	"bytes"
	"embed"
	"fmt"
	"os"
	"path/filepath"
	"text/template"
)

//go:embed templates
var templates embed.FS

// initFiles maps each file rh init creates to its compiled-in template.
// Raw templates are copied verbatim (the Makefile is full of $ and braces).
var initFiles = []struct {
	name, template string
	raw            bool
}{
	{"CLAUDE.md", "templates/claude.md", false},
	{"AGENTS.md", "templates/agents.md", false},
	{"Makefile", "templates/makefile", true},
}

func cmdInit(args []string) int {
	dir, err := os.Getwd()
	if err != nil {
		fmt.Fprintln(os.Stderr, "rh init:", err)
		return 1
	}
	failed := false
	for _, f := range initFiles {
		body, err := renderTemplate(f.template, filepath.Base(dir), f.raw)
		if err == nil {
			err = syncFile(filepath.Join(dir, f.name), body)
		}
		if err != nil {
			fmt.Fprintln(os.Stderr, "rh init:", err)
			failed = true
		}
	}
	if err := createDir(filepath.Join(dir, "workdir")); err != nil {
		fmt.Fprintln(os.Stderr, "rh init:", err)
		failed = true
	}
	if failed {
		return 1
	}
	return 0
}

func renderTemplate(path, project string, raw bool) ([]byte, error) {
	if raw {
		return templates.ReadFile(path)
	}
	tmpl, err := template.ParseFS(templates, path)
	if err != nil {
		return nil, fmt.Errorf("parse %s: %w", path, err)
	}
	var buf bytes.Buffer
	if err := tmpl.Execute(&buf, struct{ Project string }{project}); err != nil {
		return nil, err
	}
	return buf.Bytes(), nil
}

// syncFile makes path match body. A differing existing file is first copied
// to path.bak so local edits are never lost.
func syncFile(path string, body []byte) error {
	name := filepath.Base(path)
	old, err := os.ReadFile(path)
	switch {
	case os.IsNotExist(err):
		if err := os.WriteFile(path, body, 0o644); err != nil {
			return err
		}
		fmt.Printf("  create  %s\n", name)
		return nil
	case err != nil:
		return err
	case bytes.Equal(old, body):
		fmt.Printf("  ok      %s (up to date)\n", name)
		return nil
	}
	if err := os.WriteFile(path+".bak", old, 0o644); err != nil {
		return fmt.Errorf("back up %s: %w", name, err)
	}
	if err := os.WriteFile(path, body, 0o644); err != nil {
		return err
	}
	fmt.Printf("  update  %s (previous saved as %s.bak)\n", name, name)
	return nil
}

func createDir(path string) error {
	if info, err := os.Stat(path); err == nil {
		if !info.IsDir() {
			return fmt.Errorf("%s exists and is not a directory", path)
		}
		fmt.Printf("  ok      %s/ (exists)\n", filepath.Base(path))
		return nil
	}
	if err := os.Mkdir(path, 0o755); err != nil {
		return err
	}
	fmt.Printf("  create  %s/\n", filepath.Base(path))
	return nil
}
