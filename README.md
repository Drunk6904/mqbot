# mqbot

一个用于学习 Go 和 MQTT 实践的小项目。通过模拟"控制中心 ↔ 机器人"的通信场景，练习 MQTT 通信、消息协议设计、WebSocket 实时推送、并发控制等内容。

## 项目结构

```
mqbot/
├── cmd/
│   ├── bothub/            # 控制中心
│   │   ├── main.go        # 入口：启动 MQTT + HTTP 服务
│   │   └── static/        # 前端页面（单 HTML 文件）
│   │       └── index.html
│   └── robot/             # 机器人客户端
│       └── main.go        # 入口：上报状态、执行移动任务
├── internal/
│   ├── http/              # HTTP + WebSocket 服务封装
│   │   └── server.go      # 路由、WS 连接池、MQTT 桥接
│   ├── mqtt/              # MQTT 客户端封装（连接、订阅、重试）
│   │   └── client.go
│   └── robot/             # 机器人移动逻辑
│       └── move.go
├── protocol/              # 消息协议定义
│   ├── message.go         # Header / Status / Task / Command
│   └── topic.go           # Topic 格式常量
├── go.mod
└── go.sum
```

## 架构

```
┌─────────────┐   MQTT    ┌───────────────┐  WebSocket  ┌────────────┐
│   Robot     │ ────────► │   Bothub       │ ──────────► │  浏览器     │
│ (cmd/robot) │  status   │  (cmd/bothub)  │   推送状态   │ (前端页面)  │
│             │ ◄──────── │                │ ◄────────── │            │
│             │   task    │                │  下发任务    │            │
└─────────────┘           └───────────────┘             └────────────┘
```

- Robot 每秒通过 MQTT 上报状态（位置、电量、状态）
- Bothub 订阅 `robot/+/status`，收到后通过 WebSocket 推给所有浏览器
- 浏览器在页面上发送任务，Bothub 通过 MQTT 下发给对应 Robot

## 运行方式

需要先启动一个 MQTT Broker（如 mosquitto），默认监听 `127.0.0.1:1883`。

启动控制中心：

```bash
go run ./cmd/bothub
```

控制中心启动后访问 `http://localhost:8080` 即可看到监控页面。

启动机器人（可指定参数，可启动多个）：

```bash
go run ./cmd/robot -server 127.0.0.1 -port 1883 -id bot_0001
```

| 参数       | 默认值     | 说明                    |
| ---------- | ---------- | ----------------------- |
| `-server`  | localhost  | MQTT Broker 地址        |
| `-port`    | 1883       | MQTT Broker 端口        |
| `-id`      | 随机生成   | 机器人 ID               |
| `-username`| 空         | MQTT 用户名             |
| `-password`| 空         | MQTT 密码               |

## 通信主题

| Topic                  | 方向           | 说明                     |
| ---------------------- | -------------- | ------------------------ |
| `robot/{id}/status`    | 机器人 → 中心  | 定时上报位置/状态        |
| `robot/{id}/task`      | 中心 → 机器人  | 下发任务（如 move_to）   |
| `robot/{id}/command`   | 中心 → 机器人  | 下发实时指令             |

## 消息格式

所有消息共享统一信封结构：

```json
{
  "header": { "ver": "1.0", "msg_id": "uuid", "ts": 1234567890 },
  "body": { ... }
}
```

状态上报 body（`StatusBody`）：

```json
{ "id": "bot_0001", "x": 1.5, "y": 2.3, "battery": 80, "state": "IDLE", "speed": 1 }
```

任务下发 body（`TaskBody`）：

```json
{ "task_id": "uuid", "action": "move_to", "params": { "x": "5.0", "y": "3.0" }, "priority": "NORMAL" }
```

## 主要依赖

- [paho.golang](https://github.com/eclipse/paho.golang) - MQTT v5 客户端
- [gin](https://github.com/gin-gonic/gin) - Web 框架（控制中心侧）
- [gorilla/websocket](https://github.com/gorilla/websocket) - WebSocket 支持

## 学习要点

- MQTT 连接、订阅、发布流程
- 消息协议设计（统一 Header + Body 的信封结构）
- WebSocket 实时通信与连接池管理
- MQTT ↔ WebSocket 双向桥接
- Go 并发：ticker、channel、读写锁、sync.Mutex
- Canvas 绘图（坐标系、机器人位置可视化）
- 命令行参数解析、信号处理（优雅退出）
