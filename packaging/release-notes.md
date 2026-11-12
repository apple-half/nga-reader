## 下载免安装版

**[打开简洁下载页 → 只看 Windows 和 Mac 免安装包](https://github.com/apple-half/nga-reader/blob/main/DOWNLOAD.md)**

| 你的电脑 | 直接下载 | 打开方式 |
| --- | --- | --- |
| **Windows 64 位** | **[下载 Windows 免安装包](https://github.com/apple-half/nga-reader/releases/download/v0.1.11/NGAReader-0.1.11-Windows-x64-portable.zip)** | 解压整个文件夹，双击 `NGAReader.exe` |
| **Mac · Apple 芯片（M 系列）** | **[下载 Mac M 系列免安装包](https://github.com/apple-half/nga-reader/releases/download/v0.1.11/NGAReader-0.1.11-macOS-AppleSilicon-portable.zip)** | 解压后打开 `NGAReader.app` |
| **Mac · Intel 芯片** | **[下载 Mac Intel 免安装包](https://github.com/apple-half/nga-reader/releases/download/v0.1.11/NGAReader-0.1.11-macOS-Intel-portable.zip)** | 解压后打开 `NGAReader.app` |

Mac 用户可在“苹果菜单 → 关于本机”查看芯片。最低支持 macOS 12。

**从旧版升级：先退出旧版，替换程序并保留原数据目录。** 0.1.6及更早版本，或0.1.7检查更新一直显示HTTP403时，请手动下载一次0.1.11。

<details>
<summary>展开查看更新说明与使用提醒</summary>

### 本版改进

- **NGA表情直接显示**：七组共265个官方表情在正文和引用中原生行内显示；支持旧数字格式，不再统一显示为“[表情]”。
- **尺寸有边界**：保持原比例、随字号缩放，通常最高32逻辑像素；较大素材最高48，不放大超过原图，也不铺满阅读区。
- **离线可用**：固定表情图片内置，总计约680KiB；不为表情调用图床、启动WebView或增加后台请求。普通附件依旧点击展示。
- **主题与动画**：深色主题给透明黑线条表情加浅色底衬，不修改原图。内置脸红GIF原生播放，只有可见含动画楼层按需重绘，上限8次/秒。
- **旧格式兼容**：修复数字表情可能吞掉后文；未知表情保留代码，代码块不转换，静态HTML兼容正文中的官方表情也能识别。

板块名称搜索、阅读续读、分页、发帖与回复等功能继续保留，使用说明同步更新。自动续页及此前体验优化仍是后续清单，本版聚焦表情显示。

### 自动更新

- 正式便携版启动10秒后自动检查，此后每24小时检查。发现新版会在侧栏提示，可查看版本说明；不会自动下载、安装或重启。
- 点击“下载并安装”后显示进度，校验更新签名再替换程序；完成后点击“重启使用新版”。可在侧栏“软件更新”关闭自动检查或手动检查。
- 保留设置、收藏、历史、草稿和待确认发送记录。发帖或回复发送期间禁止安装；保存失败时中止更新。勾选“记住本机登录”的会话会保存，仅本次有效的登录在重启后需重新登录。
- Windows保留程序名NGAReader.exe并放在可写文件夹；Mac先解压再运行。程序位置不支持自动安装时，按提示手动下载便携ZIP。
- 更新包及元数据通过Ed25519签名，绑定版本、平台、地址和摘要。私钥离线保存，GitHub Actions核验全部签名和发布文件后公开。

Windows数据位于EXE同级`data`，Mac位于`~/Library/Application Support/NGAReader`。图片仍默认折叠、点击才加载；Windows官方登录和图片预览需要已有WebView2 Runtime。文字发帖与回复等既有功能继续保留，使用说明已集成软件。

### 验证范围与安全状态

- 核对265份官方图片的摘要、尺寸与帧数；回归覆盖名称／旧数字／未知表情、代码与引用、HTML兼容正文、内置路径白名单、字体12～22、三主题、GIF透明补丁／处置／时序和解码预算。
- GitHub Actions在Windows x64、macOS arm64与Intel环境测试、构建和检查包结构，Mac检查ad-hoc签名。这些检查不覆盖全部真实GUI操作；Mac实际更新仍需真机使用核验。
- Windows没有Authenticode签名；Mac仅有ad-hoc完整性签名，没有Apple Developer ID或公证。更新签名不等同于系统代码签名，系统仍可能阻止启动。
- 早期Windows 0.1.2曾被360检出，0.1.2/0.1.3复核未通过且未提供具体依据。**0.1.11尚无360放行结论**；功能测试、更新签名与源码公开不能代替恶意软件鉴定。
- `SHA256SUMS.txt`核对三份ZIP和六份更新资产的下载内容，不构成安全认证；包内保留MIT及依赖许可；NGA表情素材保留原作者权利，另附来源声明，不适用源码MIT许可证。

</details>

<details>
<summary>开发者附件说明</summary>

下方 Assets 中的 `.tar.gz` 和 `update-*.json` 是软件自动更新附件；`SHA256SUMS.txt` 用于校验，Source code 是源码。日常使用请直接点击上方对应系统的免安装包链接。

</details>
