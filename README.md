# UptimeNest

基于 Vue 3 与 Go 的服务、端口和网络监控系统。前台面向访客展示服务状态，后台管理监控、分组、面板和系统设置。

## 已实现功能

- HTTP/HTTPS、TCP 端口、Ping、DNS 检测；失败重试、反转模式、启用与停用。
- HTTP/3（优先尝试并回退，或仅 HTTP/3）、单项重定向与有效状态码、请求方法、请求体编码/内容、自定义请求头；支持 Basic、OAuth2 客户端凭据和 mTLS。
- HTTPS 的 TLS/HTTP 版本、TLS 握手耗时、证书剩余有效期及签发者组织（O）；展开查看实际密钥交换机制与加密套件。
- 后台父子树形分组、折叠与子树状态汇总；状态面板独立展示分组、拖拽及按钮排序、跨组移动。支持监控标签、备注及单项历史查询和独立清空。
- 多状态面板、独立路径、默认面板及根路径入口配置。
- 简体中文、可扩展国际化结构、浅色/深色主题、可配置显示时区。
- 全局 HTTP User-Agent；历史保留期限、手动清理、启动时及每小时自动清理。
- 首次设置管理员密码、登录、修改密码、会话撤销。
- SQLite 与 PostgreSQL 二选一，启动时执行版本化数据库迁移。

当前版本不包含通知管理、通知配置、通知推送或其他语言。监控引擎的 `StateHandler` 为后续状态变化订阅提供简单入口。

## 本地运行

需要 Go **1.27.3+** 和 Node.js **22.12+**（建议使用 Node.js 24）。Ping 监控需要操作系统提供 `ping` 命令，并允许 ICMP。

在项目根目录执行：

```sh
cd web
npm ci
npm run build
cd ..
go run ./cmd/uptimenest
```

打开 `http://127.0.0.1:8090/admin`，首次访问时设置管理员密码。系统没有通用初始密码。初始化后创建监控项，再创建状态面板。

默认仅监听本机。首次初始化应在可信环境完成，完成后再开放外部访问。

前端开发时分别运行：

```sh
# 终端一：项目根目录
go run ./cmd/uptimenest

# 终端二：前端目录
cd web
npm run dev
```

访问 Vite 输出的地址。开发服务器将 `/api` 转发到 `127.0.0.1:8090`。

## 运行配置

配置通过环境变量读取；程序不会自动读取 `.env`。可参考 `.env.example`。

| 变量 | 默认值 | 说明 |
| --- | --- | --- |
| `UPTIMENEST_ADDRESS` | `127.0.0.1:8090` | HTTP 监听地址 |
| `UPTIMENEST_DB_TYPE` | `sqlite` | `sqlite` 或 `postgres` |
| `UPTIMENEST_DB_DSN` | `data/uptimenest.db` | SQLite 文件路径，或 PostgreSQL 连接串 |
| `UPTIMENEST_WEB_DIR` | 空 | 发行程序使用内嵌页面；未内嵌的源码运行回退到 `web/dist`。可指定目录覆盖，支持绝对路径 |
| `UPTIMENEST_COOKIE_SECURE` | `false` | 经 HTTPS 对外提供服务时设为 `true` |

设置示例（PowerShell）：

```powershell
$env:UPTIMENEST_DB_TYPE = 'postgres'
$env:UPTIMENEST_DB_DSN = 'postgres://uptimenest:YOUR_PASSWORD@127.0.0.1:5432/uptimenest?sslmode=disable'
go run ./cmd/uptimenest
```

PostgreSQL 数据库应预先创建。生产连接应使用合适的 TLS 设置，如 `sslmode=verify-full`，并配置可信证书；连接串中的特殊字符需要按 URI 规则编码。

系统显示时区默认为 `Asia/Shanghai`，可在后台更改。语言仅支持 `zh-CN`。主题偏好保存在当前浏览器，首次访问默认浅色。

## 发行程序与 CI/CD

发行程序内嵌前端，下载、校验和启动方式见 [二进制部署](docs/binary.md)。本地构建完整程序：

