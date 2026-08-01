---
name: work-issue
description: Use when picking up a numbered issue from the gimme backlog, when asked to work on the next task, or when starting any change that will become a pull request on this repository.
---

# Work an issue

Four hands, in order:

| Who | Does |
|---|---|
| Claude | plans, writes the issue, writes Codex's prompt |
| Codex | implements, reviews its own diff |
| Claude | reviews second, with a reviewing sub-agent |
| Sonnet agent | runs the quality gate |

Codex never sees the planning conversation and cannot fetch the missing context itself — everything it needs must be **in the prompt**, copied out of the issue and `TODO.md` by the agent writing that prompt. And it cannot run the suite: that is why the gate is dispatched to an agent with an ordinary shell.

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

```bash
codex exec --full-auto "<self-contained prompt>"
```

**Never tell Codex to read the issue.** Its `gh` cannot authenticate: the token lives in the macOS keyring, not in `~/.config/gh/hosts.yml`, and `gh auth status` inside the sandbox reports *the token in default is invalid*. The failure surfaces as `error connecting to api.github.com`, which is misleading — `curl https://api.github.com` returns `200` from that same sandbox. Codex does not stop on the missing issue; it guesses, and the guess can look right.

So the prompt carries, in full text: the branch name, the scope, the files, what to leave alone, the failure modes to watch, and the decisions already taken — including the ones considered and rejected, so Codex does not re-open them. Reading the issue and assembling that is the planning agent's job.

Ask Codex to review its own work before returning: a first pass on its own diff against the stated scope.

**Do not ask it for the test suite.** The sandbox denies `bind`, so any test opening a listener panics before it does anything useful:

```
panic: httptest: failed to listen on a port:
       listen tcp6 [::1]:0: bind: operation not permitted
```

That hits `api/` and `internal/storage/` (both use `httptest.NewServer`) plus the Redis and listener tests. Pre-starting Garage does not help — the wall is `bind`, not Docker. `gofmt -l .` and `make build` are the only checks worth asking Codex for.

If the issue is too thin to write such a prompt, that is a planning defect. Fix the issue, don't fill the gap from memory.

If the task has no entry in `TODO.md` — an issue filed mid-flight, say — add one in the same branch. An issue that never reaches `TODO.md` is invisible to the next session.

**5. Review, then have the tests run**

Codex never exercised the change, so nothing is verified yet.

Review the diff yourself first — second pass, after Codex's own. Then hand it to a reviewing sub-agent: `code-reviewer` does not exist in every setup, `caveman:cavecrew-reviewer` does. Give it the issue context and the checks already run, so it verifies rather than repeats.

Then dispatch a **Sonnet agent** to run the gate. It has an ordinary shell, so Docker and `bind` are available to it:

```bash
gofmt -l .              # must output nothing
golangci-lint run ./...
make test               # starts and stops Garage
make build
```

Ask it for the raw output, not a verdict. Codex reported #70's failures as "sandbox network restrictions" and the real cause was `bind`; a summarised result is where a wrong diagnosis hides.

`golangci-lint` is not installed on every machine (`No such file or directory`). Report it as not run — never as passing. Installing it is #52's job, not the current task's.

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
| Who runs the suite | a Sonnet agent — not Codex, whose sandbox denies `bind` |
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
| "I'll tell Codex to go read the issue" | Its `gh` has no usable token. Paste the content into the prompt. |
| "Codex says the tests pass" | Its sandbox denies `bind`; the suite never ran. Dispatch a Sonnet agent. |
| "The agent says it's green" | Ask for the raw output. A summary is where a wrong diagnosis hides. |
| "I'll tag a release now that this landed" | Everything ships in one release at Phase 5. |

## Common mistakes

**Fixing code and leaving the fixture.** Some bugs are invisible to the current tests by construction. Change the fixture first and watch the existing suite go red.

**Updating `TODO.md` separately.** A doc-only commit after the fact is how the plan and the code drift apart.
