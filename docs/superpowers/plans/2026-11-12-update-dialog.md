# Update Dialog Readability Implementation Plan

> **For agentic workers:** Use superpowers:executing-plans inline in the sole authorized GitHub checkout; one fresh whole-branch review before merge. User delegated implementation and publication choices earlier; preserve that authorization.

**Goal:** Make upgrade prompts readable, keep progress/actions visible, publish0.1.9 without changing0.1.8.
**Architecture:** Separate compact signed app notes from full GitHub release notes; reuse native Markdown for bounded changelog content and preserve existing update actions.
**Tech Stack:** Go1.27.3/MyGo0.8.3 and Python stdlib packaging.
**Spec:** ../specs/2026-11-12-update-dialog-design.md

## Global Constraints
- Use existing `work/nga-reader-public` checkout on `fix/update-dialog`, base4c0beb1; no new dependencies or signing format changes.
- Preserve immutable published assets, private signing key, account/draft protections and D-drive app/data.
- Deliver Windows x64, Mac arm64 and Mac amd64 portable ZIPs.

## Review Focus
- Legacy full-page Markdown without a recognized changes heading; empty, very long or HTML-only notes.
- Long errors/reasons and login warnings in compact windows; primary controls and progress must remain visible.
- Native Markdown must not fetch images or execute HTML/code; only deliberate safe links may open.
- Installing/restarting states, disabled actions, keyboard dismissal and repeat checks.
- Short app notes are what gets signed, while the website retains platform download links and full documentation.

### Task1: Render compact native upgrade prompts
**Files:** view_updates.go, update_notes.go, updates.go, updates_view_test.go, testdata/update-notes-0.1.8.md.
**Interfaces:** `compactUpdateNotes(raw string) string`, cached native markdown options in update state; existing update offer/install interfaces unchanged.
- [ ] Add native regression using actual0.1.8 notes, verify raw download Markdown/table is absent and changes visible; progress, errors and actions remain inside980x620 and760x520. Run and watch failure.
- [ ] Implement bounded legacy section selection, native formatting, version emphasis and fixed status/action layout. Run focused and full Go suites; inspect three-theme previews. Expected GREEN.
- [ ] Commit renderer fix.

### Task2: Produce compatible app notes and release0.1.9
**Files:** packaging/update-notes.txt, cmd/update-release/main.go, packaging tests/UPDATES.md, version/download/help/release docs.
- [ ] Add failing test requiring separate short app notes (no table/HTML/URL), and signing CLI default points at it. Expected missing file/default failure.
- [ ] Add short notes, change signing default/docs; bump0.1.9 and update web docs. Expected all Go/vet/Node/Python/actionlint/privacy checks GREEN.
- [ ] One fresh read-only review of whole branch; fix Important/Critical with RED/GREEN. Merge, verify and push new tag.
- [ ] CI builds/stages native ZIPs; compare CI/draft hashes, offline-sign compact app notes, upload six verified assets, CI finalize/index publication.
- [ ] Verify public assets/signatures and actual index offers; save local3ZIPs, report limits and archive/remove owned scratch files.
