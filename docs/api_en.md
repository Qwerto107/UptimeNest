# API reference

[简体中文](api.md) · English

[Documentation](README_en.md) · [Usage guide](usage_en.md) · [Deployment guide](deployment_en.md)

| Topic | Sections |
| --- | --- |
| Integration | [Requests and errors](#request-conventions) · [Sessions](#administrator-account) |
| Status display | [Public endpoints](#public-endpoints) |
| Monitor configuration | [Monitor management](#monitor-management) · [HTTP configuration](#per-monitor-http-configuration) |
| Administration | [Groups and panels](#groups-and-panels) · [Settings and cleanup](#settings-and-cleanup) |

## Request conventions

Requests and responses use JSON. Errors have the form `{ "error": "中文提示" }` (a Chinese error message). Write endpoints require `Content-Type: application/json`; bodies are limited to 1 MiB. Cross-origin access is not provided; browser API requests and pages must share an origin.

Public endpoints use `/api/public`, admin endpoints `/api/admin`, and health checks `/api/health`. Full paths are listed below.

| Status | Meaning |
| --- | --- |
| 400 | Invalid parameters, invalid/oversized JSON, or incorrect current password |
| 401 | Admin request without a login session |
| 403 | Cross-origin write rejected or incorrect setup token |
| 404 | Record or endpoint not found |
| 409 | Name, path, initialization, or concurrent configuration conflict |
| 415 | Non-JSON request body type |
| 429 | Request rate, password failure, or bcrypt concurrency limit exceeded |
| 500 / 503 | Processing failure or temporarily unavailable database |

## Public endpoints

| Method | Path | Response |
| --- | --- | --- |
| GET | `/api/public/bootstrap` | Site name, timezone, language, entry settings, default panel path, panel names/paths, TLS handshake warning threshold |
| GET | `/api/public/panels/:slug` | Panel, display monitors, groups, and measured statistics |
| GET | `/api/health` | `{ "status": "ok" }` when the database is available |

Public monitors contain only `id`, `name`, `type`, `protocol`, optional `address`, `description`, `tags`, optional `certExpiryWarningDays`, and `summary`. `groups` contains independent panel display groups (`id`, `name`, `monitorIds`), without admin `groupId` or hierarchy. `protocol` is the configured check protocol (HTTP, HTTPS, WS, WSS, gRPC, TCP, PING, or DNS), independent of the latest result. `address` contains host/IP and port only, and is omitted with `hideAddress: true`. Full URL paths, queries, check parameters, and internal errors are not returned.

`summary` contains `status`, `uptime`, `average`, `latest`, and chronologically ascending `heartbeats`. Public panels and admin monitor lists accept `heartbeatCount` (1–1000) to fetch recent records for available display width. Missing or invalid values default to 90 public records or 60 admin records; values above the maximum are capped at 1000. `uptime` and `average` still use the last 24 hours and are `null` without data. Status values are `up`, `down`, `unknown`, and `paused`.

Normal public `summary.heartbeats` contain only `time`, `status`, and `latency`. Recognizable failures additionally include `failure` (`timeout`, `refused`, or `response`); abnormal HTTP responses may include `statusCode`. These are allowlisted mappings from existing diagnostics; unknown causes are omitted, without raw error text. Historical points do not repeat IDs or connection details. `summary.latest` retains display fields for TLS, HTTP, QUIC, and certificates, with internal messages cleared.

Admin summary heartbeats also use compact reads, retaining IDs, monitor IDs, timestamps, status, and latency; latest and failed records retain full extra fields. `summary.latest` and the separate heartbeat history API still return complete records.

Internally, public panels round `heartbeatCount` up to 30, 60, 90, 180, 360, 720, or 1000 to share queries, then trim responses to the requested count. Bootstrap and panels return content-based `ETag` headers and support `If-None-Match` / 304. See [Reverse proxies](deployment_en.md#https-and-reverse-proxies) for public limits and cache headers.

## Administrator account

| Method | Path | Body |
| --- | --- | --- |
| GET | `/api/admin/session` | None; returns initialization and login status |
| POST | `/api/admin/setup` | `{ "password": "...", "token": "setup_token from startup logs" }`; first setup only |
| POST | `/api/admin/session` | `{ "password": "..." }` |
| DELETE | `/api/admin/session` | None; log out of the current session |
| PUT | `/api/admin/password` | `{ "current": "...", "password": "..." }` |

Successful login/setup sets an HttpOnly, SameSite=Strict cookie required by subsequent admin requests. Source addresses use trusted proxy rules. Failures are counted per IPv4 address or IPv6 `/64`; ten failures within 15 minutes temporarily block the source, and success resets the count. Request frequency has a separate limit. Without trusted proxies, the connection address is used. There is no global account lockout.

The setup token appears only in startup logs while the database has no administrator. Restart invalidates previous tokens. After administrator creation, setup returns 409. An incorrect or missing token returns 403 without creating an account or session.

## Monitor management

| Method | Path | Purpose |
| --- | --- | --- |
| GET | `/api/admin/monitors` | Display fields and live statistics, without protocol/alert configuration |
| GET | `/api/admin/monitors/:id` | One monitor's display fields and live statistics |
| GET | `/api/admin/monitors/:id/config` | Full configuration structure on demand, with secrets redacted |
| POST | `/api/admin/monitors` | Create a monitor; returns 201 |
| PUT | `/api/admin/monitors/:id` | Replace monitor configuration |
| DELETE | `/api/admin/monitors/:id` | Delete monitor, heartbeats, logs, and panel references |
| PUT | `/api/admin/monitors/:id/enabled` | `{ "enabled": true }` or `false` |
| GET | `/api/admin/monitors/:id/heartbeats` | Heartbeat history, newest first |
| GET | `/api/admin/monitors/:id/latency` | Time-range latency aggregates and raw-sample statistics |
| DELETE | `/api/admin/monitors/:id/heartbeats` | Clear this monitor's heartbeats; returns deleted count |
| GET | `/api/admin/monitors/:id/logs` | Logs, newest first |
| DELETE | `/api/admin/monitors/:id/logs` | Clear this monitor's logs; returns deleted count |

Configuration reads and create/update responses return empty strings for secrets, with a read-only `configured` array identifying stored fields: `http.auth.password`, `http.auth.clientSecret`, `http.auth.clientKey`, `http.headers`, `http.body`, `websocket.headers`, and `grpc.metadata`.

On update, blank retains a secret and nonempty replaces it. Add field paths to `clearSecrets` to delete them; clearing takes precedence over new values. Changing protocol or authentication type does not inherit incompatible credentials. Required credentials must still pass current authentication validation after clearing. After merging secrets, the backend checks the configuration snapshot inside the save transaction; concurrent changes return 409.

Creation may include `copyFrom: <source-monitor-id>` to inherit secrets for matching protocol/authentication types. Submit all other new configuration as usual. History and panel assignments are not inherited. `copyFrom` cannot update an existing monitor.

Example configuration:

```json
{
  "name": "Main site",
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
  "description": "Website entry point",
  "tags": ["production"],
  "groupId": 0,
  "enabled": true
}
```

Types are `http` (including HTTPS), `websocket` (WS/WSS), `grpc`, `tcp`, `ping`, and `dns`. `groupId: 0` means ungrouped. Intervals are 1–86400 seconds for Ping, 5–86400 for others, measured after the previous complete cycle. Check time and retries extend the actual period. Timeout is 1–120 seconds, retries 0–10, and retry interval 1–3600 seconds.

Optional per-monitor `certExpiryWarningDays` accepts integers 0–3650. Omitted or `null` inherits system settings. Zero disables advance highlighting, while expired certificates remain red. Explicit overrides appear in public monitors, including 0; inherited values are omitted. This controls display only, independently of `alerts.certificate.days`.

Heartbeat/log queries accept `?limit=100&before=<record-id>&beforeTime=<record-time>`. Default page size is 100, valid range 1–200, with out-of-range values reverting to 100. Pass the previous page's last ID and timestamp for combined time/ID pagination. Legacy ID-only requests remain supported by looking up that record's time. Timestamps use Unix milliseconds.

Latency queries use `?range=24h`, supporting `1h`, `3h`, `6h`, `12h`, `24h`, `3d`, and `1w`; omitted defaults to `24h`, other values return 400. Responses contain requested `start`/`end`, retention-limited `availableStart`, raw record `total`, `failures`, `statistics`, and `buckets`. Administrator login is required; this does not change public chart ranges.

`statistics.average`, `p95`, and `maximum` use all valid nonnegative successful latency samples in the period, in milliseconds, or `null` when none exist. P95 uses the nearest-rank method with upward rounding. `buckets` has at most 480 nonempty chronological periods, each with `start`, `end`, `count`, `successes`, `failures`, `average`, `minimum`, and `maximum`. Buckets without valid successful checks have three `null` latency values. Totals include unknown/paused records; `failures` counts only `down`. With no records, return an empty array. Clients should retain the requested time axis and leave gaps between missing periods. Range endpoints are inclusive; boundary samples enter the later bucket, except the final endpoint enters the final bucket.

Heartbeat extra fields include `tlsVersion`, `httpVersion`, `tlsLatency` (ms), `certExpiry` (leaf expiry, Unix ms), `certIssuerOrg` (leaf Issuer O values joined by ` / `), `tlsKeyExchange` (negotiated mechanism), and `tlsCipherSuite` (full negotiated name). HTTP/3 additionally has `quicVersion` (such as `v1`, `v2`) and `zeroRTT` (actual use, explicitly `false` when unused). Other protocols omit these two fields. HTTP/3 handshake time measures QUIC/TLS connection establishment.

Unmeasured fields are omitted. Missing O is not replaced by CN or a root certificate name. Legacy `certIssuer` and `certRootCA` are not displayed as organization names or rewritten; subsequent checks record the organization.

### Per-monitor HTTP configuration

HTTP create/update requests may include `http`. If omitted, defaults are GET, h3 > h2 > h1.1, up to ten redirects, and successful codes 200–399. Explicitly saved settings remain unchanged. Non-HTTP monitors do not store this object.

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

- `version`: `http1` (HTTP/1.1 only), `http2` (HTTP/2 only, h2c for plaintext), `http3` (HTTP/3 only, HTTPS required), `standard` (h2 > h1.1), or default `auto` (h3 > h2 > h1.1). Explicit modes never downgrade. HTTPS HTTP/2-only fails without h2 negotiation and sends no HTTP/1.1 request. With environment proxies, both automatic modes negotiate h2 > h1.1 over TCP; HTTP/3-only does not support that proxy. Connection IP overrides connect directly. Missing `version` uses `auto`; existing explicit choices remain valid.
- `connectIP`: Optional single IPv4/IPv6 string; blank/omitted uses DNS. Trimmed and normalized on save; brackets, ports, interface zones, and multiple addresses are rejected. Overrides dialing for the target URL host only, preserving URL port, default Host, TLS SNI, and certificate hostname verification across HTTP/1.1, HTTP/2, and HTTP/3. Matching hosts bypass environment proxies, including same-host redirects. Other hosts and OAuth2 token requests keep their usual behavior. Not public.
- `ignoreTLS`: Boolean, default `false`. Skips chain, validity, and hostname checks only for this monitor's target over HTTPS/HTTP/3. OAuth2 token verification remains enabled. HTTP configuration is not public.
- `maxRedirects`: 0–100, default 10; 0 disables following redirects. Status code settings still determine success.
- `validStatusCodes`: Array of individual codes or inclusive range strings, 100–599, such as `200` or `200-299`; 1–100 entries.
- `method`: GET, POST, HEAD, PUT, PATCH, DELETE, OPTIONS, TRACE, CONNECT.
- `bodyEncoding`: `json`, `form`, `xml`, or `text`. JSON syntax is checked; forms use `key=value&key2=value2`; XML/text are sent as written. Body limit: 256 KiB.
- `headers`: JSON object text with string values, up to 64 entries / 64 KiB. Host sets the request host separately; Content-Length, Transfer-Encoding, and Trailer are managed internally. Configured headers, Authorization, and Cookie are not forwarded across origins.

`auth.type` supports `none`, `basic`, `oauth2`, and `mtls`:

| Method | Fields |
| --- | --- |
| Basic | `username`, `password` |
| OAuth2 client credentials | `tokenURL`, `clientID`, `clientSecret`, optional space-separated `scope`, `tokenAuthMethod` (`header` by default, or `body`) |
| mTLS | `clientCert` (PEM chain), `clientKey` (unencrypted PEM key), optional `caCert` (additional server trust); each limited to 64 KiB |

OAuth2 uses `client_credentials`; token requests do not follow redirects, and errors do not expose token responses. mTLS retains system trust and server verification and rejects cross-origin redirects. HTTP configuration and credentials are omitted from public APIs.

### Per-monitor WebSocket configuration

With `type: "websocket"`, `target` is a full WS/WSS URL. Optional `websocket` defaults to handshake-only validation. Other types do not store it; WebSocket monitors do not store `http`.

```json
{
  "mode": "handshake",
  "connectIP": "",
  "origin": "https://example.com",
  "subprotocols": ["chat"],
  "headers": "{\"Authorization\":\"Bearer example-token\"}"
}
```

`mode` is `handshake` or `ping`. Handshake validates HTTP/1.1 upgrade and the accept value; ping also requires a Pong matching a random Ping payload. The whole check shares `timeout`, creates a new connection, and does not follow redirects. Ping mode discards business messages; a message over 64 KiB fails. Latency covers the whole check. WSS uses heartbeat TLS/certificate fields and always verifies certificates.

Optional `connectIP` is one IPv4/IPv6 address without a port. Overrides connect directly while preserving Host, TLS SNI, and hostname verification; blank uses DNS/environment proxies. `origin` is blank or an HTTP/HTTPS origin without path, query, or credentials; trailing `/` is removed. `subprotocols` permits up to 16 distinct valid protocol tokens, each at most 128 bytes. Servers may choose one or none, but not an unrequested protocol.

`headers` uses the HTTP header format, limited to 64 entries / 64 KiB. Origin, Connection, Upgrade, and Sec-WebSocket-* are controlled by dedicated fields or the system and cannot be overridden. Targets reject embedded credentials and fragments. Configuration, paths, queries, and headers are private; the public address contains only host and port.

### Per-monitor gRPC configuration

With `type: "grpc"`, `target` is a hostname or IP and `port` is 1–65535. Optional `grpc` is not stored for other types:

```json
{
  "security": "tls",
  "serviceName": "",
  "metadata": "{\"authorization\":\"Bearer example-token\"}",
  "caCert": ""
}
```

`security` is `tls` (default) or `none` (plaintext). TLS always verifies certificates. `caCert` adds PEM trust certificates, up to 64 KiB, and is unused for plaintext. `serviceName` is trimmed, limited to 1000 bytes, and rejects newlines/NUL; blank checks the whole instance.

`metadata` is JSON object text with string names/values, up to 64 entries / 64 KiB. Names are case-insensitive, at most 100 characters, allowing letters, digits, underscores, dots, and hyphens. Values must be printable ASCII. Reserved `grpc-` prefixes, binary `-bin` fields, duplicate names, and transport header overrides are rejected.

Each check calls `grpc.health.v1.Health/Check`; only `SERVING` succeeds, without falling back to TCP reachability. Connection and check share `timeout`, with a 64 KiB response limit. Connections are direct without environment HTTP proxies. TLS uses heartbeat metadata and supports certificate alerts. Public APIs return `protocol: "gRPC"` and host/port, omitting metadata, CA, and service name.

## Groups and panels

`/api/admin/groups` and `/api/admin/panels` support GET/POST; `/:id` supports PUT/DELETE.

Admin group body: `{ "name": "Database", "parentId": 1 }`. Default `parentId: 0` means root. Parents must exist and cannot be self/descendants; names are globally unique. Deletion ungroups direct monitors and moves children to the deleted group's parent. Monitor history and independent panel groups remain unchanged.

Panel body:

```json
{
  "name": "Production",
  "pageTitle": "Production service status",
  "slug": "production",
  "description": "Live service status and availability",
  "groups": [
    { "id": "public-api", "name": "Service APIs", "monitorIds": [2, 1] },
    { "id": "public-network", "name": "Network nodes", "monitorIds": [3] }
  ],
  "isDefault": true
}
```

`groups` order determines display order; `monitorIds` orders monitors within each group. Group IDs are unique within a panel, with 1–64 letters, digits, underscores, or hyphens. Names are nonempty and at most 200 bytes. A panel permits up to 100 display groups and 1000 distinct monitors; no monitor may repeat within a panel, but it may use different groups in other panels. Empty groups can be saved and are hidden publicly.

Admin responses retain a flat `monitorIds` compatibility index, rebuilt in group order when saving `groups`. Legacy clients submitting only `monitorIds` create an independent snapshot from current admin groups. Clients maintaining independent layouts should submit `groups`. Schema v1-to-v2 migration freezes existing names/order; later admin group changes do not alter saved display groups. Removing display groups preserves monitors/history; deleting a monitor removes panel references.

`slug` is unique, at most 64 characters, starts with a lowercase letter/digit, and otherwise permits lowercase letters, digits, and hyphens. The first panel becomes default. Setting one default clears others; deleting it promotes the first remaining panel.

Optional `pageTitle` is at most 200 characters and replaces the upper-left status page name, without adding a heading row. Blank uses the global site name. Public panels return it. Browser titles always use bootstrap's global `siteName`, with no panel-specific browser title.

## Settings and cleanup

GET/PUT `/api/admin/settings`:

GET additionally returns read-only `clientIP`, resolved using saved trusted proxy rules and matching login rate limits. It is neither saved nor public. GET again after changing trusted proxies to obtain the updated address.

```json
{
  "siteName": "UptimeNest",
  "timezone": "Asia/Singapore",
  "language": "en-US",
  "entryPage": "status",
  "statusPageView": "detailed",
  "userAgent": "",
  "tlsLatencyWarningMs": 0,
  "certExpiryWarningDays": 30,
  "trustedProxies": [],
  "retentionDays": 30
}
```

`siteName` is at most 100 characters, blank defaults to `UptimeNest`, and controls branding and browser titles. Bootstrap returns it publicly. `timezone` uses IANA names, defaulting to `Asia/Singapore` for new installations while retaining saved values. `entryPage` is `status` or `admin`. Blank User-Agent uses `UptimeNest`. Retention is 1–3650 days, default 30 for new installations, preserving saved values. `language` supports `en-US` and `zh-CN`.

`tlsLatencyWarningMs` is an integer 0–120000 milliseconds, default 0 (off), returned by public bootstrap. A positive threshold highlights TLS handshake time in amber only when strictly exceeded; equal/lower values or disabled settings remain neutral. It does not affect status, timeout, retries, or statistics. Missing legacy values default to off.

`statusPageView` is `detailed` (default), `compact`, or `cards`, returned by public bootstrap. Missing legacy values default to detailed. Visitor choices live only in page memory, survive data refresh, and reset on reload or panel switching. There is no visitor preference persistence API or browser storage.

`certExpiryWarningDays` is an integer 0–3650, default 30, returned by bootstrap. Certificates at or below the threshold are amber; 0 disables advance highlighting, while expired certificates remain red. It does not notify and is independent of `alerts.certificate.days`. Missing legacy values retain 30 days.

`trustedProxies` contains up to 100 IPs/CIDRs. Forwarded headers are parsed only for trusted connection sources, preferring the `X-Forwarded-For` chain and using valid `X-Real-IP` if absent. Blank uses the connection address. This configuration is not public.

POST `/api/admin/cleanup` has no body, removes expired data using saved retention, and returns `{ "heartbeats": 0, "logs": 0 }`.

## Alerts and notifications

All endpoints below require an administrator session. Alert information and channel credentials are not public.

| Method | Path | Purpose |
| --- | --- | --- |
| GET / POST | `/api/admin/notification-channels` | List/create channels |
| PUT / DELETE | `/api/admin/notification-channels/:id` | Update/delete channel |
| POST | `/api/admin/notification-channels/test` | Synchronous test using current settings/templates; success returns `{ "sent": true }`. No save required; disabled channels allowed; no retry queue. An existing channel `id` retains redacted secrets without updating the channel |
| POST | `/api/admin/notification-channels/:id/test` | Queue a test for a saved enabled channel; returns 202; rejects another test while one is pending for that channel |
| GET | `/api/admin/notifications/templates` | `variables` and `channels`; each channel type has service `subject`/`body`, `certificateTemplate`, and message `format`; legacy defaults remain for compatibility |
| POST | `/api/admin/notifications/preview` | Request `{ "type": "webhook", "subject": "{{alert_type}}", "body": "{\"message\":\"{{monitor_name}}\"}", "alertType": "service_down" }`; returns rendered `subject` and `body` |
| GET | `/api/admin/notifications/records` | Latest 50 records; next page uses `?before=<last-id>` |

Channels have `id`, `name`, `type`, `enabled`, `subject`, `body`, `certificateTemplate`, and `config`. Top-level subject/body handle service failure/recovery. `certificateTemplate` has independent subject/body for approaching expiry, expiry, and recovery. Legacy channels without it inherit their original template. Omitting it when updating a split channel preserves the saved certificate template.

`type` is `email`, `webhook`, `feishu`, `dingtalk`, or `telegram`, and cannot change after creation. Each template allows a 1000-byte subject and 12000-byte body; remote platforms may impose smaller limits.

Draft tests may include `alertType` to choose service/certificate templates: `service_down`, `service_recovered`, `certificate_expiring`, `certificate_expired`, or `certificate_recovered`. Omitted uses `test` and the service template. Tests save nothing and do not enter the retry queue.

Channel-specific `config` fields:

- Email: `host`, `port`, `security` (`tls`, `starttls`, `none`), `username`, `password`, `from`, `to` (email array).
- Webhook: `url`, `headers` (string map); body template must be valid JSON, with variables in string values only.
- Feishu/DingTalk: `url`, optional `secret`.
- Telegram: `token`, `chatId`, `threadId` (0 means unspecified).
- HTTP channels: optional HTTP/HTTPS `proxyURL`.

Channel reads return empty values for `url`, `password`, `secret`, `token`, `headers`, and `proxyURL`, identifying stored fields through a `configured` string array. PUT retains blank secrets; clear optional ones with top-level `clearSecrets`. Ordinary fields replace the full configuration rather than merging partially.

Monitor create/update accepts optional `alerts`; legacy configurations without it have alerts disabled:

```json
{
  "alerts": {
    "service": { "enabled": true, "channelIds": [1], "recovery": true, "repeatMinutes": 0, "days": 0, "failureThreshold": 3 },
    "certificate": { "enabled": true, "channelIds": [1], "recovery": false, "repeatMinutes": 0, "days": 15 }
  }
}
```

Each rule supports up to 50 channels. `repeatMinutes` is 0–525600; 0 disables repeats. Certificate `days` is an integer 1–3650, for HTTPS, WSS, and TLS-enabled gRPC. Service `failureThreshold` is an independent consecutive failure count, 1–1000, defaulting to 1 when omitted or zero. Each final failed cycle counts once, success resets it, and timer reminders do not count. See [Notifications](usage_en.md#notifications) for variables and events.

Delivery records contain an `event` snapshot, `channelId`, `channelName`, `monitorId`, `kind`, `status`, `attempts`, `nextAttempt`, and sanitized `error`. `kind` is `service`, `certificate`, or `test`; status is `pending`, `sending`, `sent`, `failed`, or `cancelled`. Channel credentials and full remote responses are omitted.
