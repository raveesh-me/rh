# rh

Small, stateless, config-free CLI for workspace chores. One static binary — templates are compiled in.

```sh
go install github.com/raveesh-me/rh@latest   # more options: INSTALL.md
```

| Command | What it does |
|---|---|
| `rh init` | Creates or updates `CLAUDE.md`, `AGENTS.md` and the worktree `Makefile` to the templates built into this binary (`templates/`), plus `workdir/`. These files are owned by rh — hand edits are overwritten. Run `rh update` first to get the newest templates. |
| `rh add <url> [url...]` | Clones every repo into the current folder in parallel. Repeated URLs are deduped; folders that already exist are skipped. |
| `rh update` | Replaces the running `rh` binary with the latest GitHub release (checksum-verified, atomic swap). Use `sudo rh update` if it lives in a root-owned dir. |
| `rh pull` | Runs `make pull` if the Makefile has that target. Otherwise: for the current repo, or every direct child repo, checks out the default branch (`origin/HEAD`, else main/master) and `git pull --ff-only`. Dirty repos and worktrees are skipped. |
| `rh mon` | Live view of `.` and `workdir/` (tasks, their repos, and each worktree's branch). |
| `rh clean` | Removes every `workdir/<task>/<repo>` worktree via `git worktree remove` and the emptied task dirs, then prunes worktree metadata. Branches are kept; worktrees with uncommitted changes are kept unless `-f`. Runs `make clean` instead if the Makefile defines it. |
| `rh diff` | Live diff explorer — [diffnav](https://github.com/dlvhdr/diffnav) file tree + [delta](https://github.com/dandavison/delta) rendering, refreshed every second. Shows every git worktree below here (or just the one you're in) against its merge-base with `origin/<default>`: commits, uncommitted edits and untracked files, i.e. what its PR would contain, grouped as `workdir/<task>/<repo>/…`. Reference checkouts are skipped; worktrees added or removed while it runs appear and disappear. `rh diff --live` instead snapshots all text files below here (no git needed; skips `.git`, `node_modules`, binaries, >1MB) and shows changes since start. delta is embedded (darwin/arm64, linux/amd64, linux/arm64); elsewhere it must be on `PATH`. |
| `rh <anything>` | Not an rh command → `make <anything>`. |
| `rh -m <target>` | Always `make <target>`, even if it shadows an rh command. |
| `rh -c some words` | `claude -p "some words"`. |
