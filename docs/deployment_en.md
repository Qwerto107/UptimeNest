# Deployment guide

[简体中文](deployment.md) · English

[Documentation](README_en.md) · [Binary deployment](binary_en.md) · [Usage guide](usage_en.md)

Docker Compose is the recommended deployment method. One container serves the status pages, admin interface, and API. To run a release executable directly, see [Binary deployment](binary_en.md). Run one application instance per deployment.

| Stage | Topics |
| --- | --- |
| Installation and configuration | [Single container](#single-container-deployment) · [Data directories](#data-and-directories) · [Startup parameters](#startup-parameters) · [Admin settings](#admin-settings) |
| Data and access | [Databases](#database-configuration) · [HTTPS](#https-and-reverse-proxies) · [Monitor networking](#monitor-networking-and-permissions) |
| Maintenance | [Backups](#backup-and-restore) · [Upgrades](#upgrades-and-rollback) · [Troubleshooting](#troubleshooting) |

## Single container deployment

### Prepare the working directory

On Linux, create a dedicated directory and grant write access to the container user, UID/GID `65532:65532`:

```sh
mkdir -p uptimenest/data
sudo chown 65532:65532 uptimenest/data
sudo chmod 700 uptimenest/data
cd uptimenest
curl -fL https://raw.githubusercontent.com/Qwerto107/UptimeNest/main/docker-compose.yml -o docker-compose.yml
```

These permission commands are for a new directory. For an existing directory, preserve its ownership or grant access through ACLs. On Windows Docker Desktop, use host permissions and prepare the files in PowerShell:

```powershell
New-Item -ItemType Directory -Force uptimenest/data
Set-Location uptimenest
Invoke-WebRequest https://raw.githubusercontent.com/Qwerto107/UptimeNest/main/docker-compose.yml -OutFile docker-compose.yml
```

### Start and set the password

```sh
docker compose up -d --pull always
docker compose ps
docker compose logs uptimenest
```

Copy `setup_token` from the startup log, open <http://127.0.0.1:8090/admin>, and enter the token to set the administrator password. Passwords must be nonempty and at most 72 UTF-8 bytes; no minimum length or complexity rules are enforced. There is no preset username or shared initial password, and no initial password is read from environment variables. Complete setup before exposing the application through a reverse proxy.

The token is printed only when the current database has no administrator. It is neither returned by the API nor stored in the database. Each restart before setup generates a new token; creating the administrator invalidates it. Existing accounts can log in after upgrading. Restrict access to startup logs because the token allows administrator creation.

Default SQLite deployments do not require an `.env` file. To change database or HTTPS cookie settings, create one in the working directory using [.env.example](../.env.example).

`docker-compose.yml` uses `ghcr.io/qwerto107/uptimenest:latest`. To pin a version, change `image` to `ghcr.io/qwerto107/uptimenest:0.6.1`. To change the host port, edit the first `8090` in `127.0.0.1:8090:8090`. The default port is accessible only locally; configure a reverse proxy for public access.

Compose names the container `uptimenest`, so `docker logs uptimenest` also works. Separate deployments on the same machine need different container names, host ports, and data directories.

## Data and directories

Compose bind-mounts `./data:/app/data`:

| Container path | Contents |
| --- | --- |
| `/app/data/uptimenest.db` | SQLite database: accounts, monitors, groups, panels, settings, and history |
| `/app/data/uptimenest.db-wal`, `-shm` | SQLite runtime WAL and shared memory files |
| `/app/data/secret.key` | Automatically generated configuration encryption key; persist it with either SQLite or PostgreSQL |
| `/tmp` | Temporary directory; Compose uses tmpfs with a 1 GiB limit |

The container runs as a non-root user with a read-only root filesystem; `/app/data` and `/tmp` are writable. tmpfs consumes memory as data is written; its limit is not a reservation. Keep SQLite on local disk and do not share it among multiple monitoring instances.

Monitor configuration, notification channels, and alert states are encrypted using AES-256-GCM with a separate key file. Anyone obtaining both the database and key can decrypt them, so restrict directory access. Frontend assets are embedded in the executable and need no separate mount.

## Startup parameters

Compose reads `.env` in the working directory and passes variables declared in the Compose file into the container. For direct executable or Go runs, set variables through the shell or service manager; the application does not load `.env` itself.

| Environment variable | Default for direct runs | Purpose |
| --- | --- | --- |
| `UPTIMENEST_ADDRESS` | `127.0.0.1:8090` | Listen address; Compose sets `0.0.0.0:8090` |
| `UPTIMENEST_DB_TYPE` | `sqlite` | `sqlite` or `postgres` |
| `UPTIMENEST_DB_DSN` | `data/uptimenest.db` | SQLite path or PostgreSQL connection string |
| `UPTIMENEST_WEB_DIR` | Empty | Use embedded pages; source runs without embedded assets fall back to `web/dist`; may specify an external directory |
| `UPTIMENEST_COOKIE_SECURE` | `false` | Set to `true` for HTTPS access so login cookies are sent only over HTTPS |
| `UPTIMENEST_SECRET_KEY_FILE` | `data/secret.key` | Configuration encryption key file, generated on first use; Compose defaults to `/app/data/secret.key` |

Compose defaults the database path to `/app/data/uptimenest.db`. Edit the image and host port directly in Compose. Changing the listen address or frontend directory also requires updating Compose environment settings and mounts.

## Admin settings

After login, configure these options in System settings. Settings are stored in the database except for the browser theme preference.

| Category | Settings |
| --- | --- |
| Display | Site name, display timezone, entry page, default status page view |
| Language and theme | English or Simplified Chinese; new installations default to English. Switch light/dark themes manually; the preference stays in the current browser |
| Global monitor settings | HTTP/HTTPS User-Agent, status page TLS handshake latency warning threshold, certificate expiry highlighting days with per-monitor overrides |
| Access security | Current client IP and trusted proxy IPs/CIDRs; login limits use the resolved client address |
| History | Retention period, manual cleanup, automatic cleanup at startup and hourly |
| Account | Change the administrator password under Account settings |

The entry page can be the default status panel or admin interface. See [System settings](usage_en.md#system-settings) for details. Monitors, admin groups, and status panels have separate management pages.

## Database configuration

### SQLite

The default needs no separate database service. SQLite enables WAL, foreign keys, and busy waiting, using one write connection and up to four read-only connections. Connection settings are reapplied on reconnect; the default `synchronous=FULL` is retained.

You may change `UPTIMENEST_DB_DSN`, but container paths must be in a writable mount. Use retention settings or per-monitor cleanup to remove history rather than deleting the database.

### PostgreSQL

Create the database and application account first, with permission to create and migrate tables and indexes in that database. Compose connects to an existing PostgreSQL server; it does not include one.

Set these values in the working directory's `.env`:

```dotenv
UPTIMENEST_DB_TYPE=postgres
UPTIMENEST_DB_DSN='postgres://uptimenest:REPLACE_PASSWORD@pg.example.com:5432/uptimenest?sslmode=verify-full'
```

Run `docker compose up -d` after changing them. Configure appropriate TLS verification and trusted CAs for production, and URI-encode special characters in connection strings. `127.0.0.1` inside a container refers to that container; the database must be reachable from its network.

Direct executable runs use the same environment variables. Data is not migrated automatically between SQLite and PostgreSQL. A different database has its own accounts and configuration.

### Configuration encryption and keys

On first startup, the application generates 32 random bytes and writes the key as Base64 text to `UPTIMENEST_SECRET_KEY_FILE`. Linux file permissions are `0600`; on Windows, set directory ACLs for the application account. Key contents are never printed in logs or returned by the API.

During upgrade, monitor configuration, notification channels, alert snapshots, and notification events are encrypted in a transaction, with a key verification value recorded. If encrypted data already exists, a missing, malformed, or mismatched key prevents startup; the application does not replace it. The schema advances to version 6, and older executables cannot read the encrypted data directly. Heartbeat details (`heartbeats.data`) and monitor log messages (`monitor_logs.message`) remain unencrypted; errors may contain target addresses.

**Restoring a database requires its original key.** Back up the key separately from database backups. By default, both use the same persistent directory. To separate them, move the original key to another persistent mount and update its path. The directory must be writable for initial key generation; afterward, the key file may be readable only by the application account.

SQLite migration enables secure deletion and truncates the old WAL to reduce plaintext remnants from replaced records. Earlier backups, disk snapshots, and storage versions may still contain plaintext and require the same protection as before.

## HTTPS and reverse proxies

Point the domain to your server, terminate TLS at the reverse proxy, and proxy both pages and API to `127.0.0.1:8090`. Replace the domain and certificate paths in this Nginx example:

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

Preserve the original Host to keep browser pages and API on the same origin; write requests validate Origin. Preserve the application's CSP, Cache-Control, nosniff, and Referrer-Policy headers. Redirect HTTP to HTTPS and manage certificate renewal at the proxy.

Set `UPTIMENEST_COOKIE_SECURE=true` in `.env`, run `docker compose up -d`, and log in over HTTPS. The application serves HTTP and does not terminate external TLS.

Password verification failures are counted by the resolved client IP, aggregating IPv6 by `/64`. Ten failures within 15 minutes temporarily block that source; successful verification resets the failure count. A separate request token bucket allows bursts of 20 and refills one token per second. At most two bcrypt verifications run concurrently. Full limiter tables evict the least recently used source; the administrator account is not globally locked.

Under System settings → Access security, enter the proxy source IPs or CIDRs actually seen by the application. Trust only proxies you control.

Forwarded headers are accepted only from trusted connections. The application scans `X-Forwarded-For` from right to left, skips trusted proxies, and selects the first untrusted address. Without that header, it uses a valid `X-Real-IP`. If the list is empty or the connection is untrusted, it uses the connection address, so clients behind the same proxy share limits. Current IP shows the resolved result and updates after saving trusted proxies.

Preserve the following `Cache-Control` and `ETag` headers in Nginx; a blanket `expires` rule would override API policies. Configure compression in Nginx; the application does not generate gzip or Brotli responses.

| Content | Cache-Control | Conditional requests |
| --- | --- | --- |
| Content-hashed files in `assets/` | `public, max-age=31536000, immutable` | ETag |
| HTML and other static files without content hashes | `no-cache` | ETag |
| Public bootstrap and panel JSON | `public, max-age=0, must-revalidate` | ETag; identical content returns 304 |
| Admin APIs and API errors | `no-store` | No public response caching |

Public endpoints are rate-limited by IP, aggregating IPv6 by `/64`: bursts of 120 requests, refilling four tokens per second. Exceeding the limit returns 429 with `Retry-After: 1`.

## Monitor networking and permissions

| Monitor type | Deployment requirements |
| --- | --- |
| HTTP/HTTPS | Container DNS resolution and access to the target TCP port; system CAs verify certificates |
| WebSocket | Access to the WS/WSS port and server support for HTTP/1.1 Upgrade; WSS certificates must be trusted by system CAs |
| gRPC | Standard Health/Check service reachable from the container; TLS verification is enabled by default, with optional private CAs per monitor |
| HTTP/3 | HTTPS and QUIC support; container and host allow outbound UDP to the target |
| TCP | Target host and port reachable from the container |
| Ping | Native IPv4/IPv6 ICMP; Compose configures `net.ipv4.ping_group_range` for GID 65532, without system commands or raw socket privileges |
| DNS | Container system DNS by default, or a reachable server configured per monitor |

Loopback inside the container refers to that container. Choose addresses appropriate to the container network when monitoring host or private services.

Compose uses `cap_drop: [ALL]` and `no-new-privileges:true`. `net.ipv4.ping_group_range=65532 65532` permits only the application's group to create ICMP datagram sockets. This does not require `CAP_NET_RAW`, and the image does not install SUID Ping. Set the same sysctl for `docker run`; adjust the GID when using a different runtime user. With host networking or runtimes that disallow this container sysctl, an administrator must configure the group range in the relevant network namespace.

Linux and macOS use unprivileged ICMP sockets; Windows uses system ICMP APIs. None launches an external `ping`. Checks still fail if targets or firewalls block ICMP. See the Linux [kernel documentation](https://docs.kernel.org/networking/ip-sysctl.html#ping-group-range) for group range semantics.

HTTP/3 preferred mode falls back only when QUIC connection establishment fails. With an environment proxy, automatic mode negotiates HTTP/2 or HTTP/1.1 over TCP; explicit HTTP/3 reports that proxies are unsupported. Targets with a connection IP override connect directly. Declare proxy environment variables in Compose if needed.

## Backup and restore

Back up the database, original encryption key, and deployment configuration, including `docker-compose.yml` and any `.env`. Store key backups separately from database backups. For PostgreSQL, use `pg_dump` or the database's backup mechanism; the application's key file still needs a separate backup.

For SQLite, stop the application and back up the entire `data/` directory. An active WAL may contain committed data. Online backups must use the SQLite Backup API or `VACUUM INTO`; do not copy only an actively written main database file.

PowerShell example from the working directory:

```powershell
New-Item -ItemType Directory -Force backups
docker compose stop uptimenest
docker run --rm -v "${PWD}/data:/source:ro" -v "${PWD}/backups:/backup" alpine:3.23 tar czf /backup/uptimenest-data.tgz -C /source .
docker compose start uptimenest
```

Restore into a separate directory or instance, and verify login, monitor count, panels, and history before switching over. Retain the original backup for rollback and restrict backup access.

## Upgrades and rollback

1. Stop the application and back up the database and deployment configuration.
2. Change the Compose image version and run `docker compose up -d --pull always`.
3. Check container logs, health, administrator login, monitor checks, and public status pages.

Startup automatically applies versioned migrations. Rollback after a schema change requires restoring the pre-upgrade data snapshot as well. Run one monitoring instance; the pipeline does not update server deployments automatically.

v0.6.1 upgrades the schema to version 6: version 4 adds a covering index for heartbeat statistics, version 5 encrypts configuration, and version 6 encrypts notification events. Initial upgrade time depends on history volume, so back up first. Keep the newly generated key file permanently.

v0.6.0 upgrades the schema to version 3, adding notification channels, alert states, and a delivery queue. Back up before upgrading from v0.5.0; restore the pre-upgrade database for rollback. Alerts are disabled by default; configure channels, then enable them per monitor.

If an older deployment uses the `uptimenest-data` named volume in `compose.yaml`, stop it using the old configuration, copy all volume data into `./data`, adjust UID/GID permissions, then start with the new configuration. Do not run both instances simultaneously. Keep the old volume and backups until migration is verified. For executable upgrades, see [Binary deployment](binary_en.md#upgrade).

## Troubleshooting

| Symptom | Check |
| --- | --- |
| Container directory is not writable | Bind mount path and UID/GID `65532:65532` permissions |
| Database connection or migration fails | Startup logs, connection address, TLS, account permissions, and database existence |
| Missing key or configuration decryption failure | `UPTIMENEST_SECRET_KEY_FILE`, mounts, and the original key for this database |
| Initial password setup appears | Current database is uninitialized; if previously configured, check mounts or the database connection string |
| Invalid setup token | Copy `setup_token` from the current startup log; tokens from earlier starts are invalid |
| HTTPS login does not persist | Access protocol, `UPTIMENEST_COOKIE_SECURE`, and proxy preservation of Host |
| Login returns 429 | Wait 15 minutes; check the resolved client IP and trusted proxy configuration |
| Ping fails | Target reachability and ICMP policy; on Linux, runtime GID and `ping_group_range`; IPv6 also needs working IPv6 connectivity |
| HTTP/3 fails | Target QUIC support, outbound UDP, and proxy settings |
| Status page has no services | Panel monitor assignments, empty display groups, and URL path |
| Statistics change after cleanup | Uptime and average latency are recalculated from remaining records in the last 24 hours |

Use `docker compose logs --tail=100 uptimenest` for recent logs and `docker compose ps` for container health. Direct executable runs can use `uptimenest healthcheck`.
