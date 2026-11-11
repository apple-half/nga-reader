# Text Posting Implementation Plan

> **For agentic workers:** Use superpowers:executing-plans for native execution of this approved continuation, with one independent final review. Track RED/GREEN evidence in the private progress ledger.

**Goal:** Ship native text new-topic, reply and quote controls in portable NGA Reader.
**Architecture:** Add a scoped posting API and account/target-specific persisted draft state; bind them to a native MyGo composer. Keep all writes user initiated and conservatively retain uncertain results.
**Tech Stack:** Existing Go 1.27.3, MyGo v0.8.3, x/text, stdlib JSON/HTTP; no new runtime dependencies.
**Spec:** ../specs/2026-11-11-text-posting-design.md

## Global Constraints

- Windows x64, macOS arm64/amd64; preserve portable packaging and existing reading behavior.
- Only new/reply/quote; title128 runes and body20000 runes; no image upload.
- Submit with UTF8 input and output8; never automatically retry or replay a submission.
- Require account/target match, preserve ambiguous outcomes across restart, no live test posting.
- Source and release exclude private drafts, cookies, account data and diagnostics.

## Review Focus

- Server returns HTTP200 but incomplete/misleading success data: retain draft as uncertain.
- Login or target changes during prepare/send: never submit under a different account or topic.
- Timeout followed by app restart: require explicit verification before another send.
- Main floor PID0, negative board IDs and quote text containing HTML entities: preserve semantics.
- Small window and Chinese input composition: no clipped buttons or Enter-to-send.

### Task 1: Posting protocol

**Files:** internal/nga/post.go, post_test.go; client.go.
**Interfaces:** PostTarget{Action,FID,TID,PID}; PreparePost(ctx,target)(PostPreparation,error); PublishPost(ctx,preparation,subject,content)(PublishResult,error); SessionUID()int64; PublishError.Uncertain.
- [x] Write failing httptest cases for UTF8/output8, preparation identity/scope, quotes, GB18030, explicit success/refusal, HTML/invalid results, redirects and lost responses without retry.
- [x] Run focused tests and observe RED.
- [x] Implement bounded parsing, isolated non-replaying transport and structured result classification.
- [x] Run package tests and observe GREEN; commit.

### Task 2: Persisted draft/send state

**Files:** internal/local/drafts.go, drafts_test.go.
**Interfaces:** Draft{UID,Target,Subject,Content,Uncertain,UpdatedAt}; LoadDrafts/SaveDrafts(dir,[]Draft); DraftKey; state helper for begin/result/acknowledge.
- [x] Write failing tests for per-account/per-target restoration, limits/corruption handling, uncertain persistence, success removal and duplicate suppression.
- [x] Observe RED, implement using existing atomicWrite, verify GREEN; commit.

### Task 3: Native composer

**Files:** compose.go, view_compose.go, compose_test.go; app.go, view.go, view_reader.go, view_settings.go, demo.go.
**Interfaces:** openComposer(target,label), prepareComposer, sendComposer, saveComposer, closeComposer; use Task1/2 types.
- [x] Write UI/state tests for new/reply/quote entry, preserved board binding, demo no-network, send lock, no Enter submission, prepared quote/draft precedence, signout and uncertain retry confirmation.
- [x] Observe RED; build editor, classification selector, actions and callbacks; test GREEN.
- [x] Render editor previews at1280x820 and980x620 in dark/light/NGA classic; fix visual defects and commit.

### Task 4: Documentation, verification and portable delivery

**Files:** main.go, mygo.json, README.md, SECURITY.md, resources/usage.md, packaging/release-notes.md, workflow dispatch version default.
- [x] Run full Go tests/vet, Node login tests, Python publisher tests, actionlint and private-data scan.
- [x] Complete one independent code review and fix blocking findings with regression tests. Added exclusive data-directory ownership before snapshot load and consistent supplementary-character limits; both regressions observed RED then GREEN.
- [ ] Bump to0.1.6, build/verify Windows and both Mac architectures with existing CI; create new immutable tag and complete public release.
- [ ] Download all three ZIPs, verify CRC/architectures/licenses/signature envelopes/SHA256SUMS; report links and real-account verification boundary.
