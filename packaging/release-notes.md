本版新增原生文字发帖、普通回复和引用回复，继续提供 Windows 与两种芯片的 Mac 免安装 ZIP。

| 系统 | 下载文件 | 使用方法 |
| --- | --- | --- |
| Windows x64 | `NGAReader-0.1.6-Windows-x64-portable.zip` | 解压整个文件夹，双击 `NGAReader.exe` |
| macOS Apple 芯片（M 系列） | `NGAReader-0.1.6-macOS-AppleSilicon-portable.zip` | 解压后打开 `NGAReader.app` |
| macOS Intel | `NGAReader-0.1.6-macOS-Intel-portable.zip` | 解压后打开 `NGAReader.app` |

Mac 最低 macOS 12，不强制复制到 Applications。下方 GitHub 自动生成的 Source code 是源码，运行程序请下载上表中的 ZIP。更新前退出旧版并保留原数据目录。

**发帖与回复**

- 中栏板块名称旁点击“发帖”，填写标题、分类和正文；正文顶部点击“回复”；各层点击“引用回复”。
- 编辑器原生绘制，沿用深色、浅色和 NGA经典屎黄色主题。支持文字和 NGA BBCode，暂不上传图片。Enter 只换行，点击按钮才提交。
- 草稿按账号和目标自动保存。关闭编辑器后，重新点击同一目标即可继续编辑。草稿以明文保存在本机 `drafts.json`，请勿公开分享；最多32份，标题128字、正文20000字，论坛可能有更严格限制。
- 发送期间锁定编辑器与账号切换。连接中断或返回内容无法确认时，保留发送记录并阻止自动重发；先检查原帖或板块，再选择已发表清理，或确认未发表后手动解除保护。审核中会单独提示。
- 论坛要求额外验证、登录或发言权限时会显示提示并保留正文，可打开官方页面处理。

Windows 数据保存在 EXE 同级 `data` 目录；Mac 保存在 `~/Library/Application Support/NGAReader`。图片默认折叠，点击后才加载；Windows 官方登录和图片预览需要已有 WebView2 Runtime。

**验证范围与安全状态**

- 本版使用模拟 HTTP 服务验证发帖协议、中文编码、权限拒绝、异常响应、账号/目标隔离、草稿恢复和防重复发送；使用原生渲染测试检查编辑器。未代真实账号发布测试帖子，特殊板块与验证码流程仍需实际使用核验。
- GitHub Actions 在 Windows x64、macOS arm64 和 macOS Intel 环境运行测试、构建与包结构检查，Mac 另检查 ad-hoc 签名和许可输出；这些检查不覆盖全部真实 GUI 操作。
- Windows 没有 Authenticode 签名；Mac 只有 ad-hoc 完整性签名，没有 Apple Developer ID 签名或公证，系统可能阻止启动。
- 早期 Windows 0.1.2 曾被360检出，0.1.2/0.1.3复核未通过且未提供具体依据。**0.1.6 尚无360放行结论**；功能测试与源码公开不能代替恶意软件鉴定。遇到拦截请保留隔离并向厂商核实。
- `SHA256SUMS.txt` 用于核对本次下载内容，不构成安全认证。包内包含 MIT 与全部依赖许可。
