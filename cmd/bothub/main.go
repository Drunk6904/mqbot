package main

import (
	"fmt"
	"log"
	"net/http"
	"os"
	"os/signal"
	"strings"
	"syscall"

	"github.com/Drunk6904/mqbot/internal/mqtt"
	"github.com/Drunk6904/mqbot/protocol"
	"github.com/eclipse/paho.golang/paho"
	"github.com/gin-gonic/gin"
)

// 类型定义 ======================================================

type Response struct {
	Code int    `json:"code"`
	Msg  string `json:"msg"`
	Data any    `json:"data"`
}

// 常量 ===========================================================

var host = "127.0.0.1"
var port = 1883
var clientId = "hub_10001"
var username = ""
var password = ""

var webPort = 8080

// Main ==========================================================

func main() {
	// 启动web服务
	go func() { log.Fatal(Web().Run(fmt.Sprintf(":%d", webPort))) }()

	// mqtt 服务
	c, err := mqtt.NewClient(&mqtt.MQTTBrokerInfo{
		Host: host,
		Port: port,

		ClientId:   clientId,
		UserName:   username,
		Password:   []byte(password),
		CleanStart: true,
		KeepAlive:  30,

		Auth: false,

		OnPublishReceived: MsgHandler,
	})
	if err != nil {
		log.Fatalf("创建 mqtt 客户端失败：%v\n", err)
	}

	err = mqtt.SubscribeTopic(c, fmt.Sprintf(protocol.StatusTopic, "+"), 0)
	if err != nil {
		log.Fatalf("订阅状态主题失败\n")
	}

	// 停止
	ic := make(chan os.Signal, 1)
	signal.Notify(ic, os.Interrupt, syscall.SIGTERM)
	<-ic
	if c != nil {
		err := c.Disconnect(&paho.Disconnect{ReasonCode: 0})
		if err != nil {
			log.Fatalf("发生错误: %s\n", err)
		}
	}
	os.Exit(0)
}

// Web 服务相关 ==================================================

// web 服务
func Web() (r *gin.Engine) {
	r = gin.Default()
	r.GET("ping", func(ctx *gin.Context) {
		ctx.JSON(http.StatusOK, gin.H{
			"message": "pong",
		})
	})
	return
}

// MQTT 服务相关 =================================================

// mqtt 消息处理函数
func MsgHandler(pr paho.PublishReceived) (bool, error) {
	topic := pr.Packet.Topic
	switch {
	case strings.HasSuffix(topic, "/status"):
		handStatus(pr)
	default:
		log.Printf("未知主题：%s\n", topic)
	}
	return true, nil
}

func handStatus(pr paho.PublishReceived) {
	// TODO
	log.Printf("[status] %s", pr.Packet.Payload)

}
