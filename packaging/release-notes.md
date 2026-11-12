## 下载免安装版

**[打开简洁下载页 → 只看 Windows 和 Mac 免安装包](https://github.com/apple-half/nga-reader/blob/main/DOWNLOAD.md)**

| 你的电脑 | 直接下载 | 打开方式 |
| --- | --- | --- |
| **Windows 64 位** | **[下载 Windows 免安装包](https://github.com/apple-half/nga-reader/releases/download/v0.1.9/NGAReader-0.1.9-Windows-x64-portable.zip)** | 解压整个文件夹，双击 `NGAReader.exe` |
| **Mac · Apple 芯片（M 系列）** | **[下载 Mac M 系列免安装包](https://github.com/apple-half/nga-reader/releases/download/v0.1.9/NGAReader-0.1.9-macOS-AppleSilicon-portable.zip)** | 解压后打开 `NGAReader.app` |
| **Mac · Intel 芯片** | **[下载 Mac Intel 免安装包](https://github.com/apple-half/nga-reader/releases/download/v0.1.9/NGAReader-0.1.9-macOS-Intel-portable.zip)** | 解压后打开 `NGAReader.app` |

Mac 用户可在“苹果菜单 → 关于本机”查看芯片。最低支持 macOS 12。

**从旧版升级：先退出旧版，替换程序并保留原数据目录。** 0.1.6及更早版本，或0.1.7检查更新一直显示HTTP403时，请手动下载一次0.1.9。

<details>
<summary>展开查看更新说明与使用提醒</summary>

### 本版改进

- **升级提示重新排版**：突出当前与新版本，原生显示改动列表，不再将Markdown源码直接显示在弹窗里。
- **说明分别编写**：App内使用简短更新内容，GitHub保留完整下载说明。0.1.7／0.1.8重新检查后也能读到清楚的简短文字；升级后使用新版弹窗。
- **安装状态保持可见**：下载进度、错误信息、手动下载及安装／重启操作不随长说明滚出窗口；安装时收起改动清单，集中显示进度。
- **继续包含0.1.8全部功能**：签名索引更新检查、已读／有新回复标记、按账号记住页码与楼层、首页／末页和页码跳转、三种主题与图片按需展示。

内置使用说明已同步更新。以上读取与UI操作不会自动发帖或回复。

### 自动更新

- 正式便携版启动10秒后自动检查，此后每24小时检查。发现新版会在侧栏提示，可查看版本说明；不会自动下载、安装或重启。
- 点击“下载并安装”后显示进度，校验更新签名再替换程序；完成后点击“重启使用新版”。可在侧栏“软件更新”关闭自动检查或手动检查。
- 保留设置、收藏、历史、草稿和待确认发送记录。发帖或回复发送期间禁止安装；保存失败时中止更新。勾选“记住本机登录”的会话会保存，仅本次有效的登录在重启后需重新登录。
- Windows保留程序名NGAReader.exe并放在可写文件夹；Mac先解压再运行。程序位置不支持自动安装时，按提示手动下载便携ZIP。
- 更新包及元数据通过Ed25519签名，绑定版本、平台、地址和摘要。私钥离线保存，GitHub Actions核验全部签名和发布文件后公开。

Windows数据位于EXE同级`data`，Mac位于`~/Library/Application Support/NGAReader`。图片仍默认折叠、点击才加载；Windows官方登录和图片预览需要已有WebView2 Runtime。文字发帖与回复等既有功能继续保留，使用说明已集成软件。

### 验证范围与安全状态

- 新回归使用真实0.1.8说明复现源码显示与状态被挤出窗口，再检查三种主题、窄窗口、长错误、安装／重启／手动下载、Unicode截断和旧说明兼容。发布前还核对三平台签名元数据使用简短App说明。
- GitHub Actions在Windows x64、macOS arm64与Intel环境测试、构建和检查包结构，Mac检查ad-hoc签名。这些检查不覆盖全部真实GUI操作；Mac实际更新仍需真机使用核验。
- Windows没有Authenticode签名；Mac仅有ad-hoc完整性签名，没有Apple Developer ID或公证。更新签名不等同于系统代码签名，系统仍可能阻止启动。
- 早期Windows 0.1.2曾被360检出，0.1.2/0.1.3复核未通过且未提供具体依据。**0.1.9尚无360放行结论**；功能测试、更新签名与源码公开不能代替恶意软件鉴定。
- `SHA256SUMS.txt`核对三份ZIP和六份更新资产的下载内容，不构成安全认证；包内保留MIT及依赖许可。

</details>

<details>
<summary>开发者附件说明</summary>

下方 Assets 中的 `.tar.gz` 和 `update-*.json` 是软件自动更新附件；`SHA256SUMS.txt` 用于校验，Source code 是源码。日常使用请直接点击上方对应系统的免安装包链接。

</details>
