# 文档中心

[项目首页](../README.md) · [下载版本](https://github.com/Qwerto107/UptimeNest/releases)

## 使用

| 要做的事 | 文档 |
| --- | --- |
| 查看状态页和管理后台的实际界面 | [界面截图](screenshots.md) |
| 创建监控、检查连接信息和历史记录 | [使用指南](usage.md#监控管理) |
| 配置 HTTP 协议、指定 IP 或 WebSocket 检测 | [HTTP 配置](usage.md#http-请求与验证) · [WebSocket 检测](usage.md#websocket-检测) |
| 配置 gRPC 健康检查 | [gRPC 检测](usage.md#grpc-检测) |
| 查看延迟曲线、时间范围和统计口径 | [响应延迟图表](usage.md#响应延迟图表) |
| 配置通知渠道、告警与消息模板 | [告警通知](usage.md#告警通知) |
| 管理分组、状态面板与三种视图 | [监控分组](usage.md#监控分组) · [状态面板](usage.md#状态面板) |
| 配置语言、主题、延迟提醒、可信代理和历史保留 | [系统设置](usage.md#系统设置) |
| 用脚本读取状态或管理监控 | [API 参考](api.md) |
| 查看版本变化 | [更新记录](changelog.md) |

## 部署

推荐使用 Docker Compose。直接运行程序时，选择二进制部署；两种方式的数据库、后台设置和运维说明统一放在部署指南。

| 要做的事 | 文档 |
| --- | --- |
| 用 Docker 安装，配置数据库和 HTTPS | [部署指南](deployment.md) |
| 下载并运行 Linux、Windows 或 macOS 程序 | [二进制部署](binary.md) |
| 备份、恢复、升级或排查故障 | [备份与恢复](deployment.md#备份与恢复) · [升级与回退](deployment.md#升级与回退) · [故障排查](deployment.md#故障排查) |

## 开发

| 要做的事 | 文档 |
| --- | --- |
| 本地开发、源码构建及了解内部实现 | [开发指南](development.md) |
| 配置 CI、构建产物并发布版本 | [发布流程](releasing.md) |

## 默认地址

| 运行方式 | 地址 | 命令执行位置 |
| --- | --- | --- |
| Docker Compose | <http://127.0.0.1:8090> | 自建运行目录，内含 `docker-compose.yml`；`.env` 按需创建 |
| 发行程序 | <http://127.0.0.1:8090> | 解压后的程序目录 |
| 本地前端开发 | <http://127.0.0.1:5173> | 源码仓库，前后端分别启动 |

容器和发行程序均包含前端。首次访问 `/admin` 时，填写启动日志中的 `setup_token` 并设置密码；访客面板路径为 `/status/<面板路径>`。平台以对应 Release 附件为准，实际界面见 [界面截图](screenshots.md)。
