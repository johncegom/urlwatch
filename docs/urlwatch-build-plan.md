# urlwatch — Build Plan & Ledger

A concurrent URL health checker, built to learn Go goroutines, channels, and
worker pools by building a real CLI tool instead of isolated exercises.

Status legend: `done` | `next` | `pending`

---

## Phase 1 — Goroutine basics
Outcome: understand goroutines don't block, and why that causes bugs.

- [x] **Checkpoint 1** — `done` — Ran a single goroutine next to `main`, observed
  `main` exit before the goroutine finished. Core lesson: `main` exiting kills
  every goroutine, done or not.
- [x] **Checkpoint 2** — `done` — Added `sync.WaitGroup` to make `main` wait.
  Hit and understood a real deadlock (`Add` count ≠ actual goroutine count).
  Confirmed one `WaitGroup` can track many goroutines.

## Phase 2 — Channels
Outcome: goroutines send results back safely through a channel.

- [x] **Checkpoint 3** — `done` — Added a `results` channel. Understood
  directional channel types (`chan<-` / `<-chan`) and why `main` — not the
  sender — is the correct party to `close()` the channel once `wg.Wait()`
  confirms no more sends are coming.

## Phase 3 — Worker pool
Outcome: a bounded number of workers pull jobs from a queue.

- [x] **Checkpoint 4** — `done` — Built the real worker pool: `jobs` channel,
  fixed `numWorkers`, each worker loops with `for job := range jobs`. Hit and
  fixed a deadlock caused by a missing `close(jobs)`.
- [x] **Checkpoint 5** — `done` — Reasoned through pool sizing as a **safety
  tradeoff, not a speed tradeoff** — workers = urls is fastest but unsafe at
  scale; fewer workers is slower but protects both client and target servers.

## Phase 4 — Production shape (concurrency control)
Outcome: per-request timeout, cancellation, clean shutdown.

- [x] **Checkpoint 6** — `done` — Added `context.WithTimeout` with `select`
  racing `time.After` vs `ctx.Done()`. Fixed a real bug: context must be
  created **per url inside `checkURL`**, not shared once in `main`, or the
  deadline silently exhausts across the whole batch instead of resetting per
  request.
- [x] **Checkpoint 7** — `done` — Added `signal.NotifyContext` for graceful
  Ctrl+C shutdown. Converted `worker`'s `for range jobs` into a manual
  `select` loop racing `jobs` against `ctx.Done()`. Scoped deliberately to
  "stop picking up new work" — in-flight checks still finish on their own.

## Phase 5 — Real functionality (scoped)
Outcome: `urlwatch` actually checks real urls and takes real input.

- [x] **Checkpoint 8** — `done` — Replaced fake `time.Sleep` with real
  `http.Get`. Learned to separate "did the network call succeed" (`err`) from
  "what did the site say" (`resp.StatusCode`) — a 403/500 is not an `err`.
- [x] **Checkpoint 9** — `done` — Wired `context.WithTimeout` into the real
  HTTP call via `http.NewRequestWithContext` + `http.DefaultClient.Do`, with
  `errors.Is(err, context.DeadlineExceeded)` to distinguish timeout from other
  failures. Fixed a missing `return` after an error branch that would have
  caused a nil-pointer panic on malformed URLs.
- [x] **Checkpoint 10** — `done` — Replaced the hardcoded `urls` slice with a
  `-file` CLI flag read via `bufio.Scanner`. Fixed a deadlock caused by
  creating `jobs`/`results` channels *before* `urls` was populated, sizing
  them to buffer `0`.

## Phase 6 — Refactor to production shape
Outcome: same behavior, organized like a real Go project.

- [x] **Checkpoint 11** — `done` — Split single `main.go` into
  `main.go` / `checker.go` / `worker.go` / `input.go` — one package, multiple
  files (not multiple packages — correctly scoped to project size). `main.go`
  is now pure orchestration.
- [x] **Checkpoint 12** — `done` — Defined and applied a real error-handling
  rule: **fatal errors stop the program** (missing/unreadable file); **per-item
  errors report and continue** (malformed url, timeout, bad status) so one bad
  line can't erase results for the rest of the batch. Changed `readURLs` to
  *return* warnings instead of printing them internally — the caller (`main`)
  decides how to surface them.
- [x] **Checkpoint 13** — `done` — Designed a real status model instead of a
  formatted string: `checkStatus` (healthy / reachable / failure) +
  `checkResult` struct, built via a `newCheckResult` free function (not a
  method — no instance exists yet to call a method on). Classification rule:
  2xx → healthy, 401/403 → reachable (server up, just access-gated), 404/5xx/
  timeout/network error → failure (404 specifically treated as failure since
  the watched endpoint is gone, not just protected). Exit code: non-zero if
  **any** url fails, decided only after every result is collected — fixed a
  bug where `os.Exit(1)` inside the print loop was killing the process before
  later results could print.
- [x] **Checkpoint 14** — `done` — README covering build/usage, output format,
  exit code meaning, and the status-category table — written directly rather
  than as a fill-in-the-blank exercise, since documentation isn't the
  concurrency skill this project is for.

## Phase 7 — Correctness
Outcome: prove what already exists works, before anything changes shape.

- [x] **Checkpoint 15** — `done` — Extracted `classify(statusCode int)
  checkStatus` out of `checkURL` into its own pure function in `checker.go` —
  no dependency on network, time, or channels, making it directly testable.
