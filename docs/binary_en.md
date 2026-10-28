# Binary deployment

[简体中文](binary.md) · English

[Documentation](https://github.com/Qwerto107/UptimeNest/blob/main/docs/README_en.md) · [Deployment guide](https://github.com/Qwerto107/UptimeNest/blob/main/docs/deployment_en.md)

UptimeNest release executables include the frontend. Extract a package to serve the pages and API without installing Go or Node.js. Ping uses native ICMP and does not depend on a system `ping` command; network policy must allow ICMP.

[Download](#download-and-verify) · [Linux](#linux) · [Windows](#windows) · [macOS](#macos) · [Configuration](#configuration-and-data) · [Upgrade](#upgrade)

## Download and verify

Download the archive and its matching `.sha256` file from [GitHub Releases](https://github.com/Qwerto107/UptimeNest/releases).

| OS / architecture | File |
| --- | --- |
| Linux x86-64 | `uptimenest-linux-amd64.tar.gz` |
| Linux ARM64 | `uptimenest-linux-arm64.tar.gz` |
| Linux ARMv6 | `uptimenest-linux-armv6.tar.gz` |
| Linux ARMv5 | `uptimenest-linux-armv5.tar.gz` |
| Linux LoongArch64 | `uptimenest-linux-loong64.tar.gz` |
| Windows x86-64 | `uptimenest-windows-amd64.zip` |
| macOS Apple Silicon | `uptimenest-darwin-arm64.tar.gz` |

Each package contains the executable and a README. Available assets depend on the release. The pipeline cross-compiles these targets; this does not mean every device has been tested in person.

## Linux

For x86-64, run these commands in the download directory:

```sh
sha256sum -c uptimenest-linux-amd64.tar.gz.sha256
tar -xzf uptimenest-linux-amd64.tar.gz
cd uptimenest-linux-amd64
./uptimenest
```

Substitute the package name for another Linux architecture.

Linux Ping uses unprivileged ICMP datagram sockets. If access is denied, an administrator must set `net.ipv4.ping_group_range` to include the runtime account's GID. To authorize one group, use that GID for both endpoints. The application does not need SUID or `CAP_NET_RAW`.

## Windows

Run in PowerShell:

```powershell
$archive = 'uptimenest-windows-amd64.zip'
$expected = (Get-Content "$archive.sha256").Split(' ')[0]
if ((Get-FileHash $archive -Algorithm SHA256).Hash -ne $expected) {
    throw 'Checksum mismatch; download the archive again'
}
Expand-Archive $archive -DestinationPath .
Set-Location uptimenest-windows-amd64
./uptimenest.exe
```

## macOS

```sh
shasum -a 256 -c uptimenest-darwin-arm64.tar.gz.sha256
tar -xzf uptimenest-darwin-arm64.tar.gz
cd uptimenest-darwin-arm64
./uptimenest
```

## Configuration and data

The default address is `http://127.0.0.1:8090`. On your first visit to `/admin`, enter the startup log's `setup_token` and set an administrator password. There is no shared default password. Before initialization, restarting generates a new token; creating the administrator invalidates it. Complete initialization before exposing the service through a reverse proxy.

Configuration comes from environment variables; the application does not load `.env` automatically. The default SQLite path, `data/uptimenest.db`, is relative to the process working directory. The runtime account needs write access. For long-running services, fix the working directory or use an absolute path. Configure PostgreSQL with `UPTIMENEST_DB_TYPE=postgres` and `UPTIMENEST_DB_DSN`.

The encryption key is generated at `data/secret.key` by default. Set `UPTIMENEST_SECRET_KEY_FILE` to use another persistent path. SQLite and PostgreSQL both require this file. Back up the original key with the database, store the two backups separately, and retain the key when restoring. Lost keys cannot recover encrypted configuration.

Leave `UPTIMENEST_WEB_DIR` empty to use the embedded frontend. Setting it overrides the embedded pages with that directory. See [startup parameters](https://github.com/Qwerto107/UptimeNest/blob/main/docs/deployment_en.md#startup-parameters) for database and process settings, and [admin settings](https://github.com/Qwerto107/UptimeNest/blob/main/docs/deployment_en.md#admin-settings) for runtime UI settings.

Run `./uptimenest healthcheck`, or `./uptimenest.exe healthcheck` on Windows, to query the health endpoint at the configured listening address. A nonzero exit status indicates failure. This command does not open the database or start monitor workers.

## Upgrade

Stop the old process and back up the database. Verify and extract the new package, preserve the working directory, data path, and environment, then start the application. The frontend updates with the executable. After a schema migration, rolling back requires a database backup compatible with the old application. Use a consistent backup method for a running SQLite database; do not copy only its actively written main file.

See [backup and restore](https://github.com/Qwerto107/UptimeNest/blob/main/docs/deployment_en.md#backup-and-restore) and [upgrades and rollback](https://github.com/Qwerto107/UptimeNest/blob/main/docs/deployment_en.md#upgrades-and-rollback).

## Build from source

Frontend build, embedding, and Go build commands are documented in the [development guide](https://github.com/Qwerto107/UptimeNest/blob/main/docs/development_en.md#complete-executables-and-images).
