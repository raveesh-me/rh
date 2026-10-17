# Worktree workspace
#
# Repos in this directory stay clean on their default branch, tracking origin.
# All work happens in git worktrees under workdir/<task>/<repo>.
#
# Compatible with GNU Make 3.81 (macOS default): no .ONESHELL, recipes use
# explicit line continuations.

SHELL   := /bin/bash
ROOT    := $(abspath $(dir $(lastword $(MAKEFILE_LIST))))
WORKDIR := $(ROOT)/workdir

# Every top-level directory that is a git repo (workdir excluded).
ALL_REPOS := $(sort $(filter-out workdir,$(patsubst %/.git,%,$(wildcard */.git))))

TASK   ?=
REPOS  ?= $(ALL_REPOS)
BRANCH ?= $(TASK)
BASE   ?=

# Resolve a repo's default branch from origin/HEAD, falling back to main.
default_branch = $$(git -C "$(ROOT)/$(1)" symbolic-ref --quiet --short refs/remotes/origin/HEAD 2>/dev/null | sed 's@^origin/@@' || true)

.DEFAULT_GOAL := help
.PHONY: help repos sync status check new rm list tasks rebase prune

help: ## Show targets
	@echo "Worktree workspace — repos stay on latest, work lives in workdir/<task>/<repo>"
	@echo
	@grep -E '^[a-z-]+:.*## ' $(MAKEFILE_LIST) | awk -F':.*## ' '{printf "  make %-8s %s\n", $$1, $$2}'
	@echo
	@echo "Variables: TASK=<name> REPOS=\"<repo> ...\" BRANCH=<branch, default TASK> BASE=<base branch, default repo default>"
	@echo "Repos:     $(ALL_REPOS)"

repos: ## List detected repos and their default branch
	@for r in $(ALL_REPOS); do \
	  d=$(call default_branch,$$r); echo "$$r (default: $${d:-main})"; \
	done

sync: ## Fetch all repos and fast-forward each to origin/<default>
	@set -e; for r in $(ALL_REPOS); do \
	  d=$(call default_branch,$$r); d=$${d:-main}; \
	  echo "==> $$r ($$d)"; \
	  git -C "$(ROOT)/$$r" fetch --prune --quiet origin; \
	  if [ -n "$$(git -C "$(ROOT)/$$r" status --porcelain)" ]; then \
	    echo "    SKIP: uncommitted changes in root repo — move them to a worktree"; continue; \
	  fi; \
	  cur=$$(git -C "$(ROOT)/$$r" symbolic-ref --quiet --short HEAD || echo DETACHED); \
	  if [ "$$cur" != "$$d" ]; then \
	    echo "    switching $$cur -> $$d"; git -C "$(ROOT)/$$r" switch --quiet "$$d"; \
	  fi; \
	  git -C "$(ROOT)/$$r" merge --ff-only --quiet "origin/$$d" \
	    || echo "    WARN: $$d has diverged from origin/$$d — fix manually"; \
	  git -C "$(ROOT)/$$r" worktree prune; \
	done

status: ## Show root repo state and all worktrees
	@for r in $(ALL_REPOS); do \
	  d=$(call default_branch,$$r); d=$${d:-main}; \
	  cur=$$(git -C "$(ROOT)/$$r" symbolic-ref --quiet --short HEAD || echo DETACHED); \
	  dirty=$$([ -n "$$(git -C "$(ROOT)/$$r" status --porcelain)" ] && echo " DIRTY" || true); \
	  behind=$$(git -C "$(ROOT)/$$r" rev-list --count HEAD..origin/$$d 2>/dev/null || echo "?"); \
	  echo "==> $$r [$$cur$$dirty, $$behind behind origin/$$d]"; \
	  git -C "$(ROOT)/$$r" worktree list | tail -n +2 | sed 's/^/    /'; \
	done

check: ## Fail if any root repo is dirty or off its default branch
	@fail=0; for r in $(ALL_REPOS); do \
	  d=$(call default_branch,$$r); d=$${d:-main}; \
	  cur=$$(git -C "$(ROOT)/$$r" symbolic-ref --quiet --short HEAD || echo DETACHED); \
	  if [ "$$cur" != "$$d" ]; then echo "FAIL: $$r is on $$cur, expected $$d"; fail=1; fi; \
	  if [ -n "$$(git -C "$(ROOT)/$$r" status --porcelain)" ]; then echo "FAIL: $$r has uncommitted changes"; fail=1; fi; \
	done; [ $$fail -eq 0 ] && echo "OK: all root repos clean on default branch"; exit $$fail

