package http

import (
	"context"
	"encoding/json"
	"fmt"
	"log"
	"net/http"
	"sync"

	"github.com/Drunk6904/mqbot/protocol"
	"github.com/eclipse/paho.golang/paho"
	"github.com/gin-gonic/gin"
	"github.com/gorilla/websocket"
)

type Server struct {
	Router     *gin.Engine
	MqttClient *paho.Client
	wsConns    map[*websocket.Conn]bool
	mu         sync.Mutex
}

var upgrader = websocket.Upgrader{
	CheckOrigin: func(r *http.Request) bool { return true }, // 开发阶段允许所有来源
}

func NewServer() *Server {
	r := gin.Default()
	s := &Server{Router: r, wsConns: make(map[*websocket.Conn]bool)}
	return s
}

// 启动服务
func (s *Server) Start(port int) error {
	s.registerRoutes()
	return s.Router.Run(fmt.Sprintf(":%d", port))
}

// 注册路由
func (s *Server) registerRoutes() {
	s.Router.GET("/ping", s.ping)
	s.Router.GET("/ws", s.handleWS)
	s.Router.Static("/static", "./static")
	s.Router.GET("/", func(ctx *gin.Context) {
		ctx.File("./static/index.html")
	})
}

func (s *Server) ping(ctx *gin.Context) {
	ctx.JSON(http.StatusOK, gin.H{
		"message": "pong",
	})
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

func (s *Server) Broadcast(data []byte) {
	s.mu.Lock()
	defer s.mu.Unlock()
	for conn := range s.wsConns {
		err := conn.WriteMessage(websocket.TextMessage, data)
		if err != nil {
			conn.Close()
			delete(s.wsConns, conn)
		}
	}
}
