# rh

Small, stateless, config-free CLI for workspace chores. One static binary — templates are compiled in.

```sh
go install github.com/raveesh-me/rh@latest   # more options: INSTALL.md
```

| Command | What it does |
|---|---|
| `rh init` | Creates `CLAUDE.md`, `AGENTS.md` (from `templates/`, embedded at build time) and `workdir/`. Never overwrites. |
| `rh add <repo-url> [dir]` | `git clone` into the current folder. Non-URL args fall through to `make add` (e.g. `rh add TASK=x REPOS=y`). |
| `rh update` | Runs `make update` if the Makefile has that target. Otherwise: for the current repo, or every direct child repo, checks out the default branch (`origin/HEAD`, else main/master) and `git pull --ff-only`. Dirty repos and worktrees are skipped. |
| `rh mon` | Live view of `.` and `workdir/` (tasks, their repos, and each worktree's branch). |
| `rh diff` | Watches text files below here (skips `.git`, `node_modules`, binaries, >1MB) and prints colored unified diffs as they change. |
| `rh <anything>` | Not an rh command → `make <anything>`. |
| `rh -m <target>` | Always `make <target>`, even if it shadows an rh command. |
| `rh -c some words` | `claude -p "some words"`. |
