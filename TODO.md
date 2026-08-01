# TODO — Work Plan

Source of truth for scheduled work on gimme. `CLAUDE.md` points agents here.

Every task maps to a GitHub issue that holds the full detail: cause, evidence, proposed fix, tests. **This file carries the order, the groupings and the gates — read the issue before starting a task.**

---

## Workflow

Planning and implementation are split across two agents:

| Role | Who | Output |
|---|---|---|
| Plan | Claude | This file, the issues, the sequencing |
| Code | Codex | The implementation, from the issue + this file |

Codex does not have the planning conversation. If an issue is ambiguous, that is a planning defect — raise it rather than guessing.

## Rules

1. **One task at a time.** Pick a single task, finish it fully, stop. Do not batch.
2. **Respect the order.** Phases are sequenced for a reason; the rationale is stated per phase.
3. **Grouped tasks ship together.** Where two issues are marked as a pair, they share a function and a test table — splitting them means writing the tests twice.
4. **Every bug is proven by a failing test before it is fixed.** Mandatory, no exceptions in Phase 3:
   1. Write a test that reproduces the buggy behaviour.
   2. **Run it and watch it fail.** Paste the failure output into the PR or commit message.
   3. Apply the fix.
   4. Run it again and watch it pass.

   Writing the fix first and the test after is not the same thing: a test written against already-corrected code proves the code does what it does, not that the bug is gone. The red step is the only evidence the test actually exercises the defect.

   This is not theoretical here — see #45. Twenty-five green tests cover `content-service.go` today and none of them catches the version-resolution bug, because the mock fixture cannot express it. A test that passes before the fix is a test that tests nothing.

   The test artifact is not always a Go test — see each task for its form.
5. **Quality gate before proposing any commit** — all four must pass:

   ```bash
   gofmt -l .              # must output nothing
   golangci-lint run ./...
   make test               # unit + integration (starts/stops Garage)
   make build
   ```

6. **Code review after implementation.** Invoke the `code-reviewer` sub-agent and address its findings before proposing a commit.
7. **Never commit automatically.** Propose the message; the decision to commit is the maintainer's.
8. **Every change goes through a branch and a pull request.** `main` is protected — direct pushes are refused, and a pull request cannot merge until the `test` and `helm-test` checks are green. Branches must also be up to date with `main` before merging; if dependabot has landed a bump since you branched, update the branch.

   Admin accounts can bypass the checks. **Do not** — bypassing here means merging with a red CI, which is exactly what the gate exists to prevent.

   One branch per task, named after the issue it closes:

   ```text
   fix/58-gitignore
   ci/52-lint-gosec
   fix/42-43-zip-entries
   ```

   Paired tasks share a branch — they ship together, so they review together.

   Tick the task's box in this file **in the same pull request** as the change, so the plan and the code never drift.
9. **No release until Phase 5.** Everything merges to `main` as it lands; nothing is tagged until the end. Merging is not releasing — the `v3.0.0` tag is what makes the release, so there is no need for a long-lived integration branch.

## Status legend

`[ ]` not started `[~]` in progress `[x]` done

---

## Phase 1 — Immediate

One-line fixes, no dependencies, no behaviour change.

- [x] **#58 — `.gitignore` malformed pattern**
  `gimme.yml.worktrees/` is two patterns collapsed into one; neither `gimme.yml` nor `.worktrees/` is actually ignored. Real S3 credentials can be committed.
  *Files:* `.gitignore`
  *Prove it:* not unit-testable — this is the one Phase 1/3 exception to rule 4. Record the shell output instead: `git check-ignore -v gimme.yml` returns nothing before the fix, matches after.
  *Done when:* `git check-ignore gimme.yml` and `git check-ignore .worktrees/` both match.

- [x] **#70 — Canonical Go module path**
  Rename the module and its internal imports to `github.com/ziggornif/gimme`, matching the repository's canonical identity. Keep the path unversioned — **no `/v3` suffix**, even though Phase 5 tags a major: gimme is a server, nothing imports it, and releases ship as binaries and Docker images, so the module path is an internal identifier no proxy ever resolves.
  *Files:* `go.mod`, Go imports, coverage filters, `CONTRIBUTING.md`, `CLAUDE.md`
  *Watch:* the coverage filters in `Makefile` and `.github/workflows/build.yml` match the module path as a literal string. Miss one and it silently stops excluding `test/mocks` — nothing turns red.

