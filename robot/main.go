package main

import (
	"context"
	"encoding/json"
	"flag"
	"fmt"
	"log"
	"math/rand"
	"net"
	"os"
	"os/signal"
	"syscall"
	"time"

	protocol "github.com/Drunk6904/mqbot"
	"github.com/eclipse/paho.golang/paho"
)

type RoBot struct {
	X     float64 `json:"x"`
	Y     float64 `json:"y"`
	Speed float64 `json:"speed"`
}

var selfBot = RoBot{
	X:     0,
	Y:     0,
	Speed: 1,
}

func main() {
	server := flag.String("server", "localhost:1883", "指定mqtt的broker地址")
	clientId := flag.String("id", "", "客户端id，不输入则按某规则进行生成")
	username := flag.String("username", "", "指定用户名")
	password := flag.String("password", "", "登录密码")
	flag.Parse()

	if *clientId == "" {
		*clientId = fmt.Sprintf("bot_%d", rand.Intn(10000))
	}

	conn, err := net.Dial("tcp", *server)
	if err != nil {
		log.Fatalln("连接到服务器 "+*server+"发送错误：", err)
	}

	// 创建一个 mqtt 客户端
	c := paho.NewClient(paho.ClientConfig{
		Conn:     conn,
		ClientID: *clientId,
	})

	// 添加回调函数
	c.AddOnPublishReceived(handMsg)

	// 创建 mqtt 连接数据包
	cp := &paho.Connect{
		ClientID: *clientId,
		Username: *username,
		Password: []byte(*password),

		// 保持连接时间
		KeepAlive: 30,
		// 建立全新连接
		CleanStart: true,
	}
	if *username != "" {
		cp.UsernameFlag = true
	}
	if *password != "" {
		cp.PasswordFlag = true
	}

	ca, err := c.Connect(context.Background(), cp)
	if err != nil {
		log.Fatalf("客户端 %s 连接到Broker发生错误：%s", *clientId, err)
	}

	if ca.ReasonCode != 0 {
		log.Fatalf("连接到 %s 发生错误：%d - %s\n", *server, ca.ReasonCode, ca.Properties.ReasonString)
	}

	ic := make(chan os.Signal, 1)
	signal.Notify(ic, os.Interrupt, syscall.SIGTERM)
	go func() {
		<-ic
		if c != nil {
			err := c.Disconnect(&paho.Disconnect{ReasonCode: 0})
			if err != nil {
				log.Fatalf("断开连接时发生错误：%s", err)
			}
		}
		os.Exit(0)
	}()

	// 订阅
	_, err = c.Subscribe(context.Background(), &paho.Subscribe{
		Subscriptions: []paho.SubscribeOptions{
			{Topic: fmt.Sprintf(protocol.TaskTopic, *clientId)},
			{Topic: fmt.Sprintf(protocol.CommandTopic, *clientId)},
		},
	})
	if err != nil {
		log.Fatalf("订阅频道发生错误：%s\n", err)
	}
	log.Printf("已订阅主题\n")

	for {
		msg, err := json.Marshal(selfBot)
		if err != nil {
			log.Fatalf("报备状态时，解析状态发生错误：%s\n", err)
		}
		props := &paho.PublishProperties{}
		props.User.Add("botId", *clientId)
		cp := &paho.Publish{
			Topic:      fmt.Sprintf(protocol.StatusTopic, *clientId),
			QoS:        2,
			Payload:    msg,
			Properties: props,
		}
		_, err = c.Publish(context.Background(), cp)
		if err != nil {
			log.Fatalf("报备状态时，发布消息发生错误：%s\n", err)
		}

		selfBot.X += 0.1
		selfBot.Y += 0.2

		time.Sleep(time.Second * 3)
	}
}

// 回调函数，对接收的消息进行处理
func handMsg(paho.PublishReceived) (bool, error) {
	return true, nil
}
