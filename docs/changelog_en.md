# Changelog

[简体中文](changelog.md) · English

[Documentation](README_en.md) · [Downloads](https://github.com/Qwerto107/UptimeNest/releases) · [Release process](releasing_en.md)

## Unreleased

## v0.6.2 · 2026-10-31

- Batch heartbeat strip measurements and updates to avoid repeated layout from interleaved row reads and writes. Retain positions and tooltip labels when dimensions are unchanged, reducing loading stalls on status pages and monitor lists.
- Add `_en.md` English versions of the project README and official documentation, with language switches, English navigation, actual English interface screenshots, and a matching homepage composite.
- Fix hidden headings and spacing in the monitor editor's saved sensitive configuration section. Move it into the scroll area while keeping save actions fixed, and update HTTP/WebSocket screenshots.
- Read the admin sidebar version from frontend package metadata, matching the console and avoiding stale hardcoded version text after releases.
- Refresh interface screenshots and move the gallery into a separate document. The project homepage uses a composite of actual screenshots and links to the gallery.

## v0.6.1 · 2026-10-27

- Replace external Ping commands with native IPv4/IPv6 ICMP: unprivileged sockets on Linux/macOS and system APIs on Windows. Remove external process and SUID BusyBox entry points. Restrict Compose permissions and configure the Ping group range.
- Require a random token from startup logs for initial setup. It rotates on restart before setup and expires after administrator creation; existing account login is unchanged.
- Use separate SQLite file read/write pools, reapplying foreign keys and busy waiting on reconnect while retaining default FULL synchronization.
- Cache settings, skip unchanged alert state writes, and compact recent heartbeat reads while retaining latest connection details, failure classifications, and independent history APIs.
- Redact monitor credentials consistently. List/detail polling omits full protocol configuration; editing fetches it on demand. Blank retains secrets, explicit clearing removes them, and duplication inherits them through the backend.
- Upgrade the schema to version 6, including a covering heartbeat statistics index and configuration encryption. Monitors, notification channels, alert snapshots, and queued event snapshots use AES-256-GCM with an automatically generated separate key file. Restore requires the original key; rollback requires a pre-upgrade snapshot. Heartbeat details and monitor log messages remain unencrypted.
- Share public panel cache construction by heartbeat count bucket, trim to requested counts, and add IP request limits and ETag conditional requests.
- Count login failures by IP, aggregating IPv6 by `/64`. Separate request frequency limits from failures, reset failures on success, evict old entries when full, and limit bcrypt concurrency.
- Cache hashed assets long-term and revalidate HTML. External Nginx handles compression. Deduplicate bootstrap requests, with status pages refetching at most once per minute.

## v0.6.0 · 2026-10-27

- Lower the minimum Ping interval from five seconds to one, consistently across frontend/backend validation. Other protocols retain a five-second minimum.
- Add compact and card status views, keeping detailed rows as default. Buttons are ordered compact/detailed/cards with an animated selection background. Administrators choose defaults; visitor choices are temporary. Compact rows expand on row click; cards open details across columns, close previous details on selection changes, and identify the selected card with borders/arrows.
- Add standard gRPC health checks with TLS/plaintext, service names, custom metadata, and additional CAs. Reuse history charts, service alerts, and TLS certificate alerts; hide private configuration publicly.
- Add email, generic Webhook, Feishu card bot, DingTalk bot, and Telegram Bot channels. Configure service/certificate alerts per monitor, with optional recovery and repeat notifications.
- Store separate service/certificate templates per channel, supporting variable insertion, independent previews, and tests while preserving existing templates. Add a persistent queue, bounded retries, restart deduplication, and delivery records.
- Support independent consecutive failure thresholds for service alerts. Certificate display highlighting supports global settings and per-monitor overrides, blank inheritance, and zero to disable advance highlighting, independently of notification thresholds.
- Upgrade the schema to version 3; rollback requires restoring a pre-upgrade snapshot.

## v0.5.0 · 2026-10-26

- Add status dot breathing, heartbeat entry, value transitions, refresh countdowns, detail/chart animations, and brief emphasis for new failures. Add admin dialog/tab/feedback transitions and static skeletons. Respect reduced motion, pause refresh on hidden pages, and animate loops only for visible status dots.
- Fade the status banner grid downward; show uptime percentages with tooltips. Color protocol/TLS/HTTP versions by type, and highlight approaching/expired certificates in amber/red. Add an optional TLS handshake latency warning threshold, off by default.
- Change the group plus action to add a monitor in that group; use a separate folder-plus for child groups.
- Print project name, version, short build commit, and copyright in the browser console.
- Set the Compose container name to `uptimenest`.
- Add WS/WSS monitors with handshake and Ping/Pong checks, custom headers, Origin, subprotocols, and connection IP overrides. Reuse retries/history; WSS verifies certificates and records TLS details. Hide paths, queries, and private configuration publicly.
- Add `1h`, `3h`, `6h`, `12h`, `24h`, `3d`, and `1w` detail chart ranges, defaulting to `24h`. Query real time ranges and aggregate curves while retaining peaks/failure counts. Statistics use all successful samples in the period, with retention notices.
- Add explicit HTTP/1.1, HTTP/2, and HTTP/3 modes alongside automatic h2 > h1.1 and default h3 > h2 > h1.1. Explicit modes do not downgrade; plaintext HTTP/2 supports h2c. Existing explicit settings are retained.
- Add per-monitor IPv4/IPv6 connection overrides for HTTP/HTTPS while preserving target hostname, Host, TLS SNI, and certificate checks across standard HTTP, HTTP/3, and fallback. Overridden hosts connect directly; other hosts and OAuth2 requests retain usual behavior.
- Add detail-page latency charts reusing public components and recent heartbeats, with statistics, interaction, and light/dark themes.
- Add duplication from monitor lists/details, allowing edits before creating an independent monitor in either language. Copies retain enabled state and HTTP authentication, without history or panel assignments.

## v0.4.0 · 2026-10-25

- Add read-only Current IP under access security, sharing trusted proxy resolution with login limits and updating after settings save.
- Compact public heartbeat history: successful records return only time/status/latency, latest connection details remain in `latest`, and recognizable failures include only sanitized classifications and optional HTTP status codes.
- Show expanded public latency charts using existing recent records, with successful average/P95/maximum statistics, both languages, light/dark themes, mobile layouts, and pointer/touch/keyboard interaction.
- Show red failure bands, abnormal response curves, timeout/recovery markers, and generic unknown failures without exposing raw diagnostics or drawing a bottom legend.
- Place tooltips above the curve to avoid covering points; reduce default band labels on narrow screens to avoid overlap.

## v0.3.0 · 2026-10-25

- Unify selects with themed components supporting both languages, keyboard/touch input, and grouped options.
- Add English UI with language switching under System settings. New installations and missing language settings default to English; retain existing choices.
- Merge admin group management into Monitors, supporting creation, editing, deletion, and child groups. Search/group filters include empty groups; the old page redirects automatically.
- Limit login by client IP and support trusted proxy IPs/CIDRs, accepting forwarded client addresses only from trusted proxies.
- Add admin/public pagination while preserving global search, filters, and statistics. Fetch monitor details individually on demand.
- Batch monitor summaries/recent heartbeats and add a bounded three-second public panel cache to reduce repeated queries. Reuse date formatters and heartbeat labels.
- Paginate history by timestamp plus ID to avoid omissions at equal timestamps. Clean expired history in cancellable batches that can resume.
- Fix a brief incorrect header when clicking the status page title to return to the entry page.
- Use an icon button for public admin navigation, matching the theme control. Animate detail expansion/collapse with reduced-motion support.
- Use default pointers for ordinary text and text cursors for inputs, preserving selection/copying. Standardize setting section numbering.

## v0.2.0 · 2026-10-25

- Treat paused monitors as healthy in panel summaries. When all others are healthy, or all are paused, show all systems operational without an extra paused note.
- Support whole-item panel dragging, live reordering, cross-group moves, and editor edge scrolling, including touch and reduced motion.
- Default User-Agent to `UptimeNest` and new-install history retention to 30 days, retaining existing settings.
- Add per-monitor HTTPS TLS/SSL error ignoring, off by default and also applying to HTTP/3.
- Use timezone selects with common cities and browser-supported IANA zones. Default new installations to Singapore, preserving old timezone aliases.
- Support global site names. Panel titles replace the upper-left status name; browser titles consistently use the global site name.
- Use fixed heartbeat bar widths/gaps in public/admin views, adapting record counts to container width and fetching more history as needed.
- Rebuild admin using the Pulse Status design, unifying light/dark themes, compact navigation, lists, settings, and login. Use tabbed monitor editors with mobile layouts.
- Reorganize documentation into an index, usage, deployment, development, API, binary, and release guides.
- Refresh light/dark status, monitor, group, panel, and settings previews using separate demo data.
- Pull the unified image directly in Compose, using host `./data`, a read-only root filesystem, temporary storage, and explicit health checks. See [Deployment guide](deployment_en.md#upgrades-and-rollback) for named-volume migration.

## v0.1.0 · 2026-10-24

- Provide HTTP/HTTPS, TCP, Ping, and DNS monitors with intervals, timeouts, retries, inversion, and enable/disable controls.
- Support HTTP/3 preference/fallback or HTTP/3-only, redirect limits, accepted codes, methods, bodies, headers, Basic authentication, OAuth2 client credentials, and mTLS.
- Show TLS/HTTP versions, handshake time, certificate lifetime, and issuer organization. Details include key exchange/cipher suites; HTTP/3 additionally shows QUIC version and actual 0-RTT use.
- Support tree admin groups and independent panel display groups, multiple panel paths, default panels, drag/button sorting, and per-monitor address hiding.
- Support heartbeat/log queries, independent per-monitor clearing, retention, and automatic/manual expiry cleanup.
- Provide Simplified Chinese, light/dark themes, display timezones, entry page selection, global User-Agent, and administrator password management.
- Support SQLite/PostgreSQL, embedded-frontend executables for seven platforms, container images for three architectures, and automated releases.
