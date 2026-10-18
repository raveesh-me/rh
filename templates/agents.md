<!-- Managed by rh. Do not edit: `rh init` overwrites this file with the latest template. -->
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

1. **Every change ships as a pull request from a worktree. No exceptions.** Never commit to, push to, or merge into a default branch directly — not for one-line fixes, docs, or config. If a change can't go through a PR, stop and ask.
2. **Never edit, commit, or switch branches in a root repo** (`./<repo>`). Root repos stay clean on their default branch and are only fast-forwarded (`make sync` / `rh pull`).
3. **All work happens in `workdir/<task>/<repo>`**, a git worktree created with `make new TASK=<task> REPOS=<repo>`. Run builds, tests, and git commands from inside that worktree.
4. One task folder = one unit of work = one PR per repo. A task spanning repos gets one worktree (and one PR) per repo under the same task folder, all on the same branch name.
5. New branches are cut from the latest `origin/<default>`, never from the local root checkout.
6. Before starting work, run `make check`. If it fails, stop and report it — never discard changes in a dirty root repo.
7. Don't delete worktrees or branches with uncommitted or unpushed work without asking.

## Workflow for every change

```sh
make check                                  # root repos clean?
make new TASK=<task> REPOS=<repo>           # worktree on branch <task>
cd workdir/<task>/<repo>                    # do all work here
# ...edit, build, test...
git add -A && git commit -m "<type>: <what>"
git push -u origin <task>
gh pr create --fill                         # PR into the default branch
# after the PR merges:
cd ../../.. && make rm TASK=<task> && rh pull
```

A task isn't done until its PR is open and the link has been shared.
