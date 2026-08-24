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

- [x] **#115 — `make test` is not reproducible: `garage-start` assumes virgin metadata** *(filed mid-flight, during #47)*
  `garage-start` bind-mounted `/tmp/garage/{meta,data}` and then hardcoded `garage layout apply --version 1`, which is only correct against a virgin metadata directory. When the directory survives a previous run the layout is already at version 1, `layout assign` stages version 2, and the hardcoded apply is refused: `Error: Internal error: Invalid new layout version`. Same cause, second symptom: the objects of one run survived into the next, so an interrupted suite left packages in the bucket and the next upload of the same `name@version` was answered `409` by the duplicate-package check.
  *Files:* `Makefile` — **build tooling only, no Go code**
  *Prove it:* shell output. **The issue's own repro was stale** — `ca12632` had since added a `/tmp/garage` cleanup to `garage-stop`, so two *complete* `make test` runs pass on `main` and prove nothing. Corrected in a comment on the issue: the state only survives when `garage-stop` is not reached, so the red step must leave it behind first — `make garage-start` twice in a row, or `make garage-start` followed by `make test`.
  *Fix:* `--tmpfs` for `meta` and `data` instead of bind mounts, so every start gets virgin metadata and `--version 1` is correct **by construction** rather than by luck. `garage.toml` stays a bind mount — the recipe regenerates it on each start.
  *Rejected:* reading the next layout version from `garage layout show`. Fixes the layout error only, leaves the object pollution, and adds shell parsing for it.
  *Two follow-ons in the same change:* `garage-stop`'s `alpine` round-trip existed only to delete root-owned files Docker had written into the bind mount — with tmpfs there are none, so a plain `rm -f /tmp/garage.toml` replaces it. And a second readiness wait on `:3900` was added after the bucket is created: the existing loop waits on `garage status` (RPC, `:3901`), which is necessary since `NODE_ID` is read from it, but nothing waited on the port the suite actually calls. It also catches a stale Docker Desktop port forwarding, which has cost a debugging session before.
  *One-off on existing checkouts:* a `/tmp/garage` left by the old Makefile is root-owned and is never touched again. Remove it once with `docker run --rm -v /tmp:/tmp alpine rm -rf /tmp/garage`. Deliberately not automated — migration code in a Makefile outlives its purpose.

> **#53 — Multi-arch Docker image: closed, not scheduled.** No one has asked for an arm64 image, and the `linux/arm64` binary already covers the rare case. The published image is `linux/amd64` only — verified: the registry serves a plain `manifest.v2`, not a manifest list. Reopen if someone actually deploys on ARM; the `Dockerfile` already honours `TARGETOS`/`TARGETARCH`, so it is two `docker build` steps away.

---

## Phase 3 — Bugs

**Do these while there is no known production usage.** Four of them change behaviour in ways that would need a deprecation path once real deployments exist.

- [x] **#44 — `errgroup` has no concurrency limit**
  One line: `eg.SetLimit(...)`. Land it first — it clears the upload path before the larger changes.
  *Files:* `internal/content/content-service.go`
  *Prove it:* the hardest one to red-test. Instrument the mock's `AddObject` with an atomic counter tracking peak concurrency, upload an archive with many entries, assert the peak stays at or below the limit. Before the fix the peak equals the entry count; after, it is capped.

- [x] **#42 + #43 — ZIP entry handling** ⚠️ *ship together*
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
  *Settled while implementing:* dot-prefixed segments (`.well-known/probe.txt`, `.hidden/x.js`) are **accepted and namespaced**, not rejected — they are legitimate asset paths, and the whitelist rule (key must start with `<pkg>@<version>/`) already contains them. Rejection is reserved for empty names, absolute paths, `..` escapes and duplicate target keys, and it applies to the **whole archive** — never a silently skipped entry.

- [x] **#45 + #46 — Version and filename resolution** ⚠️ *ship together*
  Same function, same test table. `pkg@1` currently resolves to `10.0.0`; `/app.js` also matches `app.js.map`.
  *Files:* `internal/content/content-service.go`, `internal/content/content-service_test.go`, **`test/mocks/objectstorage-manager.go`**
  ⚠️ **The mock fixture must change or the tests stay green and prove nothing.** It holds only `1.0.0`/`1.1.0`/`1.1.1` — no two-digit components, one major — and filters with `Contains` instead of prefix matching, reproducing the bug it should catch.
  *Prove it:* **fixture first.** Add `1.9.9`, `10.0.0`, `1.10.0` and switch the mock to prefix filtering, then run the existing suite. Only then write the component-wise comparison.
  ⚠️ *Correction, measured 2026-08-03 — the fixture change alone does **not** turn the suite red.* It was tried (added `1.9.9`, `1.10.0`, `1.11.0`, `10.0.0`, swapped `Contains` for `HasPrefix`) and everything stayed green. `TestContentService_GetMajorFile` asserts only `NotNil(file)` / `Nil(err)`, and `MockOSManager.GetObject` returns a `&minio.Object{}` for **any** path — so `@1` resolving to `10.0.0` raises nothing. The red step therefore needs a **third change**: a mock that records the resolved key, plus assertions on that key (`@1` → `test@1.11.0/test.js`, not `test@10.0.0/test.js`; `test.js` must not match `test.js.map`). Fixture + prefix filtering are necessary, not sufficient.

