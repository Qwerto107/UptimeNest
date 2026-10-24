# API 参考

[文档中心](README.md) · [使用指南](usage.md) · [部署指南](deployment.md)

| 内容 | 章节 |
| --- | --- |
| 接入约定 | [请求与错误](#请求约定) · [登录会话](#管理员账户) |
| 状态展示 | [访客接口](#访客接口) |
| 监控配置 | [监控管理](#监控管理) · [HTTP 配置](#单项-http-配置) |
| 系统管理 | [分组与面板](#分组与面板) · [设置与清理](#设置与清理) |

## 请求约定

请求与响应均使用 JSON，错误格式为 `{ "error": "中文提示" }`。写入接口需要 `Content-Type: application/json`，请求体上限为 1 MiB。不提供跨域访问；浏览器 API 与页面需保持同源。

公开接口前缀为 `/api/public`，管理接口前缀为 `/api/admin`；健康接口为 `/api/health`。下文使用完整路径。

| 状态码 | 含义 |
| --- | --- |
| 400 | 参数错误、JSON 请求无效或超出上限、当前密码不正确 |
| 401 | 管理接口未登录 |
| 403 | 跨源写入被拒绝 |
| 404 | 记录或接口不存在 |
| 409 | 名称、路径或初始化冲突 |
| 415 | 请求体类型不是 JSON |
| 429 | 密码尝试限流 |
| 500／503 | 服务处理失败或数据库暂不可用 |

## 访客接口

| 方法 | 路径 | 返回内容 |
| --- | --- | --- |
| GET | `/api/public/bootstrap` | 站点名称、时区、语言、入口设置、默认面板路径、面板名称与路径 |
| GET | `/api/public/panels/:slug` | 当前面板信息、展示监控、所属分组和真实统计 |
| GET | `/api/health` | 数据库可用时返回 `{ "status": "ok" }` |

访客监控仅包含 `id`、`name`、`type`、`protocol`、可选的 `address`、`description`、`tags`、`summary`。`groups` 返回当前面板的独立展示分组（`id`、`name`、`monitorIds`），不返回后台 `groupId` 或父子结构。`protocol` 为配置的检测协议（HTTP、HTTPS、TCP、PING 或 DNS），不依赖最近一次检测是否成功。`address` 仅包含主机名/IP 及端口；`hideAddress: true` 时省略，不返回完整 URL 路径、查询参数、检测参数或内部错误消息。

`summary` 包含 `status`、`uptime`、`average`、`latest` 和按时间升序排列的 `heartbeats`。公开面板与后台监控列表支持查询参数 `heartbeatCount`（1～1000），供界面按容器容量补取最近记录；未传或无效时，公开接口默认 90 条、后台默认 60 条，超出上限时截取为 1000。两者的 `uptime` 和 `average` 仍按近 24 小时计算，无数据时为 `null`。状态为 `up`、`down`、`unknown` 或 `paused`。

## 管理员账户

| 方法 | 路径 | 请求体 |
| --- | --- | --- |
| GET | `/api/admin/session` | 无；返回初始化与登录状态 |
| POST | `/api/admin/setup` | `{ "password": "..." }`，仅首次设置可用 |
| POST | `/api/admin/session` | `{ "password": "..." }` |
| DELETE | `/api/admin/session` | 无；退出当前会话 |
| PUT | `/api/admin/password` | `{ "current": "...", "password": "..." }` |

登录与首次设置成功后写入 HttpOnly、SameSite=Strict Cookie。后续管理接口需要该 Cookie。密码尝试按客户端连接地址限流，每 15 分钟最多 10 次；反向代理场景下同一代理后的客户端共用额度。

## 监控管理

| 方法 | 路径 | 说明 |
| --- | --- | --- |
| GET | `/api/admin/monitors` | 监控配置与实时统计 |
| POST | `/api/admin/monitors` | 创建监控，返回 201 |
| PUT | `/api/admin/monitors/:id` | 替换监控配置 |
| DELETE | `/api/admin/monitors/:id` | 删除监控及其心跳、日志和面板引用 |
| PUT | `/api/admin/monitors/:id/enabled` | `{ "enabled": true }` 或 `false` |
| GET | `/api/admin/monitors/:id/heartbeats` | 查询心跳，最新优先 |
| DELETE | `/api/admin/monitors/:id/heartbeats` | 清空该项心跳，返回删除数量 |
| GET | `/api/admin/monitors/:id/logs` | 查询日志，最新优先 |
| DELETE | `/api/admin/monitors/:id/logs` | 清空该项日志，返回删除数量 |

监控配置示例：

```json
{
  "name": "主站",
  "type": "http",
  "target": "https://example.com",
  "hideAddress": false,
  "port": 443,
  "dnsRecord": "A",
  "dnsServer": "",
  "expected": "",
  "interval": 60,
  "retries": 2,
  "retryInterval": 10,
  "timeout": 10,
  "invert": false,
  "description": "官网入口",
  "tags": ["生产环境"],
  "groupId": 0,
  "enabled": true
}
```

监控类型为 `http`（包括 HTTPS）、`tcp`、`ping`、`dns`。`groupId: 0` 表示未分组。心跳间隔为 5–86400 秒，超时 1–120 秒，重试 0–10 次，重试间隔 1–3600 秒。

心跳、日志查询支持 `?limit=100&before=记录ID`。默认每页 100 条，上限 200 条；下一页传上一页最后一条的 ID。时间使用 Unix 毫秒。

心跳附加字段包括 `tlsVersion`、`httpVersion`、`tlsLatency`（毫秒）、`certExpiry`（站点证书到期时间，Unix 毫秒）、`certIssuerOrg`（站点证书 Issuer 的组织 O 名称，多值以 ` / ` 连接）、`tlsKeyExchange`（实际协商的密钥交换机制）、`tlsCipherSuite`（实际协商的完整加密套件名）。HTTP/3 另有 `quicVersion`（如 `v1`、`v2`）、`zeroRTT`（布尔值，实际是否使用 0-RTT，未使用时明确返回 `false`）；其他协议不返回这两项。HTTP/3 握手耗时为建立 QUIC/TLS 连接所需时间。

未测得的字段省略。O 缺失时不返回该字段，不用 CN 或根证书名称代替。旧记录的 `certIssuer`、`certRootCA` 不作为组织名称展示，不改写历史记录；组织名称随下一次检测写入。

### 单项 HTTP 配置

创建或更新 HTTP 监控时可附加 `http` 对象。旧记录省略该对象时保持 GET、HTTP/1.1 / HTTP/2、最多 10 次重定向、200–399 成功的行为；非 HTTP 类型不保存此对象。

```json
{
  "version": "auto",
  "ignoreTLS": false,
  "maxRedirects": 0,
  "validStatusCodes": ["200-299", "401"],
  "method": "POST",
  "bodyEncoding": "json",
  "body": "{\"ready\":true}",
  "headers": "{\"Host\":\"example.com\",\"Accept\":\"application/json\"}",
  "auth": { "type": "none" }
}
```

- `version`：`standard`（默认，HTTP/1.1 / HTTP/2）、`auto`（优先 HTTP/3，连接失败时回退）、`http3`（仅 HTTP/3，需 HTTPS）。自动模式在存在环境代理时使用 TCP 协议；仅 HTTP/3 不支持该代理。
- `ignoreTLS`：布尔值，默认 `false`。仅此监控的检测目标跳过证书链、有效期及域名校验；适用于 HTTPS 和 HTTP/3，不关闭 OAuth2 令牌请求的证书校验。HTTP 配置不通过公开接口返回。
- `maxRedirects`：0–100，省略为 10；0 不跟随跳转。响应是否成功仍由状态码配置决定。
- `validStatusCodes`：单码或闭区间字符串数组，范围 100–599，如 `200`、`200-299`；至少一项、最多 100 项。
- `method`：GET、POST、HEAD、PUT、PATCH、DELETE、OPTIONS、TRACE、CONNECT。
- `bodyEncoding`：`json`、`form`、`xml`、`text`。JSON 验证语法；表单填写 `key=value&key2=value2`；XML、纯文本按原文发送。请求体最多 256 KiB。
- `headers`：字符串值的 JSON 对象文本，最多 64 项、64 KiB。Host 单独应用于请求主机字段；Content-Length、Transfer-Encoding、Trailer 由系统管理。跨来源跳转不转发配置的请求头、Authorization 或 Cookie。

`auth.type` 可为 `none`、`basic`、`oauth2`、`mtls`：

| 方式 | 配置字段 |
| --- | --- |
| Basic | `username`、`password` |
| OAuth2 客户端凭据 | `tokenURL`、`clientID`、`clientSecret`、可选 `scope`（空格分隔）、`tokenAuthMethod`（`header` 默认或 `body`） |
| mTLS | `clientCert`（PEM 证书链）、`clientKey`（未加密的 PEM 私钥）、可选 `caCert`（附加服务端信任 CA，每项最多 64 KiB） |

OAuth2 使用 `client_credentials`，Token 请求不跟随重定向，错误不会输出令牌响应。mTLS 保留系统信任及服务端证书校验，拒绝跨来源重定向。HTTP 配置和凭据不通过公开接口返回。

## 分组与面板

`/api/admin/groups` 与 `/api/admin/panels` 都支持 GET、POST；`/:id` 支持 PUT、DELETE。

后台分组写入体：`{ "name": "数据库", "parentId": 1 }`。`parentId: 0`（默认）表示顶级分组。父级必须存在，不能选自身或子孙；名称仍全局唯一。删除后，直属监控移至未分组、子分组上移至被删分组的父级，监控历史及独立面板分组不变。

面板写入体：

```json
{
  "name": "生产环境",
  "pageTitle": "生产环境服务状态",
  "slug": "production",
  "description": "各项服务的实时状态与可用性",
  "groups": [
    { "id": "public-api", "name": "服务接口", "monitorIds": [2, 1] },
    { "id": "public-network", "name": "网络节点", "monitorIds": [3] }
  ],
  "isDefault": true
}
```

`groups` 数组顺序决定展示分组顺序，各 `monitorIds` 决定组内顺序。分组 ID 在当前面板内唯一，为 1–64 位字母、数字、下划线或连字符；名称不能为空，最多 200 字节。每个面板最多 100 个展示分组、1000 个不同监控，同一个监控在一个面板内不能重复，可在不同面板中使用不同展示分组。空分组可保存，公开页不展示空分组。

管理响应仍有扁平 `monitorIds` 兼容索引，保存 `groups` 时自动按分组顺序重建。旧客户端只提交 `monitorIds` 时，按提交时的后台分组生成独立快照；维护独立布局的客户端应提交 `groups`。数据库 v1 升级到 v2 时冻结旧面板名称与顺序；之后的后台分组调整不会改变已保存展示分组。移除展示分组不删除监控或历史；删除监控会同步清理各面板中的引用。

`slug` 唯一，最长 64 位，以小写字母或数字开头，其余允许连字符。首个面板自动成为默认面板，设为默认会取消其他默认标记；删除默认面板后递补剩余的首个面板。

`pageTitle` 为可选字符串，最多 200 个字符；替换状态页左上角名称，留空使用全局站点名称，不新增标题行。公开面板响应返回该字段。浏览器标题统一使用 bootstrap 返回的全局 `siteName`，不提供面板级浏览器标题。

## 设置与清理

GET、PUT `/api/admin/settings`：

```json
{
  "siteName": "UptimeNest",
  "timezone": "Asia/Singapore",
  "language": "zh-CN",
  "entryPage": "status",
  "userAgent": "",
  "retentionDays": 30
}
```

`siteName` 最多 100 个字符，留空使用 `UptimeNest`，用于界面品牌及所有页面的浏览器标题；bootstrap 公开返回该字段。`timezone` 使用 IANA 时区，新安装默认 `Asia/Singapore`，已保存值保留。`entryPage` 为 `status` 或 `admin`。UA 留空使用 `UptimeNest`。保留期限为 1–3650 天，新安装默认 30 天，已保存值保留。

POST `/api/admin/cleanup` 无请求体，按已保存期限清理过期数据，返回 `{ "heartbeats": 0, "logs": 0 }`。
