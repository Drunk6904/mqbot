package http

import (
	"fmt"
	"sync"

	"github.com/eclipse/paho.golang/paho"
	"github.com/gin-gonic/gin"
	"github.com/gorilla/websocket"
)

type Server struct {
	Router     *gin.Engine
	MqttClient *paho.Client
	wsConns    map[*websocket.Conn]chan []byte
	msgBuffer  [][]byte // 消息缓冲区
	mu         sync.Mutex
}

func NewServer() *Server {
	r := gin.Default()
	s := &Server{
		Router:    r,
		wsConns:   make(map[*websocket.Conn]chan []byte),
		msgBuffer: make([][]byte, 0, maxBufferSize),
	}
	// 启动广播循环
	go s.broadcastLoop()
	return s
}

// 启动服务
func (s *Server) Start(port int) error {
	s.registerRoutes()
	return s.Router.Run(fmt.Sprintf(":%d", port))
}