> **Placeholder policy — applies to #59 and #60.** Demonstration stacks must start; a real installation must not run with a published secret.
>
> | Config | Role | Secret placeholder |
> |---|---|---|
> | `with-garage` | demo, runs as-is | passes — no change |
> | `with-managed-s3` | demo, user supplies S3 only | passes (#60) |
> | `gimme.example.yml` | real install (release + from source) | **must fail** (#59) |
>
> No application code either way. The existing 32-byte check does the work — the placeholder value decides the outcome.

- [x] **#59 — `gimme.example.yml` placeholder secret passes validation**
  The placeholder says "at least 32 chars" and is 50 characters long, so it satisfies its own instruction and forces nothing. A user who edits only the S3 block runs with a secret published in this repo — which derives the token-file AES key and the OIDC session signing key.
  *Files:* `gimme.example.yml` — **no application code**
  *Fix:* short failing value, guidance moved to a comment: `# Required. Generate one with: openssl rand -hex 32` / `secret: "CHANGEME"`. Consider the same for `admin.user`/`admin.password`.
  *Prove it:* Go test — `NewConfig()` on `gimme.example.yml` must be **rejected**. It is accepted today; that is the red step.
  *Not breaking:* changes a shipped template, not the behaviour of a running instance.

- [x] **#60 — Compose example `with-managed-s3` cannot start**
  `secret: secret` (6 chars) fails the 32-byte minimum, so the stack dies on a field the example never asks the user to fill. Align to the sibling `with-garage` value, which passes.
  *Files:* `examples/deployment/docker-compose/with-managed-s3/gimme.yml`
  *Prove it:* Go test — `NewConfig()` on the example file must be **accepted** as far as the secret goes. Red output today: `secret must be at least 32 bytes long (got 6)`.
  *Independent of #59* — opposite directions, no ordering constraint.

---

## Phase 4 — Features

- [x] **#64 — `validateConfig` reports all invalid fields at once**
  Today it returns on the first, so first-run setup is a chain of restart-and-discover cycles. Gets worse once #59 makes the secret the first error every new user hits. Do this early in the phase — it is what makes the tightened placeholder policy pleasant instead of tedious.
  *Files:* `configs/config.go`, `configs/config_test.go`
  *Settled while implementing:* the `configuration is not valid:` prefix moved into `validateConfig` and `NewConfig` passes the error through, so a **single** problem keeps its exact previous wording — that is what leaves the fifteen existing single-field assertions untouched. Two or more problems are listed as two-space-indented `-` bullets under the prefix. Mode-dependent checks still skip what does not apply: an unknown `auth.mode` reports only the unsupported-mode message, never the OIDC fields that a non-existent mode has no use for.
  *Also removed:* the `logrus.Errorf` that logged the validation error inside `NewConfig`. logrus quotes the message, so a multi-problem report came out as one unreadable line of escaped `\n` — printed immediately before the readable one from `log.Fatalln` in `application.loadConfig`, the only caller. Verified on a running instance with six problems.

- [x] **#61 — Environment-only configuration**
  A missing config file is currently fatal even when every value is set via `GIMME_*`. Pairs naturally with #64 — same file, same first-run concern.
  *Settled while implementing:* only `viper.ConfigFileNotFoundError` is tolerated — a file that exists but cannot be parsed stays fatal with the same message, since silently ignoring a typo in a mounted `gimme.yml` would start an instance the operator did not configure. No flag makes the file optional: absence is detected, not configured. `validateConfig` (#64) is what reports what is missing, so an env-only deployment with a hole fails per field rather than on the file.
  *Also:* the README documented the YAML keys but never named a single `GIMME_*` variable, so the feature was undiscoverable — an *Environment variables* section now carries the mapping, the `docker run` form, and the `GIMME_APP_PORT` trap (not `GIMME_PORT`, which Kubernetes injects for any Service named `gimme` — and the chart names it after the release).

- [x] **#95 — `cors.allowed_origins` from the environment** *(found while writing #61's docs, shipped in the same PR)*
  Writing the variable table exposed the one key with no `viper.BindEnv`, which made "every key has a variable" false. It is not that a list cannot be bound: viper binds it fine, but `GetStringSlice` splits an env value on **whitespace**, so the comma an operator writes yields one malformed origin — accepted silently, matching nothing, with no CORS line in the logs to explain the blocked requests.
  *Settled:* the separator is the **comma**, and only the comma — the convention operators expect, and the one Traefik documents for its own `accessControlAllowOriginList` (`foobar, foobar`). Spaces around a comma are trimmed. Normalisation is applied to file values too, so `cors.allowed_origins: "a,b"` written as a string behaves like the YAML list. The empty case stays `[]string{}` and never `nil`: `corsConfig` tells an empty list (allow all, with a warning) apart from a configured one.
  *Worth knowing:* a purely space-separated value still yields several origins, and gimme has nothing to do with it — `viper.GetStringSlice` runs an env value through `strings.Fields` before `splitOrigins` ever sees it. The tolerance is viper's, undocumented on our side and pinned by one test case so that a viper upgrade changing it is noticed rather than silently shipped.
  *Deliberately left out:* validating that each entry looks like an origin. It would catch more typos, but it turns a previously-starting file configuration into a startup failure — its own change, not this one.
  *Files:* `configs/config.go`, `configs/config_test.go`, `test/config/cors-origins.yml`, `README.md`

- [x] **#62 — Upload limits** (size, entry count, decompressed size)
  Needs a new `PayloadTooLarge` kind in `internal/errors/business-error.go`.
  *Measured before the change:* a 3 409 409-byte archive holding 20 001 entries that expand to 629 285 600 bytes is accepted and all 20 001 objects are pushed to storage — a 185:1 expansion, no rejection anywhere.
  *Settled while implementing:* three keys, `upload.max_size` (100 MB), `upload.max_entries` (10 000), `upload.max_uncompressed_size` (500 MB), as plain byte integers — no `"100MB"` parsing, since viper has no size type and the alternative was a new dependency for one field. `validateConfig` rejects any of the three when `<= 0`: **no "0 means unlimited"** escape hatch, an operator who wants no limit sets a large number. Inside `internal/content` a non-positive field disables its own check, which is what lets the forty existing `NewContentService` call sites pass `UploadLimits{}` and keep their current behaviour.
  *Where the defaults live:* `configs` only. `internal/content` gets no `DefaultUploadLimits()` helper — `configs` cannot import it (`internal/content` → `internal/storage` → `configs`), so the values would have been duplicated across two packages that cannot share them.
  *Wiring:* `NewContentService` gains a fourth parameter; `NewPackageController` does **not**. The controller reads `MaxSize` through `ContentService.UploadLimits()`, which keeps one source of truth and leaves the ~30 controller call sites in `api/package-controller_integration_test.go` untouched.
  *Measured:* `errors.As(err, &*http.MaxBytesError)` does match through `c.FormFile` — probed before trusting it — so the string fallback on `"request body too large"` was dropped. If a future Go release stops propagating the typed error, the 413 controller test turns red rather than the limit silently answering 400.
  *Counts are exact:* the entry/size loop runs over the whole central directory instead of stopping at the first entry past the limit — the directory is already parsed and in memory, so early exit bought nothing and made the message report a partial total ("holds 10 001 entries" for an archive of 50 000).
  *Settled after comparing with the field:* sizes accept a suffix — `max_size: 100MB` as well as `104857600` — base **1024**, the nginx convention, so the readable defaults resolve to the exact byte values they replaced. Verdaccio's `max_body_size: 10mb` is the precedent; raw byte counts for values in the hundreds of megabytes are a needless transcription step.
  *Entry default stays at 10 000 — maintainer's call, and the population is what settles it.* Measured on 75 npm packages through the jsDelivr API (`/v1/stats/packages` for the sixty most served, then `/v1/packages/npm/<pkg>@<version>` counting `type=="file"` nodes): **two exceed 10 000** — `@mui/icons-material@9.2.0` at 43 010 and `@material-design-icons/svg@0.14.15` at 10 613 — and **none of the sixty most-served does**, the largest being `@shoelace-style/shoelace` at 5 870. Nine are above 1 000. A raised default was considered, to accept mui icons out of the box, and rejected: gimme hosts private code first, and market libraries are the exception, so the low default holds and the rare package that needs more gets a raised limit. The 413 names `upload.max_entries`, which is what makes that exception self-diagnosing.
  *Worth knowing:* `@mui/icons-material` went from 31 843 files (v5.15.21, the figure #84 and #85 are measured against) to 43 010 (v9.2.0) — roughly 3 000 per major, so any default picked to fit it would need revisiting anyway.
  *Why the entry count is not redundant with the expansion limit:* one million 1-byte files total 1 MB, 0,2 % of the 500 MB limit, and produce one million objects. S3 charges and slows per object, and the cost outlives the upload — #84 measures 39,7 MB of HTML to list 31 843 files, #85 measures 35x on the partial-version path.
  *Documented:* nginx caps request bodies at 1 MB by default, so its 413 fires before `upload.max_size` and names no gimme key.
  *Correction, measured after the first commit — the declared size is not a weak point.* The commit message and the first version of this entry claimed an archive lying about `UncompressedSize64` would slip past the expansion limit. It cannot: `archive/zip` bounds the decompressed output by the declared size itself (`reader.go`, `checksumReader.Read`: `if r.nread > r.f.UncompressedSize64 { return 0, ErrFormat }`), so an entry declaring 100 bytes fails at the 101st. `AddObject` then hands that same declared size to `PutObject`, and minio-go with a known size reads exactly that many bytes. The sum of declared sizes is a genuine upper bound, and no limited reader needs to be added anywhere.
  *What is actually left:* the entry stream is never read to EOF — minio stops at the declared size — so the CRC32 that `checksumReader` verifies at EOF never runs. A corrupted entry can be stored and served with a 200. Bounded in size, unrelated to zip bombs, not worth an issue on its own.
  *Deliberately out of scope:* exposing the three keys as Helm chart values.

- [x] **#97 — Every `c.FormFile` failure answers "input file is required"** *(found while reviewing #62)*
  `createPackage` handles only the body-too-large error and lets every other `c.FormFile` failure fall through with a nil file, so a malformed multipart and a write failure inside `ParseMultipartForm` both come back as `400 input file is required`. The second one is a server fault labelled as a client error, and the client retries a request that was never the problem.
  *Files:* `api/package-controller.go`, `api/package-controller_test.go`
  *Approach:* three branches — `http.ErrMissingFile` keeps today's wording (asserted in `internal/archive_validator/archive-validator_test.go:21`), `*http.MaxBytesError` keeps the 413, anything else names its cause. Check what `mime/multipart` actually propagates before splitting parse errors from I/O errors.
  *The open question — can parse errors and I/O errors be told apart? — was measured, and the answer is yes.* Probed through `c.FormFile` on this codebase: no `file` field gives `http.ErrMissingFile`; a boundary mismatch and an aborted request body give `*fmt.wrapError` (`multipart: NextPart: EOF`, `multipart: NextPart: <cause>`); a truncated body gives a bare `unexpected EOF`; a non-multipart or boundary-less `Content-Type` gives `*http.ProtocolError`; and the temp-file spill failure — the only server fault of the set — is the **only** one arriving as `*fs.PathError`, since that type comes solely from the `os` file-creation path. So the split is a type match, not a string match, and it covers EACCES, EROFS and ENOSPC alike.
  *Settled while implementing:* four branches, in order — 413 stays first so a wrapped `*http.MaxBytesError` cannot be caught by the generic branch; `http.ErrMissingFile` keeps **falling through with the nil file** to `ValidateFile` rather than answering directly, which is what leaves its pinned wording untouched; `*fs.PathError` answers `500 could not process the uploaded file` with the error **logged, not returned** — it carries a server filesystem path; everything else answers `400 malformed upload request: <err>`, the parser's own message being safe to return.
  *A client aborting mid-upload gets a 400, not a 500* — the read failure arrives as `*fmt.wrapError` and lands in the generic branch. The connection is the client's side.
  *Prove it:* Go tests, red first — malformed multipart, non-multipart `Content-Type`, and the spill failure all answered `400 input file is required. (accepted types : application/zip)` before the change. The spill test forces `MaxMultipartMemory = 1` and points `TMPDIR` at a `0500` directory; it skips under uid 0, where a read-only directory constrains nothing.

- [x] **#98 — Env-only configuration fails from the repository root** *(found while running a live instance for #62)*
  `SetConfigType("yaml")` makes viper accept a file named `gimme` with **no extension**, and `make build` writes the binary to exactly that path, so viper parsed 32 MB of Mach-O as YAML and the instance refused to start — defeating #61 in the one flow the README documents (`make build && ./gimme` plus `GIMME_*`). Docker was never affected: the binary is at `/bin/gimme` and the workdir holds only assets.
  *Fixed by:* removing `SetConfigType` — nothing mounts an extension-less config, viper infers the format from the extension, and `ConfigFileNotFoundError` still fires so #61 keeps working. `make build` and `make release` now write to `dist/gimme`, which keeps a build artifact out of a config search path but is hygiene, not the fix: an operator dropping the binary next to their config directory would hit the same wall.
  *Proved red first:* a test writing a non-YAML file named `gimme` next to the config path failed with `unable to read the config file` before the change.

- [x] **#88 — Archive shapes that produce surprising object keys without erroring** *(after #42 + #43)*
  Follow-up to #42/#43, all of it measured against `archiveKeys()`, none of it a regression — these are the archives the new code accepts while still producing keys the user did not mean.
  1. **the common-root heuristic is disarmed by a single root-level file.** `dist/app.js` + `dist/css/style.css` strips to `pkg@1.0.0/app.js`; add a `README.md` at the root and the same archive yields `pkg@1.0.0/dist/app.js` — one added file moves **every** asset URL. A Finder-made zip on macOS is therefore *never* stripped: the root-level `.DS_Store` and the `__MACOSX/` top-level segment both defeat the detection, and the junk is published inside the package.
  2. **a content folder is indistinguishable from a wrapper folder** — `img/logo.svg` + `img/icon.svg` strips `img/`. Undecidable without a convention; accepted deliberately in #42, restated here because any fix for 1 must settle it too.
  3. **Unicode NFC vs NFD yields two objects, one unreachable.** `café.js` in NFC (8 bytes) and NFD (9 bytes) produce two distinct keys and no rejection. macOS stores NFD, browsers send NFC. Duplicate detection itself is byte-exact and works — the gap is normalisation.
  4. minor: `dir\app.js` keeps its literal backslash (not a separator per the ZIP spec), `App.js`/`app.js` coexist (S3 is case-sensitive), a file `dist` and a folder `dist/` coexist.
  *Files:* `internal/content/content-service.go`, `internal/content/content-service_test.go`
  *Splits cleanly in two:* **A** make root detection predictable (ignore known junk / key off a single top-level directory / make it explicit at upload time) — this is the one carrying a product decision; **B** normalise entry names to NFC before computing the key and reject post-normalisation duplicates, which is small and self-contained. Shipped together: same function, same test table.
  *The product decision on A, settled — the detection rule itself does not change; junk is removed before it runs.* Symptom 1 splits in two, and only the macOS half is fixed here. A Finder zip is now stripped correctly because `__MACOSX/` and `.DS_Store` are gone before the common-root loop sees them. **The hand-made `dist/app.js` + `README.md` case stays as it is: a root-level file still disables stripping.** Accepted, not forgotten — see below.
  ⚠️ *Two richer rules were built, measured and then dropped, in this order.* First, "one top-level directory wins, whatever sits at the root": rejected because `app.js` + `style.css` + `img/logo.svg` has exactly one top-level directory, so it would have stripped `img/` and flattened an ordinary package root with an assets folder — worse than symptom 2, since `img/` is visibly content. Second, that same rule plus an allowlist of non-servable root names (`README`/`LICENSE`/`CHANGELOG`/`.gitignore`) that would not block detection: it worked and was green, but the allowlist is a taste judgement sitting on the upload path, and it buys exactly one archive shape. **Maintainer's call: not worth an arbitrary list in the code.** Worth knowing if it ever comes back — "exactly one top-level directory and no root-level files" is *identical* to the original "every entry shares a first segment", so dropping the allowlist made the whole detection rewrite disappear and restored the original loop verbatim.
  *What stays open from symptom 1:* adding a `README.md` beside a `dist/` folder still moves every asset URL. Two things address it from different sides, neither by guessing: **#103** lets the archive declare that the file is not content, which works for a hand-made zip; **#55** never produces the shape at all, since it archives the *contents* of a directory rather than the directory — but only for people using the CLI or the Action.
  *Junk is dropped from the upload, not merely ignored for detection, and the matching is recursive.* Two patterns only: `__MACOSX` as first segment, and basename `.DS_Store` at any depth. Recursion is what the "not uploaded" half requires: the Finder writes a `.DS_Store` into **every** folder a window has opened, so a root-only rule fixes detection but still publishes `dist/.DS_Store`. This is the single deliberate exception to #42's "reject the whole archive, never skip an entry silently" rule, and it is compensated by one aggregate `logrus.Infof` line — aggregate, not per-entry, since an archive may hold tens of thousands.
  *A third pattern, AppleDouble `._*` at any depth, was implemented and then removed — maintainer's call.* It would have caught the `._app.js` companions that the Finder writes beside the real files when the zip is built from a non-HFS volume, but `._` is a prefix a legitimate file could carry, and dropping someone's file is worse than publishing a stray one. Restricting it to `__MACOSX/` made it redundant outright: the first-segment rule already covers everything under that prefix. **The general answer is not a longer hardcoded list — it is letting the archive declare what to exclude: #103, filed from here.**
  *Junk removal runs after the escape checks, never before* — `__MACOSX/../../evil.js` is rejected, not quietly dropped.
  *NFC normalisation happens before `path.Clean` and before root detection*, so two encodings of the same directory name cannot count as two distinct top-level directories. Duplicate detection now catches entries colliding only after normalisation, and the error still quotes the **original** archive names: the whole point is that the two are visually identical, so echoing the normalised form would print the same string twice.
  *The NFC/NFD test fixtures are `\u` escapes, not literal characters* — written literally, any editor or tool normalising the source file would make the two constants byte-identical and the tests vacuous. `TestContentService_NFCFixturesDiffer` pins their lengths at 8 and 9 bytes so that silent collapse fails loudly.
  *Point 2 stays as it was:* `img/logo.svg` + `img/icon.svg` still has `img/` stripped. One top-level directory with no root-level content file is a wrapper — undecidable otherwise, accepted in #42, unchanged here.
  *Point 4 deliberately untouched:* the literal backslash, `App.js`/`app.js` coexisting, and a file `dist` beside a folder `dist/`. Measured, minor, no user-visible loss.
  *Rejected:* a `strip` form field at upload time — it answers points 1 and 2 outright, but it is new API surface with its own default to choose and its own documentation, which makes it its own change rather than this one. Also rejected: `Thumbs.db` and other Windows junk, not measured.
  *Note:* #55 removes several of these at the source by building the archive itself, but does nothing for a hand-made zip.

- [x] **#103 — Honour `.gimmeignore` inside the uploaded archive** *(after #88, filed from it)*
  #88 leaves two hardcoded exclusions (`__MACOSX/`, `.DS_Store`) and no way for a user to add a third. Growing that list is a guessing game — #88 implemented AppleDouble `._*` removal and backed it out for exactly that reason. Let the archive declare what it does not want published instead.
  npm's rule as the precedent: an ignore file applies to its directory and all children, and `.npmignore` wins over `.gitignore` when both exist. Simplified here to `.gitignore` plus a dedicated `.gimmeignore` that wins.
  *Why it is more than convenience:* it gives a principled answer to what #88 left open. `dist/app.js` + `README.md` is not stripped today because any root file blocks detection; #88's allowlist of root names was removed for being a taste judgement. An ignore file replaces the guess with a declaration — a file the archive itself calls non-content should not count as content when looking for the wrapper.
  *Files:* `internal/content/content-service.go`, `internal/content/content-service_test.go`
  ⚠️ ***`.gitignore` is not honoured — only `.gimmeignore`, and that is the whole shape of the decision.*** A project `.gitignore` almost always lists `dist/`, because build output is not versioned. Someone zipping their whole repository would have the very directory they meant to publish excluded, silently and with no error — the class of defect #88 just closed. npm invented `.npmignore` for exactly this. A `.gimmeignore` exists only because someone wrote it for gimme, so its contents are a deliberate declaration. `.gitignore` support stays possible later; it is not in this change, and a `.gitignore` in the archive is uploaded like any other file.
  *Two fixed locations, no recursion:* the archive root, or — when the entries satisfy the wrapper condition — inside that single top-level folder. Root-only was decided first and then corrected, because it makes the feature useless for `zip -r pkg.zip dist`, which is the more common reflex: the file would sit at `dist/.gimmeignore` and never be read. Patterns anchor to the directory holding the file, so rules are written against the package contents either way. A `.gimmeignore` anywhere else is an ordinary uploaded file.
  *The file that was used is not published*, which is also what lets it unblock wrapper detection. Excluded entries do not count when looking for the wrapper — that is the point, and it is what gives #88's symptom 1 a principled answer: a declaration instead of the name allowlist that was rejected there.
  *#62's limits are untouched, and that was already true by construction* — `checkArchiveLimits` runs on the raw archive before `archiveKeys`. The limits bound the work a client can force on the server, not the output, so filtering must not relax them. Pinned by `TestContentService_CreatePackage_GimmeIgnoreDoesNotRelaxLimits`.
  *An archive whose entries are all excluded is rejected* with the existing `archive contains no files`, exactly as a junk-only archive is. The hardcoded junk stays and the ignore file only adds to it — making `__MACOSX/` and `.DS_Store` default patterns would mean an archive carrying a `.gimmeignore` silently stops dropping them.
  *Library: `github.com/sabhiram/go-gitignore`, chosen on measurement* — 8 modules added to the graph against 51 for `go-git`, for the same matcher. Its support was probed rather than assumed: `*.map` at any depth, `node_modules/` at any depth, `/anchored.txt` root-only, `docs/**/*.tmp`, `!` negation, and CRLF line endings all behave. It is untagged and frozen since 2021; gitignore semantics do not change, so a frozen matcher is acceptable here.
  *Note:* #55 can filter client-side, but only for people using the CLI or the Action. This one covers hand-made zips.

- [x] **#48 — ETag / `If-None-Match` → 304**
  Do before #47: smaller and self-contained, and #47 then extends it per encoding variant.
  *Hand-rolled, not `http.ServeContent`.* `minio.Object` is an `io.ReadSeeker`, so `ServeContent` would have supplied the conditional logic for free — and Range/206 with it, which is a separate feature: a new status code, and a `Seek` on a `minio.Object` re-issues a GET against S3. The reason this task is scheduled before #47 is that it is small; taking Range along would have undone that.
  *Precedence is an early return, not an `||`* — a present `If-None-Match` that does not match answers 200 and `If-Modified-Since` is never consulted, which is the rule that makes a stale-ETag revalidation carrying an old date behave.
  *`LastModified` is truncated to the second before comparison.* S3 keeps sub-second precision while `Last-Modified` is second-granular, so without the truncation a client echoing back our own header looks stale on every request. Pinned by a unit case.
  *`minio.ObjectInfo.ETag` arrives unquoted* — the header value is wrapped in `"`. A multipart upload yields a `-N` suffix instead of an MD5; it is an opaque validator either way.
  *The `If-None-Match` list is split on `,` without honouring commas inside a quoted ETag.* S3 ETags are hex plus an optional `-N` and never contain one; a client that sends one gets a 200 with a body, never a wrong 304.
  *304 answers pinned versions too* — `immutable` does not stop a cache from revalidating, and answering costs nothing.
  *Tests are split, and not by preference:* `etagMatches` and `notModified` are pure functions unit-tested in `api/package-controller_test.go`, while the end-to-end 200-then-304 has to be an integration test. `MockOSManager.GetObject` returns `&minio.Object{}`, whose `Stat()` carries no ETag and no modification time — no mock fixture can express this behaviour.
  *Left for #47:* no `Vary` header, since nothing varies yet. When brotli/gzip lands it must add `Vary: Accept-Encoding` **and** suffix the ETag per encoding variant, or a cache serves a brotli body to a client that asked for identity.
  *Files:* `api/package-controller.go`, `api/package-controller_test.go`, `api/package-controller_integration_test.go`, `README.md`, `docs/site/index.html`, `docs/api/swagger.json`

- [x] **#47 — Serve brotli/gzip**
  Pre-compress at upload, negotiate on `Accept-Encoding`. Touches `CreatePackage`, so it must come after #42/#43.
  *Settled before implementing:* both `br` and `gzip`; variants stored as **sibling keys** `<key>.br` / `<key>.gz`, the nginx `gzip_static` layout, so `DeletePackage`'s prefix removal cleans them up with no extra code; **one config key**, `compression.enabled` (default `true`), everything else a constant — 1 KiB minimum size, 0.8 minimum ratio, brotli q=5, gzip level 9, 8 MiB maximum entry size. A reserved directory (`pkg@ver/.gimme-enc/`) was the alternative: no collision risk, but it adds a reserved name, a URL serving raw brotli bytes, and machinery the sibling layout does not need.
  *An archive that already carries a variant wins, and gimme generates none for that encoding.* A `dist/` built with `vite-plugin-compression` or `compression-webpack-plugin` already holds `app.js.gz` and `app.js.br`, and zipping that directory is the common reflex. Rejecting the archive on the collision was considered and dropped: hostile to the most common build output, and the user's own variants are usually compressed harder than ours.
  *Variants are hidden from the HTML listing* — a key ending in `.br`/`.gz` whose sibling exists **and** is a compressible type. A genuine `foo.tar.gz` beside `foo.tar` stays visible, since `foo.tar` is not a compressible type. Without this the listing triples its rows and #84 gets worse.
  ⚠️ *Brotli quality is 5, not the 11 the JS ecosystem defaults to, and the gap is much larger than the estimate that motivated the choice.* Measured on `svgo.browser.js` (783 017 B): **q=5 → 145 814 B in 18 ms, q=11 → 124 408 B in 1.044 s** — 58x the time for 15 % smaller output, 2.7 points of the source. On `ajv.min.js` (119 444 B): 28 339 B in 3 ms against 26 046 B in 147 ms, 49x for 8 %. gzip level 9 lands at 191 212 B and 30 138 B, in 19 ms and 3 ms. webpack and vite default to q=11 because they run at build time with nobody waiting; here compression runs inside the synchronous `POST /packages`. The 1 KiB threshold and the 0.8 `minRatio` rule are taken from those same plugins.
  *Memory is bounded by a second semaphore.* The errgroup limit is `NumCPU*4` (#44), and letting four times the cores compress 8 MiB entries at once is how an upload becomes a memory spike, so the compression step alone is capped at `NumCPU`. Only the compressed output is buffered — the entry streams once through an `io.MultiWriter` feeding both compressors. An `io.LimitReader` at the cap plus a post-copy size check is what keeps `gosec` G110 quiet without a `nolint`, and it detects a lying ZIP header instead of silently truncating.
  *Serving:* version resolution and the Redis cache are untouched — the cache still stores the resolved **identity** path and the suffix is appended after, so no cache key changes and no encoding can poison another. `Vary: Accept-Encoding` goes on every file response including the 304. The ETag is the variant object's own, so it differs per encoding for free, which is what #48 left open here. `Content-Type` for a variant is computed from the requested file name and not from the stored object: a user-supplied `app.js.gz` was stored as `application/gzip` by the upload path.
  *Packages uploaded before this change keep working, at one extra S3 round trip* — the variant GET 404s, then the identity GET. A fresh package costs one request. Knowing in advance which variants exist would mean stat'ing the identity object first, which is two requests in the *common* case instead of the legacy one.
  ⚠️ *Variant selection cannot be unit-tested against the mocks.* `MockOSManager.GetObject` returns `&minio.Object{}`, and `Stat()` on that zero value panics — it reaches `doGetRequest`, which selects on a nil context. The probe therefore only runs when the caller passes accepted encodings, and the negotiation is proven in `api/package-controller_integration_test.go` against Garage. Same wall as #48.
  *Dependency:* `github.com/andybalholm/brotli v1.2.2`, measured as #103 required — **+2 modules**, 298 → 300. gzip comes from the standard library.
  *Deliberately out of scope:* Range/206, zstd, compression at request time, a `compress` form field, and Helm chart values for the new key.

- [ ] **#50 — SRI integrity hashes**
  Same upload hashing pass as #47 — do it right after, or together.

- [ ] **#84 — Paginate the package listing (UI + API)** ⚠️ *settle before or with #49*
  `GET /gimme/<pkg>@<version>` returns every object under the prefix in one response. Measured on `@mui/icons-material@5.15.21` (31 843 files): **39.7 MB of HTML** for an 18.7 MB package, 1.3 s server-side, and a browser that visibly struggles. A design-system icon package is the primary use case for a CDN, not an exotic input.
  *Files:* `templates/package.tmpl`, `api/package-controller.go`, `internal/content/content-service.go`, `internal/storage/objectstorage-manager.go`
  *Start at the storage layer:* `ObjectStorageManager.ListObjects` hardcodes `Recursive: true` and drains the channel into a full slice, which makes pagination impossible for every caller above it. S3 `ListObjectsV2` is natively paginated (`MaxKeys` + continuation token); minio-go already exposes it.
  *Order:* #49 has no listing contract yet — agreeing this one first means #49 adopts it instead of inventing a second.
  *Also:* `GetFiles` uses the raw prefix with no version resolution, so `pkg@5` lists every version's files mixed together with no indication of which version each belongs to.

- [ ] **#85 — Partial-version requests pay a full recursive listing** *(after #45 + #46)*
  `getLatestPackagePath` calls `ListObjects` on every partial-version file request, so serving one 489 B file drains all 31 843 objects. Measured: **0.023 s pinned vs 0.810 s partial — 35x**, cache disabled. Cost scales with the file count of the package, not the size of the file served. This is the asset-serving hot path, not the browse UI.
  *Files:* `internal/content/content-service.go`, `internal/storage/objectstorage-manager.go`
  *Approach:* resolve from the version list, not the object list — a delimited (non-recursive) list on the `pkg@` prefix returns one common prefix per version instead of one entry per file, then build the path directly as the pinned branch already does.
  *Order:* same function as #45 + #46, which rewrite the resolution. Land after them or fold in — doing it first means writing the resolution tests twice.
  *Note:* the cache mitigates repeat hits but is optional and does nothing for the cold path, so it does not close this.

- [ ] **#49 — Browse: `GET /packages` and version listing**
  Independent of everything else. Good candidate if a visible win is wanted early. See #84 — the pagination contract should be settled first, or in the same pass.

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
| #42 + #43 | Archives that previously uploaded are now rejected: `..` escapes, absolute paths, empty entry names, and any archive whose entries collide on the same target key. Archives with no root folder, or with several top-level folders, now upload correctly instead of being flattened — so the object keys they produce **change**, and URLs written against the old flattened layout break. |
| #45 | `pkg@1` serves different content than before |

Not breaking, despite touching sensitive ground: #59 and #60 change shipped template files rather than the behaviour of a running instance; #57 is documentation. #65 makes an unworkable Helm configuration fail to render, which only affects deployments that were already misbehaving.

A major bump is consistent with this project's own precedent: v2 was the major for replacing JWT tokens with opaque tokens — the same kind of breaking change.

Release notes are auto-generated from PR titles in GitHub Releases. **Add a hand-written preamble for this one** listing the three breaking changes and their upgrade actions — PR titles alone will not tell an operator that their `helm upgrade` will fail, that an archive their pipeline has always uploaded is now rejected, or that a `@1` URL now serves different content.

**The preamble opens with the upgrade actions, in bold, above everything else** — not at the bottom, where nobody reads them. First line: this release fixes bugs in version resolution and in how archive entries become object keys, and **it does not repair content already stored**. The fixes apply at upload and at resolution time only.

There is no index to rebuild — gimme keeps none. `ListObjects` is called against S3 on every request (`internal/content/content-service.go`), so S3 is the only source of truth. The two actions are therefore:

- **Re-upload the affected packages.** #42 + #43 change the object keys produced *at upload*; they rewrite nothing already in the bucket. A package uploaded from an archive without a single root folder stays flattened where it is (`img/logo.svg` stored as `pkg@1.0.0/logo.svg`), and root-level files stay orphaned outside the `<pkg>@<version>/` namespace — `DeletePackage` lists on that prefix and cannot reach them, so they survive a package deletion and need an S3 client to remove. Already-published URLs keep resolving; what stays wrong is the layout, until the package is uploaded again. Re-uploading is also what produces the brotli and gzip variants from #47 — a package stored before that change is served uncompressed, and costs one extra S3 round trip per compressible file until it is uploaded again.
- **Flush the Redis cache if it is enabled** (off by default). `GetFile` caches the resolution of partial versions — key `pkg@1/app.js` → resolved object path. Entries written before the upgrade encode the #45 bug (`@1` → `10.0.0`) and keep serving it until the TTL expires (3600 s by default). Pinned versions never go through the cache, so they are unaffected.

---

## Open decisions

*None open.*

Project identity was settled in #70: repository, Go module and Docker image are all `ziggornif/gimme`, and the module path deliberately carries no major-version suffix.
