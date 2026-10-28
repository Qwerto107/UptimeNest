# Usage guide

[简体中文](usage.md) · English

[Documentation](README_en.md) · [Deployment guide](deployment_en.md) · [API reference](api_en.md)

[Getting started](#first-use) · [Monitors](#monitor-management) · [WebSocket](#websocket-checks) · [gRPC](#grpc-checks) · [Latency charts](#latency-charts) · [HTTP](#http-requests-and-authentication) · [Groups](#monitor-groups) · [Panels](#status-panels) · [Settings](#system-settings) · [Account](#account-settings)

## First use

1. Start the service using the [Deployment guide](deployment_en.md). Copy `setup_token` from the startup log, open `/admin`, and enter it to set the administrator password.
2. Create a monitor under Monitors and wait for its first check.
3. Under Status panels, create a panel and display groups, add monitors, and select the default panel.
4. Visitors view `/status/<panel-slug>`; the site root redirects according to the entry page setting.

Administrators log in with a password, without a username. There is no shared initial password. Complete initial setup in a trusted environment.

## Monitor management

The list shows counts for all, up, down, and paused monitors. Search by name, target, tag, or group, combined with group and status filters. Press `Ctrl+K` (`⌘K` on macOS) to focus search.

The editor shows tabs appropriate to the monitor type. HTTP has HTTP options and Authentication; WebSocket has WebSocket settings. All types share basic information, check settings, and alerts. Switching tabs preserves unsaved content; validation focuses the relevant tab and field.

### Duplicate a monitor

Use the duplicate action in the list or detail page to open a prefilled creation dialog. The name receives a copy suffix; long names are shortened to leave room for it.

Copies retain type, target, group, tags, check settings, enabled state, and HTTP request/authentication or WebSocket configuration. Save creates an independent monitor; cancel creates nothing. Heartbeats, logs, and status panel assignments are not copied. Add the new monitor to panels separately.

Saved passwords, OAuth secrets, mTLS private keys, request headers, HTTP bodies, and gRPC metadata are not shown again. Leave a field blank to keep it, enter a value to replace it, or mark it for clearing under saved sensitive configuration; deletion occurs on save. Duplication inherits secrets through the backend when protocol and authentication type match.

### Types and targets

| Type | Target and check |
| --- | --- |
| HTTP/HTTPS | Full URL; success depends on response status codes. HTTPS also records connection and certificate details |
| WebSocket | Full WS/WSS URL; validate the handshake, optionally sending Ping and requiring a matching Pong |
| gRPC | Hostname or IP and port; call the standard health check over TLS or plaintext |
| TCP port | Hostname or IP and port; check whether a connection can be established |
| Ping | Hostname or IP; check reachability using native IPv4/IPv6 ICMP Echo |
| DNS | Domain and record type, with optional server and expected answer |

DNS supports A, AAAA, CNAME, MX, NS, and TXT. Leave the server blank for system DNS, or specify `IP:port`. A blank expected answer checks only successful resolution; otherwise, any exact matching answer is accepted.

### WebSocket checks

Use a target such as `ws://example.com/ws` or `wss://example.com/ws`. Business paths and query parameters are allowed; embedded credentials and fragments are not. Default handshake mode validates HTTP/1.1 Upgrade and the handshake accept value. Handshake + Ping/Pong also sends a random Ping and requires a Pong with matching contents. Business messages and unrelated Pongs do not count as success; a business message larger than 64 KiB fails the heartbeat check.

Each check opens and closes a new connection. Dialing, TLS, upgrade, and optional heartbeat share one attempt timeout; redirects are not followed. Latency covers the whole check. WSS always verifies certificates and separately records TLS handshake time and certificate details. Ping/Pong confirms protocol responsiveness, without validating business message handling.

WebSocket settings include optional Origin, subprotocols, custom headers, and a connection IP override. Origin must be an HTTP/HTTPS origin without a path, query, or credentials; blank omits it. Enter up to 16 comma-separated subprotocols in preference order; the server may also choose none. Headers are a JSON object with string values; Authorization, Cookie, and Host are supported. Upgrade, Connection, and Sec-WebSocket-* are system-managed.

A connection IP override changes only the dial address, preserving the URL, Host, WSS SNI, and certificate hostname verification, and connects directly. Without an override, DNS and environment proxies apply. Public pages show only host and port; paths, query parameters, headers, and protocol settings remain private. Hiding the address also omits host and port.

### gRPC checks

Choose gRPC, enter the hostname or IP, and set the port separately. gRPC settings default to TLS, verifying the chain, hostname, and validity period. Additional PEM CAs for private or self-signed services are merged with system trust. Plaintext mode does not encrypt the connection.

Checks call `grpc.health.v1.Health/Check`. The target must implement and register that service; only `SERVING` is healthy. Other states, an unimplemented endpoint, unknown service name, authentication failure, or timeout are failures. Leave the service name blank to check the whole instance, or specify a service. See [gRPC health checking](https://grpc.io/docs/guides/health-checking/).

Metadata is a JSON object with string values, such as `{"authorization":"Bearer example-token"}`, limited to 64 entries and 64 KiB. Names are case-insensitive; values must be printable ASCII. Binary `-bin` fields and overrides of reserved gRPC or transport headers are not supported. Metadata, CA certificates, and service names are admin-only.

Each check creates a direct connection without environment HTTP proxies. Dialing, TLS, and health checks share one timeout; latency covers the whole check. TLS mode records handshake time and certificates and supports certificate alerts. Service alerts, retries, inversion, and history charts use common settings.

### Common settings

| Setting | Behavior |
| --- | --- |
| Display name | Shown in admin and public panels |
| Heartbeat interval | Seconds to wait after each full cycle: 1–86400 for Ping, 5–86400 for other protocols |
| Retries | Additional attempts after failure, 0–10 |
| Retry interval | Wait 1–3600 seconds between failed attempts |
| Request timeout | Limit of 1–120 seconds per attempt |
| Invert mode | An unreachable target is healthy; a reachable target is down |
| Description and tags | Used for explanation and search; also public when included in a panel |
| Admin group | Tree group containing the monitor |
| Hide address on status pages | Omit the public address while retaining the full target in admin |

Enabling starts a check immediately. Failed attempts retry as configured; only the final failure is recorded after retries are exhausted. Disabling stops future checks and cancels active requests and retry waits. Re-enabling resumes checks.

### Latency charts

Monitor details show connection parameters, heartbeats, and logs. State changes and failures create logs; unchanged successful checks create heartbeats only.

The latency chart below the heartbeat strip supports `1h`, `3h`, `6h`, `12h`, `24h`, `3d`, and `1w`, defaulting to `24h`, with automatic refresh every 60 seconds. Changing range independently queries that period; history table pagination does not affect the chart.

The curve shows averages for up to 480 time buckets, with vertical lines preserving successful minimum and maximum latency. Buckets containing failures are red; missing data breaks the line. Average, P95, and maximum statistics use all valid successful samples in the selected period, while failure counts include all failed checks. Pointer, touch, and keyboard interactions show each bucket's time, checks, failures, and latency. Queries respect retention and show a notice when the requested range exceeds retained history.

Chart latency measures the whole check, while connection details report TLS handshake time separately. A bucket with one valid sample naturally has equal average, minimum, and maximum values.

The following screenshot shows the English interface.

![UptimeNest monitor details and time-range latency chart](assets/monitor-detail_en.jpg)

### History and deletion

| Action | Effect |
| --- | --- |
| Clear heartbeats | Delete only this monitor's heartbeats and recalculate statistics |
| Clear logs | Delete only this monitor's logs |
| Delete monitor | Delete configuration and history, removing references from every panel |
| Pause monitor | Keep configuration and history while stopping checks |

## HTTP requests and authentication

### Protocols, redirects, and status codes

| Setting | Behavior |
| --- | --- |
| HTTP/1.1 | HTTP/1.1 only; no HTTP/2 or HTTP/3 negotiation |
| HTTP/2 (h2) | HTTP/2 only; HTTPS uses ALPN, plaintext requires h2c; no HTTP/1.1 downgrade |
| HTTP/3 (h3) | HTTP/3 only; requires HTTPS and UDP connectivity; no downgrade |
| Automatic h2 > h1.1 | Prefer HTTP/2 for HTTPS, otherwise HTTP/1.1; plaintext uses HTTP/1.1 |
| Automatic h3 > h2 > h1.1 | Default; try QUIC for HTTPS, falling back to HTTP/2 or HTTP/1.1 on connection establishment failure; plaintext uses HTTP/1.1 |
| Maximum redirects | 0–100, default 10; 0 disables following redirects |
| Valid status codes | Individual codes or ranges such as `200-299`, `401`; default 200–399 |
| Connection IP | Optional single IPv4/IPv6 address without brackets, port, or interface zone; blank uses DNS |

With redirects disabled, the current response is still evaluated against valid status codes; an accepted 3xx can succeed. HTTP/3 preferred mode does not resend an already-issued request through a fallback protocol.

Existing explicit protocol choices are retained. New or unspecified configurations default to h3 > h2 > h1.1. With environment proxies, both automatic modes negotiate h2 > h1.1 over TCP; explicit HTTP/3 does not support that proxy. A connection IP override connects directly.

Keep a domain URL as the target when overriding its connection IP. For `https://api.example.com/health` with `192.168.0.88`, checks dial that IP while using `api.example.com` for default Host, TLS SNI, and certificate hostname verification. The port comes from the URL; HTTP/1.1, HTTP/2, and HTTP/3 are supported.

The override applies only to the original host, including same-host redirects. Other redirect hosts use normal resolution. The overridden host bypasses environment proxies; other hosts and OAuth2 requests retain their usual behavior. This setting is local to the monitor, does not change system DNS or hosts files, and is not public.

### Request contents

Ignoring HTTPS TLS/SSL errors is off by default. Enabling it skips chain, validity, and hostname verification only for this monitor's target, including HTTP/3. OAuth2 token requests still verify certificates.

Supported methods are GET, POST, HEAD, PUT, PATCH, DELETE, OPTIONS, TRACE, and CONNECT. Body encodings are JSON, URL-encoded form, XML, and text. JSON syntax is validated; forms use `key=value&key2=value2`; XML and text are sent as written.

Headers use a JSON object with string values:

```json
{
  "Accept": "application/json",
  "X-Monitor": "UptimeNest"
}
```

Custom Host is supported; Content-Length, Transfer-Encoding, and Trailer are system-managed. A per-monitor User-Agent overrides the global value. Cross-origin redirects omit custom headers and authentication credentials.

### Authentication methods

| Method | Configuration |
| --- | --- |
| None | No additional authentication |
| HTTP Basic | Username and password |
| OAuth2 | Token URL, Client ID, Client Secret, optional Scope; client credentials flow |
| mTLS | PEM client certificate and unencrypted private key, optional additional server CA |

OAuth2 fetches a token for every check within the check timeout. mTLS retains system trust and server certificate verification and rejects cross-origin redirects. Credentials are stored in the configuration database; public APIs and heartbeat records do not contain these request settings.

## Connection details and statistics

HTTPS displays TLS/HTTP versions, handshake time, certificate remaining lifetime, and issuer organization (O). The organization comes from the leaf certificate Issuer's O field; if absent, it is omitted rather than replaced by CN or the root CA name.

Expanded details show the negotiated key exchange and full cipher suite name. Actual HTTP/3 connections additionally show QUIC version and 0-RTT usage. The application does not actively enable 0-RTT and has no toggle for it.

| Display | Calculation |
| --- | --- |
| Public/admin heartbeat strips | Fixed bar widths and gaps; recent records fit available width, oldest to newest, with gray padding on the left when short |
| Monitor uptime | Share of final heartbeats that are healthy in the last 24 hours |
| Average response time | Mean successful latency in the last 24 hours |
| Panel uptime | Mean uptime of monitors with data |
| Panel average response time | Mean of successful per-monitor averages, excluding inverted monitors |

Heartbeat strips do not represent a fixed number of days. Missing data is shown as missing, not 100% uptime. HTTP latency includes a small response body read; Ping includes DNS, native ICMP calls, and reply waiting, without external process startup overhead.

## Notifications

Alerts default to off, with no predefined channels. Add a channel under Notification channels, then enable service or certificate alerts in each monitor's Alerts tab and select channels. Channels may serve multiple monitors, and each alert may use multiple channels.

### Channels and testing

| Channel | Configuration |
| --- | --- |
| Email | SMTP host, port, TLS/STARTTLS, account, sender, up to 50 recipients; trusted internal relays may use plaintext |
| Generic Webhook | HTTP/HTTPS URL, custom JSON headers, JSON body template; POST without redirects |
| Feishu card bot | Custom bot webhook, optional signing secret; title and body form a message card |
| DingTalk bot | Custom bot webhook, optional signing secret; Markdown message; include configured validation keywords in the template |
| Telegram Bot | Bot Token, Chat ID, optional topic ID; plaintext messages with HTTP/HTTPS proxy support |

HTTP channels may use a dedicated proxy; blank uses server environment proxies. SMTP does not use this HTTP proxy. Platform limits on message size, permissions, and request rates still apply.

All five channel types support testing. Test send in the create/edit dialog uses the current connection settings and templates without saving, even for disabled channels. Results appear in the dialog; no automatic retries or configuration updates occur. Blank secret fields retain saved values when editing.

Testing a saved channel from the list queues a notification; see delivery records for results. Disabled channels cannot be tested from the list. Remove monitor references before deleting a channel. Passwords, tokens, bot URLs, custom headers, and proxy URLs are redacted on read; optional secrets can be explicitly cleared. Protect database backups as sensitive configuration.

### Triggering and recovery

Service alerts have an independent consecutive failure threshold, 1–1000, default 1. Each cycle still uses its check retry settings, and a final failed cycle increments the count once. Notification starts at the threshold; success resets the count. Counts persist across restarts, and timer-based reminders do not increment them. Recovery notifications are optional; `downtime` starts at the first failed cycle in the sequence.

Certificate alerts support HTTPS, WSS, and TLS-enabled gRPC. Set a lead time of 1–3650 days: notify once on entering the threshold and again on expiry. A new certificate is evaluated using its new expiry time; recovery notifications are optional. Failed checks that cannot read a new certificate reuse the previously recorded expiry. Without certificate data, expiry cannot be evaluated.

Both alert types support repeat intervals in minutes; 0 disables repeats. Certificate expiry and reminders are checked at least once per minute. Changing the target or alert rules resets the corresponding state for reevaluation after the next real check. Disabling a monitor or alert cancels pending tasks, although already-started network requests may finish.

Failed deliveries retry up to three times after 30 seconds, 2 minutes, and 10 minutes. Persistent queue and alert state prevent repeated initial alerts during normal restarts. Delivery timeouts or exit after remote acceptance can cause duplicates. Repeated reminders are coalesced during ongoing delivery failures to avoid buildup. Records do not indicate that users read messages; HTTP success means the receiver accepted the request. Completed records follow system history retention.

### Templates and variables

Each channel has default templates, suitable fields, and previews: email subject/plaintext body; Webhook JSON body; Feishu plaintext card title/Markdown body; DingTalk title/Markdown body; Telegram first line/plaintext body. Webhook does not send a notification title.

Channels save separate service and certificate templates. Service templates cover failure and recovery; certificate templates cover approaching expiry, expiry, and recovery, distinguished by `alert_type`. Existing custom templates initialize both sets and can then be edited independently.

Select the template type in Notification templates. Switching categories or new channel types preserves drafts. Reset affects only the current channel's current template. Variable buttons insert into the most recently focused field. Select an event to preview or test with sample monitor data, without saving drafts. Saving validates both templates and focuses the category containing an error.

Templates substitute variables only, without expressions, conditions, or loops. Webhook variables must be JSON string values; quotes and newlines are escaped automatically.

Collapsible syntax help below the editor explains the current channel format. Hover or focus variable buttons for meanings. Editing warns about unknown variables, malformed braces, invalid JSON, and variables in JSON keys; fix these before previewing, testing, or saving.

Feishu bodies use [Feishu Markdown (lark_md)](https://open.feishu.cn/document/common-capabilities/message-card/message-cards-content/using-markdown-tags): `**text**` for bold, `*text*` for italics, `[text](https://example.com)` for links, and literal newlines. Existing custom templates are retained and Markdown is parsed as rich text. Feishu/DingTalk previews show source after substitution; use Test send to inspect final receiver styling.

| Variable | Meaning |
| --- | --- |
| `{{alert_type}}` | `service_down`, `service_recovered`, `certificate_expiring`, `certificate_expired`, `certificate_recovered`, or `test` |
| `{{site_name}}` | System site name |
| `{{monitor_id}}`, `{{monitor_name}}` | Monitor ID and name |
| `{{monitor_target}}`, `{{monitor_type}}` | Full target and monitor type; recipients must be authorized to see the target |
| `{{status}}` | Check status; a service may still be `up` when its certificate is expiring |
| `{{error_message}}` | Latest check error, empty on success |
| `{{event_time}}` | Event time in the configured timezone |
| `{{downtime}}` | Duration of the current alert, such as `5m0s` |
| `{{cert_expiry}}`, `{{cert_days_remaining}}` | Certificate expiry and remaining days, or `-` when not applicable |
| `{{latency_ms}}` | Latest check latency in milliseconds |

Body example:

```text
{{alert_type}}: {{monitor_name}}
Target: {{monitor_target}}
Time: {{event_time}}
Error: {{error_message}}
Duration: {{downtime}}
```

## Monitor groups

Admin groups organize monitors. Alert channels are configured per monitor and are not inherited from groups.

Groups form a parent/child tree. Parent status summarizes the entire subtree. Expand or collapse groups, create children, change parents, and find monitors by group or name.

All group operations are under Monitors. The top Create group action creates a root group. The plus beside a group adds a monitor with that group preselected; the folder-plus creates a child group. Other actions edit name/parent or delete the group.

Empty groups remain visible and searchable by name or group filter. With a specific monitor status filter, only groups containing matching monitors appear. `/admin/groups` redirects to `/admin/monitors`.

- A monitor belongs to one admin group or remains ungrouped.
- Group names are globally unique; a parent cannot be the group itself or a descendant.
- Deleting a group moves its direct monitors to ungrouped and its children up one level, preserving monitors and history.
- Admin group changes do not alter independent display groups in status panels.

## Status panels

View buttons are ordered compact rows, detailed rows, and cards, with a sliding selection background. Administrators choose the default under System settings; initially it is detailed rows. Visitors can switch temporarily without browser storage. Data refresh preserves the choice; page reload or panel switching restores the administrator's default.

Click a compact row to expand/collapse details; interacting with expanded content does not collapse it. Cards use one to three columns, opening full-width details below their row. A border and arrow identify the selected card; selecting another closes the previous details. All views share groups, pagination, heartbeats, latency charts, and connection details. Reduced motion disables transitions.

Panels have independent names, descriptions, and unique URL slugs, with one default panel. When the entry page is set to status, the site root redirects to the default panel.

Panel summaries treat paused monitors as healthy. If the rest are up, or all monitors are paused, the summary says all systems are operational without a separate paused note. Down and pending states reflect actual results.

The banner grid fades downward. Uptime shows a percentage with a tooltip explaining the monitor's last 24 hours; heartbeat bars remain ordered by check count. Protocol tags are blue; certificate lifetime and issuer are green. TLS 1.2/1.3 are green, other TLS versions yellow. HTTP/1.x is gray, HTTP/2 cyan, HTTP/3 purple; ordinary latency is neutral. Certificate tags turn amber at or below the warning days; 0 disables advance highlighting, while expired certificates remain red. The latency tag shows a value with a TLS handshake tooltip, turning amber only when an enabled threshold is exceeded.

Certificate highlighting inherits the system setting, initially 30 days. HTTPS, WSS, and TLS-enabled gRPC can override it with 0–3650 days under check settings; blank restores inheritance. All views use the same display rule, independent of certificate notification thresholds.

Healthy status dots breathe slowly, failures pulse faster, and animation pauses offscreen or when the page is hidden. Heartbeat bars enter left to right on first visibility, without replay on data refresh; hover or keyboard focus shows check information. Changed values transition briefly; a new failure animates its red line downward and briefly highlights its background.

Expanded details show recent latency and connection parameters. Status pages refresh every 30 seconds, synchronized with the refresh icon and countdown. Hidden pages pause the timer and refresh once when visible again if overdue.

A static layout-specific skeleton appears only after requests take 150 ms, then is replaced when data arrives. Save/delete feedback appears at the lower right. Switching monitor editor tabs preserves unsaved content.

Motion follows the operating system's reduced-motion setting; there is no separate application switch. It disables animation while retaining colors, tooltips, and data refresh. On Windows 11, turn off Animation effects under Settings → Accessibility → Visual effects.

Clicking a monitor name expands its latency chart and connection details. Public charts use currently returned recent heartbeats with actual timestamps, without implying a fixed 24-hour window or offering additional ranges. Average, P95, and maximum use valid successful samples from those records only.

Failures without confirmed response latency appear as red bands with curve gaps. Known abnormal responses retain latency as red lines/dots; timeouts use triangles and recoveries hollow markers. Band boundaries use neighboring timestamp midpoints to indicate sampling coverage. Labels use actual first/last failure timestamps, not exact downtime. Use pointer, touch, or focus plus arrow keys, Home, and End to inspect records. Tooltips sit above the curve without covering points; no legend is drawn.

Panel title replaces the upper-left name, up to 200 characters. Blank uses the global site name without adding a heading row. Browser tab titles always use the global site name; there is no panel-specific browser title. Footer copyright remains fixed.

Create display groups in the panel editor and add monitors to each. Drag or use up/down buttons to reorder groups and monitors; monitors can move between display groups. A monitor appears only once per panel but may appear in multiple panels.

Drag the six-dot handle to move the whole monitor row or group while neighbors make room. Monitors can enter empty groups. Approaching the editor's top/bottom edges scrolls automatically; the bottom trigger is above the fixed action bar. Dropping updates the draft; Save persists it. Touch dragging is supported. Reduced motion removes placement animations while keeping drag feedback and button sorting.

| Action | Behavior |
| --- | --- |
| Remove display group or monitor | Remove only from the current panel; retain monitor and history |
| Keep an empty display group | Save in admin; omit on public pages |
| Change admin monitor groups | Do not change panel names, groups, or order |
| Delete monitor | Remove references from all panels |
| Delete default panel | First remaining panel becomes the default |

All panels are public. Names, descriptions, and tags are public; host/IP and port are shown by default and may be hidden per monitor. Full URL paths, queries, DNS servers, headers, and credentials are omitted from public APIs.

## System settings

| Setting | Behavior |
| --- | --- |
| Site name | Default `UptimeNest`; admin/public branding and all browser titles; up to 100 characters |
| Display timezone | Common cities or another IANA timezone; new installations default to `Asia/Singapore`; saved values are retained |
| Entry page | Default status panel or admin interface |
| Default status view | Compact, detailed, or cards; detailed by default; visitor choices reset on page reload |
| Interface language | English or Simplified Chinese, English by default; saving applies to admin and public pages; existing choices are retained |
| Theme | Light by default; manually switch light/dark; stored in the current browser |
| User-Agent | HTTP/HTTPS; blank uses `UptimeNest` |
| TLS latency warning threshold | Integer 0–120000 ms, default 0 (off); controls status page tag color only, not monitor status |
| Certificate warning days | 0–3650, default 30, with per-monitor overrides; 0 disables advance highlighting, expired remains red; independent of notification days |
| Current IP | Read-only client IP resolved using saved trusted proxy rules, matching login rate limits |
| Trusted proxy IPs/CIDRs | One per line, at most 100; accept forwarded headers only from trusted proxies; blank uses the connection source |
| History retention | 1–3650 days, default 30 for new installations; saved values are retained |
| Clean old data | Delete expired heartbeats and logs using the saved retention period |

Expired history is cleaned at startup and hourly, or manually. Save retention changes before manual cleanup. Cleanup preserves monitor configuration and recalculates statistics from remaining data.

## Account settings

Enter the current and new passwords under Account settings. Passwords must be nonempty and at most 72 UTF-8 bytes, without mandatory case, digit, or special-character combinations.

Sessions last seven days. Changing the password revokes other sessions. HTTPS cookies are controlled by deployment environment variables; see [Startup parameters](deployment_en.md#startup-parameters).
