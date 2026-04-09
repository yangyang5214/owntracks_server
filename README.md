# owntracks_server

基于 Go 的 [OwnTracks](https://owntracks.org/) HTTP 接收端与轨迹 Web 控制台。手机端在 App 里选择 **HTTP** 模式，将位置上报到本服务；数据写入 **ClickHouse**，浏览器访问内置页面查看轨迹与统计。

## 功能概览

- **HTTP 上报**：兼容 OwnTracks 的 `POST /pub/...` 路径与 `topic` 查询参数；支持 Basic 认证（可关闭）。
- **Web 控制台**：内置静态前端，通过 `/api/*` 拉取元数据、旅程折线、统计。
- **存储**：位置点写入 ClickHouse `locations` 表；仓库内提供建表 SQL 与物化视图（最新位置等）。
- **SQL 执行**：`ck` 子命令按文件执行 DDL/迁移（见下文）。

## 要求

- Go **1.25+**
- **ClickHouse**：必填；启动时会连接并 `ping`，失败则进程退出。

## 快速开始

```bash
cd /path/to/owntracks_server
go run ./cmd/server web
```

默认读取工作目录下的 `configs/config.yaml`，监听 `:8080`。须在配置或 `CLICKHOUSE_DSN` 中提供 ClickHouse 连接；未配置或 `ping` 失败时服务无法启动。

### 连接 ClickHouse

1. 在 ClickHouse 中执行 `resource/init.sql` 创建库表，例如：`go run ./cmd/server ck --database default resource/init.sql`（若配置里已连到已存在的 `owntracks` 库，也可直接 `ck resource/init.sql`）。
2. 编辑 `configs/config.yaml`：填写 `clickhouse.dsn` 或 `host` 等字段。
3. 可选：用环境变量 `CLICKHOUSE_DSN` 覆盖连接串。

### OwnTracks 客户端设置

- 模式：**HTTP**
- URL 示例：`https://你的主机:8080/pub/用户名/设备名`  
  或 `POST /pub?topic=owntracks%2F用户名%2F设备名`
- 若在配置中设置了 `auth.http_user` / `auth.http_pass`，请在 App 中填写对应的 HTTP 基本认证。

## 命令行

| 命令 | 说明 |
|------|------|
| `owntracks_server web` | 启动 HTTP 服务（上报 + 控制台） |
| `owntracks_server server` | 与 `web` 相同 |
| `owntracks_server ck <sql-file>...` | 在 ClickHouse 上执行 SQL 文件（按分号拆成多条语句依次执行） |

常用参数：

- `--config <path>`：配置文件路径，默认 `configs/config.yaml`
- `--listen <addr>`：监听地址（默认来自配置，如 `:8080`），仅 `web` / `server` 使用
- `ck --database <name>`：覆盖连接串中的库名（脚本含 `CREATE DATABASE` 时常用 `default`）

## HTTP 接口摘要

| 方法 | 路径 | 说明 |
|------|------|------|
| `POST` | `/pub/{user}/{device}` | 上报位置 JSON（写入 ClickHouse） |
| `POST` | `/pub` | 需 `topic` 查询参数，或 body 内 JSON 含 `topic`，或配合路径式 URL |
| `GET` | `/api/meta` | 元信息 |
| `GET` | `/api/journey` | 轨迹；支持 `from` / `to`（RFC3339）、`interval_sec` |
| `GET` | `/api/stats` | 统计；支持 `from` / `to` |
| `GET` | `/*` | 静态资源与单页控制台 |

成功上报时响应体为 `[]`（与常见 OwnTracks HTTP 行为一致）。上报体大小限制约 **1 MiB**。

## 环境变量

| 变量 | 作用 |
|------|------|
| `WEB_ADDR` | 覆盖监听地址 |
| `HTTP_USER` / `HTTP_PASS` | 覆盖 Basic 认证（与 YAML `auth` 一致） |
| `CLICKHOUSE_DSN` | 覆盖 ClickHouse 连接串 |
| `LOG_FILE` | 日志文件路径；`-` 或 `none` 表示仅标准输出 |
## 配置说明（`configs/config.yaml`）

- **`log.file`**：默认写入 `logs/owntracks.log`，与控制台双写；可被 `LOG_FILE` 覆盖。
- **`auth`**：`http_user` / `http_pass` 为空则 `/pub` 不校验认证。
- **`web`**：`listen`、`title`、`members`（非空时仅展示所列用户）、`static_dir`（非空则以外部目录替代内置静态文件）。
- **`clickhouse`**：`dsn` 与 `host` 等二选一；须能成功连接，否则无法启动。

## 开发与构建

```bash
# 编译
go build -o owntracks_server ./cmd/server

# 若修改了 internal/di/wire.go，重新生成依赖注入
task wire
# 或: go run github.com/google/wire/cmd/wire@latest ./internal/di
```

交叉编译 Linux 二进制（见 `Taskfile.yml`）：

```bash
task build
```

## 部署

- **systemd**：参考 `deploy/owntracks-server.service`。默认工作目录 `/opt/owntracks`，可配合 `/etc/default/owntracks-server` 设置环境变量。
- **Task**：`task deploy -- <host>` 推送二进制；`task install-service -- <host>` 安装并启用单元（需已配置 SSH）。

## 仓库结构（节选）

```
cmd/server/          # 入口与 Cobra 子命令
internal/webapp/     # HTTP 路由、静态资源、上报处理
internal/store/      # ClickHouse 存储
internal/owntracks/  # Topic、消息解析
configs/             # 默认 YAML 与 ClickHouse 建表 SQL
deploy/              # systemd 单元示例
```

## 许可证

若未在仓库中另行声明，请以项目根目录许可证文件为准。
