# owntracks_server

基于 [OwnTracks](https://owntracks.org/) 的 HTTP 接收服务，并带网页控制台查看轨迹与统计。手机端使用 **HTTP** 模式上报位置；数据保存在 **ClickHouse**，用浏览器打开服务地址即可使用控制台。

## 使用前准备

- 已安装并可用 **ClickHouse**；服务启动时会检查连接，失败则无法运行。
- 先在 ClickHouse 中准备好库表：使用仓库里的 `resource/init.sql`（可通过自带的 `ck` 子命令按文件执行，详见程序帮助）。
- 配置好 `configs/config.yaml`中的 ClickHouse 连接，或使用环境变量 `CLICKHOUSE_DSN` 覆盖。

## 启动服务

使用已编译好的 `owntracks_server`（或你部署路径下的同名程序），启动 **Web** 模式即可同时接收手机端上报并打开网页控制台。默认读取工作目录下的 `configs/config.yaml`，默认监听地址以配置为准（常见为 `:8080`）。

常用启动参数（以程序 `--help` 为准）：

- `--config`：配置文件路径。
- `--listen`：监听地址（也可通过环境变量 `WEB_ADDR` 覆盖）。

子命令说明：

- `web` 或 `server`：启动服务（手机端上报 + 控制台）。
- `ck`：在 ClickHouse 上依次执行指定 SQL 文件中的语句（初始化或迁移时使用）。

## OwnTracks 手机端设置

1. 模式选择 **HTTP**。
2. **URL** 示例（将主机、端口、用户名、设备名换成你的）：
   - `https://你的主机:端口/pub/用户名/设备名`
   - 或使用带 `topic` 查询参数的地址（与 OwnTracks 文档一致）。
3. 若在配置里开启了 HTTP 基本认证，请在 App 中填写对应的用户名和密码。

## 网页控制台

启动服务后，在浏览器访问服务根地址（例如 `http://主机:端口/`），即可查看轨迹与统计。若配置里设置了 `web.members`，控制台可能只显示所列用户的数据。

## 配置与环境变量（摘要）

配置文件默认为 `configs/config.yaml`，主要关注：

- **clickhouse**：数据库连接（必填）。
- **auth**：HTTP 上报是否需要 Basic 认证；留空则不校验。
- **web**：监听地址、页面标题、成员过滤、静态资源目录等。
- **log.file**：日志文件路径；也可通过 `LOG_FILE` 指定，或设为仅输出到终端。

环境变量（会覆盖部分配置）：

| 变量 | 作用 |
|------|------|
| `WEB_ADDR` | 监听地址 |
| `HTTP_USER` / `HTTP_PASS` | HTTP 基本认证 |
| `CLICKHOUSE_DSN` | ClickHouse 连接串 |
| `LOG_FILE` | 日志文件；`-` 或 `none` 表示仅标准输出 |

## 部署提示

生产环境可使用 systemd 等方式托管进程。仓库中提供示例单元文件 `deploy/owntracks-server.service`，常与工作目录 `/opt/owntracks` 及环境文件配合使用；具体以你机器上的路径与运维规范为准。

## 许可证

以项目根目录许可证文件为准。
