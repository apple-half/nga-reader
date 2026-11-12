# Board Search Implementation Plan

> **For agentic workers:** Use superpowers:executing-plans inline in the existing authorized checkout, one fresh whole-branch review at the end. User already delegated routine design and release decisions.

**Goal:** Search homepage boards by name and add in native UI, release0.1.10.
**Architecture:** Parse fixed official CDN data without script execution; embedded snapshot + public cache; native picker and existing ordinary board model.
**Tech Stack:** Go1.27.3/MyGo0.8.3, existing x/net and x/text, Python stdlib packaging.
**Spec:** ../specs/2026-11-12-board-search-design.md

## Global Constraints
- Base84ee60a, branchfeat/board-search, sole formal checkout; no new dependencies or published-asset changes.
- Public directory fetch has no forum credentials; fixed HTTPS source, redirect/size/time bounds, no script evaluation.
- No D-drive or account data changes; private signing key stays offline.

## Review Focus
- Collection `stid` sharing a normal `fid` must never be added or renamed as its parent.
- Invalid/oversized/truncated/HTML directory responses must retain a valid cached or embedded catalog.
- Duplicate custom names, persist failure, demo/live transitions and refresh completing after close/shutdown.
- Query Unicode/aliases, result limits, keyboard accidental additions and native narrow-window layout.
- Background refresh must not issue per-key requests, leak session cookies, overwrite shared form errors or pause posting/update protections.

### Task1: Public directory, search and cache
**Files:** internal/nga/directory.go, directory_seed.json, directory_test.go; internal/local/directory.go/tests.
**Interfaces:** DirectoryEntry{ID,CollectionID,Name,Description,Category,Aliases}; ParseBoardDirectory(raw), EmbeddedBoardDirectory(), SearchBoardDirectory(entries,query); Client.BoardDirectory(ctx); local directory cache timestamp/read/write.
- [x] RED parser GB18030/wrapper/nesting/duplicates/collections, query normalization/aliases, public request no-cookie/limits/redirect/cancel and cache corruption/TTL tests.
- [x] Implement fixed-source safe decoding/normalization and bounded catalog, seed from observed public data, local search and cache; full Go suite GREEN; commit.

### Task2: Native picker and local addition
**Files:** board_search.go, view_boards.go, app.go/view.go/view_settings.go/actions.go, native tests, usage docs.
- [x] RED name search/add-and-open/custom-name duplicate, unique Enter/ambiguous Enter/Escape, persist failure, collection guard, no-query-network, cache fallback and demo tests.
- [x] Implement search/manual tabs, bounded virtual results, refresh state and safe add actions; inspect three-theme/narrow previews; Go/vet GREEN; commit.

### Task3: Verify and release
- [x] Version0.1.10, compact App notes, full release/download/help docs; Go/vet/Node/Python/actionlint/privacy checks.
- [x] One independent whole-branch review; fix Important/Critical with RED/GREEN, record limits. Merge/test/push new tag.
- [x] CI native3ZIPs/draft, verify hashes, offline sign/upload/finalize, verify live directory and signed indexes/public assets plus isolated Windows update.
- [x] Retain local3ZIPs and previews, archive records and remove only owned scratch copies.
