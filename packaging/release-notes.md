## 下载免安装版

**[打开简洁下载页 → 只看 Windows 和 Mac 免安装包](https://github.com/apple-half/nga-reader/blob/main/DOWNLOAD.md)**

| 你的电脑 | 直接下载 | 打开方式 |
| --- | --- | --- |
| **Windows 64 位** | **[下载 Windows 免安装包](https://github.com/apple-half/nga-reader/releases/download/v0.1.10/NGAReader-0.1.10-Windows-x64-portable.zip)** | 解压整个文件夹，双击 `NGAReader.exe` |
| **Mac · Apple 芯片（M 系列）** | **[下载 Mac M 系列免安装包](https://github.com/apple-half/nga-reader/releases/download/v0.1.10/NGAReader-0.1.10-macOS-AppleSilicon-portable.zip)** | 解压后打开 `NGAReader.app` |
| **Mac · Intel 芯片** | **[下载 Mac Intel 免安装包](https://github.com/apple-half/nga-reader/releases/download/v0.1.10/NGAReader-0.1.10-macOS-Intel-portable.zip)** | 解压后打开 `NGAReader.app` |

Mac 用户可在“苹果菜单 → 关于本机”查看芯片。最低支持 macOS 12。

**从旧版升级：先退出旧版，替换程序并保留原数据目录。** 0.1.6及更早版本，或0.1.7检查更新一直显示HTTP403时，请手动下载一次0.1.10。

<details>
<summary>展开查看更新说明与使用提醒</summary>

### 本版改进

- **直接搜索板块**：点击“我的板块”旁的 +，按名称、简介或别名搜索“汽车”“数码”“大漩涡”等首页公开板块，一键添加并打开；不必复制链接。
- **离线可搜索**：内置公开目录并缓存，打开添加窗口时目录过期24小时才后台刷新；支持手动刷新，失败时保留本地目录。搜索在本机完成，不需要登录，不发送逐字查询。
- **保留已有名称**：已添加板块直接打开，不重复添加或覆盖自定义名称。保存失败保留输入和窗口；Enter仅在唯一匹配的普通板块时添加，Esc关闭。
- **合集明确区分**：标记合集并提供“官网查看”，暂不支持原生添加，不会将其误加成父板块。保留链接／编号添加页。
- **原有功能继续保留**：三栏阅读、三种主题、楼层续读、页码跳转、文字发帖与回复、图片点击展示、签名自动更新；使用说明已集成软件并同步更新。

### 自动更新

- 正式便携版启动10秒后自动检查，此后每24小时检查。发现新版会在侧栏提示，可查看版本说明；不会自动下载、安装或重启。
- 点击“下载并安装”后显示进度，校验更新签名再替换程序；完成后点击“重启使用新版”。可在侧栏“软件更新”关闭自动检查或手动检查。
- 保留设置、收藏、历史、草稿和待确认发送记录。发帖或回复发送期间禁止安装；保存失败时中止更新。勾选“记住本机登录”的会话会保存，仅本次有效的登录在重启后需重新登录。
- Windows保留程序名NGAReader.exe并放在可写文件夹；Mac先解压再运行。程序位置不支持自动安装时，按提示手动下载便携ZIP。
- 更新包及元数据通过Ed25519签名，绑定版本、平台、地址和摘要。私钥离线保存，GitHub Actions核验全部签名和发布文件后公开。

Windows数据位于EXE同级`data`，Mac位于`~/Library/Application Support/NGAReader`。图片仍默认折叠、点击才加载；Windows官方登录和图片预览需要已有WebView2 Runtime。文字发帖与回复等既有功能继续保留，使用说明已集成软件。

### 验证范围与安全状态

- 回归覆盖目录编码与格式、合集与板块隔离、别名和全角查询、匿名请求、缓存失效、断网回退、保存失败、演示隔离和键盘操作；检查三种主题与窄窗口。官方CDN仅解析公开数据，不执行脚本或携带论坛会话。
- GitHub Actions在Windows x64、macOS arm64与Intel环境测试、构建和检查包结构，Mac检查ad-hoc签名。这些检查不覆盖全部真实GUI操作；Mac实际更新仍需真机使用核验。
- Windows没有Authenticode签名；Mac仅有ad-hoc完整性签名，没有Apple Developer ID或公证。更新签名不等同于系统代码签名，系统仍可能阻止启动。
- 早期Windows 0.1.2曾被360检出，0.1.2/0.1.3复核未通过且未提供具体依据。**0.1.10尚无360放行结论**；功能测试、更新签名与源码公开不能代替恶意软件鉴定。
- `SHA256SUMS.txt`核对三份ZIP和六份更新资产的下载内容，不构成安全认证；包内保留MIT及依赖许可。

</details>

<details>
<summary>开发者附件说明</summary>

下方 Assets 中的 `.tar.gz` 和 `update-*.json` 是软件自动更新附件；`SHA256SUMS.txt` 用于校验，Source code 是源码。日常使用请直接点击上方对应系统的免安装包链接。

</details>