new: _require-task ## Create worktrees: make new TASK=x [REPOS="a b"] [BRANCH=y] [BASE=z]
	@set -e; for r in $(REPOS); do \
	  [ -d "$(ROOT)/$$r/.git" ] || { echo "ERROR: $$r is not a repo here"; exit 1; }; \
	  path="$(WORKDIR)/$(TASK)/$$r"; \
	  if [ -e "$$path" ]; then echo "==> $$r: exists at $$path"; continue; fi; \
	  d=$(call default_branch,$$r); base="$(BASE)"; base=$${base:-$${d:-main}}; \
	  git -C "$(ROOT)/$$r" fetch --prune --quiet origin; \
	  mkdir -p "$(WORKDIR)/$(TASK)"; \
	  if git -C "$(ROOT)/$$r" show-ref --verify --quiet "refs/heads/$(BRANCH)"; then \
	    echo "==> $$r: checking out existing local branch $(BRANCH)"; \
	    git -C "$(ROOT)/$$r" worktree add --quiet "$$path" "$(BRANCH)"; \
	  elif git -C "$(ROOT)/$$r" show-ref --verify --quiet "refs/remotes/origin/$(BRANCH)"; then \
	    echo "==> $$r: tracking origin/$(BRANCH)"; \
	    git -C "$(ROOT)/$$r" worktree add --quiet --track -b "$(BRANCH)" "$$path" "origin/$(BRANCH)"; \
	  else \
	    if git -C "$(ROOT)/$$r" show-ref --verify --quiet "refs/remotes/origin/$$base"; then from="origin/$$base"; \
	    elif git -C "$(ROOT)/$$r" show-ref --verify --quiet "refs/heads/$$base"; then from="$$base"; \
	    else echo "ERROR: $$r: base $$base not found locally or on origin"; exit 1; fi; \
	    echo "==> $$r: new branch $(BRANCH) from $$from"; \
	    git -C "$(ROOT)/$$r" worktree add --quiet --no-track -b "$(BRANCH)" "$$path" "$$from"; \
	  fi; \
	  echo "    $$path"; \
	done


rm: _require-task ## Remove a task's worktrees (keeps branches): make rm TASK=x [REPOS="a"] [FORCE=1]
	@set -e; for r in $(REPOS); do \
	  path="$(WORKDIR)/$(TASK)/$$r"; \
	  [ -e "$$path" ] || continue; \
	  echo "==> removing $$path"; \
	  git -C "$(ROOT)/$$r" worktree remove $(if $(FORCE),--force) "$$path"; \
	done; \
	rmdir "$(WORKDIR)/$(TASK)" 2>/dev/null && echo "==> removed task $(TASK)" || true

list: ## List all worktrees per repo
	@for r in $(ALL_REPOS); do echo "==> $$r"; git -C "$(ROOT)/$$r" worktree list | sed 's/^/    /'; done

tasks: ## List tasks in workdir with repos and branches
	@shopt -s nullglob; for t in "$(WORKDIR)"/*/; do \
	  echo "$$(basename "$$t")"; \
	  for w in "$$t"*/; do \
	    b=$$(git -C "$$w" symbolic-ref --quiet --short HEAD || echo DETACHED); \
	    dirty=$$([ -n "$$(git -C "$$w" status --porcelain)" ] && echo " *" || true); \
	    echo "    $$(basename "$$w") [$$b$$dirty]"; \
	  done; \
	done

rebase: _require-task ## Rebase a task's worktrees onto latest origin/<base>: make rebase TASK=x
	@set -e; for r in $(REPOS); do \
	  path="$(WORKDIR)/$(TASK)/$$r"; [ -e "$$path" ] || continue; \
	  d=$(call default_branch,$$r); base="$(BASE)"; base=$${base:-$${d:-main}}; \
	  git -C "$$path" fetch --prune --quiet origin; \
	  if git -C "$$path" show-ref --verify --quiet "refs/remotes/origin/$$base"; then onto="origin/$$base"; else onto="$$base"; fi; \
	  echo "==> $$r onto $$onto"; \
	  git -C "$$path" rebase "$$onto"; \
	done

prune: ## Prune stale worktree metadata in all repos
	@for r in $(ALL_REPOS); do git -C "$(ROOT)/$$r" worktree prune -v; done

.PHONY: _require-task
_require-task:
	@[ -n "$(TASK)" ] || { echo "ERROR: TASK is required, e.g. make $(MAKECMDGOALS) TASK=my-feature"; exit 1; }
	@case "$(TASK)" in */*|.*|*' '*) echo "ERROR: TASK must be a single path segment"; exit 1;; esac