- [x] **#73 — `make garage-start`: `grep -oP` is not portable, `NODE_ID` is empty on macOS**
  `Makefile:102` uses `grep -oP`, a GNU extension that BSD grep rejects. `NODE_ID` ends up empty and `garage layout assign` silently matches the only node by empty prefix. Replace it with `-oE` and add a non-empty guard.
  *Files:* `Makefile`
  *Independent of everything else.*

> **Optional pull-forward:** the version badge in #63 says `v1` while the latest release is v2.0.9 — publicly wrong right now, one line in `docs/site/index.html`. Fix it here if convenient; the rest of #63 stays in Phase 4.

---

## Phase 2 — CI

Before touching application code, so the lint inventory is known in advance.

- [x] **#52 — `golangci-lint` + `gosec` in CI** ⚠️ *ship together with #74*
  Create `.golangci.yml` (none exists) and add a blocking lint job. The inventory is clean: the prediction that `gosec` would flag the upload and file-handling paths touched in Phase 3 was measured and is false — there is no `gosec` finding in `content-service.go` or anywhere on the upload path. ⚠️ **Also add `lint` to the required status checks** on the `main` branch protection — otherwise the job runs and blocks nothing.
  *Files:* `.golangci.yml`, `.github/workflows/build.yml`
  *Careful:* required check names are coupled to job names in the workflow. Renaming a job without updating the branch protection leaves every pull request waiting on a check that will never report.

- [x] **#74 — CI path filtering + Markdown lint** ⚠️ *ship together with #52*
  Run each CI job only when the file types it covers change, and lint Markdown when documentation changes. Keep the workflow triggers unchanged and gate jobs from a path-filtering job: a required job that never reports leaves pull requests waiting forever, so `paths-ignore` must not be used.
  *Files:* `.github/workflows/build.yml`, `.markdownlint-cli2.jsonc`, Markdown files
  *Prove it:* the CI run itself — none of this is observable locally. A branch touching only a `.md` file must show `test` and `helm-test` as **Skipped** and stay mergeable; that is where the required-check deadlock would appear.

- [x] **#77 — CI plumbing: duplicate pipelines, no concurrency, `helm-test` on Markdown**
  Three defects that together doubled the CI bill. `push` and `pull_request` both listened on `**`, so every commit on a branch with an open pull request ran two complete pipelines on the same SHA. No `concurrency` group, so successive pushes ran in parallel instead of superseding each other. And the `helm` path filter matched `scripts/helm/**/*.md`, firing `helm-test` on documentation-only changes.
  *Files:* `.github/workflows/build.yml`
  *Prove it:* CI behaviour, not observable locally. One run per push; a second push cancels the first; a pull request touching only `scripts/helm/**/*.md` shows `helm-test` as **Skipped** while one touching a template still runs it.

- [x] **#81 — Drop UPX from the Docker build**
  `upx --fast` made the pull heavier and doubled container startup. Measured: compressed image 18.9 MB → 17.1 MB, startup 740 ms → 372 ms. A registry transfers layers gzipped, and an uncompressed Go binary gzips far better than an already-packed one — so UPX only won on uncompressed local size, which nobody pays for. Published binaries were never affected: `go-release-action` compiles them itself.
  *Files:* `Dockerfile`, `Makefile`
  *Prove it:* shell output — binary size inside the image, `docker save | gzip | wc -c`, and startup timing.

> **#53 — Multi-arch Docker image: closed, not scheduled.** No one has asked for an arm64 image, and the `linux/arm64` binary already covers the rare case. The published image is `linux/amd64` only — verified: the registry serves a plain `manifest.v2`, not a manifest list. Reopen if someone actually deploys on ARM; the `Dockerfile` already honours `TARGETOS`/`TARGETARCH`, so it is two `docker build` steps away.

---

## Phase 3 — Bugs

**Do these while there is no known production usage.** Four of them change behaviour in ways that would need a deprecation path once real deployments exist.

- [x] **#44 — `errgroup` has no concurrency limit**
  One line: `eg.SetLimit(...)`. Land it first — it clears the upload path before the larger changes.
  *Files:* `internal/content/content-service.go`
  *Prove it:* the hardest one to red-test. Instrument the mock's `AddObject` with an atomic counter tracking peak concurrency, upload an archive with many entries, assert the peak stays at or below the limit. Before the fix the peak equals the entry count; after, it is capped.

