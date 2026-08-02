package http

import (
	"net/http"

	"github.com/gin-gonic/gin"
)

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
