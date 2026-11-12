# 0.1.14 正式版验证

本版发布已合入main的可选图标功能，详见[功能验证](validation-selectable-icon.md)与[图标设计](logo-design.md)。用户明确授权更新Windows、Mac AppleSilicon和Intel免安装包及自动更新索引。

功能代码沿用已完成独立审查、RED→GREEN与完整测试的提交，不新增功能。默认简约工作图标，可在阅读设置切换工位摸鱼；非法图标值仅回退图标，不重置其他有效偏好。程序文件／快捷方式保持静态工作图标，运行中的图标随选择切换。

## 发布结果

- 源码标签`v0.1.14`固定于`38d9e7b7f9d59e0348d005666d130497dd93cc49`；旧版标签和公开资产保持不变。
- [原生构建43186296100](https://github.com/apple-half/nga-reader/actions/runs/43186296100)三平台及草稿阶段全部通过：Go测试/vet、Node登录脚本、22项Python测试、原生构建、包结构/许可输出；Mac额外检查ad-hoc签名。
- 三份CI artifact的外层ZIP大小及SHA256与API一致，每份只含一个正确的便携ZIP；内部包通过架构、路径和许可检查。
- 本地离线签名三平台更新归档与元数据，再用公开密钥验证签名及归档与便携ZIP逐文件一致。三份App说明与短说明源文件一致，没有使用网页长说明。
- 独立目录中运行实际MyGo Windows更新器，四份模拟设置、草稿和用户文件保留；安装后的EXE与CI包内字节一致，SHA256为`1fa81fc5a8e5c851323d65fc8cec114d8da09c2af6a9edef2f3bef0304a32f69`，许可输出检查通过。没有操作用户正在运行的程序或账号。
- [发布流程43187135145](https://github.com/apple-half/nga-reader/actions/runs/43187135145)验证完整草稿后公开[v0.1.14](https://github.com/apple-half/nga-reader/releases/tag/v0.1.14)，已设为Latest，`draft=false`、`prerelease=false`。
- 重新下载公开的全部10个附件：大小、SHA256与离线原件及GitHub摘要一致，9条校验清单全部匹配；三个便携ZIP和更新签名/归档一致性再次通过。
- 三份公开更新索引与发布元数据一致，均为正式版。生产客户端使用系统代理实际读取三平台索引，均向0.1.13返回0.1.14和正确短说明，共3次raw请求、0次API回退，不携带Cookie或Authorization。
- Windows、Mac AppleSilicon和Intel的三份正式ZIP已保存到本地交付目录。

## 验证边界

实机边界：Windows系统图标联动及Mac Dock没有完成目视验收；Mac实际GUI升级仍需真机验证。更新签名不替代系统代码签名或360厂商复核。既有含表情文档中独立普通表格复制为段落分隔的Minor限制保持。
