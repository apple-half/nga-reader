# Native NGA Emoticons Implementation Plan

> **For agentic workers:** Execute inline with superpowers:executing-plans, TDD and one fresh whole-branch review. Existing user delegation authorizes routine design and GitHub release decisions.

**Goal:** Display official NGA emoticons at bounded inline sizes, release0.1.11.
**Architecture:** Embedded official public data/assets; pure whitelist registry; BBCode and HTML mapping; native MyGo inline bitmaps and bounded GIF animation.
**Tech Stack:** Go1.27.3, MyGo0.8.3, existing standard library only; no added runtime dependency.
**Spec:** ../specs/2026-11-12-native-emoticons-design.md

## Constraints
- Formal checkout only, base93ca62d, branchfeat/native-emoticons; no published-asset overwrites or D-drive/private-data access.
- No runtime emote HTTP, browser engine, script execution or arbitrary file/URL loader; ordinary images stay click-to-show and Markdown.LoadImage nil.
- Exact sizing, GIF budgets and source notices follow spec. Compact App notes separate from web notes and offline signing key never uploaded.

## Review Focus
- Old numeric tags, invalid/unknown tags, code blocks and following text must not be swallowed or confused with strikethrough.
- Arbitrary Markdown/HTML, URLs and lookalike filenames cannot access filesystem/network or acquire built-in-emote privileges.
- Aspect ratio, glyph baseline, line wrapping, quotes, font12～22, long runs, small windows and themes.
- GIF transparent patches/disposal/timing/budgets, shared decoded cache, no idle timer on static/offscreen content, stale reading anchors.
- Embedded resource provenance, binary hashes and bounded decoding; ordinary image, posting and update protections retained.

### Task1: Catalog and safe conversion
- [ ] RED registry completeness/hash/config/lookup/path rejection and BBCode legacy/unknown/quote/code/HTML regressions.
- [ ] Add265official resources and source notices, data-only registry and bounded pure conversion; full Go GREEN; commit.

### Task2: Native renderer and sizing
- [ ] RED inline drawing/size/aspect/cache/baseline, GIF composition/frame bounds/timing, visibility-driven redraw and no external image loads.
- [ ] Implement lazy shared bitmaps, native inline objects and bounded animation, native preview QA at3themes/fonts/min window; Go/vet GREEN; commit.

### Task3: Review and release
- [ ] Version0.1.11, usage/download/source/short App and full web notes; full Go/vet/Node/Python/actionlint/privacy checks.
- [ ] One fresh whole-branch review; fix Important/Critical once via RED→GREEN and green suite; merge/post-merge verify/push new tag.
- [ ] Native CI3ZIPs/draft, wrapper+ZIP digest checks, offline sign and verify compact notes, upload/finalize, re-download all public files and validate live feeds plus isolated Windows SDK update.
- [ ] Retain local portable ZIPs/previews, archive QA and remove only owned scratch/server, sync final public validation docs.
