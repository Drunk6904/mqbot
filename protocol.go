package protocol

import (
	"time"

	"github.com/google/uuid"
)

// ============ 通用信封 Header ============

// Header 所有 MQTT 消息共享的元数据
type Header struct {
	Ver   string `json:"ver"`    // 协议版本，便于以后升级兼容
	MsgID string `json:"msg_id"` // 消息唯一ID，用于去重/链路追踪
	TS    int64  `json:"ts"`     // 发送时间戳(秒)
}

// newHeader 自动生成 header，调用方无需关心
func newHeader() Header {
	return Header{
		Ver:   "1.0",
		MsgID: uuid.New().String(),
		TS:    time.Now().Unix(),
	}
}

// ============ 状态常量（协议层用字符串，跨语言友好）============

const (
	StatusTopic  = "robot/%s/status"
	TaskTopic    = "robot/%s/task"
	CommandTopic = "robot/%s/command"
)

const (
	StateIdle     = "IDLE"
	StateMoving   = "MOVING"
	StateCharging = "CHARGING"
	StateError    = "ERROR"
	StateOffline  = "OFFLINE"

	PriorityHigh   = "HIGH"
	PriorityNormal = "NORMAL"
	PriorityLow    = "LOW"

	// 常见 action 常量，避免拼写错误
	ActionMoveTo   = "move_to"
	ActionCharge   = "charge"
	ActionStop     = "stop"
	ActionResume   = "resume"
	ActionSetSpeed = "set_speed"
)

// ============ 上行：状态上报 ============

// StatusBody 机器人上报的业务数据
type StatusBody struct {
	ID      string  `json:"id"`
	X       float64 `json:"x"`
	Y       float64 `json:"y"`
	Battery int     `json:"battery"`
	State   string  `json:"state"`
	Speed   float64 `json:"speed"`
	TaskID  string  `json:"task_id,omitempty"` // 当前执行的任务ID，空则省略
}

// StatusMessage 完整的状态消息信封
type StatusMessage struct {
	Header Header     `json:"header"`
	Body   StatusBody `json:"body"`
}

func NewStatusMessage(body StatusBody) StatusMessage {
	return StatusMessage{Header: newHeader(), Body: body}
}

// ============ 下行：任务下发 ============

// TaskBody 任务的业务数据
type TaskBody struct {
	TaskID   string                 `json:"task_id"`
	Action   string                 `json:"action"`
	Params   map[string]interface{} `json:"params,omitempty"`
	Priority string                 `json:"priority"`
}

type TaskMessage struct {
	Header Header   `json:"header"`
	Body   TaskBody `json:"body"`
}

func NewTaskMessage(body TaskBody) TaskMessage {
	if body.TaskID == "" {
		body.TaskID = uuid.New().String() // 没给ID就自动生成
	}
	if body.Priority == "" {
		body.Priority = PriorityNormal
	}
	return TaskMessage{Header: newHeader(), Body: body}
}

// ============ 下行：实时指令 ============

// CommandBody 指令的业务数据
type CommandBody struct {
	Action string                 `json:"action"`
	Params map[string]interface{} `json:"params,omitempty"`
}

type CommandMessage struct {
	Header Header      `json:"header"`
	Body   CommandBody `json:"body"`
}

func NewCommandMessage(body CommandBody) CommandMessage {
	return CommandMessage{Header: newHeader(), Body: body}
}

// ============ Params 取值工具（避免到处写类型断言）============

// FloatParam 安全地从 params 取浮点数，取不到返回默认值
func FloatParam(params map[string]interface{}, key string, def float64) float64 {
	if params == nil {
		return def
	}
	if v, ok := params[key].(float64); ok { // JSON 数字默认解成 float64
		return v
	}
	return def
}

// StringParam 安全地从 params 取字符串
func StringParam(params map[string]interface{}, key string, def string) string {
	if params == nil {
		return def
	}
	if v, ok := params[key].(string); ok {
		return v
	}
	return def
}
