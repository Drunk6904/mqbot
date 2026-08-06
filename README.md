# mqbot

一个用于学习 Go 和 MQTT 实践的小项目。通过模拟"控制中心 ↔ 机器人"的通信场景，练习 MQTT 通信、消息协议设计、WebSocket 实时推送、并发控制等内容。

## 已实现特性

| 特性 | 说明 |
|------|------|
| MQTT 通信 | 支持 MQTT v5，带连接重试、遗嘱消息 |
| 状态上报 | 增量上报（仅状态变化时发送，节省流量） |
| 任务下发 | 移动任务、实时停止、速度调节 |
| 自动充电 | 电量低于20%自动充电，充到90%停止 |
| WebSocket | 实时推送到浏览器，支持多客户端连接 |
| Web 控制台 | 单页应用，Canvas 可视化机器人位置 |

## 项目结构

```
mqbot/
├── cmd/
│   ├── bothub/            # 控制中心服务
│   │   └── main.go        # 入口：启动 MQTT + HTTP/WebSocket 服务
│   └── robot/             # 机器人客户端
│       └── main.go        # 入口：状态上报、任务执行、自动充电
├── internal/
│   ├── http/              # HTTP + WebSocket 服务封装
│   │   ├── server.go      # 服务启动与初始化
│   │   ├── routes.go      # 路由注册
│   │   └── websocket.go   # WS 连接池、MQTT 消息桥接
│   ├── mqtt/              # MQTT 客户端封装
│   │   └── client.go      # 连接重试、订阅重试、遗嘱消息
│   └── robot/             # 机器人业务逻辑
│       └── move.go        # 移动算法（带取消支持）
├── protocol/              # 消息协议定义（与实现无关）
│   ├── message.go         # 统一信封结构、消息类型、工具函数
│   └── topic.go           # MQTT Topic 格式常量
├── static/                # Web 前端资源
│   └── index.html         # 控制台页面（Canvas 可视化）
├── go.mod
└── go.sum
```

## 架构

```
┌─────────────┐   MQTT    ┌───────────────┐  WebSocket  ┌────────────┐
│   Robot     │ ────────► │   Bothub      │ ──────────► │  浏览器     │
│ (cmd/robot) │  status   │ (cmd/bothub)  │   推送状态   │ (前端页面)  │
│             │ ◄──────── │               │ ◄────────── │            │
│             │   task    │               │  下发任务    │            │
│             │  command  │               │              │            │
└─────────────┘           └───────────────┘             └────────────┘
```

- **Robot**：通过 MQTT 上报状态（位置、电量、状态），支持任务取消和实时指令
- **Bothub**：订阅所有机器人状态，通过 WebSocket 广播；接收 Web 指令转发到 MQTT
- **浏览器**：Canvas 实时绘制机器人位置，发送移动/停止/调速指令

## 运行方式

### 前置条件

需要先启动一个 MQTT Broker（推荐 [mosquitto](https://mosquitto.org/)），默认监听 `127.0.0.1:1883`。

### 启动控制中心

```bash
go run ./cmd/bothub
```

控制中心启动后访问 `http://localhost:8080` 即可看到监控页面。

### 启动机器人

可启动多个机器人实例，每个会在页面上独立显示：

```bash
go run ./cmd/robot -id bot_0001
```

**命令行参数：**

| 参数       | 默认值     | 说明                    |
| ---------- | ---------- | ----------------------- |
| `-server`  | localhost  | MQTT Broker 地址        |
| `-port`    | 1883       | MQTT Broker 端口        |
| `-id`      | 随机生成   | 机器人 ID               |
| `-speed`   | 1.0        | 移动速度（像素/秒）     |
| `-username`| 空         | MQTT 用户名（如需认证） |
| `-password`| 空         | MQTT 密码               |

### 完整示例

```bash
# 终端 1：启动控制中心
go run ./cmd/bothub

# 终端 2：启动机器人 1
go run ./cmd/robot -id bot_001

# 终端 3：启动机器人 2
go run ./cmd/robot -id bot_002 -speed 1.5
```

访问 `http://localhost:8080`，点击画布即可控制机器人移动。

## 通信协议

### MQTT 主题

| Topic                  | QoS | 方向           | 说明                     |
| ---------------------- | --- | -------------- | ------------------------ |
| `robot/{id}/status`    | 0   | 机器人 → 中心  | 状态上报（遗嘱使用QoS=1）|
| `robot/{id}/task`      | 2   | 中心 → 机器人  | 下发任务（异步）         |
| `robot/{id}/command`   | 2   | 中心 → 机器人  | 下发实时指令（同步）     |

### 消息信封格式

所有消息共享统一信封结构，便于版本兼容和链路追踪：

```json
{
  "header": {
    "ver": "1.0",      // 协议版本
    "msg_id": "uuid",  // 消息唯一ID
    "ts": 1234567890   // 发送时间戳（秒）
  },
  "body": { ... }      // 业务数据
}
```

### 状态上报（StatusBody）

```json
{
  "id": "bot_0001",
  "x": 1.5,
  "y": 2.3,
  "battery": 80,
  "state": "MOVING",    // IDLE / MOVING / CHARGING / ERROR / OFFLINE
  "speed": 1.0,
  "task_id": "uuid"     // 当前执行的任务，空闲时省略
}
```

**上报策略**：并非定时上报，只有以下情况才发送：
- 电量变化超过 1%
- 状态机状态变更
- 位置变化超过 0.01
- 速度变化超过 0.1

### 任务下发（TaskBody）

```json
{
  "task_id": "uuid",
  "action": "move_to",
  "params": { "x": "5.0", "y": "3.0" },
  "priority": "NORMAL"   // HIGH / NORMAL / LOW
}
```

### 实时指令（CommandBody）

```json
{
  "action": "stop",      // stop / set_speed
  "params": { "speed": 2.0 }
}
```

> 💡 指令与任务的区别：任务有ID可追踪，指令无ID即时生效

## 📦 主要依赖

| 库 | 用途 |
|----|------|
| [eclipse/paho.golang](https://github.com/eclipse/paho.golang) | MQTT v5 客户端 |
| [gin-gonic/gin](https://github.com/gin-gonic/gin) | Web 框架 |
| [gorilla/websocket](https://github.com/gorilla/websocket) | WebSocket 支持 |
| [google/uuid](https://github.com/google/uuid) | UUID 生成 |

## 📚 学习要点

这个项目覆盖了以下 Go 和分布式系统知识点：

### MQTT 协议
- MQTT v5 连接流程与参数配置
- 遗嘱消息（Will Message）实现离线检测
- QoS 级别选择与适用场景
- 订阅/发布的重试机制设计

### 并发编程
- `time.Ticker` 定时任务
- `context.WithCancel` 任务取消
- `sync.Mutex` 共享资源保护
- Channel 用于 goroutine 通信
- WebSocket 连接池管理

### 协议设计
- 统一信封结构的扩展性考虑
- 增量上报的流量优化策略
- 类型安全的参数读取工具
- 状态机设计与转换

### 工程实践
- 清晰的分层架构（protocol → internal → cmd）
- Go 标准项目布局
- 错误处理与日志
- 信号处理与优雅退出