- [x] **Checkpoint 16** — `done` — Table-driven unit tests for `classify`
  (boundaries: 199/200/299/300, 401/403, 404, 500 — caught a duplicate case
  and missing boundaries in the first draft) and `readURLs`'s filtering
  (`t.TempDir()` + exact-content assertions via `reflect.DeepEqual` and
  `strings.Contains`, not just counts — counts alone can't catch wrong urls
  or a wrong warning line getting through).
- [x] **Checkpoint 17** — `done` — Integration tests for `checkURL` using
  `httptest.NewServer` (healthy + timeout cases). Along the way, pulled
  `checkURL`'s hardcoded 500ms timeout into a real `timeout time.Duration`
  parameter — threaded through `checkURL` → `worker` → `main` — after
  noticing the timeout test was silently coupled to that hardcoded value and
  slow (500ms/test). Pulled forward from checkpoint 22's original scope once
  a concrete need for it showed up.
- [x] **Checkpoint 18** — `done` — Ran `go test -race ./...`, clean. Flagged
  honestly: existing tests call `checkURL` directly from a single goroutine,
  so `-race` hasn't yet exercised real concurrent access to `jobs`/`results`
  — that's what checkpoint 20 is for.
- [x] **Checkpoint 19** — `done` — Brainstormed a full `checkURL` case list
  (happy/edge/negative), then deliberately cut it down using the same
  redundancy filter applied to `classify`: kept only cases hitting genuinely
  distinct code paths. Along the way, designed and implemented real 429
  retry-with-backoff behavior in `checkURL` (not originally planned — added
  because the brainstorm surfaced it):
  - `doOneAttempt` helper extracted so `defer cancel()` fires per-attempt,
    not accumulated across the retry loop (loop-scoped `defer` doesn't run
    per-iteration — a real correction made mid-checkpoint).
  - Exponential backoff (`100ms, 200ms, 400ms`) between up to 3 retries;
    exhausts to `statusFailure`/429 if all retries are rate-limited.
  - Verified `http.Request` can't be reused across `Do()` calls — a fresh
    request is built per attempt.
  - Empirically confirmed (via sandbox test) that malformed/empty URLs fail
    at `Do()` with `"unsupported protocol scheme"`, not at
    `NewRequestWithContext` as initially assumed — corrected an earlier
    wrong claim before writing tests around it.
  - Final `checkURL` test set (6 total): healthy, timeout, retry-then-
    succeed, retry-then-exhaust, connection-refused, empty-url — each
    justified by hitting a distinct branch, not just a distinct scenario.
    404/500/503 and DNS-failure cases were deliberately cut as redundant
    with `classify`'s coverage or too flaky/network-dependent for a unit
    test, respectively.
- [x] **Checkpoint 20** — `done` — Coverage-driven gap hunting on `checkURL`:
  stale `cover.out` caught once (regenerated before trusting the number
  again); learned line-coverage vs scenario-coverage are different claims —
  a fourth error-path test (CRLF-injection URL, confirmed via
  `net/url: invalid control character in URL`) added no new coverage
  because it landed on an already-green line, while a missing `401/403`
  (`statusReachable`) test turned out to be the real gap. `checkURL` reached
  100%; `worker`/`main`/`parseFlags` left at 0% deliberately (covered by
  checkpoint 22, or left to integration/manual testing as orchestration
  code).
- [x] **Checkpoint 21** — `done` — `FuzzCheckURL` using Go's built-in
  fuzzing (`testing.F`, seeded with known-tricky inputs from checkpoint 19).
  Ran 30s / ~1.37M executions, 252 new "interesting" inputs discovered and
  saved to `testdata/fuzz/`, zero panics or hangs. Those saved inputs now
  replay automatically on every future `go test ./...` as permanent
  regression cases.
- [x] **Checkpoint 22** — `done` — Concurrent worker-pool test:
  `TestWorkerPool_Concurrent` — 5 real `worker` goroutines, real
  `jobs`/`results` channels, 20 urls against a shared `httptest.Server`,
  run under `go test -race`. Clean pass, worker-ID interleaving visible in
  logs as real evidence of concurrent access, not just single-goroutine
  `checkURL` calls dressed up as a pool test. **Phase 7 (Correctness)
  complete — checkpoints 15–22 all done.**
- [x] **Checkpoint 22a** — `done` — Extended the concurrent pool test to
  mixed conditions: `TestWorkerPool_MixedConditions_Race` — 5 workers, 15
  urls split across healthy (5), 404/500 failures (6), and 429-then-recover
  retries (4, each on its own path so per-url attempt counts stay
  independent), all against one `httptest.Server` routed by path. Asserted
  both final classification counts (9 healthy / 6 failure) and exact retry
  attempt counts per retry-url (proving retries actually ran, not luck),
  clean under `-race`. Learned `sync.Mutex` from scratch to safely
  instrument the test server's own shared attempt-counter map — a real
  case of the test harness itself needing the same race-safety discipline
  as the code under test. Test placed in a new `worker_test.go`, alongside
  `TestWorkerPool_Concurrent` from checkpoint 22, grouped by what they
  test (the pool) rather than by which checkpoint wrote them.

## Phase 8 — Security hardening (future)
Outcome: decide what `urlwatch` should refuse to do, backed by Phase 7's
tests so new restrictions can be verified, not just hoped for.

