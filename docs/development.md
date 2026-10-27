# 开发指南

[文档中心](README.md) · [API 参考](api.md) · [发布流程](releasing.md)

[环境](#环境与结构) · [本地运行](#分离开发) · [构建](#完整程序与镜像) · [验证](#按需验证) · [数据与并发](#数据库与并发约定) · [检测逻辑](#监控检测与扩展)

## 环境与结构

使用 Go 1.27.3 或更高版本（工具链见 `go.mod`）、Node.js 24 和 npm。数据库驱动和原生 ICMP 均支持无 CGO 发行构建，不依赖系统 `ping`。依赖由 `go.sum` 和 `web/package-lock.json` 固定。

```text
cmd/uptimenest/       启动与健康检查命令
internal/api/         认证、管理接口、访客接口与静态页面
internal/config/      环境变量配置
internal/model/       监控、心跳、分组、面板与设置模型
internal/store/       SQLite / PostgreSQL、迁移、历史与统计
internal/monitor/     检测、任务调度、历史清理与状态事件
internal/webui/       按构建标签内嵌前端
web/src/              Vue、TypeScript、路由与中文语言资源
scripts/              内嵌前端准备脚本
.github/              检查、打包与发布
docker-compose.yml    统一镜像运行配置
docs/                 使用与维护文档
```

## 分离开发

第一个 PowerShell 终端从仓库根目录启动后端：

```powershell
go run ./cmd/uptimenest
```

第二个终端从仓库根目录启动前端：

```powershell
cd web
npm ci
npm run dev
```

访问 <http://127.0.0.1:5173>。Vite 将 `/api` 代理到 `127.0.0.1:8090`。Go 不自动读取 `.env`，数据库默认位于仓库的 `data/uptimenest.db`。首次从 Go 启动日志复制 `setup_token`，通过页面填写令牌并设置密码。

未启用 `with_frontend` 且未设置 `UPTIMENEST_WEB_DIR` 时，后端回退到 `web/dist`；该目录不存在时 API 仍可用，页面请求会提示先构建前端。开发页面由 Vite 提供，不要求预先构建。

## 完整程序与镜像

### 构建完整程序

从仓库根目录构建前端并准备内嵌目录：

```sh
npm --prefix web ci
npm --prefix web run build
node scripts/prepare-webui.mjs
```

在仓库根目录执行：

```sh
CGO_ENABLED=0 go build -tags=with_frontend -trimpath -ldflags='-s -w' -o bin/uptimenest ./cmd/uptimenest
```

PowerShell 使用：

```powershell
$env:CGO_ENABLED = '0'
go build -tags=with_frontend -trimpath -ldflags='-s -w' -o bin/uptimenest.exe ./cmd/uptimenest
```

`with_frontend` 内嵌页面。准备脚本只替换固定的 `internal/webui/dist` 生成目录，并拒绝隐藏配置、源码映射、依赖目录及符号链接。生成的程序与 [发行程序](binary.md) 使用同一启动配置。

### 构建本地镜像

在仓库根目录执行：

```sh
docker build -t uptimenest:local .
```

按 [部署指南](deployment.md) 准备可写的数据目录。把运行目录中 Compose 的 `image` 改为 `uptimenest:local`，执行 `docker compose up -d --pull never`。

Dockerfile 默认 `standalone` 目标从源码构建；CI 的 `release` 目标复用打包程序。镜像保留 Alpine，不额外复制 SUID BusyBox；Compose 配置无特权 ICMP 所需的组范围。平台范围见 [发布流程](releasing.md#统一镜像)。

### 版本与构建标识

浏览器控制台在页面加载时输出 `UptimeNest v版本号_短提交号` 和版权，路由切换与数据刷新不会重复打印。版本取自 `web/package.json`；提交号优先读取构建环境的 `BUILD_COMMIT`，其次为 `GITHUB_SHA`，本地构建则读取 Git。提交号不可用时显示 `unknown`，不阻止构建。

Docker 源码构建不包含 `.git`，可显式传入提交号：

```sh
docker build --build-arg BUILD_COMMIT="$(git rev-parse HEAD)" -t uptimenest:local .
```

CI 先构建一次前端，再复用内嵌资源打包各平台程序和镜像；同一版本的控制台提交号保持一致。

## 按需验证

仓库根目录的常用命令：

```sh
go vet ./...
go test ./...
```

前端目录执行 `npm run build`，包含 TypeScript 检查和生产构建。按改动影响选择检查；纯文档调整检查链接、命令和截图即可。项目不额外添加测试框架或数据库测试服务，CI 常规检查及可选 `full_checks` 见 [发布流程](releasing.md#触发方式)。

## 数据库与并发约定

### 连接、事务与迁移

| 配置 | SQLite | PostgreSQL |
| --- | --- | --- |
| 驱动 | modernc.org/sqlite | pgx |
| 最大连接数 | 写池 1、读池 4；内存库共用 1 个连接 | 12 |
| 占位符 | `?` | 由 Store.Q 转换为 `$1` 等 |
| 运行方式 | WAL、本机文件 | 独立数据库服务 |

多语句原子操作显式使用事务，事务内使用同一个 tx。SQL 来自内部固定语句，用户值通过参数传递。数据库结构版本独立于应用版本，新增变更使用新迁移。

无事务的读取使用 `Store.ReadDB()`，写入和事务使用 `Store.DB`，退出时使用 `Store.Close()` 关闭两个池。SQLite 通过 DSN 为每个连接配置外键和繁忙等待，读池启用 `query_only`，写池使用 immediate 事务；保留默认的 FULL 同步级别。数据库版本 4 增加心跳统计覆盖索引。

Settings 在进程内缓存，首次读取时加载，`SaveSettings` 提交成功后更新。运行中设置变更统一通过保存接口；直接修改数据库后需要重启。缓存按单应用实例使用，读取与保存均复制可信代理列表，避免共享可变切片。

数据库版本 5 将 `monitors.data`、`notification_channels.data` 和 `alert_states.data` 转为 AES-256-GCM 密文，版本 6 增加 `notification_deliveries.event`。附加认证数据绑定表和主键，告警状态另绑定类型；通知事件须通过 `queueDelivery` 入队，它先取得记录编号再写入绑定该编号的密文；随机 nonce 由标准库生成。读写必须经过 Store 的加解密方法，不能直接对这些列调用 JSON 解码。主程序通过 `UPTIMENEST_SECRET_KEY_FILE` 持久化密钥，恢复数据库时需要原文件。

数据库 v2 增加后台父级和独立面板展示分组。升级时冻结旧面板名称及顺序，后续后台分组变化不影响面板。分组调整禁止形成循环；面板保存及监控删除保持引用一致。

### 时间与历史

心跳和日志时间使用 Unix 毫秒。汇总统计使用近 24 小时记录，后台延迟接口按请求范围查询，最多返回 480 个非空时间段。曲线使用段平均值并保留最小值、最大值；平均值、P95 和最大值根据全部有效成功记录计算。接口与保留期限规则见 [API 参考](api.md#监控管理)。

前后台心跳条使用固定条宽和间距，按容器宽度决定显示数量，并通过 `heartbeatCount` 按需请求最近记录，最多 1000 条。未指定有效数量时，公开接口默认 90 条，后台默认 60 条；条数不代表固定时间范围。

应用启动时及每小时清理超出保留期限的心跳和日志。按项清空仅删除指定监控的对应记录，统计根据剩余数据重新计算。

### 账户与公开边界

管理员密码使用 bcrypt，数据库只保存会话令牌摘要。写请求校验同源 Origin，管理接口使用 HttpOnly、SameSite=Strict Cookie。登录限流与后台当前 IP 共用可信代理解析规则；仅接受可信连接来源的转发头，详见 [部署指南](deployment.md#https-与反向代理)。

访客接口使用字段白名单，启用隐藏地址时也省略地址字段。管理列表和详情轮询只读取展示数据；编辑时读取独立配置接口，敏感内容不回显，更新使用 `configured`、`clearSecrets` 和后端继承规则。所有状态面板均公开，描述与标签属于公开内容。

公开缓存按心跳档位共享构建，响应时裁剪条数。ETag 根据实际响应内容生成，不能只使用 `PublicVersion`，因为正常心跳也会更新延迟和时间。前端仅对公开数据保留有界条件请求缓存；bootstrap 合并并发请求，状态页最多每分钟重新读取一次。

## 监控检测与扩展

每项监控独立调度，总并发上限 32；完整检测结束后再等待正常间隔。重试期间不提前记录故障，耗尽重试后只写入最终心跳。停用或修改配置会取消旧任务，旧配置的检测结果不会写回新配置。

Ping 共用带超时的域名解析，并使用解析结果中的首个地址发送一次 Echo。Linux/macOS 通过 `golang.org/x/net/icmp` 的 `udp4` / `udp6` 端点发送 ICMP，检查来源、序列号和随机载荷；Linux 另校验内核分配的 Echo ID。取消时关闭套接字以唤醒等待。

Windows 通过系统目录中的 `iphlpapi.dll` 调用 `IcmpSendEcho2` / `Icmp6SendEcho2`，校验状态、来源和随机载荷，无需外部进程或原始套接字。任务取消可立即返回；底层同步调用仍持有固定的缓冲区，完成或到达原超时后释放，尚未完成的调用另限制为 32 个。

HTTP/3 优先模式只在 QUIC 连接建立失败时回退。跨来源跳转移除自定义头与验证信息，mTLS 拒绝跨来源跳转。TLS 与 QUIC 元数据取自真实连接，不从配置推断。单项 `ignoreTLS` 默认关闭，开启后仅跳过检测目标的证书校验，同时适用于 HTTP/3；OAuth2 令牌请求仍独立校验证书。

手动 HTTP 协议模式不降级。指定连接 IP 只替换原目标主机的拨号地址，保留 Host、SNI 和证书域名校验。WebSocket 每次新建连接，支持握手或匹配 Ping/Pong，WSS 始终校验证书；其配置和路径不进入访客接口。

告警状态与通知事件在最终心跳事务中提交。`internal/notify` 在独立后台任务中消费数据库队列，不占用检测线程。服务异常使用独立连续失败次数，每轮最终失败计一次，成功清零，计数持久化；证书到期及重复提醒额外按分钟检查，不增加失败计数。`StateHandler` 保留给可选的进程内订阅，不承担可靠投递。

数据库结构版本 3 增加渠道、告警状态和投递队列。发送使用一分钟租约，同一监控／渠道按事件顺序处理；首次失败后最多重试 3 次。进程退出后恢复未完成的队列任务，网络超时或在投递成功后意外退出仍可能造成重复接收，不保证恰好一次。已完成的发送记录随历史保留期限清理。

前端使用 Vue 3、Vue Router、vue-i18n、Tailwind CSS 和基于 Reka UI 的本地 shadcn-vue 组件。当前中文文案集中在 `web/src/locales/zh-CN.ts`。状态页与后台样式分别维护；字体版权及 OFL 许可见 `web/public/fonts/OFL.txt`。

## 协作约定

自写代码和关键逻辑注释使用中文，保持代码简洁、可读。功能变更同步更新使用说明或 API 文档，发布变化记录在 [更新记录](changelog.md)。

凭据、私钥、本地数据库、临时工具、协作规则及设计过程文件不写入日志、截图或仓库。首次初始化令牌是日志输出的明确例外，仅在尚未创建管理员时输出，重启后更新，不通过 API 返回。提交、推送和发布需先按项目协作约定取得确认。
