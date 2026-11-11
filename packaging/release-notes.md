Windows 和 macOS 的首个免安装便携预发布版本。

| 系统 | 下载文件 | 使用方法 |
| --- | --- | --- |
| Windows x64 | `NGAReader-0.1.5-Windows-x64-portable.zip` | 解压整个文件夹，双击 `NGAReader.exe` |
| macOS Apple 芯片（M 系列） | `NGAReader-0.1.5-macOS-AppleSilicon-portable.zip` | 解压后打开 `NGAReader.app` |
| macOS Intel | `NGAReader-0.1.5-macOS-Intel-portable.zip` | 解压后打开 `NGAReader.app` |

这些包不包含安装向导，不需要管理员权限。macOS 最低版本为 12；不强制将 App 复制到 Applications。下方 GitHub 自动生成的 Source code 文件是源码，运行程序请下载上表中的 ZIP。

本版新增正式应用图标、Windows 产品与版本属性、macOS App Bundle，以及 Mac 用户数据目录修正。保留三栏原生阅读、收藏板块导入、NGA经典屎黄色主题、图片按需展示和正文兼容读取。

Windows 设置与加密会话保存在 EXE 同级 `data` 目录；更新时保留该目录。Mac 保存在 `~/Library/Application Support/NGAReader`，避免写入 App 包内。两种系统的加密会话通常不能跨电脑直接复用。

图片默认不加载，点击“展示图片”才打开。Windows 官方登录和图片预览需要电脑已有 WebView2 Runtime。完整说明在软件侧栏“使用说明”和 ZIP 内。

**验证范围与安全状态**

- GitHub Actions 在 Windows x64、macOS arm64 和 macOS Intel 环境运行测试、构建及包结构检查；Mac 检查 ad-hoc 签名和命令行许可输出。这些检查不覆盖真实账号登录、验证码和全部 GUI 操作。
- Windows 程序未做 Authenticode 签名；Mac 只有 ad-hoc 完整性签名，没有 Apple Developer ID 签名或 Apple 公证，系统可能阻止启动。
- 早期 Windows 0.1.2 曾被 360 检出，0.1.2 / 0.1.3 复核未通过，收到的回复未提供具体判定依据。**0.1.5 尚无 360 放行结论，不能认定此前检出为误报。** 被拦截时请保留隔离并向厂商核实，不要关闭系统防护。
- `SHA256SUMS.txt` 用于检查下载内容是否与本次发布一致，不构成安全认证。

本项目采用 MIT 许可证，各依赖的许可原文包含在下载包中。
