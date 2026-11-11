# NGA Reader · 工作台

用 Go 和 [MyGo](https://github.com/egoist/mygo) 编写的 NGA 桌面阅读器。采用类似 Codex 的三栏布局：左侧板块，中间话题列表，右侧正文与回复。

列表和正文由原生 UI 绘制，通过 Go 直接读取论坛数据。官方登录与点击后打开的图片预览使用系统 WebView。

这是独立的非官方项目，与 NGA、OpenAI / Codex 均无隶属关系。

## 界面预览

![三栏深色界面](docs/images/three-column.png)

![NGA经典屎黄色主题](docs/images/nga-classic.png)

截图来自离线演示，账号和帖子内容均为样例。

## 功能

- 三栏阅读、帖子与回复独立分页、只看楼主、专注阅读。
- 导入 NGA 账号收藏的普通板块；本地收藏帖子、最近阅读、自定义板块。
- 深色、浅色、跟随系统，以及「NGA经典屎黄色」主题。
- 图片默认折叠，点击「展示图片」后才请求；关闭预览后释放临时 WebView。
- 官方登录成功后返回阅读器；可选择在本机加密保存会话。
- 个别帖子 JSON 数据不完整时，自动追加一次同站静态正文请求，继续原生阅读；不执行正文网页脚本。
- 侧栏内置离线使用说明；支持 F5 刷新、Ctrl + Shift + Space 隐藏或恢复窗口。

## 构建与运行

Go 版本以 [go.mod](go.mod) 为准，目前为 **Go 1.27.3**。依赖固定为 MyGo v0.8.3，无需 Node.js、Bun 或 C 编译器。

Windows PowerShell：

```powershell
go mod download
go test ./...
go vet ./...
./build.ps1
./release/NgaReader.exe
```

程序为便携版。将 EXE 放进可写文件夹即可运行；设置与本机加密会话保存在 EXE 同级的 `data` 目录。

离线演示：

```powershell
./release/NgaReader.exe --demo
```

可选参数：

| 参数 | 用途 |
| --- | --- |
| `--demo` | 查看明确标注的离线样例，不连接账号 |
| `--data-dir <目录>` | 指定独立数据目录 |
| `--check-api` | 检查未登录接口并输出不含凭据的结果，不创建窗口 |
| `--licenses` | 输出程序内嵌的第三方许可 |

macOS / Linux 交叉编译：

```powershell
./build.ps1 -TargetOS darwin -Arch arm64 -OutputDirectory ./release-mac
./build.ps1 -TargetOS linux -Arch amd64 -OutputDirectory ./release-linux
```

当前实际运行验证以 Windows x64 为主；macOS / Linux 仅做过交叉编译验证。对应平台的窗口、系统凭据和 WebView 行为需要在真实系统上测试。

## 开始阅读

1. 点击「登录 NGA」，在官方页面完成登录。Windows 官方登录和 App 内图片预览需要已有 WebView2 Runtime。
2. 登录成功后返回工作台。左栏选择板块，中栏选择话题，右栏阅读。
3. 点击「我的板块 → 导入收藏板块」导入账号收藏；该操作只读，不修改论坛收藏。
4. 点击每张图片的「展示图片」才会加载图片。图床要求验证时，在预览窗口完成官方验证。

完整操作说明见 [使用说明](resources/usage.md)，也可以在软件侧栏离线查看。

## 数据与安全

默认只保留本次运行的登录。选择「记住本机登录」后，会话使用 AES-GCM 加密保存，密钥保存在系统凭据存储；换电脑通常需要重新登录。请勿上传 `data` 文件夹、Cookie、密码或系统凭据。

官方网页、图床验证及系统代理配置可能引发相应网络请求。本项目遵守论坛登录、权限和频率限制，不提供绕过验证、发帖、投票或批量操作功能。

**杀毒状态：** Windows 0.1.2 构建曾被 360 检出为 `Trojan.Win64.SilverFox.ZAX`。包含 0.1.2 / 0.1.3 样本的复核申请未通过；收到的回复未给出具体检出依据。当前代码版本为 0.1.4，尚无该版本的 360 放行结论。公开源码、功能测试及依赖漏洞扫描不能替代恶意软件鉴定。本次开源仅发布源码和演示图，不附预编译 EXE。详情见 [安全说明](SECURITY.md)。

## 开发与验证

```powershell
go test ./...
go vet ./...
node resources/login-ui.test.cjs
```

Node.js 仅用于可选的登录页脚本测试，构建和运行软件不依赖它。Go 测试使用本地样例与测试服务器，不使用开发者的真实账号会话。

当前回归测试覆盖 API / 静态正文解析、分页、会话加密、登录引导、主题保存、图片按需展示和内置帮助。论坛接口不稳定，复杂视频、部分 BBCode 和表情可能降级显示，可打开原帖查看。

目录说明：

```text
*.go              原生界面、登录和阅读操作
internal/nga/     论坛请求、解析、BBCode 与附件地址处理
internal/local/   本地设置、加密会话与请求取消
resources/        内置说明、登录页辅助脚本与第三方许可
LICENSES/         第三方组件的许可原文
docs/images/      离线演示界面图
```

提交问题前请参阅 [贡献说明](CONTRIBUTING.md)，勿附带账号凭据或个人数据。

## 许可与参考

本项目原创代码采用 [MIT License](LICENSE)。第三方代码的许可保留在 [LICENSES](LICENSES) 和程序内嵌的第三方说明中。

收藏板块的接口结构参考 [MNGA 上游实现](https://github.com/BugenZhao/MNGA/blob/main/logic/service/src/forum.rs)，接口格式参考 [NGA API 文档](https://github.com/AgMonk/nga-api-doc/blob/main/README_v2.md)。这些项目分别保留各自许可。
