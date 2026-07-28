# mqbot

一个用于学习 Go 和 MQTT 实践的小项目。通过模拟"控制中心 ↔ 机器人"的通信场景，练习 MQTT 通信、消息协议设计、并发控制等内容。

## 项目结构

```
mqbot/
├── cmd/
│   ├── bothub/      # 控制中心：订阅机器人状态，提供 Web 接口
│   └── robot/       # 机器人客户端：上报状态，接收并执行移动任务
├── internal/
│   ├── mqtt/        # MQTT 客户端封装（连接、订阅、重试）
│   └── robot/       # 机器人移动逻辑
├── protocol/        # 消息协议定义（Header / Status / Task / Command）
├── go.mod
└── go.sum
```

## 运行方式

需要先启动一个 MQTT Broker（如 mosquitto），默认监听 `127.0.0.1:1883`。

启动控制中心：

```bash
go run ./cmd/bothub
```

启动机器人（可指定参数）：

```bash
go run ./cmd/robot -server 127.0.0.1 -port 1883 -id bot_0001
```

## 通信主题

| Topic                  | 方向     | 说明                     |
| ---------------------- | -------- | ------------------------ |
| `robot/{id}/status`    | 机器人 → 中心 | 机器人定时上报位置/状态 |
| `robot/{id}/task`      | 中心 → 机器人 | 下发任务（如 move_to）  |
| `robot/{id}/command`   | 中心 → 机器人 | 下发实时指令            |

## 主要依赖

- [paho.golang](https://github.com/eclipse/paho.golang) — MQTT v5 客户端
- [gin](https://github.com/gin-gonic/gin) — Web 框架（控制中心侧）

## 学习要点

- MQTT 连接、订阅、发布流程
- 消息协议设计（统一 Header + Body 的信封结构）
- Go 并发：ticker、channel、读写锁
- 命令行参数解析、信号处理（优雅退出）