```sh
cd web
npm ci --no-audit --no-fund
npm run build
cd ..
node scripts/prepare-webui.mjs
CGO_ENABLED=0 go build -tags=with_frontend -trimpath -ldflags='-s -w' -o bin/uptimenest ./cmd/uptimenest
```

GitHub Actions 按 QPix v0.4.0 适配：普通提交检查，手动运行打包，版本标签触发镜像与 Release；前端只构建一次，镜像复用本次二进制。支持七平台下载包及三个 Linux 架构镜像，完整规则见 [发布流程](docs/releasing.md)。

## Docker

SQLite 单实例：

```sh
docker compose up -d --build
```

默认绑定 `127.0.0.1:8090`，数据库保存在命名卷中。通过 HTTPS 反向代理开放访问时，修改监听映射及 `UPTIMENEST_COOKIE_SECURE`。

也可构建镜像后用环境变量连接已有 PostgreSQL：

```sh
docker build -t uptimenest .
docker run -d --name uptimenest -p 127.0.0.1:8090:8090 \
  -e UPTIMENEST_DB_TYPE=postgres \
  -e UPTIMENEST_DB_DSN='postgres://USER:PASSWORD@HOST:5432/uptimenest?sslmode=verify-full' \
  uptimenest
```

容器内提供 BusyBox `ping`。部分宿主机或容器策略可能需要允许 ICMP 或配置 `NET_RAW` 能力。监控目标为容器自身网络可访问的地址；容器中的 `127.0.0.1` 指容器自身。使用已发布镜像部署见 [发布流程](docs/releasing.md#容器镜像)。

## 页面与 API

公开状态页采用用户提供的《状态页设计 · Pulse Status.html》中的浅色与深色方案，正文、协议和数值分别使用附件内的 Inter 与 JetBrains Mono 字体文件；字体在本地加载，版权及 OFL 许可见 `web/public/fonts/OFL.txt`。服务名右侧显示主机地址与小号协议胶囊，连接信息按类型使用彩色胶囊。可在单项监控编辑中开启“在状态页隐藏地址”。前台显示最近 90 次真实心跳，可用率仍按近 24 小时统计，后台预览保留 60 次；不包含参考中的演示事件或 90 天统计。后台保留原有界面规范。

| 路径 | 用途 |
| --- | --- |
| `/` | 按入口设置跳转到默认面板或后台；无面板时显示配置引导 |
| `/status/:slug` | 访客状态面板 |
| `/admin/login` | 首次设置、管理员登录 |
| `/admin/monitors` | 监控管理 |
| `/admin/groups` | 分组管理 |
| `/admin/panels` | 面板管理 |
| `/admin/settings` | 系统设置 |
| `/admin/account` | 密码修改 |
| `/api/public/*` | 无需登录的展示 API，仅返回公开字段 |
| `/api/admin/*` | 管理 API，使用 HttpOnly 会话 Cookie |
| `/api/health` | 进程和数据库健康检查 |

API 详细约定见 [docs/api.md](docs/api.md)。如果单独部署前端，反向代理需将 `/api` 转发到 Go 服务，并将其他页面路径回退到 `index.html`。浏览器页面与 API 应保持同源，代理保留原始 `Host`；后台写入接口拒绝跨源 `Origin`。

## 检测与统计规则

- 每项监控单独调度，总并发上限为 32；启用后立即检测，完整检测（含重试）完成后再等待心跳间隔。
- 检测超时为每次尝试的上限；重试期间不提前记录故障，重试耗尽后写入最终心跳。
- HTTP 默认 GET、自动协商 HTTP/1.1 / HTTP/2、最多 10 次重定向、接受 200–399；可按单项修改方法、状态码和重定向上限（0 禁止跟随，仍判断当前响应状态码）。旧监控沿用默认行为。
- HTTP/3 需 HTTPS 与 UDP 连通，可选择优先尝试后回退或仅 HTTP/3；仅 QUIC 连接建立失败时自动回退，不对已发出的请求做协议回退重发。环境代理存在时，自动模式使用 HTTP/1.1 / HTTP/2，仅 HTTP/3 模式返回不支持代理的错误。
- HTTP 请求体支持 JSON、URL 编码表单、XML 和纯文本；请求头填写字符串值的 JSON 对象，支持 Host，Content-Length 等由系统管理。单项请求头的 User-Agent 可覆盖全局默认值。
- Basic、OAuth2 客户端凭据、mTLS 在单项编辑中配置。OAuth2 每次检测获取访问令牌，包含在整体超时内；mTLS 使用 PEM 证书和私钥，可附加 CA，同时保留服务端证书校验。跨来源重定向不转发自定义请求头和验证凭据，mTLS 拒绝跨来源重定向。
- 实际使用 HTTP/3 时，展开详情额外显示实际 QUIC 版本和 0-RTT 使用状态；当前不主动启用 0-RTT，按真实连接显示“未使用”。HTTP/1.1 / HTTP/2 不显示这两项。
- 每次 HTTP 检测建立新连接，确保 TLS 握手耗时对应本次连接；总响应时间包含少量响应体读取。
- Ping 使用系统命令，总响应时间包含命令执行开销。
- DNS 支持 A、AAAA、CNAME、MX、NS、TXT；服务器留空使用系统 DNS，也可指定 `IP:端口`，支持精确匹配任一解析结果。
- 反转模式对可用性判断取反：不可访问时正常，可访问时故障；全局平均响应时间不包含反转监控。
- 公开状态页心跳条显示最近 90 次完整检测，后台预览显示最近 60 次，均按时间从早到晚排列，**不代表固定 24 小时或 90 天**。不足指定条数时以灰色补齐，无数据不显示 100%。
- 单项可用率基于最近 24 小时的最终心跳计算；面板可用率是有数据服务的可用率均值，平均响应时间取各项正常检测均值的平均值。
- 状态改变和故障生成监控日志，未改变的正常检测只记录心跳。
- 停用会取消进行中的请求、重试等待及系统 Ping 子进程。旧配置的检测结果不会写回新配置。
- 清空与过期清理只删除心跳或日志，监控配置保留；统计每次从数据库重新计算。

## 数据和运行边界

- 当前按**单个应用实例**设计。同一数据库不要同时启动多个检测实例，否则会重复检测；迁移锁不等同于任务选主。
- SQLite 使用 WAL 与单数据库连接；PostgreSQL 使用连接池。
- 后台分组与状态页分组分别管理。删除后台分组时，直属监控移到未分组，子分组上移一级，各面板分组保留；删除面板展示分组只从该面板移除对应展示，不删除监控或历史。删除监控会级联删除其历史，并从所有面板分组移除。
- 数据库 v2 增加后台父级和独立面板分组；升级时按旧页面的分组名称及组内顺序生成快照，后台后续改名、删除或移动分组不改变面板布局。
- 任何面板都属于公开展示面板；备注和标签也是公开内容。默认公开主机名/IP 及端口，启用单项隐藏后公开接口不返回该地址。完整 URL 路径、查询参数、DNS 服务器、UA、账户及内部错误消息不通过访客接口返回。
- HTTP 请求配置和验证凭据仅由管理员接口读写，随配置保存在数据库中；应限制数据库及备份的访问权限。它们不进入公开响应或心跳记录。
- 密码使用 bcrypt 保存，不设置字符组合规则；非空，UTF-8 编码后最多 72 字节。会话有效期 7 天，数据库仅保存令牌摘要；修改密码后撤销其他会话。
- HTTP 服务本身不终止 TLS，生产环境可放在 HTTPS 反向代理后。
- 备份应覆盖数据库。SQLite 运行中备份需采用 SQLite 一致性备份方式，避免只复制正在写入的主文件而遗漏 WAL。

## 项目结构

```text
cmd/uptimenest/       程序入口
internal/config/     环境配置
internal/model/      共用数据类型
internal/store/      持久化、迁移和历史统计
internal/monitor/    四类检测、任务调度和状态事件
internal/api/        账户、管理及访客接口
web/src/components/  业务组件与本地 UI 组件
web/src/locales/     中文语言资源
web/src/views/       访客页与后台页面
docs/                API 和验收说明
```

前端组件按 shadcn-vue 的本地源码模式组织，使用 Reka UI 处理无障碍交互，可直接维护源码。所有自写业务注释使用中文。