- [ ] **#42 + #43 — ZIP entry handling** ⚠️ *ship together*
  Same function, same validation. The rewrite regex replaces the **first path segment, whatever it is**, which is correct only for archives shaped exactly `<one-root-folder>/...`.
  **This is the most severe defect in the backlog** — reproduced end-to-end against `ziggornif/gimme:latest`, see the issue. Four symptoms, worst last:
  1. root-level files escape the package namespace and become permanent orphans `DeletePackage` cannot reach
  2. subdirectories are flattened — the file exists, but not at the URL anyone wrote
  3. two top-level folders holding the same filename collapse onto one key: **silent data loss**, not a 404
  4. which of the two wins **varies between uploads of the same archive**
  #43 is the same code path: entry names not starting with `[a-zA-Z0-9-_]` are not rewritten at all and escape the namespace.
  *Files:* `internal/content/content-service.go`, `internal/content/content-service_test.go`
  *Prove it:* Go tests. Build fixture archives in-test — one with a root folder, one with files at the root, one with several top-level folders, one with the same filename in two folders, one with entries named `../x`, `/x`, `.hidden/x`. Assert the resulting object keys. Before the fix, `app.js` yields `awesome-lib@1.0.0.js`, `img/logo.svg` yields `<pkg>@<version>/logo.svg`, and `../../evil.js` passes through untouched.
  *Approach:* detect a common root and strip only that; otherwise **preserve the full internal path**. Assert every resulting key starts with `<pkg>@<version>/` and reject the archive otherwise. Detect duplicate target keys **before uploading anything** and reject, naming the colliding entries — never overwrite silently.
  *Note:* symptom 4 most likely comes from the unbounded `errgroup` in #44, which lands first. Fixing #44 makes the collision deterministic; only this task makes it impossible.

- [ ] **#45 + #46 — Version and filename resolution** ⚠️ *ship together*
  Same function, same test table. `pkg@1` currently resolves to `10.0.0`; `/app.js` also matches `app.js.map`.
  *Files:* `internal/content/content-service.go`, `internal/content/content-service_test.go`, **`test/mocks/objectstorage-manager.go`**
  ⚠️ **The mock fixture must change or the tests stay green and prove nothing.** It holds only `1.0.0`/`1.1.0`/`1.1.1` — no two-digit components, one major — and filters with `Contains` instead of prefix matching, reproducing the bug it should catch.
  *Prove it:* **fixture first.** Add `1.9.9`, `10.0.0`, `1.10.0` and switch the mock to prefix filtering, then run the existing suite — `TestContentService_GetMajorFile` should now fail, which is the red step. Only then write the component-wise comparison. If the suite still passes after the fixture change, the fixture change was wrong.

