package main

import (
	"context"
	"encoding/json"
	"flag"
	"fmt"
	"log"
	"math/rand"
	"os"
	"os/signal"
	"syscall"
	"time"

	"github.com/Drunk6904/mqbot/internal/mqtt"
	"github.com/Drunk6904/mqbot/protocol"
	"github.com/eclipse/paho.golang/paho"
)

type RoBot struct {
	protocol.StatusBody
}

var selfBot = RoBot{}

func main() {
	host := flag.String("server", "localhost", "指定mqtt的broker ip")
	port := flag.Int("port", 1883, "指定mqtt的broker 端口 ")
	clientId := flag.String("id", "", "客户端id，不输入则按某规则进行生成")
	username := flag.String("username", "", "指定用户名")
	password := flag.String("password", "", "登录密码")
	flag.Parse()

	if *clientId == "" {
		*clientId = fmt.Sprintf("bot_%d", rand.Intn(10000))
	}

	c, err := mqtt.NewClient(&mqtt.MQTTBrokerInfo{
		Host:     *host,
		Port:     *port,
		ClientId: *clientId,
		UserName: *username,
		Password: []byte(*password),

		OnPublishReceived: handMsg,

		CleanStart: true,
		KeepAlive:  30,
		Auth:       false,
	})
	// 错误处理
	if err != nil {
		log.Fatalf("创建MQTT客户端失败：%s\n", err)
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
	err = mqtt.SubscribeTopic(c, fmt.Sprintf(protocol.TaskTopic, *clientId), 2)
	if err != nil {
		log.Fatalf("订阅频道发生错误：%s\n", err)
	}

	err = mqtt.SubscribeTopic(c, fmt.Sprintf(protocol.CommandTopic, *clientId), 2)
	if err != nil {
		log.Fatalf("订阅频道发生错误：%s\n", err)
	}

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