- [x] **Checkpoint 23** — `done` — Input validation hardening on `readURLs`:
  three separate caps, each protecting a different resource:
  - `maxFileSize` (`os.Stat` before opening) — fatal, stops the whole run,
    with a message suggesting the file be split.
  - `maxLineSize` (`scanner.Buffer(...)`) — protects against a single
    absurdly long line, which a file-size cap alone can't catch (the size
    check happens before any bytes are read into memory; a byte-size cap
    on the file doesn't prevent one line from consuming most of it).
  - `maxURLs` — deliberately checked against `len(urls)`, not raw line
    count, so it counts *real accepted work* the pool will actually do,
    not lines that get filtered out. This was a genuine correction from a
    first instinct ("count total lines up front") to a more precise one
    ("count only what survives filtering") — same shift in thinking as
    checkpoint 12's "functions return information" rule.
  - Scheme rejection: reused the `*url.URL` `url.ParseRequestURI` already
    returns (previously discarded) to check `.Scheme` against `http`/
    `https`; non-matching schemes (e.g. `file:///etc/passwd`) go to
    `warnings`, not `urls`. Caught and fixed a real bug mid-checkpoint: a
    missing `continue` after the scheme-rejection warning let the bad url
    fall through into `urls` anyway — same shape of bug checkpoint 12 had
    already fixed once for malformed urls, reintroduced here for the new
    check. `TestReadURLs_InvalidScheme` added as a true regression test
    (asserts both `urls` and `warnings`, not warnings alone) — an
    intermediate draft that only checked `warnings` would have passed even
    with the bug present, same blind spot as the project's very first
    `readURLs` test.
