# Reading Experience Implementation Plan

> **For agentic workers:** REQUIRED SUB-SKILL: Use superpowers:executing-plans to implement this plan task-by-task. Steps use checkbox (`- [ ]`) syntax for tracking.

**Goal:** 落地前轮体验建议与自动续页，完成本地原生交互验证和 Windows 免安装预览包。用户最新指示：不提交 GitHub，等待后续需求。
**Architecture:** 在 reader 中增加有界连续段、阅读导航、正文查找及板块管理状态；复用 RequestGate、阅读锚点、Markdown 和临时图片窗口。
**Tech Stack:** Go 1.27.3, MyGo 0.8.3，无新依赖。
**Spec:** docs/superpowers/specs/2026-11-12-reading-experience-design.md

## Global Constraints

- 只在正式 checkout 开发；不碰 D 盘数据，不改写旧标签和发布资产。
- 普通图片默认不加载；Markdown.LoadImage 保持 nil；表情只走内置白名单。
- 连续保留最多 10 页或 200 楼；距底 180 DIP 触发；每次一个请求，失败显式重试。
- 记录可见楼层真实页码；账号／话题／筛选变更使旧请求无效；只看楼主不覆盖全帖进度。
- 窗口正常尺寸限制 980..3840 × 620..2160；导航最多 50 位置。
- 发布说明短文与网页说明分离，密钥不进入源码／日志／资产。

## Review Focus

- 短页初始可见底部不得同一帧循环拉取；重复页／未知页数必须结束。
- 连续阅读后调整字号／宽度、返回旧帖，进度应指向真实页与楼层。
- 同页刷新失败后旧正文仍可阅读，换目标不得出现旧正文。
- 板块移除或切换账号时编辑器与并行请求不应产生跨目标提交。
- 普通图、编辑预览、搜索和图片切换均不得暗中抓取附件。

### Task 1: Responsive layout and window memory
**Files:** Create layout.go/layout_test.go; modify view.go, view_reader.go, main.go, internal/local/state.go, view_settings.go.
**Interfaces:** layoutFor(width,sidebar,topics float32,hidden bool) readerLayout; reader.resetLayout(); state.SidebarHidden, WindowWidth/Height.
- [ ] Write tests: at 980 with 280/450 preferences reader >=420 and toolbar stays in window; collapse/restore; persisted window sizes normalized.
- [ ] Run focused tests; Expected: RED missing behavior.
- [ ] Implement sizing, toolbar simplification, titlebar controls, normal size memory.
- [ ] Run full Go suite; Expected: PASS. Commit.

### Task 2: Continuous reading and non-disruptive refresh
**Files:** Create continuation.go/continuation_test.go; modify actions.go, app.go, reading.go, view_reader.go, view_pagination.go, pagination.go, demo.go, state.go.
**Interfaces:** renderedPost.page; readingRow.Page; reader.continuation; appendNextPage(), observeContinuation(viewport,last ui.Rect), visibleThreadPage(); resetContinuousPage(); continuity uses threadGate.
- [ ] Write tests: append once preserves Y and old posts, duplicate/empty stops, failed retry, bounded segment, stale topic/account/filter ignored, page-aware restore and save, filtered progress unchanged, refresh failure keeps rendered content.
- [ ] Run focused tests; Expected: RED.
- [ ] Implement state machine and footer, page markers, exact page mapping, stable scroll key, refresh overlay, default enabled setting.
- [ ] Run full Go suite; Expected: PASS. Commit.

### Task 3: Board management and list clarity
**Files:** Create board_manager.go/board_manager_test.go; modify app.go/view.go/view_settings.go/pagination.go.
**Interfaces:** boardManagerState; openBoardManager(), editManagedBoard(action string), boardManagerView(c); all changes save candidate before state.
- [ ] Tests: rename/reorder/remove through native controls, last board protected, save error atomic and active board selection valid, IDs hidden, clear filter works.
- [ ] Focused RED; implement manager and list labels; full suite PASS; commit.

### Task 4: Reading back/forward and continue last
**Files:** Create navigation.go/navigation_test.go; modify app.go/actions.go/reading.go/view.go/view_reader.go.
**Interfaces:** readingLocation{Topic,Page,OnlyAuthor,Position}; reader.navigation; captureLocation(), navigateHistory(delta int), continueLastReading().
- [ ] Tests: A→B→back keeps floor and page, forward, new branch truncates, 50 bound, UID change reset, startup continue is account scoped.
- [ ] Focused RED; implement history capture before navigation and toolbar/shortcuts; full suite PASS; commit.

### Task 5: Loaded-body search
**Files:** Create reader_search.go/reader_search_test.go; modify app.go/view.go/view_reader.go/pagination.go.
**Interfaces:** readerSearchState; openReaderSearch(), searchMatches(), moveSearch(delta), observeSearch(c,viewport,rows).
- [ ] Tests: Unicode/case, quote/body included, current loaded pages only, native jump and result count, Escape, no mutation of source/attachments.
- [ ] Focused RED; implement search bar and floor jumps; full suite PASS; commit.

### Task 6: Composer preview and save feedback
**Files:** modify compose.go/view_compose.go; create compose_preview_test.go.
**Interfaces:** composerState.Preview,SavedAt; preview reuses MarkdownOnDemand and safe reader options.
- [ ] Tests: edit→preview renders quote/link/emote without changing draft/transport; save status only success; failure keeps text; existing uncertain send guards preserved.
- [ ] Focused RED; implement tabs and saved-time label; full suite PASS; commit.

### Task 7: On-demand image gallery and final polish
**Files:** modify images.go/app.go/help.go/resources/usage.md; create image_gallery_test.go.
**Interfaces:** reader.pictureItems,pictureIndex,pictureZoom; imageGallery(address), navigatePicture(delta), setPictureZoom(factor).
- [ ] Tests: all loaded posts extract safe images, dedup/index clicked image, next/previous bounds, no prefetch, reset zoom, unsupported targets rejected.
- [ ] Focused RED; implement gallery actions/counters/zoom, manual and shortcut hints; full suite PASS; commit.

### Task 8: Review, local portable preview and evidence
**Files:** local version metadata and docs/validation-0.1.12.md; public download links and release feed remain at published 0.1.11.
- [ ] Native previews 980/1280/wide across themes and continuation states; full Go/vet, Node, Python packaging, version/privacy/diff checks; Expected: PASS.
- [ ] Whole-branch fresh reviewer; one Critical/Important fix pass with RED→GREEN; ledger all decisions and deferred minors.
- [ ] Build/verify local 0.1.12 Windows portable preview; optional local Mac cross-compilation. Keep all remote branches/tags/assets/feeds untouched. Update local validation and AGENTS with unfinished release work; Expected: local package verified, no GitHub writes.
