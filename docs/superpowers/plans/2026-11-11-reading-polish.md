# Reading Polish Implementation Plan

> **For agentic workers:** Use superpowers:executing-plans in this existing authorized GitHub checkout, one fresh whole-branch review at the end. User delegated decisions and requested results directly; no design/implementation handoff questions.

**Goal:** Fix updater403, add the first three recommended reading features, audit interactions, release0.1.8.
**Architecture:** Signed raw GitHub manifest first with strict fallback; bounded per-account reading records and native scroll anchors; shared native pagination; existing offline signing release pipeline publishes verified update indexes.
**Tech Stack:** Go1.27.3/MyGo0.8.3 and existing Python stdlib packaging.
**Spec:** ../specs/2026-11-11-reading-polish-design.md

## Global Constraints
- Source baseline cd4bb2e7161e11d19d48a1084bbfdb1751268ec0, branch feat/reading-polish. No new dependencies.
- Preserve private signing key and remembered login/drafts; no forum writes or D-drive modifications.
- Three portable platforms; immutable released tags/assets; existing v0.1.7 updater compatibility.

## Review Focus
- Invalid signed feed must fail closed; API403 backoff respected; offline failure not reported as latest.
- Loading/cancel/error/first-layout must not replace a saved reading anchor or mark unseen threads read.
- Account/demo/author-filter boundaries; old settings; invalid finite numbers and history limits.
- Restore after resize/font changes or removed posts; do not snap back while manually scrolling.
- Invalid jump numbers, unknown total, repeated Enter, busy posting/updating, navigation cancellation.

### Task1: Recover update checking
**Files:** internal/appupdate/catalog.go, feed.go, *_test.go; updates.go/view_updates.go; updates/*.json.
**Interfaces:** Client.Latest; fixed FeedURL(target); signed Manifest unchanged; RateLimitError with retry time.
- [ ] RED tests for signed feed with API403, invalid feed rejection, HTTP fallback/no-auth/limits, backoff.
- [ ] Implement feed-first verified offers and friendly errors; seed current signed0.1.7 feed; GREEN full suite; commit.

### Task2: Reading state and native anchor restoration
**Files:** internal/local/reading.go/state.go/tests; reading.go/tests; app.go/actions.go/view.go/view_reader.go.
**Interfaces:** Reading progress/seen helpers scoped by UID, capture/flush, pending restore token, native row handles.
- [ ] RED migration, read/new markers, per-user state, only-author preservation, failed load and native restore/scroll tests.
- [ ] Add bounded validated progress, debounced save, boundary flushing and one-shot PID/floor restoration; GREEN; commit.

### Task3: Pagination and interaction audit
**Files:** pagination.go/view_pagination.go/tests; view*.go/actions.go and usage docs.
**Interfaces:** validated page target, page-jump dialog and first/last actions; Ctrl+L and back-to-top.
- [ ] RED jump bounds/unknown-total/same-page/cancellation and native dialog keyboard tests.
- [ ] Implement controls and audit loading/retry/focus/compact rendering; inspect previews at980x620 for three themes; GREEN; commit.

### Task4: Verified release and result
**Files:** packaging/publish.py/feed helper/test_publish.py; workflow/docs/version0.1.8.
- [ ] RED feed publication tests: only verified published metadata, no private key, immutable assets, failure/retry.
- [ ] Implement existing-token feed update after publication; update help and download docs/version; run required checks.
- [ ] One fresh whole-branch review; fix Important/Critical with RED/GREEN, record remaining limits.
- [ ] Merge/push/tag0.1.8, CI builds/draft, locally sign/upload/finalize, verify public indexes/assets and isolated Windows update; retain clean latest deliverables.
