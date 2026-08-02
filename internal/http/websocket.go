package http

import (
	"context"
	"encoding/json"
	"fmt"
	"log"
	"net/http"
	"time"

	"github.com/Drunk6904/mqbot/protocol"
	"github.com/eclipse/paho.golang/paho"
	"github.com/gin-gonic/gin"
	"github.com/gorilla/websocket"
)

var upgrader = websocket.Upgrader{
	CheckOrigin: func(r *http.Request) bool { return true }, // 开发阶段允许所有来源
}

// 处理WebSocket连接
func (s *Server) handleWS(ctx *gin.Context) {
	// 升级为 WebSocket
	conn, err := upgrader.Upgrade(ctx.Writer, ctx.Request, nil)
	if err != nil {
		log.Printf("WS 升级失败: %v", err)
		return
	}
	defer conn.Close()

	// 加入连接池
	s.addWSConn(conn)
	defer s.removeWSConn(conn)

	// 循环读取浏览器发来的消息
	for {
		_, msg, err := conn.ReadMessage()
		if err != nil {
			break // 连接断开
		}
		// 解析并转发到 MQTT
		s.handleWSMessage(msg)
	}
}

func (s *Server) addWSConn(conn *websocket.Conn) {
	s.mu.Lock()
	defer s.mu.Unlock()
	s.wsConns[conn] = true
}

func (s *Server) removeWSConn(conn *websocket.Conn) {
	s.mu.Lock()
	defer s.mu.Unlock()
	delete(s.wsConns, conn)
}

func (s *Server) handleWSMessage(msg []byte) {
	// 解析消息
	msgStr := string(msg)
	log.Printf("收到消息: %s", msgStr)

	var msgBody struct {
		BotID  string         `json:"bot_id"`
		Action string         `json:"action"`
		Params map[string]any `json:"params"`
	}
	err := json.Unmarshal(msg, &msgBody)
	if err != nil {
		log.Printf("解析消息失败: %v", err)
		return
	}
	log.Printf("解析后的消息: %v", msgBody)

	// 转发到 MQTT
	if s.MqttClient == nil {
		log.Printf("MQTT 客户端未初始化")
		return
	}
	// 发布消息到 MQTT
	topic := fmt.Sprintf(protocol.TaskTopic, msgBody.BotID)
	payload, err := json.Marshal(protocol.NewTaskMessage(protocol.TaskBody{
		Action: msgBody.Action,
		Params: msgBody.Params,
	}))
	if err != nil {
		log.Printf("序列化消息失败: %v", err)
		return
	}
	cp := &paho.Publish{
		Topic:   topic,
		QoS:     0,
		Retain:  false,
		Payload: payload,
	}

	log.Printf("发布消息到 MQTT: %s", topic)
	_, err = s.MqttClient.Publish(context.Background(), cp)
	if err != nil {
		log.Printf("发布消息到 MQTT 失败: %v", err)
		return
	}
}

// Broadcast 广播消息到所有WebSocket连接
func (s *Server) Broadcast(data []byte) {
	s.mu.Lock()
	defer s.mu.Unlock()

	// 将消息放入缓冲区
	s.msgBuffer = append(s.msgBuffer, data)

	// 如果缓冲区已满，丢弃最旧的消息
	if len(s.msgBuffer) > maxBufferSize {
		s.msgBuffer = s.msgBuffer[1:]
	}
}

// broadcastLoop 定期广播缓冲区中的最新消息
func (s *Server) broadcastLoop() {
	ticker := time.NewTicker(broadcastInterval)
	defer ticker.Stop()

	for range ticker.C {
		s.mu.Lock()
		if len(s.msgBuffer) > 0 {
			// 只发送最新的消息
			latestMsg := s.msgBuffer[len(s.msgBuffer)-1]
			s.msgBuffer = s.msgBuffer[:0] // 清空缓冲区

			// 复制连接列表以避免在锁内发送消息
			conns := make([]*websocket.Conn, 0, len(s.wsConns))
			for conn := range s.wsConns {
				conns = append(conns, conn)
			}
			s.mu.Unlock()

			// 在锁外发送消息
			for _, conn := range conns {
				err := conn.WriteMessage(websocket.TextMessage, latestMsg)
				if err != nil {
					s.mu.Lock()
					delete(s.wsConns, conn)
					s.mu.Unlock()
					conn.Close()
				}
			}
		} else {
			s.mu.Unlock()
		}
	}
}

const (
	maxBufferSize     = 256                    // 缓冲区最大消息数
	broadcastInterval = 100 * time.Millisecond // 广播间隔
)
