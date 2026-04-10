---
name: deploy
description: >-
  On /deploy: build owntracks_server and install ONLY under /opt/owntracks (never /opt/owntracks_server),
  restart owntracks-server.service. Binary path /opt/owntracks/owntracks_server; configs typically /opt/owntracks/configs.
---

# 部署（`/deploy`）→ `/opt/owntracks`

## 目标

用户输入 **`/deploy`** 或要求「部署 / 安装到生产目录」时：在仓库根目录构建，将二进制安装到 **`/opt/owntracks`**，并重启 **`owntracks-server.service`**。

**约定目录仅此一处**：不要使用 **`/opt/owntracks_server`**（旧说明已废弃）。若 systemd 单元里仍是其他路径，应先改 **`WorkingDirectory`** / **`ExecStart`** 指向 `/opt/owntracks` 后再部署。

## 约定

| 项 | 值 |
|----|-----|
| 安装目录 | `/opt/owntracks` |
| 二进制 | `/opt/owntracks/owntracks_server` |
| 配置（本机惯例） | `/opt/owntracks/configs`（如 `config.yaml`） |
| `WorkingDirectory` | `/opt/owntracks`（与示例单元一致） |
| systemd 单元 | `owntracks-server.service` |

示例单元：`deploy/owntracks-server.service`。

## 推荐流程（本机 / SSH 目标机）

1. 在**仓库根目录**构建（与 `Taskfile.yml` 一致）：

   ```bash
   task build
   ```

   若无 `task`：

   ```bash
   GOOS=linux GOARCH=amd64 CGO_ENABLED=0 go build -ldflags="-s -w" -trimpath -o owntracks_server ./cmd/server
   ```

2. 停止服务（若已安装单元）、替换二进制、再启动：

   ```bash
   sudo mkdir -p /opt/owntracks
   if [ -f /etc/systemd/system/owntracks-server.service ]; then sudo systemctl stop owntracks-server.service; fi
   sudo install -m 755 owntracks_server /opt/owntracks/owntracks_server
   if [ -f /etc/systemd/system/owntracks-server.service ]; then sudo systemctl start owntracks-server.service; fi
   ```

   一条命令（默认 **`INSTALL_DIR=/opt/owntracks`**）：

   ```bash
   bash .cursor/skills/deploy/scripts/install-to-opt.sh
   ```

3. 检查：

   ```bash
   systemctl status owntracks-server.service --no-pager
   ```

## 对 Agent 的指示

- **`/deploy`**：执行 `task build`（或等价 `go build`），再运行 **`install-to-opt.sh`** 或等价 `install` + `systemctl`，目标路径**仅限** `/opt/owntracks/owntracks_server`。
- 需要覆盖目录时：`INSTALL_DIR=/somewhere bash .../install-to-opt.sh`；默认勿改脚本里的 `/opt/owntracks` 除非用户明确要求其他路径。
- 远程可用 `task deploy -- <host>`（仓库内 `REMOTE_DIR` 应对齐 `/opt/owntracks`）。
