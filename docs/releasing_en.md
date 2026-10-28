# Release process

[简体中文](releasing.md) · English

[Documentation](README_en.md) · [Development guide](development_en.md) · [Deployment guide](deployment_en.md)

The [build, check, and release workflow](../.github/workflows/ci.yml) delivers complete executables with the frontend embedded, plus one multi-architecture image. Source build commands are in the [development guide](development_en.md#complete-executables-and-images).

[Triggers](#triggers) · [Build order](#build-order) · [Artifacts](#artifacts) · [Image tags](#container-image) · [Permissions](#repository-permissions) · [Release steps](#releasing-and-deploying)

## Triggers

| Trigger | Checks | Artifacts and publishing |
| --- | --- | --- |
| Branch push or PR matching workflow paths | Go formatting, static checks, existing tests, frontend type checking and build | No packaging or publishing |
| Manual run | Standard checks | Seven-platform archives as Actions artifacts; no Release |
| Manual run with `publish_images` | Standard checks | Seven-platform packages and a commit-SHA image; allowed only from `main` |
| Manual run with `full_checks` | Race testing replaces ordinary tests, plus dependency vulnerability and container health/homepage checks | Image publishing still depends on `publish_images` |
| Push `vX.Y.Z` | Standard checks and dependency vulnerability checks | Seven-platform assets, three-architecture image, a public Release, and an updated `latest` tag |
| Push `vX.Y.Z-rc.1`, etc. | Same as a stable release | Prerelease and versioned image; does not update `latest` |

`full_checks` defaults to off. CI does not start database test services or browser end-to-end tests. Routine checks keep only the latest run; version releases and manual image publishing are not cancelled midway.

Workflow path filters include frontend/backend source, Go dependency files, scripts, Docker configuration, `.github`, and the packaged documentation files `docs/binary.md` and `docs/releasing.md`. Changes only to other documentation do not trigger routine branch checks.

## Build order

1. Run backend checks and build the frontend once, saving the embedded-page artifact.
2. Reuse those pages for seven platform builds with `CGO_ENABLED=0` and `with_frontend`; package and calculate SHA-256 hashes.
3. Verify and reuse the Linux executables for images, without recompiling the frontend or backend.
4. For tag releases, create a Release draft after image publishing succeeds, upload assets, download them again to verify, then publish.

Required-job failures prevent a new Release from becoming public. Reruns add missing assets only; existing checksum files must match the current build. Go comes from `go.mod`, and Node.js is version 24. The workflow does not add checks beyond the documented process.

## Artifacts

See [binary deployment](binary_en.md#download-and-verify) for platform and file names. Each platform has an archive and a matching `.sha256` file, for seven pairs. The packaged README comes from `docs/binary.md`.

Actions artifacts named `uptimenest-binaries-{platform}` are retained for 30 days; `uptimenest-webui-dist` is retained for 7 days. Archives normalize file order, timestamps, and ownership, but toolchains and dependencies can still affect their bytes. Check each Release for historical asset availability.

## Container image

The image is `ghcr.io/qwerto107/uptimenest`, supporting `linux/amd64`, `linux/arm64`, and `linux/arm/v6`.

| Release type | Image tags |
| --- | --- |
| Stable `v1.2.3` | `1.2.3`, `latest`, `sha-<full-commit>` |
| Prerelease `v1.2.3-rc.1` | `1.2.3-rc.1`, `sha-<full-commit>` |
| Manual image publishing | `sha-<full-commit>` |

Commits to `main` do not automatically publish a dev image. The runtime uses Alpine with CA certificates, timezone data, and a non-root user. Ping uses native ICMP without an additional SUID Ping executable; Compose authorizes unprivileged ICMP through the group range. Health checks run `/app/uptimenest healthcheck`.

The official Alpine image does not cover ARMv5 or LoongArch64; standalone executables are provided for those targets. The Dockerfile's default `standalone` target builds from source, while `release` reuses packaged executables. Publishing does not require additional target-architecture emulation. See [single-container deployment](deployment_en.md#single-container-deployment).

## Repository permissions

Checks and packaging use `contents: read`. Image jobs additionally use `packages: write`, and Release jobs use `contents: write`, through the built-in `GITHUB_TOKEN`.

Actions are pinned to complete commit SHAs, and checkout does not retain credentials. The GHCR package must permit this repository to write. Anonymous pulls also require public package visibility; a public repository does not automatically make its image public.

## Releasing and deploying

1. Update frontend version metadata and the [changelog](changelog_en.md), moving **Unreleased** entries into the new version.
2. After approval, commit and push, and verify which commit the release tag will reference.
3. Create and push the version tag. Actions builds the image, packages, and Release.

The version below is only a format example:

```sh
git tag -a v1.2.3 -m "UptimeNest v1.2.3"
git push origin v1.2.3
```

Actions creates the Release automatically. Do not publish a second Release locally or move an already published tag. The workflow result determines whether artifacts and the Release are public.

Frontend builds use `GITHUB_SHA` for the console's short commit ID; every executable and image reuses that frontend artifact. For source builds, see the [`BUILD_COMMIT` setting](development_en.md#version-and-build-identity).

Different tags have no additional ordering lock. Under the existing policy, every stable tag release updates `latest`, including releases of older versions. Images become public before the Release; a later asset failure does not automatically roll back images. A failed rerun does not retract an already public Release.

The pipeline builds and distributes artifacts only. It does not store server SSH credentials or update deployments. See [upgrades and rollback](deployment_en.md#upgrades-and-rollback) or [binary deployment](binary_en.md) for installation and operations.
