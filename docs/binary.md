# 二进制部署

[文档中心](https://github.com/Qwerto107/UptimeNest/blob/main/docs/README.md) · [部署指南](https://github.com/Qwerto107/UptimeNest/blob/main/docs/deployment.md)

UptimeNest 发行程序内嵌前端，解压即可提供页面与 API，无需安装 Go 或 Node.js。Ping 监控需要操作系统提供 `ping` 命令并允许 ICMP。本说明随每个安装包提供。

[下载](#下载与校验) · [Linux](#linux) · [Windows](#windows) · [macOS](#macos) · [配置](#配置与数据) · [升级](#升级)

## 下载与校验

从 [GitHub Releases](https://github.com/Qwerto107/UptimeNest/releases) 下载压缩包及同名 `.sha256` 文件。

| 系统／架构 | 文件 |
| --- | --- |
| Linux x86-64 | `uptimenest-linux-amd64.tar.gz` |
| Linux ARM64 | `uptimenest-linux-arm64.tar.gz` |
| Linux ARMv6 | `uptimenest-linux-armv6.tar.gz` |
| Linux ARMv5 | `uptimenest-linux-armv5.tar.gz` |
| Linux LoongArch64 | `uptimenest-linux-loong64.tar.gz` |
| Windows x86-64 | `uptimenest-windows-amd64.zip` |
| macOS Apple Silicon | `uptimenest-darwin-arm64.tar.gz` |

每包包含程序和 README。实际附件以对应版本为准；流水线交叉编译，不代表已在每种设备上运行验证。

## Linux

以 x86-64 为例，在下载目录执行：

```sh
sha256sum -c uptimenest-linux-amd64.tar.gz.sha256
tar -xzf uptimenest-linux-amd64.tar.gz
cd uptimenest-linux-amd64
./uptimenest
```

其他 Linux 平台替换包名。

## Windows

在 PowerShell 中执行：

```powershell
$archive = 'uptimenest-windows-amd64.zip'
$expected = (Get-Content "$archive.sha256").Split(' ')[0]
if ((Get-FileHash $archive -Algorithm SHA256).Hash -ne $expected) {
    throw '文件校验失败，请重新下载'
}
Expand-Archive $archive -DestinationPath .
Set-Location uptimenest-windows-amd64
./uptimenest.exe
```

## macOS

```sh
shasum -a 256 -c uptimenest-darwin-arm64.tar.gz.sha256
tar -xzf uptimenest-darwin-arm64.tar.gz
cd uptimenest-darwin-arm64
./uptimenest
```

## 配置与数据

默认地址为 `http://127.0.0.1:8090`。首次访问 `/admin` 设置管理员密码，没有通用初始密码。初始化完成后再通过反向代理开放访问。

程序通过环境变量配置，不自动读取 `.env`。默认 SQLite 路径为 `data/uptimenest.db`，相对进程工作目录；运行账号须有写入权限，长期运行请固定工作目录或使用绝对路径。PostgreSQL 可通过 `UPTIMENEST_DB_TYPE=postgres` 与 `UPTIMENEST_DB_DSN` 配置。

`UPTIMENEST_WEB_DIR` 留空使用内嵌页面，设置目录后使用该目录覆盖内嵌页面。数据库和启动参数见 [启动参数](https://github.com/Qwerto107/UptimeNest/blob/main/docs/deployment.md#启动参数)；界面设置见 [后台运行设置](https://github.com/Qwerto107/UptimeNest/blob/main/docs/deployment.md#后台运行设置)。

执行 `./uptimenest healthcheck`（Windows 为 `./uptimenest.exe healthcheck`）检查当前监听地址的健康接口，非零退出表示失败；此命令不会打开数据库或启动监控任务。

## 升级

停止旧进程并备份数据库，校验并解压新包，保持原工作目录、数据路径及启动环境，再启动程序。前端随程序更新。数据库结构升级后，回退旧程序需要同时恢复与其兼容的数据库备份。SQLite 运行中备份应使用一致性备份方式，不要只复制正在写入的主文件。

备份和结构迁移后的回退说明统一见 [备份与恢复](https://github.com/Qwerto107/UptimeNest/blob/main/docs/deployment.md#备份与恢复) 与 [升级与回退](https://github.com/Qwerto107/UptimeNest/blob/main/docs/deployment.md#升级与回退)。

## 源码打包

前端构建、内嵌准备和 Go 构建命令统一放在 [开发指南](https://github.com/Qwerto107/UptimeNest/blob/main/docs/development.md#完整程序与镜像)。
