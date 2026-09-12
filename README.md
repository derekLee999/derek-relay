# derek-relay

`derek-relay` 是验证码接收器的云端中转服务。iPhone 快捷指令把短信内容 POST 到云服务器，Windows 客户端主动轮询云端并接收消息，因此 Windows 不需要公网 IP，也不需要开放家庭路由器端口。

## 接口

- `GET /api/health`：健康检查。
- `POST /api/messages`：iPhone 快捷指令发送消息。
- `GET /api/poll`：Windows 客户端长轮询接收消息。
- `GET /api/ws`：WebSocket 推送接口，预留给后续客户端使用。

## 本地运行

```powershell
$env:RELAY_SECRET='change-me-to-a-long-random-secret'
go run ./cmd/derek-relay -listen :18080
```

## iPhone POST 示例

```json
{
  "text": "您的验证码是 123456，5 分钟内有效",
  "id": "1234567",
  "secret": "change-me-to-a-long-random-secret"
}
```

也可以把密钥放到请求头：

```text
Authorization: Bearer change-me-to-a-long-random-secret
```

## Docker 部署

### 1. 准备服务器

在一台全新的 Debian/Ubuntu 服务器上安装 Docker 和 Compose 插件：

```bash
apt update
apt install -y docker.io docker-compose-plugin
systemctl enable --now docker
```

### 2. 放置项目代码

把项目代码放到服务器目录，例如：

```bash
/opt/derek-relay
```

目录中至少需要包含：

```text
Dockerfile
docker-compose.yml
go.mod
cmd/
internal/
```

### 3. 生成 `.env`

`RELAY_SECRET` 是 iPhone 快捷指令、客户端和云端服务共同使用的共享密钥。每台新服务器可以重新生成一个：

```bash
cd /opt/derek-relay
umask 077
printf "RELAY_SECRET=%s\n" "$(openssl rand -hex 32)" > .env
```

如果是迁移旧服务，并希望旧客户端继续可用，需要把旧服务器上的 `RELAY_SECRET` 保持一致。

### 4. 启动服务

```bash
cd /opt/derek-relay
docker compose up -d --build
docker compose ps
```

本机健康检查：

```bash
curl http://127.0.0.1:18080/api/health
```

正常会返回：

```json
{"name":"derek-relay","ok":true}
```

18080 只监听回环是有意为之:Docker 发布的端口会绕过 ufw/iptables 的 INPUT 策略直接暴露到公网,而这个端口是明文 HTTP,共享密钥会在公网裸奔。公网入口统一交给 Caddy 的 443。若确有从公网直连 18080 的需求,请先自行解决 TLS 与访问控制,不要直接改回 `0.0.0.0` 绑定。

### 没有域名时

`docker-compose.yml` 默认只把 18080 发布到回环地址,公网无法直连这个端口,因此**不能**再用 `http://服务器公网IP:18080/api/messages` 这类地址。iPhone 快捷指令在公网侧,没有域名就没有安全可达的入口,请先按下一节配置域名和 HTTPS。

本地联调可以用 SSH 隧道临时打通,仅适合从你自己电脑发起的请求(健康检查、轮询):

```bash
ssh -L 18080:127.0.0.1:18080 user@服务器公网IP
curl http://127.0.0.1:18080/api/health
```

## Caddy 反向代理与 HTTPS

如果服务器上已经安装 Caddy，可以让 Caddy 反向代理到本服务，并自动申请 HTTPS 证书。

前提：

- 域名的 A/AAAA 记录已经指向这台服务器，或 CDN/边缘服务已经正确回源到这台服务器。
- 服务器的 `80` 和 `443` 端口可被公网访问。
- `derek-relay` 已经在本机 `127.0.0.1:18080` 正常运行。

Caddyfile 示例：

```caddyfile
relay.example.com {
	reverse_proxy 127.0.0.1:18080
}
```

检查并加载配置：

```bash
caddy validate --config /etc/caddy/Caddyfile --adapter caddyfile
caddy reload --config /etc/caddy/Caddyfile --adapter caddyfile
```

验证 HTTPS：

```bash
curl https://relay.example.com/api/health
```

正常会返回：

```json
{"name":"derek-relay","ok":true}
```

如果使用 Docker 或 1Panel 管理 Caddy，请修改对应挂载出来的 Caddyfile，然后在 Caddy 容器内执行 `caddy validate` 和 `caddy reload`。

## 迁移到新服务器

迁移时只需要带走项目代码和 `.env`：

```text
/opt/derek-relay/
  Dockerfile
  docker-compose.yml
  go.mod
  cmd/
  internal/
  .env
```

如果不复制旧服务器的 `.env`，就按上面的步骤重新生成 `RELAY_SECRET`，并同步更新 iPhone 快捷指令和客户端配置。

Caddy 证书通常不需要迁移。只要域名解析或回源指向新服务器，Caddy 会自动重新申请证书。
