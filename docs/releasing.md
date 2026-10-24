# 构建与发布

工作流为 [构建、检查与发布](../.github/workflows/ci.yml)，按 [QPix v0.4.0](https://github.com/Qwerto107/QPix/tree/v0.4.0) 及用户提供的 CI/CD 参考文档适配。保留原有任务、触发和发布规则，没有新增检查任务或测试用例。

## 触发方式

| 触发 | 检查 | 交付 |
| --- | --- | --- |
| 分支提交或 PR 命中工作流路径 | Go 格式、静态检查、现有测试，前端类型检查与构建 | 不打包、不发布 |
| 手动运行，选项均关闭 | 常规检查 | 七平台下载包，保存为 Actions 产物 |
| 手动启用 `publish_images` | 常规检查 | 七平台下载包及提交 SHA 镜像，仅允许 main |
| 手动启用 `full_checks` | 竞态检查替代普通测试，加依赖漏洞和容器健康／首页检查 | 是否推送镜像由 `publish_images` 决定 |
| 推送 `vX.Y.Z` | 常规检查及依赖漏洞检查 | 下载包、镜像及正式 Release，更新 latest |
| 推送 `vX.Y.Z-rc.1` 等 | 与正式版相同 | 预发布 Release 和版本镜像，不更新 latest |

`full_checks` 默认关闭。没有数据库测试服务或浏览器端到端测试。普通 main 提交不发布 dev 镜像。

路径过滤覆盖 `cmd`、`internal`、Go 依赖文件、`web`、构建脚本、Docker 配置、`.github`，以及参与打包的 `docs/binary.md` 和本文。仅修改其他文档不会触发常规分支检查。

## 构建顺序

1. 后端检查与前端构建并行，前端只构建一次，保存到 `internal/webui/dist`。
2. 七个平台复用页面，以 `CGO_ENABLED=0`、`with_frontend` 构建并生成压缩包及 SHA-256。
3. 镜像任务核对并复用 Linux 包中的程序，不再次编译。
4. 标签发布在镜像成功后创建 Release 草稿，上传并重新下载校验附件，全部成功后公开。

后端使用 `go.mod` 指定的 Go 版本；前端使用 Node.js 24 和锁文件。移除仅供 QPix 图片库使用的 `nodynamic` 标签。

前端产物 `uptimenest-webui-dist` 保留 7 天，`uptimenest-binaries-{平台}` 保留 30 天。下载包说明见 [binary.md](binary.md)，包内 README 来自该文档。归档固定文件时间，tar 同时固定排序和所有者；这不保证跨工具链完全可复现。

## 容器镜像

镜像地址为 `ghcr.io/qwerto107/uptimenest`。

| 场景 | 镜像标签 |
| --- | --- |
| 正式版 `v1.2.3` | `1.2.3`、`latest`、`sha-完整提交号` |
| 预发布 `v1.2.3-rc.1` | `1.2.3-rc.1`、`sha-完整提交号` |
| main 手动发布 | `sha-完整提交号` |

UptimeNest 的 Ping 使用系统命令，运行层采用 Alpine，包含证书、时区、非 root 用户及对应架构的 BusyBox Ping。主程序使用 UID/GID `65532:65532`；专用 Ping 入口通过 setuid 获取 ICMP 所需权限，容器环境仍须允许 ICMP。数据目录保留 `/app/data`，端口为 8090。健康检查执行 `/app/uptimenest healthcheck`，不初始化数据库或启动监控。

镜像支持 `linux/amd64`、`linux/arm64`、`linux/arm/v6`。Alpine 官方镜像未提供 ARMv5 和 LoongArch64，因此这两个平台仅提供独立下载包，不宣称容器支持。Dockerfile 保留 `release`（复用本次二进制）和默认 `standalone`（本地源码构建）两个入口；发布时无需执行目标架构程序或新增模拟器步骤。

容器部署示例：

```sh
docker run -d --name uptimenest --restart unless-stopped \
  -p 127.0.0.1:8090:8090 \
  -v uptimenest-data:/app/data \
  ghcr.io/qwerto107/uptimenest:latest
```

首次访问 `/admin` 设置管理员密码。HTTPS 反向代理部署时设置 `UPTIMENEST_COOKIE_SECURE=true`。更多环境变量见 [README](../README.md#运行配置)。

## 权限与重跑

检查和打包使用 `contents: read`，镜像额外使用 `packages: write`，Release 使用 `contents: write`，均使用内置 `GITHUB_TOKEN`。Actions 固定完整提交 SHA，checkout 不保留凭据。GHCR 包需允许本仓库写入；匿名拉取还需将包设为 Public。

日常检查取消同 ref 的旧运行；标签发布和手动镜像发布不中途取消。不同版本没有额外发布顺序锁。

Release 重跑只补传缺失附件，已有校验文件必须与本次构建一致，不自动覆盖。首次发布失败时保留草稿；已经公开的 Release 不因重跑失败而撤回。镜像先于 Release 公开，后续附件失败不会自动回滚镜像。沿用 QPix 策略，每次正式标签发布均更新 latest，包含补发旧版本。

## 发版

提交并推送已确认的代码后，创建并推送版本标签；Actions 自动构建镜像、附件和 Release。以下仅为版本格式示例：

```sh
git tag -a v1.2.3 -m "UptimeNest v1.2.3"
git push origin v1.2.3
```

流水线交付下载包、镜像和 Release，不自动更新服务器。实际公开状态以对应工作流结果为准。
