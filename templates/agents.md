# {{.Project}} workspace — agent instructions

This directory is a multi-repo workspace. It is not itself a git repo.

```
{{.Project}}/
├── Makefile        # worktree tooling (run `make` for help)
├── <repo>/         # root clone — read-only mirror of origin/<default>
└── workdir/
    └── <task>/     # one folder per task / ticket / PR
        └── <repo>/ # worktree on branch <task>
```

## Rules

1. **Never edit, commit, or switch branches in a root repo** (`./<repo>`). Root repos stay clean on their default branch and are only fast-forwarded (`make sync` / `rh update`).
2. **All work happens in `workdir/<task>/<repo>`**, a git worktree created with `make new TASK=<task> REPOS=<repo>`. Run builds, tests, and git commands from inside that worktree.
3. One task folder = one unit of work. A task spanning repos gets one worktree per repo under the same task folder, all on the same branch name.
4. New branches are cut from the latest `origin/<default>`, never from the local root checkout.
5. Before starting work, run `make check`. If it fails, stop and report it — never discard changes in a dirty root repo.
6. Don't delete worktrees or branches with uncommitted or unpushed work without asking.

## Repos

<!-- One line per repo: what it is and what it's used for here. -->
