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
	wsConns    map[*websocket.Conn]bool
	mu         sync.Mutex
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

