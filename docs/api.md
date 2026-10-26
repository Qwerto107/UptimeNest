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
| GET | `/api/public/bootstrap` | 站点名称、时区、语言、入口设置、默认面板路径、面板名称与路径、TLS 握手延迟提醒阈值 |
| GET | `/api/public/panels/:slug` | 当前面板信息、展示监控、所属分组和真实统计 |
| GET | `/api/health` | 数据库可用时返回 `{ "status": "ok" }` |

访客监控仅包含 `id`、`name`、`type`、`protocol`、可选的 `address`、`description`、`tags`、`summary`。`groups` 返回当前面板的独立展示分组（`id`、`name`、`monitorIds`），不返回后台 `groupId` 或父子结构。`protocol` 为配置的检测协议（HTTP、HTTPS、TCP、PING 或 DNS），不依赖最近一次检测是否成功。`address` 仅包含主机名/IP 及端口；`hideAddress: true` 时省略，不返回完整 URL 路径、查询参数、检测参数或内部错误消息。

`summary` 包含 `status`、`uptime`、`average`、`latest` 和按时间升序排列的 `heartbeats`。公开面板与后台监控列表支持查询参数 `heartbeatCount`（1～1000），供界面按容器容量补取最近记录；未传或无效时，公开接口默认 90 条、后台默认 60 条，超出上限时截取为 1000。两者的 `uptime` 和 `average` 仍按近 24 小时计算，无数据时为 `null`。状态为 `up`、`down`、`unknown` 或 `paused`。

公开面板的 `summary.heartbeats` 正常记录仅包含 `time`、`status`、`latency`，供心跳条与图表使用。可明确识别的故障记录额外包含 `failure`（`timeout`、`refused` 或 `response`）；HTTP 异常响应可附带 `statusCode`。这些值从既有标准诊断映射为白名单，未知原因不填，不返回原始错误内容。历史点不重复携带 ID 或连接详情。`summary.latest` 保留最新检测的 TLS、HTTP、QUIC 和证书等展示字段，内部消息仍清空。后台监控与历史接口继续返回完整心跳记录。

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
| GET | `/api/admin/monitors/:id/latency` | 按时间范围查询聚合延迟曲线和原始记录统计 |
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

监控类型为 `http`（包括 HTTPS）、`websocket`（WS/WSS）、`tcp`、`ping`、`dns`。`groupId: 0` 表示未分组。心跳间隔为 5–86400 秒，超时 1–120 秒，重试 0–10 次，重试间隔 1–3600 秒。

心跳、日志查询支持 `?limit=100&before=记录ID`。默认每页 100 条，上限 200 条；下一页传上一页最后一条的 ID。时间使用 Unix 毫秒。

延迟接口使用 `?range=24h`，支持 `1h`、`3h`、`6h`、`12h`、`24h`、`3d`、`1w`，省略时默认 `24h`，其他值返回 400。响应包含请求时段 `start`、`end`，受历史保留设置限制后的查询起点 `availableStart`，原始检测总数 `total`、故障次数 `failures`，以及 `statistics` 和 `buckets`。该接口需要管理员登录，不改变公开状态页的数据范围。

`statistics` 的 `average`、`p95`、`maximum` 均基于时段内全部成功检测的有效非负延迟，单位毫秒；无有效成功记录时为 `null`。`p95` 使用向上取整的最近秩计算。`buckets` 最多 480 项，按时间升序返回非空时间段，每项包含 `start`、`end`、`count`、`successes`、`failures`、`average`、`minimum`、`maximum`；无有效成功检测的段，其三个延迟值均为 `null`。总数包含未知和暂停状态的记录，`failures` 仅统计 `down`。无记录时返回空数组，前端应保留完整请求时间轴，不跨空时间段连线。范围包含起止时刻，时间段边界上的记录归入后一个时间段，请求终点归入最后一段。

心跳附加字段包括 `tlsVersion`、`httpVersion`、`tlsLatency`（毫秒）、`certExpiry`（站点证书到期时间，Unix 毫秒）、`certIssuerOrg`（站点证书 Issuer 的组织 O 名称，多值以 ` / ` 连接）、`tlsKeyExchange`（实际协商的密钥交换机制）、`tlsCipherSuite`（实际协商的完整加密套件名）。HTTP/3 另有 `quicVersion`（如 `v1`、`v2`）、`zeroRTT`（布尔值，实际是否使用 0-RTT，未使用时明确返回 `false`）；其他协议不返回这两项。HTTP/3 握手耗时为建立 QUIC/TLS 连接所需时间。

未测得的字段省略。O 缺失时不返回该字段，不用 CN 或根证书名称代替。旧记录的 `certIssuer`、`certRootCA` 不作为组织名称展示，不改写历史记录；组织名称随下一次检测写入。

### 单项 HTTP 配置

