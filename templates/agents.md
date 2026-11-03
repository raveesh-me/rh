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

## This workspace is an isolated environment

The workspace root is the directory that contains this file. Treat it as a sealed sandbox: everything you need is inside it, and nothing outside it exists for you. Every rh workspace is isolated in the same way, including any others on this machine.

- **Your boundary is the workspace root and its children.** You may read, search, create, and edit only paths inside it. Before you touch any path, resolve it to an absolute path. If it does not start with the workspace root, don't touch it.
- **Never go up or sideways.** Don't read, search, list, or edit the parent directory, sibling directories (including other workspaces or projects next to this one), your home directory, or any other location on the machine. `../` is fine only when the resolved path stays inside the workspace, such as `cd ../../..` from a worktree back to the workspace root.
- **Ignore copies of repos that live outside.** If the same repo is already cloned somewhere else on the machine, act as if it isn't there. Don't read it, run commands in it, add worktrees from it, symlink to it, or copy files out of it.
- **Need a repo that isn't here? Clone it fresh into this workspace.** From the workspace root, run `git clone <url>` so it lands at `./<repo>/`. Then run `make new TASK=<task> REPOS=<repo>` and work in `workdir/<task>/<repo>/`. The new clone is a root repo: read-only, like the others.
- **Keep scratch files inside too.** Put notes, logs, and temporary files in `workdir/<task>/`, next to the worktrees and outside them. Don't use `/tmp` or other outside directories.
- **Toolchain side effects are fine. Direct edits are not.** Normal commands may read and write their own caches and config (Go module cache, npm cache, git and gh credentials). Never edit those locations yourself.
- **Subagents inherit this boundary.** When you delegate, give the subagent the workspace root's absolute path and these rules.
- **If a task seems to need anything outside the workspace, stop and ask.** Don't find a way around the boundary.

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
git clone <url>                             # only if <repo> isn't here yet
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
