# TokenFlow 部署文档

更新时间：2026-08-17

## 核对结果

- 应用默认监听地址：`:8019`，来自 `GATEWAY_ADDR`。
- 应用默认数据目录：`data`，来自 `GATEWAY_DATA_DIR`。
- 数据库文件：`$GATEWAY_DATA_DIR/gateway.db`。
- 加密密钥文件：`$GATEWAY_DATA_DIR/app.secret`。这个文件必须和 `gateway.db` 一起保留，否则已保存的上游供应商 API Key 将无法解密。
- 当前版本启动时会自动迁移数据库。除 Token 成本报表外，还会新增兼容旧版 APK 的 `mobile_sessions` 表及相关索引；不会修改 `request_logs` 字段，也不会改变用户 Token 配额逻辑。
- 健康检查：`GET /healthz`，正常返回 `{"ok":true}`。
- 当前部署容器名：`anthropic_api`。
- 当前部署方式：本地编译 Linux 二进制，上传到服务器项目目录，然后重启容器。
- Nginx 对外入口：`https://token.030399.xyz:9443`，反向代理到本服务。
- 注意：`tokenflow.030399.xyz` 当前无法解析，生产验证请使用 `token.030399.xyz`。

## 服务器登录信息

本节只记录非秘密连接信息。服务器密码不写入仓库；需要应急密码登录时，从团队密码管理器获取。日常部署使用本地 SSH 私钥。

```text
SSH Host: g.030399.xyz
SSH Port: 65432
Username: root
SSH Key Path (本地): ~/.ssh/LotusSSL
Project Path: /root/Mek/anthropic
Container Name: anthropic_api
Public URL: https://token.030399.xyz:9443
```

登录命令：

```bash
# 使用证书登录（推荐）
ssh -i <SSH_KEY_PATH> -p 65432 root@g.030399.xyz
```

## 本地编译 Linux 版本

先运行测试：

```bash
go test ./...
```

在 Windows PowerShell 编译 Linux amd64 二进制：

```powershell
$env:GOOS='linux'
$env:GOARCH='amd64'
$env:CGO_ENABLED='0'
go build -o tokenflow-linux-amd64 ./cmd/server
```

编译完成后，本地应生成：

```text
tokenflow-linux-amd64
```

## 上传二进制

从本地项目根目录执行。`<SSH_KEY_PATH>` 替换为本地私钥路径 `~/.ssh/LotusSSL`（Windows PowerShell 通常写作 `$HOME\.ssh\LotusSSL`）。

上传到 `/tmp`（避免目标文件被容器占用导致覆盖失败）：

```bash
scp -i <SSH_KEY_PATH> -P 65432 -o StrictHostKeyChecking=accept-new tokenflow-linux-amd64 root@g.030399.xyz:/tmp/tokenflow-linux-amd64
```

## 服务器上安装并重启容器

### 更新前数据库注意事项

本版本包含数据库迁移。生产更新前必须先备份 `data` 目录，尤其是 `data/gateway.db` 和 `data/app.secret`：

```bash
cd /root/Mek/anthropic
mkdir -p backups
tar -czf "backups/tokenflow-data-before-upgrade-$(date +%Y%m%d-%H%M%S).tgz" data
```

迁移行为：

- 新增表：`provider_model_prices`，用于保存每个上游供应商模型的输入、输出、缓存读取、缓存写入价格。
- 新增表：`mobile_sessions`，用于兼容旧版原生 APK 的 Bearer 会话；数据库只保存 SHA-256 令牌哈希。
- 新增索引：`idx_request_logs_provider_model`，用于按 `provider_id + upstream_model` 计算成本报表。
- 新增索引：`idx_mobile_sessions_consumer` 和 `idx_mobile_sessions_expires_at`，用于账号撤销和会话过期清理。
- 不新增或修改 `request_logs` 字段，历史请求成本按当前模型价格实时重算。
- 不改变用户 Token 配额、分发 Key、请求日志和上游 API Key 加密逻辑。

首次启动新版本时会自动建表和建索引。如果 `request_logs` 数据量较大，建索引可能需要一些时间，建议在低峰期更新，并先停止容器再替换二进制。

### 方式一：一键部署（推荐）

在本地一条命令完成替换和重启。`<SSH_KEY_PATH>` 替换为本地证书路径：

