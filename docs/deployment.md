# 部署指南

简体中文 · [English](deployment_en.md)

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
docker compose logs uptimenest
```

复制启动日志中的 `setup_token`，打开 <http://127.0.0.1:8090/admin>，填写初始化令牌并设置管理员密码。密码须非空、UTF-8 编码后最多 72 字节，不强制最短长度或复杂度。没有预设用户名或通用初始密码，也不从环境变量读取初始密码。初始化完成后再通过反向代理开放访问。

令牌仅在当前数据库没有管理员时输出，不通过 API 返回或写入数据库。未初始化时每次重启都会生成新令牌；创建管理员后失效，已有账户升级后直接登录。令牌可用于创建管理员，应限制启动日志的访问权限。

默认 SQLite 部署无须创建 `.env`。需要调整数据库或 HTTPS Cookie 时，在运行目录创建 `.env`，可参考仓库的 [.env.example](../.env.example)。

`docker-compose.yml` 直接指定镜像 `ghcr.io/qwerto107/uptimenest:latest`。固定版本时修改 `image` 为 `ghcr.io/qwerto107/uptimenest:0.6.1`；更换端口时修改 `127.0.0.1:8090:8090` 中的第一个 `8090`。默认端口仅本机可访问，公网访问需配置反向代理。

Compose 将容器命名为 `uptimenest`，可使用 `docker logs uptimenest` 查看日志。需要在同一台机器运行独立部署时，各部署使用不同的容器名称、宿主机端口和数据目录。

## 数据与目录

Compose 使用 `./data:/app/data` 绑定宿主机目录：

| 容器路径 | 内容 |
| --- | --- |
| `/app/data/uptimenest.db` | SQLite 数据库，包含账户、监控、分组、面板、设置和历史 |
| `/app/data/uptimenest.db-wal`、`-shm` | SQLite 运行时 WAL 与共享内存文件 |
| `/app/data/secret.key` | 自动生成的配置加密密钥，SQLite 与 PostgreSQL 部署均须持久化 |
| `/tmp` | 临时目录，Compose 使用上限 1 GiB 的 tmpfs |

容器以非 root 用户运行，根文件系统只读，`/app/data` 和 `/tmp` 可写。tmpfs 按实际写入占用内存，上限不等于预分配量。SQLite 放在本机磁盘，不让多个应用实例共用该数据库执行监控。

监控配置、通知渠道和告警状态使用 AES-256-GCM 加密保存，密钥位于独立文件。数据库与密钥同时泄露时仍可解密，应限制目录访问权限。前端资源随程序内嵌，不需要另行挂载页面目录。

## 启动参数

Compose 读取运行目录的 `.env`，将 Compose 中声明的参数传入容器。直接运行程序或 Go 时，通过终端或服务管理器设置环境变量，程序不会自动读取 `.env`。

| 环境变量 | 直接运行默认值 | 用途 |
| --- | --- | --- |
| `UPTIMENEST_ADDRESS` | `127.0.0.1:8090` | 监听地址；Compose 内固定为 `0.0.0.0:8090` |
| `UPTIMENEST_DB_TYPE` | `sqlite` | `sqlite` 或 `postgres` |
| `UPTIMENEST_DB_DSN` | `data/uptimenest.db` | SQLite 路径或 PostgreSQL 连接串 |
| `UPTIMENEST_WEB_DIR` | 空 | 使用内嵌页面；未内嵌源码运行回退到 `web/dist`，可指定外置目录 |
| `UPTIMENEST_COOKIE_SECURE` | `false` | HTTPS 对外访问时设为 `true`，使登录 Cookie 仅通过 HTTPS 传输 |
| `UPTIMENEST_SECRET_KEY_FILE` | `data/secret.key` | 配置加密密钥文件；首次自动生成，Compose 默认为 `/app/data/secret.key` |

Compose 的数据库路径默认为 `/app/data/uptimenest.db`。镜像地址和宿主机端口直接修改 Compose；监听地址和外置前端目录如需更改，也需同步调整 Compose 的参数及挂载。

## 后台运行设置

登录后在“系统设置”中配置以下内容，保存在数据库中：

| 分类 | 配置 |
| --- | --- |
| 显示 | 站点名称、系统显示时区、入口页面、状态页默认视图 |
| 语言与主题 | 英语或简体中文，新安装默认英语；浅色／深色主题手动切换，主题偏好保存在当前浏览器 |
| 监控全局配置 | HTTP/HTTPS User-Agent、状态页 TLS 握手延迟提醒阈值、证书临期高亮天数（监控项可覆盖） |
| 访问安全 | 当前客户端 IP 和可信代理 IP / CIDR，登录限流按识别出的客户端地址计数 |
| 历史数据 | 保留期限、手动清理；启动及每小时自动清理过期数据 |
| 账户 | 在“账户设置”修改管理员密码 |

入口页面可选择默认状态面板或管理后台。设置细节见 [使用指南](usage.md#系统设置)，监控、后台分组与状态面板分别在对应管理页面配置。

## 数据库配置

### SQLite

默认即可使用，无需部署独立数据库服务。SQLite 开启 WAL、外键和繁忙等待，使用 1 个写连接与最多 4 个只读连接。连接重建时自动应用连接配置，保留默认的 `synchronous=FULL`。

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

### 配置加密与密钥

首次启动会生成 32 字节随机密钥，以 Base64 文本写入 `UPTIMENEST_SECRET_KEY_FILE`。Linux 文件权限为 `0600`，Windows 按运行账号设置目录 ACL。密钥内容不会输出到日志或 API。

升级时，程序在事务内将监控配置、通知渠道、告警快照和通知事件转换为密文，并记录密钥校验值。已有加密数据时，密钥缺失、格式错误或不匹配都会阻止启动，不会自动替换原密钥。数据库结构升级为版本 6，旧程序无法直接读取密文。心跳详情（`heartbeats.data`）与监控日志消息（`monitor_logs.message`）不加密，其中的错误文本可能包含目标地址。

**恢复数据库时必须保留原密钥。** 备份密钥并与数据库备份分开保管。默认密钥与数据库使用同一持久化目录；需要隔离时，将原文件移动到独立持久化挂载，并同步修改路径。首次生成密钥时目录须可写；随后可以使用仅允许运行账号读取的密钥文件。

SQLite 迁移时启用安全删除并截断旧 WAL，减少替换记录的明文残留。迁移前的备份、磁盘快照及存储历史版本仍可能含有明文，应按原敏感级别保管。

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

密码验证按识别出的客户端 IP 累计失败次数，IPv6 使用 `/64` 网段。15 分钟内达到 10 次失败后暂时拒绝该来源，验证成功清零。请求频率另用令牌桶限制：突发最多 20 次，每秒恢复 1 次额度；bcrypt 同时最多执行 2 个验证任务。限流表满时淘汰最久未使用的来源，不锁定整个管理员账户。

使用反向代理时，在“系统设置 → 访问安全”中填写应用实际接收到的代理来源 IP 或 CIDR，仅填写你控制的代理。

应用只接受可信代理发送的转发头。从 `X-Forwarded-For` 右侧向左越过可信代理，取第一个不可信地址；未提供该头时使用有效的 `X-Real-IP`。列表留空或连接来源不可信时使用连接地址，同一代理后的客户端会共用额度。“当前 IP”显示实际识别结果，修改可信代理并保存后更新。

应用返回以下缓存响应头，Nginx 应保留 `Cache-Control` 和 `ETag`，避免用统一的 `expires` 覆盖 API 策略。压缩由外层 Nginx 配置，应用不生成 gzip 或 Brotli 响应。

| 内容 | Cache-Control | 条件请求 |
| --- | --- | --- |
| `assets/` 中带内容哈希的文件 | `public, max-age=31536000, immutable` | ETag |
| HTML 和其他未带哈希的静态文件 | `no-cache` | ETag |
| 公开 bootstrap 与面板 JSON | `public, max-age=0, must-revalidate` | ETag，相同内容返回 304 |
| 管理 API 和 API 错误响应 | `no-store` | 不启用公开响应缓存 |

公开接口按 IP 限制请求，IPv6 按 `/64` 聚合：允许突发 120 次，每秒恢复 4 次额度。超限返回 429，并带 `Retry-After: 1`。

## 监控网络与权限

| 监控类型 | 部署要求 |
| --- | --- |
| HTTP/HTTPS | 容器能解析目标域名并访问目标 TCP 端口；系统 CA 用于校验证书 |
| WebSocket | 容器能连接目标 WS/WSS 端口，服务端允许 HTTP/1.1 Upgrade；WSS 证书需受系统 CA 信任 |
| gRPC | 目标启用标准 Health/Check 服务，端口从容器网络可达；默认校验 TLS，可在监控项中附加私有 CA |
| HTTP/3 | 目标支持 HTTPS 与 QUIC，容器及宿主机允许对应 UDP 出站流量 |
| TCP | 目标主机与端口从容器网络可达 |
| Ping | 原生 IPv4/IPv6 ICMP；Compose 为 GID 65532 配置 `net.ipv4.ping_group_range`，不依赖系统命令或原始套接字权限 |
| DNS | 默认使用容器的系统 DNS，也可在单项配置可达的 DNS 服务器 |

容器中的本机地址不是宿主机或其他容器的本机地址。监控宿主机或内网服务时，按实际容器网络选择地址。

Compose 使用 `cap_drop: [ALL]` 和 `no-new-privileges:true`，通过 `net.ipv4.ping_group_range=65532 65532` 仅允许应用所属组创建 ICMP datagram socket。此设置不要求 `CAP_NET_RAW`，镜像也不再安装 SUID Ping。直接使用 `docker run` 时需配置同一 sysctl；改用其他运行用户时同步调整 GID。使用宿主机网络或运行环境不允许容器设置 sysctl 时，由管理员在对应网络命名空间配置组范围。

Linux 与 macOS 使用无特权 ICMP socket，Windows 通过系统 ICMP API 发送；均不启动外部 `ping`。目标或防火墙禁止 ICMP 时，探测仍会失败。Linux 的组范围含义见 [内核文档](https://docs.kernel.org/networking/ip-sysctl.html#ping-group-range)。

HTTP/3 优先模式仅在 QUIC 连接建立失败时回退。使用环境代理时，自动模式通过 TCP 协商 HTTP/2 或 HTTP/1.1，手动 HTTP/3 会提示不支持代理。指定连接 IP 的目标直接连接；需要环境代理时，在 Compose 中声明相应变量。

## 备份与恢复

备份数据库、原加密密钥和部署配置，包括 `docker-compose.yml` 和按需创建的 `.env`。密钥备份与数据库备份分开保管。使用 PostgreSQL 时，通过 `pg_dump` 或数据库自身的备份机制保存数据；应用侧的密钥文件仍需单独备份。

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

v0.6.1 将数据库结构升级为版本 6，包括版本 4 的心跳统计覆盖索引、版本 5 的配置加密迁移和版本 6 的通知事件加密迁移。首次升级耗时与历史数据量有关，升级前先备份。升级后新增的密钥文件必须持续保留。

v0.6.0 将数据库结构升级为版本 3，增加通知渠道、告警状态和投递队列。从 v0.5.0 升级前先备份；回退时恢复升级前的数据库。告警默认关闭，升级后需先配置渠道，再在监控项中开启。

旧部署若使用 `compose.yaml` 的 `uptimenest-data` 命名卷，先用旧配置停止服务，再把原卷数据完整迁入 `./data`，调整 UID/GID 权限后使用新配置启动；不要同时运行两套实例。保留原卷和备份，确认迁移完成后再自行处理。二进制升级见 [启动说明](binary.md#升级)。

## 故障排查

| 现象 | 检查位置 |
| --- | --- |
| 容器目录不可写 | 检查绑定挂载路径及 UID/GID `65532:65532` 的权限 |
| 数据库连接或迁移失败 | 查看启动日志，检查连接地址、TLS、账号权限和数据库是否存在 |
| 密钥文件缺失或无法解密配置 | 检查 `UPTIMENEST_SECRET_KEY_FILE` 与挂载路径，恢复该数据库原有的密钥文件 |
| 初次访问出现设置密码页面 | 当前数据库尚未初始化；若曾配置过，先检查数据挂载或数据库连接串 |
| 初始化令牌不正确 | 从本次启动日志重新复制 `setup_token`；重启前的令牌已失效 |
| HTTPS 登录后无法保持会话 | 检查访问协议与 `UPTIMENEST_COOKIE_SECURE`，以及反向代理是否保留 Host |
| 登录返回 429 | 等待 15 分钟，检查当前客户端 IP 和可信代理配置是否符合部署方式 |
| Ping 故障 | 检查目标可达性和 ICMP 策略；Linux 检查运行 GID 与 `ping_group_range`，IPv6 还需具备对应网络连通性 |
| HTTP/3 故障 | 检查目标 QUIC 支持、UDP 出站流量和代理配置 |
| 状态页没有服务 | 检查面板是否添加监控、展示分组是否为空以及访问路径 |
| 清理后统计变化 | 可用率和平均响应时间按剩余近 24 小时记录重新计算 |

运行 `docker compose logs --tail=100 uptimenest` 查看最近日志，`docker compose ps` 查看容器健康状态。直接运行程序可使用 `uptimenest healthcheck`。
