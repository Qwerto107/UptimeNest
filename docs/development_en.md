# Development guide

[简体中文](development.md) · English

[Documentation](README_en.md) · [API reference](api_en.md) · [Release process](releasing_en.md)

[Environment](#environment-and-structure) · [Local development](#separate-development-servers) · [Builds](#complete-executables-and-images) · [Validation](#validation-as-needed) · [Data and concurrency](#database-and-concurrency-conventions) · [Checks](#monitor-checks-and-extensions)

## Environment and structure

Use Go 1.27.3 or newer (see `go.mod` for the toolchain), Node.js 24, and npm. Database drivers and native ICMP support release builds without CGO or a system `ping`. Dependencies are locked by `go.sum` and `web/package-lock.json`.

```text
cmd/uptimenest/       Startup and healthcheck commands
internal/api/         Authentication, admin/public APIs, static pages
internal/config/      Environment configuration
internal/model/       Monitor, heartbeat, group, panel, settings models
internal/store/       SQLite/PostgreSQL, migrations, history, statistics
internal/monitor/     Checks, scheduling, history cleanup, state events
internal/webui/       Frontend embedding controlled by build tags
web/src/              Vue, TypeScript, routing, Chinese locale resources
scripts/              Frontend embedding preparation
.github/              Checks, packaging, releases
docker-compose.yml    Unified image runtime configuration
docs/                 Usage and maintenance documentation
```

## Separate development servers

Start the backend from the repository root in one PowerShell terminal:

```powershell
go run ./cmd/uptimenest
```

Start the frontend from the repository root in another terminal:

```powershell
cd web
npm ci
npm run dev
```

Open <http://127.0.0.1:5173>. Vite proxies `/api` to `127.0.0.1:8090`. Go does not load `.env` automatically; the default database is `data/uptimenest.db` in the repository. For initial setup, copy `setup_token` from the Go startup log and use it on the page to set the password.

Without `with_frontend` or `UPTIMENEST_WEB_DIR`, the backend falls back to `web/dist`. If missing, APIs remain available and page requests ask you to build the frontend. Vite serves development pages without a prior build.

## Complete executables and images

### Build a complete executable

Build the frontend and prepare embedded assets from the repository root:

```sh
npm --prefix web ci
npm --prefix web run build
node scripts/prepare-webui.mjs
```

From the repository root:

```sh
CGO_ENABLED=0 go build -tags=with_frontend -trimpath -ldflags='-s -w' -o bin/uptimenest ./cmd/uptimenest
```

In PowerShell:

```powershell
$env:CGO_ENABLED = '0'
go build -tags=with_frontend -trimpath -ldflags='-s -w' -o bin/uptimenest.exe ./cmd/uptimenest
```

`with_frontend` embeds the pages. The preparation script replaces only the fixed generated directory `internal/webui/dist` and rejects hidden configuration, source maps, dependency directories, and symbolic links. The resulting executable uses the same startup configuration as [release executables](binary_en.md).

### Build a local image

From the repository root:

```sh
docker build -t uptimenest:local .
```

Prepare a writable data directory following the [Deployment guide](deployment_en.md). Change the working directory's Compose `image` to `uptimenest:local`, then run `docker compose up -d --pull never`.

The Dockerfile's default `standalone` target builds from source; CI's `release` target reuses packaged executables. Images retain Alpine without an additional SUID BusyBox copy. Compose sets the group range needed for unprivileged ICMP. See [Release process](releasing_en.md#container-image) for supported platforms.

### Version and build identity

The browser console prints `UptimeNest v<version>_<short-commit>` and copyright once per page load, without repeating on route changes or refreshes. The version comes from `web/package.json`. The commit comes from `BUILD_COMMIT`, then `GITHUB_SHA`, then local Git. If unavailable, it displays `unknown` without failing the build.

Docker source builds exclude `.git`; pass the commit explicitly:

```sh
docker build --build-arg BUILD_COMMIT="$(git rev-parse HEAD)" -t uptimenest:local .
```

CI builds the frontend once and reuses the embedded assets across platform executables and images, keeping console commit identifiers consistent within a release.

## Validation as needed

Common commands from the repository root:

```sh
go vet ./...
go test ./...
```

Run `npm run build` in the frontend directory for TypeScript checking and a production build. Choose checks based on the change. Documentation changes need only link, command, and screenshot review. The project adds no extra test framework or database test service. See [Release triggers](releasing_en.md#triggers) for standard CI checks and optional `full_checks`.

## Database and concurrency conventions

### Connections, transactions, and migrations

| Setting | SQLite | PostgreSQL |
| --- | --- | --- |
| Driver | modernc.org/sqlite | pgx |
| Maximum connections | One writer, four readers; in-memory databases share one connection | 12 |
| Placeholders | `?` | Converted by Store.Q to `$1`, etc. |
| Runtime | WAL, local file | Separate database server |

Use explicit transactions for atomic operations across statements, using the same tx throughout. SQL statements are fixed internally; pass user values as parameters. Schema versions are independent of application versions; add a new migration for each schema change.

Use `Store.ReadDB()` for reads outside transactions and `Store.DB` for writes and transactions. `Store.Close()` closes both pools on shutdown. SQLite configures foreign keys and busy waiting for each connection through the DSN, with `query_only` readers and immediate write transactions, retaining FULL synchronization. Schema version 4 adds a covering index for heartbeat statistics.

Settings are cached in process, loaded on first read, and refreshed after `SaveSettings` commits. Runtime changes must use the save API; direct database edits require a restart. The cache assumes one application instance. Reads and saves copy trusted proxy lists to avoid shared mutable slices.

Schema version 5 encrypts `monitors.data`, `notification_channels.data`, and `alert_states.data` using AES-256-GCM; version 6 adds `notification_deliveries.event`. Additional authenticated data binds the table and primary key, plus alert kind for states. Enqueue notification events through `queueDelivery`: it obtains the row ID before writing ciphertext bound to that ID. The standard library generates random nonces. All reads and writes must use Store encryption methods rather than decoding those columns directly as JSON. The main program persists the key through `UPTIMENEST_SECRET_KEY_FILE`; database restoration requires the original file.

Schema version 2 adds parent admin groups and independent panel display groups. Migration freezes existing panel group names and order, so later admin group changes do not affect panels. Group changes must prevent cycles; panel saves and monitor deletion must preserve reference consistency.

### Time and history

Heartbeat and log timestamps use Unix milliseconds. Summaries use the last 24 hours; admin latency queries use the requested range and return at most 480 nonempty buckets. Curves use bucket averages with minimum and maximum values; average, P95, and maximum statistics use all valid successful records. See [Monitor management](api_en.md#monitor-management) for API and retention rules.

Public and admin heartbeat strips use fixed bar widths and gaps, requesting recent records through `heartbeatCount` according to container width, up to 1000. Defaults are 90 public records and 60 admin records. Record count does not imply a fixed time range.

Startup and hourly cleanup remove expired heartbeats and logs. Per-monitor clearing removes only the selected monitor's corresponding records; statistics are recalculated from remaining data.

### Accounts and public data

Administrator passwords use bcrypt; the database stores only session token digests. Write requests validate same-origin Origin; admin APIs use HttpOnly, SameSite=Strict cookies. Login limits and the admin Current IP use the same trusted proxy resolution, accepting forwarded headers only from trusted connections. See [Reverse proxies](deployment_en.md#https-and-reverse-proxies).

Public APIs use field allowlists and omit addresses when hiding is enabled. Admin list/detail polling reads display data only. Editing uses a separate configuration API that redacts secrets; updates use `configured`, `clearSecrets`, and backend inheritance rules. All status panels are public, including descriptions and tags.

Public caches share response construction by heartbeat count bucket, trimming records for each response. ETags must use actual response content, not only `PublicVersion`, because successful heartbeats also update latency and timestamps. The frontend keeps a bounded conditional request cache for public data only. Bootstrap requests are deduplicated; status pages refetch bootstrap at most once per minute.

## Monitor checks and extensions

Each monitor has its own schedule, with a global concurrency limit of 32. The normal interval starts after the complete check cycle finishes. Retries do not record an early failure; only the final heartbeat is written after retries are exhausted. Disabling or editing a monitor cancels the old task, preventing results from the previous configuration from being written back.

Ping resolves DNS under the shared timeout and sends one Echo to the first resolved address. Linux/macOS use `golang.org/x/net/icmp` endpoints `udp4` / `udp6`, verifying source, sequence, and random payload. Linux also verifies the kernel-assigned Echo ID. Cancellation closes the socket to wake waiting reads.

Windows loads `iphlpapi.dll` from the system directory and calls `IcmpSendEcho2` / `Icmp6SendEcho2`, checking status, source, and random payload without external processes or raw sockets. Cancellation returns promptly, while the underlying synchronous call retains pinned buffers until completion or its original timeout. Outstanding native calls are separately limited to 32.

HTTP/3 preferred mode falls back only on QUIC connection establishment failure. Cross-origin redirects remove custom headers and credentials; mTLS rejects cross-origin redirects. TLS and QUIC metadata come from the real connection rather than configuration. Per-monitor `ignoreTLS` defaults to off and skips verification only for the check target, including HTTP/3; OAuth2 token requests still verify independently.

Explicit HTTP protocol modes do not downgrade. A connection IP override changes only dialing for the original target host, preserving Host, SNI, and certificate hostname verification. WebSocket uses a new connection per check, validating the handshake or matching Ping/Pong. WSS always verifies certificates; configuration and paths are omitted from public APIs.

Alert states and notification events commit in the final heartbeat transaction. `internal/notify` consumes the database queue in separate background tasks without occupying monitor workers. Service alerts use a persistent consecutive failure count: one increment per final failed cycle, reset on success. Minute-based certificate expiry and repeat checks do not increment failures. `StateHandler` remains an optional in-process subscription mechanism, not reliable delivery.

Schema version 3 adds channels, alert states, and the delivery queue. Sending uses a one-minute lease and preserves event order for each monitor/channel pair, with up to three retries after the initial failure. Unfinished tasks resume after a process restart. Network timeouts or process exit after successful delivery can still cause duplicates; exactly-once delivery is not guaranteed. Completed delivery records follow history retention.

The frontend uses Vue 3, Vue Router, vue-i18n, Tailwind CSS, and local shadcn-vue components based on Reka UI. Chinese text is centralized in `web/src/locales/zh-CN.ts`. Public and admin styles are maintained separately. Font copyright and OFL licensing are in `web/public/fonts/OFL.txt`.

## Collaboration conventions

Use Chinese for authored code comments on important logic, keeping code concise and readable. Update usage or API documentation alongside behavior changes, and record release changes in the [Changelog](changelog_en.md).

Do not put credentials, private keys, local databases, temporary tools, collaboration rules, or design-process files into logs, screenshots, or the repository. The initial setup token is an explicit logging exception: it appears only before an administrator exists, rotates on restart, and is never returned by the API. Obtain confirmation for commits, pushes, and releases under the project's collaboration rules.
