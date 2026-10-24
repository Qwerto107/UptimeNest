# 部署指南

[文档中心](README.md) · [二进制部署](binary.md) · [使用指南](usage.md)

推荐使用 Docker Compose。一个容器提供状态页、管理后台与 API；直接运行发行程序见 [二进制部署](binary.md)。当前按单应用实例部署。

| 阶段 | 内容 |
| --- | --- |
| 安装与配置 | [单容器部署](#单容器部署) · [数据目录](#数据与目录) · [启动参数](#启动参数) · [后台设置](#后台运行设置) |
| 数据与访问 | [数据库](#数据库配置) · [HTTPS](#https-与反向代理) · [监控网络](#监控网络与权限) |
| 运行维护 | [备份](#备份与恢复) · [升级](#升级与回退) · [排障](#故障排查) |

## 单容器部署

### 准备运行目录

Linux 上创建专用目录，并授权容器用户 UID/GID `65532:65532` 写入：

```sh
mkdir -p uptimenest/data
sudo chown 65532:65532 uptimenest/data
sudo chmod 700 uptimenest/data
cd uptimenest
curl -fL https://raw.githubusercontent.com/Qwerto107/UptimeNest/main/docker-compose.yml -o docker-compose.yml
```

以上权限命令用于新目录。已有目录按原所有权或 ACL 授权。Windows Docker Desktop 按宿主机权限设置，在 PowerShell 中准备文件：

```powershell
New-Item -ItemType Directory -Force uptimenest/data
Set-Location uptimenest
Invoke-WebRequest https://raw.githubusercontent.com/Qwerto107/UptimeNest/main/docker-compose.yml -OutFile docker-compose.yml
```

### 启动并设置密码

```sh
docker compose up -d --pull always
docker compose ps
```

打开 <http://127.0.0.1:8090/admin>，首次访问时设置管理员密码。密码须非空、UTF-8 编码后最多 72 字节，不强制最短长度或复杂度。没有预设用户名或通用初始密码，也不从环境变量读取初始密码。初始化完成后再通过反向代理开放访问。

默认 SQLite 部署无须创建 `.env`。需要调整数据库或 HTTPS Cookie 时，在运行目录创建 `.env`，可参考仓库的 [.env.example](../.env.example)。

`docker-compose.yml` 直接指定镜像 `ghcr.io/qwerto107/uptimenest:latest`。固定版本时修改 `image` 为 `ghcr.io/qwerto107/uptimenest:0.2.0`；更换端口时修改 `127.0.0.1:8090:8090` 中的第一个 `8090`。默认端口仅本机可访问，公网访问需配置反向代理。

## 数据与目录

Compose 使用 `./data:/app/data` 绑定宿主机目录：

| 容器路径 | 内容 |
| --- | --- |
| `/app/data/uptimenest.db` | SQLite 数据库，包含账户、监控、分组、面板、设置和历史 |
| `/app/data/uptimenest.db-wal`、`-shm` | SQLite 运行时 WAL 与共享内存文件 |
| `/tmp` | 临时目录，Compose 使用上限 1 GiB 的 tmpfs |

容器以非 root 用户运行，根文件系统只读，`/app/data` 和 `/tmp` 可写。tmpfs 按实际写入占用内存，上限不等于预分配量。SQLite 放在本机磁盘，不让多个应用实例共用该数据库执行监控。

数据库和备份包含 HTTP 验证凭据，应限制目录访问权限。前端资源随程序内嵌，不需要另行挂载页面目录。

## 启动参数

Compose 读取运行目录的 `.env`，将 Compose 中声明的参数传入容器。直接运行程序或 Go 时，通过终端或服务管理器设置环境变量，程序不会自动读取 `.env`。

| 环境变量 | 直接运行默认值 | 用途 |
| --- | --- | --- |
| `UPTIMENEST_ADDRESS` | `127.0.0.1:8090` | 监听地址；Compose 内固定为 `0.0.0.0:8090` |
| `UPTIMENEST_DB_TYPE` | `sqlite` | `sqlite` 或 `postgres` |
| `UPTIMENEST_DB_DSN` | `data/uptimenest.db` | SQLite 路径或 PostgreSQL 连接串 |
| `UPTIMENEST_WEB_DIR` | 空 | 使用内嵌页面；未内嵌源码运行回退到 `web/dist`，可指定外置目录 |
| `UPTIMENEST_COOKIE_SECURE` | `false` | HTTPS 对外访问时设为 `true`，使登录 Cookie 仅通过 HTTPS 传输 |

Compose 的数据库路径默认为 `/app/data/uptimenest.db`。镜像地址和宿主机端口直接修改 Compose；监听地址和外置前端目录如需更改，也需同步调整 Compose 的参数及挂载。

## 后台运行设置

登录后在“系统设置”中配置以下内容，保存在数据库中：

| 分类 | 配置 |
| --- | --- |
| 显示 | 站点名称、系统显示时区、入口页面 |
| 语言与主题 | 简体中文；浅色／深色主题手动切换，主题偏好保存在当前浏览器 |
| 监控全局配置 | HTTP/HTTPS 请求的 User-Agent，留空使用系统默认值 |
| 历史数据 | 保留期限、手动清理；启动及每小时自动清理过期数据 |
| 账户 | 在“账户设置”修改管理员密码 |

入口页面可选择默认状态面板或管理后台。设置细节见 [使用指南](usage.md#系统设置)，监控、后台分组与状态面板分别在对应管理页面配置。

## 数据库配置

### SQLite

默认即可使用，无需部署独立数据库服务。SQLite 开启 WAL、外键和繁忙等待，使用单数据库连接。

可以修改 `UPTIMENEST_DB_DSN`，但容器路径须位于可写挂载目录。不要通过删除数据库来清理历史；在后台设置保留期限或按项清空记录。

### PostgreSQL

先创建数据库和应用账号，允许应用创建及迁移本数据库中的表与索引。Compose 不自带 PostgreSQL 服务，连接已有数据库。

在运行目录的 `.env` 设置：

```dotenv
UPTIMENEST_DB_TYPE=postgres
UPTIMENEST_DB_DSN='postgres://uptimenest:REPLACE_PASSWORD@pg.example.com:5432/uptimenest?sslmode=verify-full'
```

修改后执行 `docker compose up -d`。生产连接应配置适当的 TLS 校验和可信 CA，连接串中特殊字符按 URI 规则编码。容器中的 `127.0.0.1` 指容器自身，数据库地址需从容器网络可达。

直接运行程序时使用同名环境变量。SQLite 与 PostgreSQL 之间不自动迁移数据，切换到新数据库会使用该数据库自己的账户和配置。

## HTTPS 与反向代理

域名指向服务器后，由反向代理终止 TLS，并将页面和 API 一起代理至 `127.0.0.1:8090`。以下为 Nginx 示例，替换域名及证书路径：

```nginx
server {
    listen 443 ssl;
    server_name status.example.com;

    ssl_certificate /etc/nginx/certs/fullchain.pem;
    ssl_certificate_key /etc/nginx/certs/privkey.pem;

    location / {
        proxy_pass http://127.0.0.1:8090;
        proxy_set_header Host $host;
        proxy_set_header X-Real-IP $remote_addr;
        proxy_set_header X-Forwarded-For $proxy_add_x_forwarded_for;
        proxy_set_header X-Forwarded-Proto $scheme;
    }

    client_max_body_size 2m;
    proxy_read_timeout 60s;
}
```

保留原始 Host，使浏览器的页面与 API 保持同源；应用写请求校验 Origin。保留应用的 CSP、Cache-Control、nosniff 和 Referrer-Policy 响应头。HTTP 入口重定向到 HTTPS，证书续期由反向代理负责。

在 `.env` 设置 `UPTIMENEST_COOKIE_SECURE=true` 后执行 `docker compose up -d`，并通过 HTTPS 登录。应用自身提供 HTTP，不负责对外 TLS 终止。

密码尝试按连接来源限流，每 15 分钟最多 10 次。当前不解析可信代理列表，代理后的客户端共用代理来源的额度；修改转发头不会改变这个计数范围。

## 监控网络与权限

| 监控类型 | 部署要求 |
| --- | --- |
| HTTP/HTTPS | 容器能解析目标域名并访问目标 TCP 端口；系统 CA 用于校验证书 |
| HTTP/3 | 目标支持 HTTPS 与 QUIC，容器及宿主机允许对应 UDP 出站流量 |
| TCP | 目标主机与端口从容器网络可达 |
| Ping | 允许 ICMP；镜像提供专用 BusyBox Ping 入口，主程序仍以普通用户运行 |
| DNS | 默认使用容器的系统 DNS，也可在单项配置可达的 DNS 服务器 |

容器中的本机地址不是宿主机或其他容器的本机地址。监控宿主机或内网服务时，按实际容器网络选择地址。宿主机禁用 setuid、移除网络能力或限制 ICMP 时，Ping 可能无法执行，需要调整对应策略。

HTTP/3 优先模式仅在 QUIC 连接建立失败时回退。若环境代理变量存在，优先模式使用 HTTP/1.1 / HTTP/2，仅 HTTP/3 模式会提示不支持代理；需要代理时自行在 Compose 中声明相应环境变量。

## 备份与恢复

备份数据库及部署配置，包括 `docker-compose.yml` 和按需创建的 `.env`。使用 PostgreSQL 时，通过 `pg_dump` 或数据库自身的备份机制保存数据。

SQLite 可停止应用后备份整个 `data/`。运行中的 WAL 可能包含已提交数据，在线备份需使用 SQLite Backup API 或 `VACUUM INTO`，不要仅复制正在写入的主文件。

在运行目录执行的 PowerShell 示例：

```powershell
New-Item -ItemType Directory -Force backups
docker compose stop uptimenest
docker run --rm -v "${PWD}/data:/source:ro" -v "${PWD}/backups:/backup" alpine:3.23 tar czf /backup/uptimenest-data.tgz -C /source .
docker compose start uptimenest
```

在独立目录或实例恢复并核对登录、监控数量、面板和历史后再切换。保留原备份用于回退，限制备份访问权限。

## 升级与回退

1. 停止应用，备份数据库及部署配置。
2. 修改 Compose 的镜像版本，执行 `docker compose up -d --pull always`。
3. 检查容器日志、健康状态、管理员登录、监控检测及公开状态页。

程序启动时自动执行版本化迁移。结构迁移后回退，应同时恢复升级前的数据快照。当前只运行单个监控实例，流水线不会自动更新服务器部署。

旧部署若使用 `compose.yaml` 的 `uptimenest-data` 命名卷，先用旧配置停止服务，再把原卷数据完整迁入 `./data`，调整 UID/GID 权限后使用新配置启动；不要同时运行两套实例。保留原卷和备份，确认迁移完成后再自行处理。二进制升级见 [启动说明](binary.md#升级)。

## 故障排查

| 现象 | 检查位置 |
| --- | --- |
| 容器目录不可写 | 检查绑定挂载路径及 UID/GID `65532:65532` 的权限 |
| 数据库连接或迁移失败 | 查看启动日志，检查连接地址、TLS、账号权限和数据库是否存在 |
| 初次访问出现设置密码页面 | 当前数据库尚未初始化；若曾配置过，先检查数据挂载或数据库连接串 |
| HTTPS 登录后无法保持会话 | 检查访问协议与 `UPTIMENEST_COOKIE_SECURE`，以及反向代理是否保留 Host |
| 登录返回 429 | 等待 15 分钟；代理后的客户端共用来源额度 |
| Ping 故障 | 检查目标可达性、ICMP 策略、setuid 和容器网络能力 |
| HTTP/3 故障 | 检查目标 QUIC 支持、UDP 出站流量和代理配置 |
| 状态页没有服务 | 检查面板是否添加监控、展示分组是否为空以及访问路径 |
| 清理后统计变化 | 可用率和平均响应时间按剩余近 24 小时记录重新计算 |

运行 `docker compose logs --tail=100 uptimenest` 查看最近日志，`docker compose ps` 查看容器健康状态。直接运行程序可使用 `uptimenest healthcheck`。