创建或更新 HTTP 监控时可附加 `http` 对象。省略该对象时默认使用 GET、h3 > h2 > h1.1、最多 10 次重定向、200–399 成功；已明确保存的配置保持不变，非 HTTP 类型不保存此对象。

```json
{
  "version": "auto",
  "connectIP": "",
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

- `version`：`http1`（仅 HTTP/1.1）、`http2`（仅 HTTP/2，明文地址使用 h2c）、`http3`（仅 HTTP/3，需 HTTPS）、`standard`（自动协商 h2 > h1.1）、`auto`（默认，自动协商 h3 > h2 > h1.1）。手动模式不降级；HTTPS 的仅 HTTP/2 模式在未协商 h2 时直接失败，不发送 HTTP/1.1 请求。请求使用环境代理时，两种自动模式均采用 TCP 协商 h2 > h1.1；仅 HTTP/3 不支持该代理。指定连接 IP 的目标直连。缺少 `version` 时使用 `auto`，旧的明确选项仍有效。
- `connectIP`：可选的单个 IPv4 或 IPv6 字符串，省略或留空使用 DNS。保存时去除首尾空白并规范化地址，不接受方括号、端口、网卡区域标识或多个地址。仅覆盖目标 URL 的主机，保留 URL 端口、默认 Host、TLS SNI 和证书域名校验；HTTP/1.1、HTTP/2、HTTP/3 均生效。匹配主机的请求绕过环境代理，同主机跳转继续覆盖，其他主机和 OAuth2 令牌请求按原规则处理。该字段不通过公开接口返回。
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

### 单项 WebSocket 配置

`type: "websocket"` 时，`target` 为完整 WS/WSS URL，可附加以下 `websocket` 对象；省略时默认仅验证握手。其他类型不会保存此对象，WebSocket 类型不保存 `http` 对象。

```json
{
  "mode": "handshake",
  "connectIP": "",
  "origin": "https://example.com",
  "subprotocols": ["chat"],
  "headers": "{\"Authorization\":\"Bearer example-token\"}"
}
```

`mode` 支持 `handshake` 和 `ping`。前者验证 HTTP/1.1 协议升级与握手校验值；后者还要求收到与随机 Ping 内容一致的 Pong。整次检测共用 `timeout`，每次重新连接，不跟随重定向。心跳模式丢弃业务消息，单条消息超过 64 KiB 则失败。延迟记录整次检测耗时，WSS 的 `tlsLatency` 和证书信息沿用心跳字段；始终校验证书。

`connectIP` 为可选的单个 IPv4/IPv6，不含端口。指定后直连目标 IP，保留 Host、TLS SNI 和域名校验；留空遵循 DNS 和环境代理。`origin` 可空，否则为不含路径、参数或凭据的 HTTP/HTTPS 来源（末尾 `/` 会移除）。`subprotocols` 最多 16 个不重复的有效协议 token，每项最多 128 字节；服务端可选择其中一个或不选择，不接受未请求的子协议。

`headers` 最多 64 KiB、64 项，格式与 HTTP 请求头相同。Origin、Connection、Upgrade 和 Sec-WebSocket-* 头由专属字段或系统管理，禁止手动覆盖。目标不允许账户密码和片段；配置、路径、查询参数及请求头不出现在公开接口，公开地址仅含主机和端口。

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

GET 响应额外包含只读字段 `clientIP`，表示按已保存的可信代理规则解析出的当前客户端 IP，与登录限流使用的地址一致。该字段不保存为配置，不通过访客接口返回；修改可信代理后重新 GET 可获取更新后的地址。

```json
{
  "siteName": "UptimeNest",
  "timezone": "Asia/Singapore",
  "language": "zh-CN",
  "entryPage": "status",
  "userAgent": "",
  "tlsLatencyWarningMs": 0,
  "retentionDays": 30
}
```

`siteName` 最多 100 个字符，留空使用 `UptimeNest`，用于界面品牌及所有页面的浏览器标题；bootstrap 公开返回该字段。`timezone` 使用 IANA 时区，新安装默认 `Asia/Singapore`，已保存值保留。`entryPage` 为 `status` 或 `admin`。UA 留空使用 `UptimeNest`。保留期限为 1–3650 天，新安装默认 30 天，已保存值保留。

`tlsLatencyWarningMs` 为 0～120000 的整数，单位毫秒，默认 0（关闭）。公开 bootstrap 返回此展示设置，状态页仅在最新 TLS 握手耗时严格大于正阈值时使用琥珀色提示；等于阈值、低于阈值或关闭时均为中性色。它不改变检测状态、超时、重试或历史统计。旧配置缺少此字段时默认关闭。

POST `/api/admin/cleanup` 无请求体，按已保存期限清理过期数据，返回 `{ "heartbeats": 0, "logs": 0 }`。