```bash
ssh -i <SSH_KEY_PATH> -p 65432 -o StrictHostKeyChecking=accept-new root@g.030399.xyz \
  "docker stop anthropic_api && \
   cp /tmp/tokenflow-linux-amd64 /root/Mek/anthropic/tokenflow-linux-amd64 && \
   chmod +x /root/Mek/anthropic/tokenflow-linux-amd64 && \
   cp /root/Mek/anthropic/tokenflow-linux-amd64 /root/Mek/anthropic/tokenflow && \
   chmod +x /root/Mek/anthropic/tokenflow && \
   docker start anthropic_api"
```

> **重要**：必须先 `docker stop` 容器再替换二进制文件，否则会报 `Text file busy` 错误（文件被运行中的容器占用）。

### 方式二：手动操作

登录服务器：

```bash
ssh -i <SSH_KEY_PATH> -p 65432 root@g.030399.xyz
```

进入项目目录并确认文件：

```bash
cd /root/Mek/anthropic
ls -lh /tmp/tokenflow-linux-amd64
```

停止容器、替换二进制、重新启动：

```bash
docker stop anthropic_api
cp /tmp/tokenflow-linux-amd64 tokenflow-linux-amd64
chmod +x tokenflow-linux-amd64
cp tokenflow-linux-amd64 tokenflow
chmod +x tokenflow
docker start anthropic_api
```

查看容器状态和最近日志：

```bash
docker ps --filter name=anthropic_api
docker logs --tail=100 anthropic_api
```

## 部署后验证

在容器内验证健康检查：

```bash
docker exec anthropic_api wget -qO- http://127.0.0.1:8019/healthz
```

正常输出：

```json
{"ok":true}
```

通过 Nginx 对外入口验证：

```bash
curl -fsS https://token.030399.xyz:9443/healthz
```

验证兼容旧版 APK 的移动路由已挂载（不携带 Bearer Token 时应返回结构化 `401`）：

```bash
curl -i https://token.030399.xyz:9443/mobile/v1/session
```

验证数据库迁移和成本报表：

```bash
ssh -i <SSH_KEY_PATH> -p 65432 root@g.030399.xyz
cd /root/Mek/anthropic
docker exec anthropic_api sh -lc 'ls -lh data/gateway.db data/app.secret'
docker logs --tail=100 anthropic_api
```

登录管理后台后检查：

- 打开 `https://token.030399.xyz:9443/admin`。
- Provider 列表中应出现模型价格入口。
- 首次升级后模型价格默认为未配置，成本会显示 `$0.00` 或 `Unpriced`。
- 在 Provider 的价格表中配置各模型 USD/1M Token 单价后，历史请求成本会按当前价格实时重算显示。

常用页面：

- 首页：`https://token.030399.xyz:9443/`
- 管理员后台：`https://token.030399.xyz:9443/admin`
- 管理员聊天：`https://token.030399.xyz:9443/admin/chat`
- 普通用户页面：`https://token.030399.xyz:9443/account`
- 普通用户聊天：`https://token.030399.xyz:9443/account/chat`

对外 API：

- OpenAI Chat Completions：`POST /v1/chat/completions`
- OpenAI Models：`GET /v1/models`
- Anthropic Messages：`POST /v1/messages`
- 兼容旧 Anthropic 路径：`POST /anthropic/v1/messages`
- Anthropic Models：`GET /anthropic/v1/models`

## 数据备份

部署前建议备份数据目录：

```bash
cd /root/Mek/anthropic
mkdir -p backups
tar -czf "backups/tokenflow-data-$(date +%Y%m%d-%H%M%S).tgz" data
```

恢复时必须确认 `data/app.secret` 与 `data/gateway.db` 来自同一份备份。

## 故障排查

查看容器是否运行：

```bash
docker ps --filter name=anthropic_api
```

查看监听端口：

```bash
ss -lntp | grep 8019
```

查看日志：

```bash
docker logs --tail=200 anthropic_api
```

检查数据文件：

```bash
cd /root/Mek/anthropic
ls -la data
```

如果启动时报 `load app secret`、`open database` 或上游 API Key 解密失败，优先检查 `GATEWAY_DATA_DIR`、`data/app.secret` 和 `data/gateway.db` 是否来自同一部署目录。
