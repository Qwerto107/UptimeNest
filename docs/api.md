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

访客监控仅包含 `id`、`name`、`type`、`protocol`、可选的 `address`、`description`、`tags`、可选的 `certExpiryWarningDays`、`summary`。`groups` 返回当前面板的独立展示分组（`id`、`name`、`monitorIds`），不返回后台 `groupId` 或父子结构。`protocol` 为配置的检测协议（HTTP、HTTPS、WS、WSS、gRPC、TCP、PING 或 DNS），不依赖最近一次检测是否成功。`address` 仅包含主机名/IP 及端口；`hideAddress: true` 时省略，不返回完整 URL 路径、查询参数、检测参数或内部错误消息。

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

登录与首次设置成功后写入 HttpOnly、SameSite=Strict Cookie。后续管理接口需要该 Cookie。密码尝试按可信代理规则识别出的客户端 IP 限流，每 15 分钟最多 10 次；未配置可信代理时使用连接地址。

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

监控类型为 `http`（包括 HTTPS）、`websocket`（WS/WSS）、`grpc`、`tcp`、`ping`、`dns`。`groupId: 0` 表示未分组。Ping 心跳间隔为 1～86400 秒，其他协议为 5～86400 秒。间隔从上一轮完整检测结束后开始计时，检测耗时和失败重试会延长实际周期。超时为 1～120 秒，重试为 0～10 次，重试间隔为 1～3600 秒。

监控级 `certExpiryWarningDays` 可选，接受 0～3650 的整数。省略或传 `null` 时继承系统设置；0 关闭提前高亮，过期仍为红色。配置独立天数后，公开监控返回该字段，包括 0；继承时省略。此字段只影响证书标签显示，与 `alerts.certificate.days` 独立。

心跳、日志查询支持 `?limit=100&before=记录ID&beforeTime=记录时间`。默认每页 100 条，有效范围为 1～200，超出范围回退为 100。下一页传上一页最后一条的 ID 和时间，按时间与 ID 联合分页；只传 ID 的旧请求仍兼容，由服务端查找该记录的时间。时间使用 Unix 毫秒。

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

### 单项 gRPC 配置

`type: "grpc"` 时，`target` 为主机名或 IP，`port` 为 1～65535 的端口。可附加以下 `grpc` 对象，其他类型不保存该对象：

```json
{
  "security": "tls",
  "serviceName": "",
  "metadata": "{\"authorization\":\"Bearer example-token\"}",
  "caCert": ""
}
```

`security` 支持 `tls`（默认）和 `none`（明文）。TLS 始终校验证书，`caCert` 可填写附加 PEM 信任证书，最多 64 KiB；明文模式不使用该证书。`serviceName` 最多 1000 字节，去除首尾空白，不能包含换行或空字符；留空检查整个实例。

`metadata` 为 JSON 对象文本，名称和值均为字符串，最多 64 项、64 KiB。名称忽略大小写，仅允许字母、数字、下划线、点和连字符，最多 100 字符；值仅接受可打印 ASCII。拒绝 `grpc-` 保留前缀、`-bin` 二进制字段、重复名称和协议请求头覆盖。

