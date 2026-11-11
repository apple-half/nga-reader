# GitHub Updates Implementation Plan

> **For agentic workers:** Use superpowers:executing-plans; one independent final review, record RED/GREEN evidence in the private ledger.

**Goal:** User-confirmed signed GitHub updates for portable Windows and macOS NGA Reader.
**Architecture:** A strict catalog/manifest layer supplies the pinned MyGo updater; native UI coordinates checks/install/relaunch. Offline signing keeps the key off GitHub; CI stages and then verifies/finalizes the release.
**Tech Stack:** Existing Go1.27.3/MyGo0.8.3, stdlib HTTP/JSON/Ed25519/tar/zip, existing Python release orchestration.
**Spec:** ../specs/2026-11-11-github-updates-design.md

## Global Constraints

- Repository apple-half/nga-reader; Windows amd64, Darwin arm64/amd64; preserve portable data.
- Startup check10seconds then24hours; default on, manual install only, no checks in demo.
- Ed25519 signing key stays outside Git and CI; MyGo-compatible signed archives plus signed metadata.
- Preserve write-pending records; no install/restart while sending, no privileges or security bypass.

## Review Focus

- Newest release missing/wrong-platform manifest: skip without downgrading or reading source-code ZIPs.
- Signed old archive with altered version/URL: metadata verification must fail.
- Windows updating renamed executable; Mac read-only/translocated bundle: safe supported behavior.
- Restart launches before directory lock release: old process must release ownership first.
- Failed save/active submission/closing window during install: protect user content and transaction.

### Task1: Signed release protocol and catalog
**Files:** internal/appupdate/{version,manifest,catalog,archive}.go and *_test.go; cmd/update-release/main.go.
**Interfaces:** Manifest, Release, Offer; VersionCompare; Client.Latest(ctx,current,target); Source implementing MyGo.UpdateSource; Sign/Verify release directories and archives.
- [ ] Write failing tests for version/metadata authenticity, hostile/missing assets and safe archive conversion; observe RED.
- [ ] Implement strict repo-bound catalog, metadata signature, tar/ZIP conversion and offline signer/verify CLI; observe GREEN, commit.

### Task2: Native update lifecycle
**Files:** updates.go, view_updates.go, updates_test.go; app.go/main.go, local/state.go, compose.go, view.go/view_settings.go.
**Interfaces:** updateEngine.Check, updateOffer.Install/Info; reader.checkUpdates/installUpdate/restartUpdate/startUpdateChecks.
- [ ] Test settings migration/default, prompt behavior, busy/failure/progress, save-before-install, no forum-write interruption and ownership release before restart; observe RED.
- [ ] Bind MyGo engine/source and native UI, persist preference/check time, lifecycle cleanup; GREEN and render previews, commit.

### Task3: Staged signing release and validation
**Files:** packaging/publish.py/package.py/test_publish.py, workflow; mygo.json/resources/update-public.txt; docs/help/notes and version0.1.7.
- [ ] Test stage-only publication and refusal to finalize absent/mismatched signatures; archive privacy; RED then GREEN.
- [ ] Generate local key outside repo, embed public key only, stage native CI ZIPs, offline-sign trusted artifacts, verify and finalize with CI; preserve old releases.
- [ ] Run full tests/vet/Node/Python/actionlint/privacy scan; independent final review and regression fixes.
- [ ] Verify actual signed update installation in isolated directory, public GitHub catalog and all downloaded release assets; report bootstrap/manual verification boundaries.
