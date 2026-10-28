# Documentation

[简体中文](README.md) · English

[Project home](../README_en.md) · [Downloads](https://github.com/Qwerto107/UptimeNest/releases)

## Usage

| Task | Document |
| --- | --- |
| View actual status pages and the admin interface | [Interface screenshots](screenshots_en.md) |
| Create monitors and inspect connections and history | [Monitor management](usage_en.md#monitor-management) |
| Configure HTTP protocols, connection IPs, or WebSocket checks | [HTTP configuration](usage_en.md#http-requests-and-authentication) · [WebSocket checks](usage_en.md#websocket-checks) |
| Configure gRPC health checks | [gRPC checks](usage_en.md#grpc-checks) |
| Understand latency charts, ranges, and statistics | [Latency charts](usage_en.md#latency-charts) |
| Configure channels, alerts, and message templates | [Notifications](usage_en.md#notifications) |
| Manage groups, status panels, and the three views | [Monitor groups](usage_en.md#monitor-groups) · [Status panels](usage_en.md#status-panels) |
| Configure language, theme, warnings, trusted proxies, and retention | [System settings](usage_en.md#system-settings) |
| Read status or manage monitors from scripts | [API reference](api_en.md) |
| View version history | [Changelog](changelog_en.md) |

## Deployment

Docker Compose is recommended. Alternatively, run a standalone executable. The deployment guide covers shared database, admin, and operational settings for both approaches.

| Task | Document |
| --- | --- |
| Install with Docker and configure databases and HTTPS | [Deployment guide](deployment_en.md) |
| Download and run Linux, Windows, or macOS executables | [Binary deployment](binary_en.md) |
| Back up, restore, upgrade, or troubleshoot | [Backup and restore](deployment_en.md#backup-and-restore) · [Upgrades and rollback](deployment_en.md#upgrades-and-rollback) · [Troubleshooting](deployment_en.md#troubleshooting) |

## Development

| Task | Document |
| --- | --- |
| Develop locally, build from source, and understand internals | [Development guide](development_en.md) |
| Configure CI, artifacts, and releases | [Release process](releasing_en.md) |

## Default addresses

| Deployment | Address | Command working directory |
| --- | --- | --- |
| Docker Compose | <http://127.0.0.1:8090> | A deployment directory containing `docker-compose.yml`; `.env` is optional |
| Release executable | <http://127.0.0.1:8090> | The extracted package directory |
| Local frontend development | <http://127.0.0.1:5173> | The source checkout, with frontend and backend started separately |

Containers and release executables include the frontend. On your first visit to `/admin`, enter the startup log's `setup_token` and set a password. Public panels use `/status/<panel-slug>`. Platform availability depends on the assets attached to each Release. See [interface screenshots](screenshots_en.md) for actual layouts.
