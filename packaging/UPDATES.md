# 便携版发布与离线更新签名

GitHub Actions不保管更新私钥。发布分为构建草稿、维护者离线签名、工作流核验并公开三个步骤，已公开的Release禁止覆盖。

## 首次配置

维护者使用固定版本的MyGo CLI在仓库外生成Ed25519密钥，并离线备份私钥：

```powershell
go run github.com/egoist/mygo/cmd/mygo keygen -o ../update-signing
```

`mygo-update.key`必须留在仓库外，不能上传GitHub、附在Issue或放进发布包。公钥写入`resources/update-public.txt`，并保持与`mygo.json`的`updates.publicKey`一致。配置`updates.github`以及`internal/appupdate.Repository`为自己的公开仓库。已发布客户端不能自动接受另一把密钥。

## 每次发布

1. 修改`main.go`与`mygo.json`的版本，更新README、DOWNLOAD.md和发布说明中的版本与三份便携ZIP链接，运行全部检查后创建并推送对应`vX.Y.Z`标签。DOWNLOAD.md只展示用户需要的免安装包；更新器附件继续保留在Release原地址。
2. Release工作流在Windows和两种Mac环境构建。三份便携ZIP和初始`SHA256SUMS.txt`上传到**草稿Release**，暂不公开。
3. 从该草稿下载三份ZIP到同一个本地空目录，核对Actions来源、标签提交及SHA-256。私钥始终在目录之外。运行离线签名：

```powershell
go run ./cmd/update-release sign --dir ../release-files --version X.Y.Z --key ../update-signing/mygo-update.key --notes packaging/release-notes.md
go run ./cmd/update-release verify --dir ../release-files --version X.Y.Z --key resources/update-public.txt
```

生成每个平台的一份`nga-reader-X.Y.Z-PLATFORM.tar.gz`和`update-PLATFORM.json`，共六个文件。签名器验证ZIP中的程序架构、许可、路径、文件权限，拒绝数据目录和特殊文件；已存在的输出不会覆盖。归档签名兼容MyGo，元数据另行签名以绑定版本、平台、地址和摘要。

4. 将六份新文件上传到同一草稿。不要手动点击发布。
5. 在Release工作流选择`Run workflow`，填写相同标签，勾选`publish_existing`。此模式不重新构建，只下载草稿资产，验证签名及便携ZIP与更新归档逐文件一致，重算九份文件的校验值，核对上传摘要后公开预发布。最终共有十个资产（另两个Source code下载由GitHub生成）。
6. 从公开页面重新下载并验证所有资产。可以检查客户端是否识别新版本：

```powershell
go run ./cmd/update-release check --version X.Y.OLD --target windows-amd64 --key resources/update-public.txt
```

`check`只检查，不安装。可将target改为`darwin-arm64`或`darwin-amd64`。测试安装必须使用隔离程序目录和模拟数据，不替换正在使用的程序。

如果某个平台失败、签名不匹配或包里出现私人数据，草稿保持未公开。确认问题后重新构建新版本；不要更改已公开标签或绕过验证。更新签名不代替系统代码签名、公证或安全软件复核。
