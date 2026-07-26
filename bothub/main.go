package main

import (
	"context"
	"fmt"
	"log"
	"net"
	"net/http"
	"os"
	"os/signal"
	"strings"
	"syscall"

	protocol "github.com/Drunk6904/mqbot"
	"github.com/eclipse/paho.golang/paho"
	"github.com/gin-gonic/gin"
)

// 类型定义 ======================================================

type Response struct {
	Code int    `json:"code"`
	Msg  string `json:"msg"`
	Data any    `json:"data"`
}

// Main ==========================================================

func main() {
	// 启动web服务
	go func() { log.Fatal(Web().Run(":8080")) }()

	// mqtt 服务
	c := NewMqttClient()
	go ConnectToBroker(c)

	// 停止
	ic := make(chan os.Signal, 1)
	signal.Notify(ic, os.Interrupt, syscall.SIGTERM)
	<-ic
	if c != nil {
		err := c.Disconnect(&paho.Disconnect{ReasonCode: 0})
		HandErr(err)
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

var server = "localhost:1883"
var clientId = "hub_10001"
var username = ""
var password = ""
var ConnectPack = &paho.Connect{
	Username:   username,
	Password:   []byte(password),
	ClientID:   clientId,
	CleanStart: false,
	KeepAlive:  30,
}

// mqtt 服务
func NewMqttClient() *paho.Client {

	conn, err := net.Dial("tcp", server)
	if err != nil {
		log.Fatalf("连接到mqtt borker时，发生错误：%s\n", err)
	}

	c := paho.NewClient(paho.ClientConfig{
		ClientID: clientId,
		Conn:     conn,
	})

	if username != "" {
		ConnectPack.UsernameFlag = true
	}
	if password != "" {
		ConnectPack.PasswordFlag = true
	}
	// 添加消息回调函数
	c.AddOnPublishReceived(MsgHandler)

	return c
}
func ConnectToBroker(c *paho.Client) {
	ca, err := c.Connect(context.Background(), ConnectPack)
	HandErr(err)
	if ca.ReasonCode != 0 {
		log.Fatalf("连接到 %s 发生错误：%d - %s", server, ca.ReasonCode, ca.Properties.ReasonString)
	}
	log.Printf("连接到 %s\n", server)

	// 订阅主题
	topic := fmt.Sprintf(protocol.StatusTopic, "+")
	ps, err := c.Subscribe(context.Background(), &paho.Subscribe{
		Subscriptions: []paho.SubscribeOptions{
			{Topic: topic, QoS: 0},
		},
	})
	// 错误处理
	HandErr(err)
	if len(ps.Reasons) == 0 {
		log.Fatalf("订阅 %s 未收到任何 reason code", topic)
	}
	if ps.Reasons[0] < 0x80 {
		log.Printf("订阅主题成功：%s\tQos:%d\n", topic, ps.Reasons[0])
	} else {
		log.Fatalf("订阅主题失败：%s\t0x%02X", topic, ps.Reasons[0])
	}

}

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

func HandErr(err error, msg ...string) {
	if err == nil {
		return
	}
	log.Fatalf("发生错误: %s\n%v", err, msg)
}