> **Placeholder policy — applies to #59 and #60.** Demonstration stacks must start; a real installation must not run with a published secret.
>
> | Config | Role | Secret placeholder |
> |---|---|---|
> | `with-garage` | demo, runs as-is | passes — no change |
> | `with-managed-s3` | demo, user supplies S3 only | passes (#60) |
> | `gimme.example.yml` | real install (release + from source) | **must fail** (#59) |
>
> No application code either way. The existing 32-byte check does the work — the placeholder value decides the outcome.

- [ ] **#59 — `gimme.example.yml` placeholder secret passes validation**
  The placeholder says "at least 32 chars" and is 50 characters long, so it satisfies its own instruction and forces nothing. A user who edits only the S3 block runs with a secret published in this repo — which derives the token-file AES key and the OIDC session signing key.
  *Files:* `gimme.example.yml` — **no application code**
  *Fix:* short failing value, guidance moved to a comment: `# Required. Generate one with: openssl rand -hex 32` / `secret: "CHANGEME"`. Consider the same for `admin.user`/`admin.password`.
  *Prove it:* Go test — `NewConfig()` on `gimme.example.yml` must be **rejected**. It is accepted today; that is the red step.
  *Not breaking:* changes a shipped template, not the behaviour of a running instance.

- [ ] **#60 — Compose example `with-managed-s3` cannot start**
  `secret: secret` (6 chars) fails the 32-byte minimum, so the stack dies on a field the example never asks the user to fill. Align to the sibling `with-garage` value, which passes.
  *Files:* `examples/deployment/docker-compose/with-managed-s3/gimme.yml`
  *Prove it:* Go test — `NewConfig()` on the example file must be **accepted** as far as the secret goes. Red output today: `secret must be at least 32 bytes long (got 6)`.
  *Independent of #59* — opposite directions, no ordering constraint.

---

## Phase 4 — Features

- [ ] **#64 — `validateConfig` reports all invalid fields at once**
  Today it returns on the first, so first-run setup is a chain of restart-and-discover cycles. Gets worse once #59 makes the secret the first error every new user hits. Do this early in the phase — it is what makes the tightened placeholder policy pleasant instead of tedious.
  *Files:* `configs/config.go`, `configs/config_test.go`

- [ ] **#61 — Environment-only configuration**
  A missing config file is currently fatal even when every value is set via `GIMME_*`. Pairs naturally with #64 — same file, same first-run concern.

- [ ] **#62 — Upload limits** (size, entry count, decompressed size)
  Needs a new `PayloadTooLarge` kind in `internal/errors/business-error.go`.

- [ ] **#48 — ETag / `If-None-Match` → 304**
  Do before #47: smaller and self-contained, and #47 then extends it per encoding variant.

- [ ] **#47 — Serve brotli/gzip**
  Pre-compress at upload, negotiate on `Accept-Encoding`. Touches `CreatePackage`, so it must come after #42/#43.

- [ ] **#50 — SRI integrity hashes**
  Same upload hashing pass as #47 — do it right after, or together.

- [ ] **#49 — Browse: `GET /packages` and version listing**
  Independent of everything else. Good candidate if a visible win is wanted early.

- [ ] **#51 — `@latest`**
  Depends on #45: it shares the resolution path.

- [ ] **#55 — GitHub Action + upload CLI**
  Depends on #42: the archive layout must be settled first. The tool building the ZIP is what structurally prevents #42 from recurring.

- [x] **#57 — Helm chart README**
  The chart itself is correct — templates, `values.yaml` and the `emptyDir` are all right. Only the README is wrong: it claims horizontal scaling is safe without qualifying that a shared token store is required, its HPA example leaves `mode: file`, its options table has drifted from `values.yaml` (missing `postgres` and `pgUrl`), and file-mode ephemerality is stated nowhere.
  *Files:* `scripts/helm/gimme/README.md` — **documentation only, no template change**
  *Note:* the token store is the *only* thing blocking horizontal scaling. OIDC sessions already scale — the signing key is derived deterministically from the shared `GIMME_SECRET`, so any pod validates any pod's cookie.

- [ ] **#65 — Helm: guard against `file` + multiple replicas** *(after #57)*
  `fail` in the chart when `mode: file` meets `replicaCount > 1` or `hpa.enabled`, so the unworkable combination cannot be rendered. Land after #57 so the guard message and the README agree.
  *Files:* `scripts/helm/gimme/templates/_helpers.tpl`, helm-unittest tests

- [ ] **#56 + #63 — README and docs site narrative** ⚠️ *one pass*
  Both share an angle: lead with the use case, not the definition. #63 also carries the `v1` badge fix and the missing ZIP layout instruction.

> **Reference sections drift as features land.** Update the relevant docs-site section **in the PR of the feature itself**, not in a separate documentation pass. Affected: Semver resolution (#45, #51), Caching (#47, #48), API Reference (#49), Quickstart (#55).

---

## Phase 5 — Release

Single release covering everything above.

**Target: v3.0.0.**

Two items change behaviour in breaking ways:

| Issue | Breaks on upgrade |
|---|---|
| #43 | Archives that previously uploaded are now rejected (entries starting with `.`) |
| #45 | `pkg@1` serves different content than before |

Not breaking, despite touching sensitive ground: #59 and #60 change shipped template files rather than the behaviour of a running instance; #57 is documentation. #65 makes an unworkable Helm configuration fail to render, which only affects deployments that were already misbehaving.

A major bump is consistent with this project's own precedent: v2 was the major for replacing JWT tokens with opaque tokens — the same kind of breaking change.

Release notes are auto-generated from PR titles in GitHub Releases. **Add a hand-written preamble for this one** listing the three breaking changes and their upgrade actions — PR titles alone will not tell an operator that their `helm upgrade` will fail, that an archive their pipeline has always uploaded is now rejected, or that a `@1` URL now serves different content.

---

## Open decisions

*None open.*

Project identity was settled in #70: repository, Go module and Docker image are all `ziggornif/gimme`, and the module path deliberately carries no major-version suffix.
