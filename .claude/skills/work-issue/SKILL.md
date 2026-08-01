---
name: work-issue
description: Use when picking up a numbered issue from the gimme backlog, when asked to work on the next task, or when starting any change that will become a pull request on this repository.
---

# Work an issue

Claude plans, Codex implements. Codex never sees the planning conversation — everything it needs lives in the issue and in `TODO.md`.

## Order of operations

**1. Locate the task**

Read `TODO.md` first, not the issue. It holds the phase order, the pairings and the gates.

```bash
gh issue view <N>
```

Stop and ask if: the task sits in a later phase than unfinished work, or `TODO.md` marks it blocked under *Open decisions*.

**2. One branch, named after the issue**

```bash
git checkout main && git pull
git checkout -b fix/58-gitignore     # or ci/…, docs/…
```

Paired tasks (`⚠️ ship together` in `TODO.md`) share one branch. They share a function and a test table — splitting them means writing the tests twice.

**3. Bugs: failing test BEFORE the fix**

Write the test, run it, **watch it fail**, keep the output. Then fix. Then watch it pass.

A test written after the fix proves the code does what it does, not that the defect is gone.

The artifact is not always a Go test: `helm-unittest` for chart bugs, a recorded shell assertion where no unit test is possible. Each task in `TODO.md` names its form under *Prove it*.

**4. Delegate the implementation to Codex**

Hand over the issue number and the branch. If Codex has to ask what the change is, the issue is underspecified — fix the issue, don't answer from memory.

**5. Verify before proposing anything**

```bash
gofmt -l .              # must output nothing
golangci-lint run ./...
make test               # starts and stops Garage
make build
```

Then invoke the `code-reviewer` sub-agent and address what it finds.

**6. Pull request**

Tick the task's box in `TODO.md` **in the same PR** as the change.

PR body carries the failing-test output from step 3. `main` requires `test` and `helm-test` green.

Propose the commit message; the maintainer decides when to commit.

## Quick reference

| | |
|---|---|
| Task order, pairings, gates | `TODO.md` |
| Cause, evidence, fix, tests | the GitHub issue |
| Branch | `<type>/<issue>-<slug>`, off fresh `main` |
| Merge gate | `test` + `helm-test` green |
| Release | Phase 5 only — no tags before |

## Red flags

Stop if you catch yourself thinking:

| Thought | Reality |
|---|---|
| "I'll do these two issues together, they're small" | One task at a time. Finish, stop. |
| "I'll write the test after, it's the same test" | Then you never saw it fail. It proves nothing. |
| "Tests still pass, the fix must be fine" | For #45 the fixture cannot express the bug. Green means untested, not fixed. |
| "CI is red but it's unrelated" | Do not merge red. Do not use the admin bypass. |
| "I'll push straight to main, it's one line" | `main` is protected. Branch and PR, every time. |
| "The issue is vague, I'll infer the intent" | Underspecified issue = planning defect. Raise it. |
| "I'll tag a release now that this landed" | Everything ships in one release at Phase 5. |

## Common mistakes

**Fixing code and leaving the fixture.** Some bugs are invisible to the current tests by construction. Change the fixture first and watch the existing suite go red.

**Updating `TODO.md` separately.** A doc-only commit after the fact is how the plan and the code drift apart.
