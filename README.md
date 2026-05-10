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

```bash
docker compose up -d --build
```

没有域名时，快捷指令可以直接请求：

```text
http://服务器公网IP:18080/api/messages
```

如果没有 HTTPS，验证码内容会经过公网明文传输。个人临时使用可以先跑通，长期使用建议配置域名和 HTTPS，或者后续增加消息体加密。