每次调用 `grpc.health.v1.Health/Check`，仅 `SERVING` 判为正常，不回退为端口连通检测。单次连接与健康检查共用 `timeout`，响应消息上限为 64 KiB，连接直达目标，不使用环境 HTTP 代理。TLS 信息复用心跳字段，可配置证书告警。公开接口返回 `protocol: "gRPC"` 和主机／端口，省略 Metadata、CA 和服务名称。

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
  "statusPageView": "detailed",
  "userAgent": "",
  "tlsLatencyWarningMs": 0,
  "certExpiryWarningDays": 30,
  "trustedProxies": [],
  "retentionDays": 30
}
```

`siteName` 最多 100 个字符，留空使用 `UptimeNest`，用于界面品牌及所有页面的浏览器标题；bootstrap 公开返回该字段。`timezone` 使用 IANA 时区，新安装默认 `Asia/Singapore`，已保存值保留。`entryPage` 为 `status` 或 `admin`。UA 留空使用 `UptimeNest`。保留期限为 1–3650 天，新安装默认 30 天，已保存值保留。

`tlsLatencyWarningMs` 为 0～120000 的整数，单位毫秒，默认 0（关闭）。公开 bootstrap 返回此展示设置，状态页仅在最新 TLS 握手耗时严格大于正阈值时使用琥珀色提示；等于阈值、低于阈值或关闭时均为中性色。它不改变检测状态、超时、重试或历史统计。旧配置缺少此字段时默认关闭。

`statusPageView` 为 `detailed`（详细行，默认）、`compact`（紧凑行）或 `cards`（卡片网格）。公开 bootstrap 返回此字段，旧设置缺少时默认详细行。访客切换只保存在当前页面内存中，数据刷新保留，重新载入页面或切换面板后恢复默认值；没有保存访客选择的接口或浏览器存储。

`certExpiryWarningDays` 为 0～3650 的整数，默认 30。公开 bootstrap 返回该字段。证书剩余有效期不超过此天数时，状态页使用琥珀色背景；0 关闭临期高亮，过期仍使用红色。它不触发通知，与监控的 `alerts.certificate.days` 独立。旧配置缺少字段时保持 30 天。

`trustedProxies` 为最多 100 项的 IP 或 CIDR 数组。仅当连接来源可信时才解析转发头，优先按 `X-Forwarded-For` 的代理链识别客户端，缺少该头时使用有效的 `X-Real-IP`。留空时使用连接地址；配置不通过访客接口返回。

POST `/api/admin/cleanup` 无请求体，按已保存期限清理过期数据，返回 `{ "heartbeats": 0, "logs": 0 }`。

## 告警与通知

以下接口均需管理员会话。告警信息和渠道凭据不进入访客接口。

| 方法 | 路径 | 用途 |
| --- | --- | --- |
| GET / POST | `/api/admin/notification-channels` | 查询／创建渠道 |
| PUT / DELETE | `/api/admin/notification-channels/:id` | 更新／删除渠道 |
| POST | `/api/admin/notification-channels/test` | 使用当前渠道配置和模板同步测试，成功返回 `{ "sent": true }`；无需保存，支持未启用渠道，不进入重试队列。传入已保存渠道的 `id` 时可保留脱敏凭据，不更新该渠道 |
| POST | `/api/admin/notification-channels/:id/test` | 对已保存且启用的渠道排入测试通知，返回 202；同一渠道有待发测试时拒绝重复入队 |
| GET | `/api/admin/notifications/templates` | 返回 `variables` 和 `channels`；各渠道类型包含服务模板 `subject`、`body`，证书模板 `certificateTemplate` 和消息格式 `format`；保留旧的默认字段兼容已有调用 |
| POST | `/api/admin/notifications/preview` | 请求 `{ "type": "webhook", "subject": "{{alert_type}}", "body": "{\"message\":\"{{monitor_name}}\"}", "alertType": "service_down" }`，返回渲染后的 `subject` 和 `body` |
| GET | `/api/admin/notifications/records` | 最近 50 条发送记录；下一页用 `?before=上一页末条ID` |

渠道包含 `id`、`name`、`type`、`enabled`、`subject`、`body`、`certificateTemplate` 和 `config`。顶层 `subject`、`body` 用于服务故障和恢复。`certificateTemplate` 包含独立的 `subject`、`body`，用于证书临期、过期和恢复。旧渠道缺少此字段时沿用原模板；更新已拆分渠道时省略此字段，会保留已保存的证书模板。

`type` 为 `email`、`webhook`、`feishu`、`dingtalk` 或 `telegram`，创建后不能更换类型。每套模板的标题上限为 1000 字节，正文上限为 12000 字节；远端渠道可能有更小的消息限制。

草稿测试接口可额外传入 `alertType`，根据所选示例事件选择服务或证书模板，支持 `service_down`、`service_recovered`、`certificate_expiring`、`certificate_expired` 和 `certificate_recovered`。省略时使用 `test`，走服务模板。测试不保存渠道或模板，不进入重试队列。

`config` 按渠道使用以下字段：

- 邮件：`host`、`port`、`security`（`tls`／`starttls`／`none`）、`username`、`password`、`from`、`to`（邮箱数组）。
- Webhook：`url`、`headers`（字符串键值对象）；正文模板必须是有效 JSON，变量仅用于字符串值。
- 飞书／钉钉：`url`、可选 `secret`。
- Telegram：`token`、`chatId`、`threadId`（0 表示不指定）。
- HTTP 渠道可选 `proxyURL`，为 HTTP/HTTPS 代理地址。

读取渠道时，`url`、`password`、`secret`、`token`、`headers` 和 `proxyURL` 返回空值，并用 `configured` 字符串数组列出已保存的字段。PUT 时敏感字段留空保留旧值；要清除可选敏感字段，在请求顶层传入 `clearSecrets` 数组。普通字段按完整配置更新，不进行部分合并。

监控创建／更新接口增加可选的 `alerts`，旧配置缺少此字段时告警关闭：

```json
{
  "alerts": {
    "service": { "enabled": true, "channelIds": [1], "recovery": true, "repeatMinutes": 0, "days": 0, "failureThreshold": 3 },
    "certificate": { "enabled": true, "channelIds": [1], "recovery": false, "repeatMinutes": 0, "days": 15 }
  }
}
```

每类规则最多关联 50 个渠道。`repeatMinutes` 范围为 0～525600，0 关闭重复提醒；证书告警的 `days` 为一个 1～3650 的整数，适用于 HTTPS、WSS 和启用 TLS 的 gRPC。服务规则的 `failureThreshold` 为独立的连续失败次数，范围 1～1000，省略或为 0 时补齐为 1；每轮最终失败计一次，成功清零，定时提醒不计数。变量和事件类型见 [告警通知](usage.md#告警通知)。

发送记录包含事件快照 `event`、`channelId`、`channelName`、`monitorId`、`kind`、`status`、`attempts`、`nextAttempt` 和脱敏错误 `error`。`kind` 为 `service`、`certificate` 或 `test`；状态为 `pending`、`sending`、`sent`、`failed`、`cancelled`。不返回渠道凭据或完整远端响应。
