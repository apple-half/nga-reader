# Selectable Icon Implementation Plan

> **For agentic workers:** REQUIRED SUB-SKILL: Use superpowers:executing-plans inline. Steps use checkbox syntax.

**Goal:** selectable professional work icon and existing cartoon, persistent and honest about static file icons.
**Architecture:** two embedded PNGs with per-style lazy bitmap cache; state appIcon normalized to work/office; public Window/Tray/Dock APIs applied at ready and on selection.
**Tech Stack:** existing Go/MyGo, built-in image generation; no new dependency.
**Spec:** docs/superpowers/specs/2026-11-12-selectable-icon-design.md

## Global Constraints
Sole formal checkout; preserve cartoon and all user data; no D drive operations; no published asset/tag changes, new release, signing key use or new dependency. Native selector is distinct from static Explorer/Finder/shortcut icon. Demo selection not persisted.

## Review Focus
- Missing or invalid old settings must default safely without losing unrelated data.
- A failed settings write must not leave UI selection ahead of persisted state.
- Demo selection must not leak into real settings after leaving demo.
- Every runtime surface/startup uses selected bytes; static canonical package uses work.
- Added settings controls remain reachable at980x620;16px mark and both themes readable.

### Task1: Asset, state and native selector
**Files:** icons.go, app_icon_test.go, internal/local/state.go/state_test.go, view_settings.go, view.go, view_reader.go, main.go, packaging/icon.png/icon-office.png; docs/logo-design.md and resources/usage.md.
**Interfaces:** State.AppIcon string work/office; appIconPNGFor(style) []byte, appLogoFor(c,size,style); reader.selectAppIcon(style), applyAppIcon(). Existing windowIcon remains canonical packaging fallback.
- [ ] Add settings roundtrip/unknown/missing default and native selector restart/failing-save/demo tests; run RED.
- [ ] Generate original monochrome work PNG with built-in tool; preserve cartoon sibling; implement normalization, cached previews and native selected icon updates without static binary edits.
- [ ] Run focused and full Go/vet/Node/Python checks, native preview and Windows build/license check; Mac cross-compile. Document exact scope and generation prompt.
- [ ] Commit locally, one fresh whole-change review; one Important fix pass with RED→GREEN and green suite. Keep public release unchanged and provide local preview.
