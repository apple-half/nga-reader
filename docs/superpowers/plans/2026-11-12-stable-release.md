# Stable Release Implementation Plan

> **For agentic workers:** REQUIRED SUB-SKILL: Use superpowers:executing-plans to implement this plan task-by-task. Steps use checkbox (`- [x]`) syntax for tracking.

**Goal:** Fix both emoticon defects, integrate the user-specified office-worker cartoon logo, publish verified0.1.13 stable portable release.
**Architecture:** Public MyGo rich-text/selection callbacks with emote source mapping; structural animation detection; one embedded PNG; existing signed-release pipeline changed to stable.
**Tech Stack:** Go1.27.3/MyGo0.8.3, Python packaging, GitHub Actions, built-in image generation. No new dependency.
**Spec:** docs/superpowers/specs/2026-11-12-stable-release-design.md

## Global Constraints

- Sole checkout and current feature branch; latest user authorizes GitHub push and stable release. Never overwrite public tags/assets or touch D drive data.
- Preserve whitelist/offline emotes, bounded dimensions and GIF budgets; Markdown.LoadImage nil; ordinary images remain click-only.
- User corrected Log to Logo: no logger work. Final image subject is chubby middle-aged man with glasses and plaid shirt, not a fish.
- Signing private key stays local and out of logs/Git/assets. App notes short, webpage notes separate.

## Review Focus

- Partial and cross-paragraph selections, repeated visually identical emotes and Unicode must copy the correct original code without placeholders.
- Quotes, bold text, links and code adjacent to emotes must retain readable layout and safe click behavior.
- Literal URIs in code, image alt text or plain text must not be classified as drawn animated objects.
- Native icon should retain alpha and recognizable face/monitor at16/32/64 pixels; Windows/Mac packages must use same source.
- Stable release flag, Latest, signed feed and all three asset hashes must agree before claiming publication.

### Task 1: Clipboard and animation fixes
**Files:** Create emote_copy.go/emote_copy_test.go; modify emoticons.go, app.go, view_reader.go, view_compose.go as needed.
**Interfaces:** reader.bodyMarkdown(c,source) ui.Element; emoteCopyState maps rendered paragraph IDs/object rune positions to NGA source codes; structural emote detector.
- [x] Add native select-all/partial/cross-paragraph/quote/repeated-emote tests and code-false-animation tests; run RED.
- [x] Implement minimal public-API rendering/copy mapping and structural animation detection; no external image loader.
- [x] Run focused and full Go suite GREEN; commit and ledger.

### Task 2: Cartoon office-worker logo
**Files:** packaging/icon.png, icons.go, view.go/view_reader.go, README.md, docs/logo-design.md, app_icon_test.go.
**Interfaces:** windowIcon returns embedded canonical PNG; native UI image helper uses same bytes. MyGo packs platform icons from packaging/icon.png.
- [x] Generate final user-specified man/glasses/plaid icon, copy into project; test decodability/alpha/size/source consistency before embedding change.
- [x] Replace generated64px geometry with embedded asset; inspect native16/32/64 previews and packaged metadata; tests GREEN; commit.

### Task 3: Stable publisher and release documentation
**Files:** packaging/publish.py/package.py/test_publish.py, release.yml, DOWNLOAD.md/README.md/SECURITY.md, notes and validation.
**Interfaces:** stage as stable draft, finalize with prerelease=false and Latest only after complete verification; feed wrapper reflects published state.
- [x] Add publisher behavior test starting from prerelease draft, require stable Latest final state; RED then implement GREEN.
- [x] Update0.1.13 full web release notes and short App notes, manual, download pointers and version metadata; run Go/vet/Node/Python/version/privacy/diff checks.
- [x] One fresh whole-branch review; one Important/Critical fix pass RED→GREEN and full suite. Preserve reviewer rulings.

### Task 4: Verify native builds, sign and publish
**Files:** public validation docs and local QA archive/package outputs.
- [x] Fast-forward main, annotated v0.1.13 tag, atomic push; observe native CI3 platforms stage draft.
- [x] Verify downloaded CI ZIPs against artifact/draft digests and package checks, offline sign update assets, verify signatures and compact notes.
- [x] Upload six signed assets, finalize through verified stable pipeline, observe success, re-download/verify public10 assets and feed. Save3 final portable ZIPs locally, update root AGENTS, archive owned QA, clean temporary binaries.