- [x] **Checkpoint 24** — `done` — Chose `code.dny.dev/ssrf` over a
  hand-rolled `isBlockedHost` check-then-connect approach, after research
  surfaced the real reason: check-then-connect has a DNS rebinding gap
  (safe IP at check time, private IP at connect time). The library's
  `Guardian.Safe` hooks into `net.Dialer`'s `Control` function, which runs
  on the actual resolved IP at the moment of connection, closing that gap.
  Confirmed via real precedent (GitLab's own SSRF postmortem) that
  distinguishing error messages by failure type (blocked vs timeout vs
  refused) leaks information attackers can use to map an internal network —
  collapsed blocked/timeout/network-error messages toward a single generic
  failure, matching the earlier decision to not reveal *why* a check
  failed.
  Hit a real test-vs-production conflict: `httptest.NewServer` binds to
  `127.0.0.1` on a random port, which the guardian correctly flags as
  loopback/non-standard-port — diagnosed correctly as the guard working as
  intended, not a bug, and *not* fixed by loosening the security policy.
  Refactored around it properly:
  - `client *http.Client` threaded through as a parameter (same pattern as
    `timeout` in checkpoint 17) so tests can inject `http.DefaultClient`
    while production uses the guarded `safeClient`.
  - Noticed parameter creep (4 params and climbing) and bundled `timeout`
    + `client` into one `checkerConfig` struct — same type in both
    contexts, differentiated by two constructor functions
    (`newProductionConfig` / `newTestConfig`, the test one living in
    `checker_test.go`) rather than two separate types, so tests stay
    proof about the real struct shape production code uses. Sets up
    checkpoint 26 cleanly — new config fields won't require touching every
    call site.
  - Deleted `isBlockedHost` entirely — dead code that looked like active
    protection but wasn't wired in, called out as a specifically dangerous
    kind of bug in security-relevant code.
  - Caught and fixed one more small thing: `newProductionConfig(...)` was
    being recomputed inside the worker-spawn loop despite being identical
    every iteration — moved outside the loop.
- [x] **Checkpoint 25** — `done` — `go mod tidy` clean. `govulncheck ./...`
  surfaced a real finding: `CVE-2026-42505` (`GO-2026-5856`), a Go standard
  library `crypto/tls` privacy leak affecting Encrypted Client Hello (ECH)
  handshakes, present on the installed `go1.26.4` toolchain (fixed in
  `go1.26.5`). Confirmed the vulnerable code path requires
  `Config.EncryptedClientHelloConfigList` to be set, which `safeClient`
  never does — a concrete, live example of `govulncheck`'s call-graph
  analysis proving *reachability* of a vulnerable function without being
  able to prove the vulnerable *branch* inside it is ever actually
  exercised at runtime. Fixed anyway (cheap, no behavior change for
  non-ECH code) by upgrading the Go toolchain itself — this is a toolchain
  fix, not a `go.mod` dependency bump, since it's the standard library.
  Bumped `go.mod`'s `go` directive from `1.26.4` to `1.26.5` afterward —
  not because it was strictly required (no `toolchain` line was pinning an
  exact version), but to make the security floor self-documenting and
  enforced for anyone else who builds the repo. `govulncheck ./...` added
  to `ci.yml` as a permanent step, so future CVEs surface automatically on
  every push instead of requiring someone to remember to run it manually —
  closes the loop this checkpoint's own experience motivated.

## Phase 9 — SiteGuard reuse-readiness (future)
Outcome: reshape `urlwatch` into something importable, with Phase 7's tests
acting as a safety net against regressions during the reshape.

- [x] **Checkpoint 26** — `done` — Made `numWorkers` configurable via a
  `-workers` flag, bundled with `filePath` into a `cliConfig` struct
  (mirroring `checkerConfig`'s pattern) so future flags won't require
  touching every call site. Real design reasoning behind the upper bound:
  initially argued no cap was needed (machine capacity varies), then
  correctly self-corrected — the actual risk isn't the machine running
  `urlwatch`, it's how many simultaneous connections a single target might
  receive, since the pool is global rather than per-host and a url list
  could repeat one domain many times. `maxWorkers = 100` chosen from real
  reference points (browser per-host connection conventions, typical
  synthetic-monitoring tool defaults) rather than an arbitrary guess — with
  a noted, deferred improvement: a true fix would be a per-host limit, not
  a global one, out of scope for now.
  Checked whether checkpoint 24's "don't reveal why a check was blocked"
  rule applied to the new limit message, and correctly concluded it
  doesn't — that rule protected against leaking info to an *untrusted
  remote party*; a `-workers` value is supplied by the tool's own local
  operator, who already knows what they typed, so a clear, specific error
  message is safe and more usable than a silent clamp.
  Caught and fixed a real edge case testing revealed only by tracing
  through it: `-workers 0` or a negative value wasn't rejected by the
  upper-bound check alone, and rather than crashing, `main`'s loop
  (`for i := 1; i <= numWorkers; i++`) would silently spawn zero workers —
  every url would still enter `jobs`, `wg.Wait()` would return immediately
  with nothing to wait for, and the program would exit cleanly having
  checked nothing at all. Added an explicit `<= 0` rejection — arguably
  more important than the upper bound, since a silent no-op that looks
  like success is worse than an obvious crash. Verified all three real
  cases (`-workers 0`, `-workers 500`, `-workers 10`) behave correctly
  against a real 10-url file.
- [x] **Checkpoint 26a** — `done` — Extended checkpoint 26's pattern to
  `-timeout` as well (`flag.Duration`, not `flag.Int` — parses real
  duration strings like `2s`/`500ms` directly, and rejects a bare unitless
  number like `500` automatically, before any custom validation runs).
  Bounds reasoning followed the same shape as `maxWorkers`: an initial
  lower-bound guess (10s) directly contradicted the working `500ms`
  default — a concrete self-check ("does my own default pass my own
  validation?") caught it before it shipped, landing on `50ms`/`30s`
  instead. Confirmed "let the user omit the flag entirely" needs no special
  handling — `flag.Duration`'s own default parameter already covers it.
  Real-world evidence the change mattered: several urls (`github`,
  `linkedin`, `twitter`, `amazon`) that timed out under the old hardcoded
  `500ms` succeeded cleanly once run with `-timeout 2s`. Full suite
  (including `-race`, retries, fuzz corpus, both concurrent pool tests)
  confirmed green after the change.
- [x] **Checkpoint 27** — `done` — Machine-readable `-json` output, built in
  layers:
  - **Ordering:** decided output must match input file order even though
    checking itself stays concurrent/unordered — required threading a new
    `index int` through the whole pipeline. Correctly placed the new `job`
    struct (`index` + `url`) in `worker.go` rather than `main.go`, on the
    same reasoning as `checkResult` living in `checker.go` — a type belongs
    with what produces/consumes it, not with orchestration code. Confirmed
    `checkURL` only *relays* `index` without ever branching on it, so
    accepting a whole `job` parameter doesn't violate separation of
    concerns — it would if `checkURL` started making decisions based on
    position.
  - **Output shape:** chose a single JSON array (buffered, `main`
    naturally caps this cheaply at `maxURLs` results) over JSONL — a
    single complete response also fits an MCP tool call's request/response
    shape better than a stream would, tying back to Phase 10.
  - **Streaming vs batch, three shapes considered:** first instinct (one
    loop, always collect + always print) wastes memory/risks double output
    when unused; second instinct (one loop, `if` checked per-iteration)
    works but mixes two unrelated concerns in one block; landed on
    **closures chosen once before the loop** (`handleResult`/`finish`
    function values, picked based on the flag, single shared loop calling
    whichever was chosen) — first real use of functions-as-values in this
    project. Real bug caught mid-build: `hasFailure` was only being set
    inside the *text-mode* closure, so an all-failing run under `-json`
    would exit `0` — moved the check into the shared loop itself, since
    detecting a failure is a different concern from displaying one and
    shouldn't live inside only one display path.
  - **`encoding/json` export bug, hit and fixed directly:** `checkResult`'s
    lowercase fields (`url`, `status`, etc.) are invisible to
    `json.Marshal` — reflection-based stdlib code can only see *exported*
    (capitalized) fields, same visibility boundary idea as Go's `chan<-`
    directional typing. Fixed by capitalizing every field and adding
    explicit `json:"..."` tags to control the actual JSON key names
    (`url`, not `URL`). Noted the side effect: this also makes the fields
    part of `checkResult`'s public surface once checkpoint 28 promotes it
    into an importable package.
  - **Latency unit, reasoned through deliberately:** raw `time.Duration`
    marshals as a bare nanosecond integer with no unit indicator — decided
    against a human-readable string (`"403ms"`) since parsing a formatted
    string back into a number for computation is harder than the reverse;
    decided against keeping raw nanoseconds since the field name alone
    can't convey the unit and JSON has no native unit type. Landed on
    `LatencyMs int64` via `time.Duration.Milliseconds()`, with the unit
    encoded directly in the JSON key (`latencyMs`) — a self-documenting
    convention that doesn't rely on anyone having read external docs.
  - Verified end to end against real, deliberately-all-failing DNS
    lookups: correct `"status":"failure"`, accurate `"no such host"`
    error text (not misleadingly reported as `"timeout"`), exit code `1`.
    Verified against a real mixed-result file that JSON `index` order
    matches file order even though the "picked up" log lines show workers
    finishing in a different real order — direct proof the sort works, not
    just looks right by coincidence.
- [x] **Checkpoint 27a** — `done` — Extracted `processResults(w io.Writer,
  results []checkResult, jsonOutput bool) bool` out of `main`, closing the
  testability gap flagged during the planning review. Two design questions
  resolved deliberately, not defaulted:
  - **Streaming vs. collect-then-print:** chose to give up live streaming
    (originally preserved back in checkpoint 27) in favor of a simpler,
    uniform, testable shape — explicit YAGNI reasoning: don't pay the
    complexity cost of preserving responsiveness at a scale (thousands of
    urls) that isn't yet a confirmed real use case; revisit if usage data
    later shows it matters. `main` now drains `results` into a plain slice
    before calling `processResults` — the closures pattern from checkpoint
    27 (`handleResult`/`finish`) was deleted entirely as no longer needed.
  - **`hasFailure` return shape:** plain `bool`, deliberately not a struct
    "reserved" for future fields — same discipline already applied to the
    `job.index` field earlier in this project.
  Learned `io.Writer` from scratch as the mechanism that actually makes
  this testable: any type with a `Write([]byte) (int, error)` method
  satisfies the interface, so `os.Stdout` (production) and `bytes.Buffer`
  (tests) are interchangeable from the function's point of view — same
  underlying pattern as `*http.Client` injection in checkpoint 24, applied
  to output instead of network calls.
  **Caught the exact `hasFailure` bug shape a third time**, mid-refactor:
  the first draft only checked `r.Status == statusFailure` inside the
  text-mode branch, leaving json-mode's `Marshal`-based path (no natural
  per-result loop to slot a check into) silently returning `false`
  regardless of actual failures — same root cause as checkpoint 27's bug,
  now clearly a recurring personal pattern worth deliberately watching for:
  a cross-cutting check placed inside only one of several branches. Fixed
  by computing `hasFailure` once, independent of which display branch
  runs.
  Tests written directly against the extracted function (no network, no
  channels): all-healthy text output, all-failure json output (asserting
  both `hasFailure` and the actual unmarshaled JSON content, not just the
  boolean — closing the exact "check one signal, ignore the other" gap
  flagged twice earlier in this project), and a sort-correctness test using
  `json.Unmarshal` to parse output back into real Go values and check
  `Index` fields in order, rather than fragile substring/ordering checks
  on raw text.
- [x] **Checkpoint 28** — `done` — Promoted `checker` into its own
  importable package (`github.com/johncegom/urlwatch/checker`), deliberately
  scoped to *only* the checking logic — `worker`/`job`/pooling stayed in the
  root package as `urlwatch`-the-CLI's own internal implementation detail,
  not part of the reusable surface. Reasoning: an external caller (SiteGuard)
  wants "check one url, get a result," not to inherit `urlwatch`'s specific
  channel-based pool design — a caller building its own concurrency
  architecture shouldn't be forced through this project's.

  This resolved the `job`-coupling risk flagged during the planning review
  without needing options (a) or (b) at all — `checkURL` never actually used
  more than `url`/`index` from `job`, so it reverted to plain
  `(url string, index int, cfg CheckerConfig)` parameters, and `job` never
  needed to leave `worker.go`. Real distinguishing test applied: bundle
  into a struct only when the *receiving function* consumes the fields
  together and expects them to grow together for *its own* purposes
  (`CheckerConfig` passes this test — `checkURL` reads both fields
  directly and both were already flagged as likely to grow); `job` failed
  this test — `checkURL` only ever wanted two primitives out of it, and
  future growth on `job` was more plausibly worker/scheduling concerns
  `checkURL` would never touch anyway.

  Sequenced deliberately as two isolated steps, not one combined change:
  1. **`checkURL` return-value refactor first**, still inside `package
     main`, to avoid two simultaneous unknowns (signature change vs.
     package-boundary change) if something broke. Converted every
     `results <- newCheckResult(...); return` call site (5 total, across
     the retry loop's branches) to `return newCheckResult(...)`. `worker`
     updated to call `checkURL` directly and push the returned value into
     `results` itself — `checker` no longer knows a channel-based pool
     exists at all.
     - Collapsed the "make a channel, call checkURL, receive from
       channel" 3-line pattern to a single `result := checkURL(...)` call
       across every `TestCheckURL_*` test.
     - `FuzzCheckURL` needed a structurally different fix, not just a
       collapse: its channel+`select`+timeout was built to catch a hung
       *concurrent* `checkURL`, but a plain synchronous return means a
       hang blocks the very line that would create the timeout race —
       the `select`/`time.After` become unreachable dead code, the same
       "looks like protection but isn't" shape as the old `isBlockedHost`
       dead code from checkpoint 24. Fixed by wrapping the `checkURL`
       call in its own `go func()`, restoring genuine concurrent racing
       between the call and the timeout — a goroutine used specifically
       to make a synchronous call time-boundable from a test, not for
       real throughput.
  2. **Package/directory move second**, once step 1 was fully green —
     `checker/checker.go` + `checker/checker_test.go`, `package checker`.
  Export-surface decisions, each reasoned through rather than exporting
  everything or nothing by default:
  - `CheckURL`, `CheckResult`, `CheckerConfig`, `NewProductionConfig`,
    `CheckStatus` + its three status constants — exported.
  - `CheckerConfig`'s `Timeout`/`Client` fields exported specifically to
    solve a real cross-package test need (`worker_test.go`, now outside
    `checker`, needs to build a config with a plain unguarded client) —
    considered and rejected exporting a `NewTestConfig` constructor
    instead, since a test-labeled function in a production package's
    public API is a real smell; exported fields also happen to give real
    future consumers (SiteGuard) the ability to build fully custom configs,
    not just the one default `NewProductionConfig` provides — both existed
    side by side rather than one replacing the other.
  - `SafeClient` exported as a package-level **variable**, not a function —
    caught a subtle mistake before writing it: exporting it as
    `NewSafeClient()` would mean every external caller rebuilds a fresh
    `Transport`/`Dialer`/`Guardian` from scratch per call, the same
    "recomputed unnecessarily" pattern already fixed once in checkpoint 26.
  - `classify`, `newCheckResult`, `newSafeClient`, `newTestConfig`,
    `doOneAttempt` stayed unexported — genuinely internal, nothing outside
    `checker` calls them.
  - Real correction to an over-generalized rule mid-checkpoint: "exported
    struct → all fields exported" is wrong; `CheckResult`'s fields all
    happened to need exporting (for JSON, checkpoint 27), but the actual
    rule is export only the fields something outside the package needs to
    touch — a struct can legitimately mix exported and unexported fields.
  Verified end to end: real binary behavior unchanged (`-file` and `-json`
  both), `go vet ./...` and `gofmt -l .` clean, `go test ./... -race`
  passes for both packages independently (`github.com/johncegom/urlwatch`
  and its `/checker` subpackage both discovered and reported separately —
  concrete proof the boundary is real, not just a directory reorganization).
  `govulncheck` deferred to CI (already wired in since checkpoint 25) due
  to local hardware constraints — will run for real on the next push.

**Phase 9 (SiteGuard reuse-readiness) complete — checkpoints 26, 26a, 27,
27a, 28 all done.** Per the review-trigger note above, a full health check
(`build`, `test -race`, `govulncheck`, `gofmt`, coverage) is worth running
once via CI before starting Phase 10.
- [x] **Checkpoint 29** — `done` — **Moved up out of sequence**, ahead of
  Phase 8/9, once test coverage reached a level worth protecting
  automatically. `.github/workflows/ci.yml` added, running (in order,
  fail-fast) `go vet`, a `gofmt` diff check, `go test -race -v`
  (includes the fuzz corpus from checkpoint 21 as permanent regression
  cases), then `go build`. Refined after the first draft with three real
  improvements: `permissions: contents: read` (least-privilege, limits
  blast radius if the workflow or an action it depends on is ever
  compromised), `go-version-file: go.mod` (CI's Go version can never drift
  from what `go.mod` declares), and `test -z "$(gofmt -l .)"` (a simpler,
  more portable fail-on-output check than the original `diff <()` trick).
  Originally deferred `govulncheck` (checkpoint 25) via a `TODO` comment —
  that gap closed for real once checkpoint 25 surfaced an actual CVE;
  `govulncheck ./...` is now a permanent CI step. `-json` output testing
  (checkpoint 27) remains a `TODO`.

  > Note for later, not urgent: CI currently runs as one sequential job
  > (fail-fast, minimal runner overhead — right call while the test suite
  > runs in ~1s and it's a solo project). Revisit splitting into separate
  > jobs per check (lint/vet/test) once SiteGuard has multiple contributors
  > wanting per-check PR status, a slower test suite where fail-fast
  > ordering costs more than parallel runner overhead saves, or a build
  > matrix across Go versions/OSes.

---

## Phase 10 — MCP server (future, motivated by agentic toolchains)
Outcome: expose `urlwatch`'s checking capability as a tool other AI agents
or products can call directly, distinguished from a naive "give an agent a
raw HTTP fetch tool" approach by the SSRF/retry/input-hardening work
already built in Phases 7–9.

Deliberately sequenced *after* checkpoints 27 and 28, not as a detour from
them — both turn out to be direct prerequisites, not separate work:
checkpoint 27's structured output is what an MCP tool response needs
instead of human-readable text lines; checkpoint 28's package promotion is
what lets an MCP server process import `checker`/`worker` directly instead
of shelling out to the CLI binary.

- [ ] **Checkpoint 30** — `pending` — Design the MCP tool schema (what
  parameters an agent can pass — url list? single url? worker count/timeout
  overrides?) and response shape, reusing checkpoint 27's JSON
  `checkResult` structure rather than inventing a second format.
- [ ] **Checkpoint 31** — `pending` — Implement the MCP server itself as a
  thin adapter over the already-promoted `checker`/`worker` package
  (checkpoint 28) — deliberately kept thin, since the safety-critical logic
  should already exist and be tested, not be reimplemented here.
- [ ] **Checkpoint 32** — `pending` — Separately, and explicitly not a code
  problem: discoverability and trust. Distribution (an MCP registry
  listing, documentation, examples) and trust-building (usage, visible
  maintenance, a track record) are a genuinely different kind of work from
  the engineering above — noted here so "build it well" isn't mistaken for
  "get it adopted," which requires its own separate effort.

---

## Current file structure
```
urlwatch/
├── go.mod
├── README.md            — usage, output format, exit codes, status categories
├── main.go              — entry point: orchestration, processResults, print/exit
├── main_test.go          — processResults unit tests (checkpoint 27a)
├── worker.go             — job, worker (select loop over jobs / ctx.Done())
├── worker_test.go        — concurrent pool tests, builds checker.CheckerConfig
│                          directly with http.DefaultClient for local-server tests
├── input.go              — parseFlags, cliConfig, readURLs
├── input_test.go         — readURLs tests (t.TempDir-based)
├── checker/               — promoted, importable package (checkpoint 28)
│   ├── checker.go        — CheckStatus, CheckResult, CheckerConfig, SafeClient,
│   │                       NewProductionConfig, classify, checkURL (pure,
│   │                       returns CheckResult — no channel dependency)
│   └── checker_test.go   — unit + httptest.Server integration tests, fuzz target
├── testdata/fuzz/         — saved fuzz corpus from checkpoint 21, replays as
│                          regression cases on every `go test`
└── .github/workflows/
    └── ci.yml            — vet, fmt check, race-enabled tests, govulncheck, build
                             (already picks up checker/ automatically via ./...)
```

## Review trigger before Phase 10 starts
Phase 9 (SiteGuard reuse-readiness) is the last phase Phase 10's MCP work
directly depends on. Before starting checkpoint 30, run a full health check
as an explicit gate, not just trust that individual checkpoints passed in
isolation: `go build`, `go test ./... -race -v`, `govulncheck ./...`,
`gofmt -l .`, and a fresh `go tool cover -func=cover.out` pass. This is
cheap (CI already runs all of it) but worth doing deliberately as a named
checkpoint-boundary review rather than assuming Phase 9's individual green
checkmarks compose into a genuinely healthy whole.

## Core mental models earned so far
- A goroutine does not block its caller — `main` exiting kills everything.
- Channel direction types (`chan<-`, `<-chan`) are a compile-time promise
  about how *this function* uses a channel, not a property of the channel
  itself.
- Sending a value is a handoff (one receiver); closing a channel is a
  broadcast (every receiver notified at once) — this is why `ctx.Done()` can
  stop many workers simultaneously.
- Pool size is chosen for safety headroom, not maximum speed.
- Context lifetime scope must match the thing it's timing — per-request
  contexts live inside the function doing that request, not shared globally.
- Functions return information; callers decide what to do with it (errors,
  warnings) — a function shouldn't print, exit, or retry on its own.
- Batch operations isolate per-item failures instead of letting one bad item
  discard the whole batch.
- Channels should carry structured data, not pre-formatted strings — decide
  *what happened* where the work happens, decide *how to display it* only
  once, at the edge (`main`).
- Constructors in Go are plain functions (`newX`), not methods — a method
  needs an existing instance to call it on, which doesn't exist yet during
  construction.
- A summary exit code and per-item reporting are separate concerns: every
  item still gets checked and printed regardless of exit code, and the exit
  code itself must only be decided after the full batch is done — deciding
  it mid-loop (e.g. `os.Exit` inside a `case`) silently discards remaining
  results.
- Pure functions (no network, no time, no channels) are the cheapest tests
  to write and the most valuable to have — extract logic out of I/O-bound
  code specifically to make it testable in isolation.
- Test the boundaries of a condition, not comfortable values in the middle
  of each branch — edges are where off-by-one bugs actually hide.
- A passing test suite only proves what it actually exercises: single-
  goroutine calls to a function don't prove the function is safe when many
  goroutines call it concurrently — `-race` needs a genuinely concurrent
  test to have anything to catch.
- A test coupled to a callee's internal hardcoded constant (e.g. sleeping
  550ms to trigger a hardcoded 500ms timeout) is fragile and silently
  slow — the fix is making the value a parameter, not adding a comment.
- A `defer` inside a loop queues up, it doesn't fire per-iteration — it all
  runs at once when the enclosing function returns. Extracting the loop
  body into its own small function gives `defer` a function boundary that
  actually matches "once per attempt."
- Before assuming which stdlib call fails on bad input, check empirically —
  Go's `url.Parse`/`NewRequestWithContext` are lenient about malformed
  strings; the real failure often surfaces one layer later, at `Do()`.
- No single technique proves correctness. Coverage shows what's untested,
  not that what's tested is right. Fuzzing finds inputs a human wouldn't
  think to write by hand. `-race` catches unsafe concurrent memory access,
  not wrong values. Each layer covers a different failure class; none of
  them substitute for a written-down contract of what the function promises
  (e.g. "exactly one result per call, always").
- Coverage counts *lines*, not *scenarios* — two tests with different
  intent (different error source) can hit the identical line and add zero
  new coverage. A flat coverage number after adding a test is a real
  signal worth investigating, not something to shrug past.
- Always regenerate a coverage profile before trusting a re-check —
  `go tool cover` just reads whatever `cover.out` currently contains.
- Fuzzing complements hand-written tests rather than replacing them: seeds
  come from the tricky cases you already found by reasoning; the fuzzer's
  job is finding the ones you didn't think of. Its output becomes a
  permanent, growing regression corpus, not a one-time check.
- A `sync.Mutex` protects a shared resource (like a map) from concurrent
  read/modify/write from multiple goroutines — `x++` is really three steps
  (read, add, write), and two goroutines interleaving those steps loses
  updates or, for maps specifically, can crash the runtime outright. `-race`
  doesn't distinguish production code from test-harness code — a test
  server's own shared state needs the same protection as the code it's
  testing, or the race report points at the wrong place.
- When capping a resource, cap the thing you actually care about, not a
  proxy for it — file *size* and url *count* are correlated but not the
  same protection (many short lines defeat a size-only cap; padded query
  strings defeat a count-only cap). A count cap should count what survives
  filtering and becomes real work, not raw input volume.
- SSRF: a trusted internal tool relaying an untrusted url is a legitimate
  messenger carrying an attacker's request across a network boundary that
  was built to keep the attacker out — the boundary only ever sees "request
  from a trusted source," with no visibility into who ultimately wanted it.
- Check-then-connect security checks have a timing gap (DNS rebinding):
  the address validated at check time isn't guaranteed to be the address
  actually connected to moments later. The fix is validating the exact
  resolved IP at the moment of connection (a dialer's `Control` hook), not
  a smarter pre-check.
- Distinguishing failure messages by cause (blocked vs timeout vs refused)
  leaks information — an attacker can use the differences alone to map
  what exists on a network without ever seeing real data. Collapsing
  failure messages toward one generic shape is a deliberate security
  choice, not a loss of debuggability by accident.
- Parameter creep (a function accumulating more and more individually
  threaded arguments) is a real signal, not just friction — bundle related
  parameters into one config struct with named fields. Different call
  sites (production vs test) get different constructor functions building
  the *same* struct type, not different types — otherwise a test stops
  being proof about what production code actually uses.
- Dead code that looks like an active safety check (defined but never
  wired in) is worse than no code at all in security-relevant paths —
  delete it rather than leave it as a false signal to future readers.
- `govulncheck`'s call-graph analysis proves a vulnerable function is
  *reachable* from your code, not that the specific vulnerable *branch*
  inside it ever actually runs — that depends on runtime configuration
  data-flow analysis alone can't always see. Worth fixing anyway when the
  fix is cheap, rather than relying on "probably not exploitable given
  current config" as a long-term guarantee.
- A stdlib vulnerability is fixed by upgrading the Go toolchain itself, not
  by editing `go.mod`'s `require` block — those two are different kinds of
  dependencies (the compiler/runtime vs. imported modules).
- Go's Go 1 Compatibility Promise makes patch and minor version upgrades
  low-risk by design — patch releases are fixes only, no behavior changes;
  minor releases are almost always additive. A CI pipeline that runs the
  full test suite against a version bump is what makes staying current
  *cheap*; skipping upgrades to "avoid risk" just defers many small, safe
  bumps into one eventual large, risky one.
- Choosing a "maximum" for something is judgment grounded in real reference
  points (industry conventions, comparable tools' defaults), not a formula
  — first name precisely what's actually being protected against, since
  the wrong framing (protecting the local machine vs. protecting a remote
  target) leads to a different, wrong number.
- An information-hiding rule learned in one context (don't reveal why a
  remote target check was blocked, to avoid leaking data to an untrusted
  attacker) doesn't automatically transfer to a different context (a local
  operator's own config error) — check who the actual audience is before
  reapplying a security rule.
- An untested boundary doesn't always fail loudly. `-workers 0` didn't
  crash — it silently did zero real work while exiting successfully,
  which is worse than a crash because it looks like correct behavior. Trace
  degenerate inputs (0, negative, empty) through the actual control flow
  rather than assuming an upper-bound check alone covers "invalid input."
- Functions are values in Go — a variable can hold "which function to
  call," decided once, then invoked repeatedly in a shared loop. Useful
  exactly when two code paths share a loop structure but need genuinely
  different bodies, without mixing both bodies' logic into one branching
  block.
- `encoding/json` (and Go's reflection-based stdlib generally) can only see
  exported (capitalized) struct fields — same visibility boundary concept
  as directional channel types, just applied to struct fields instead of
  channel operations. Struct tags then control the external JSON key name
  separately from the Go field name.
- JSON has no native concept of units — a bare number can't say "this is
  milliseconds" on its own. Encode the unit into the field name itself
  (`latencyMs`, not `latency`) rather than relying on documentation someone
  might not read; raw numeric types are still easier for a downstream
  consumer to compute with than a formatted string they'd have to parse
  back apart.
- A cross-cutting check (like "did anything fail") doesn't belong inside a
  branch-specific code path (like one of two output-format handlers) — if
  it's only implemented in one branch, the other branch silently loses it,
  exactly the shape of bug that broke `-json`'s exit code here.
- The same bug shape can recur even after fixing it once — a refactor can
  reintroduce it in a new location if the underlying habit (a
  cross-cutting check placed inside a single branch) isn't consciously
  watched for on every subsequent change, not just the one that first
  revealed it.
- `io.Writer` is a one-method interface (`Write([]byte) (int, error)`); any
  type satisfying that shape is interchangeable to code that depends on
  the interface rather than a concrete type — this is what makes a
  function testable without touching the real terminal, same dependency-
  injection idea as swapping `*http.Client` implementations.
- Deferring a complexity cost (like streaming output) until real usage
  data justifies it, rather than building for a scale that's only
  theoretically possible, is a deliberate, defensible engineering choice
  (YAGNI) — not laziness, as long as the simpler path is revisited if
  evidence later shows it's needed.
- Bundle parameters into a struct only when the *receiving function*
  consumes them together and expects them to grow together for its own
  purposes — not just because two values happen to travel together
  somewhere in the codebase. A type failing that test (like `job` for
  `checkURL`) should stay as plain parameters, or stay local to whatever
  actually owns it.
- Exporting a struct doesn't require exporting every field — export only
  what needs to cross the package boundary; a struct can legitimately mix
  exported and unexported fields.
- A wrapper function that rebuilds an expensive shared resource (an HTTP
  client, a dialer) on every call is worse than exporting the
  already-built value directly as a variable — same "don't recompute what
  doesn't change" lesson as a loop-invariant computed once outside a loop.
- A concurrency-based safety mechanism (a goroutine + timeout racing a
  channel) stops actually protecting anything the moment the code it
  wraps becomes purely synchronous — the timeout branch becomes
  unreachable dead code sitting after a call that must finish before
  anything after it can run. Re-wrapping the call in its own goroutine is
  what restores genuine concurrent racing between "did it finish" and
  "did it time out."

**Phase 8 (Security hardening) complete — checkpoints 23–25 all done.**