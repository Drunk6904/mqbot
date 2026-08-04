package main

import (
	"context"
	"encoding/json"
	"flag"
	"fmt"
	"log"
	"math"
	"math/rand"
	"os"
	"os/signal"
	"strconv"
	"strings"
	"sync"
	"syscall"
	"time"

	"github.com/Drunk6904/mqbot/internal/mqtt"
	"github.com/Drunk6904/mqbot/internal/robot"
	"github.com/Drunk6904/mqbot/protocol"
	"github.com/eclipse/paho.golang/paho"
)

type RoBot struct {
	protocol.StatusBody
}

// 状态快照结构体
type stateSnapshot struct {
	Battery float64
	State   string
	X       float64
	Y       float64
	Speed   float64
}

var (
	selfBot = RoBot{
		protocol.StatusBody{
			Speed:   1,
			State:   protocol.StateIdle,
			Battery: 99,
		}}
	currentCancel context.CancelFunc
	stateMutex    sync.Mutex

	lastReported stateSnapshot
)

func main() {
	host := flag.String("server", "localhost", "指定mqtt的broker ip")
	port := flag.Int("port", 1883, "指定mqtt的broker 端口 ")
	clientId := flag.String("id", "", "客户端id，不输入则按某规则进行生成")
	username := flag.String("username", "", "指定用户名")
	password := flag.String("password", "", "登录密码")
	speed := flag.Float64("speed", 1.0, "速度")
	flag.Parse()

	if *clientId == "" {
		*clientId = fmt.Sprintf("bot_%d", rand.Intn(10000))
	}
	selfBot.ID = *clientId
	selfBot.Speed = *speed
	// 创建MQTT客户端
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

	// 注册信号处理函数，用于在程序结束时断开MQTT连接
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

	// 发送连接建立成功消息
	sendState(c, clientId)

	t := time.NewTicker(100 * time.Millisecond)
	defer t.Stop()
	for range t.C {
		if selfBot.State == protocol.StateCharging {
			selfBot.Battery += 2
		}
		if selfBot.Battery <= 20 && selfBot.State != protocol.StateCharging {
			selfBot.State = protocol.StateCharging
			currentCancel()
		}
		if selfBot.State == protocol.StateCharging && selfBot.Battery > 90 {
			selfBot.State = protocol.StateIdle
		}
		if selfBot.State == protocol.StateMoving {
			selfBot.Battery -= 1
		}
		sendState(c, clientId)
	}
}

func sendState(c *paho.Client, clientId *string) {
	stateMutex.Lock()
	if !shouldReportLocked() {
		stateMutex.Unlock()
		return
	}
	snap := selfBot.StatusBody
	updateLastReportedLocked()
	stateMutex.Unlock()


	msg, err := json.Marshal(snap)
	if err != nil {
		log.Printf("报备状态时，解析状态发生错误：%s\n", err)
		return
	}

	props := &paho.PublishProperties{}
	props.User.Add("botId", *clientId)
	cp := &paho.Publish{
		Topic:      fmt.Sprintf(protocol.StatusTopic, *clientId),
		QoS:        0,
		Payload:    msg,
		Properties: props,
	}
	if _, err = c.Publish(context.Background(), cp); err != nil {
		log.Printf("报备状态时，发布消息发生错误：%s\n", err)
	}
}


func shouldReportLocked() bool {
	// 电量变化超过1%即上报
	if math.Abs(selfBot.Battery-lastReported.Battery) > 1.0 {
		return true
	}

	// 状态机变化上报
	if selfBot.State != lastReported.State {
		return true
	}

	// 位置变化超过0.01
	const epsilon = 0.01
	if math.Abs(selfBot.X-lastReported.X) > epsilon ||
		math.Abs(selfBot.Y-lastReported.Y) > epsilon {
		return true
	}

	// 4. 速度变化
	if math.Abs(selfBot.Speed-lastReported.Speed) > 0.1 {
		return true
	}

	return false
}


func updateLastReportedLocked() {
	lastReported = stateSnapshot{
		Battery: selfBot.Battery,
		State:   selfBot.State,
		X:       selfBot.X,
		Y:       selfBot.Y,
		Speed:   selfBot.Speed,
	}
}

// func calculateHash(msg []byte) string {
// 	h := fnv.New32a()
// 	h.Write(msg)
// 	hashBytes := h.Sum(nil)
// 	return hex.EncodeToString(hashBytes)
// }

// 回调函数，对接收的消息进行处理
func handMsg(pr paho.PublishReceived) (bool, error) {
	switch {
	case strings.HasSuffix(pr.Packet.Topic, "/task"):
		handTask(pr)
	case strings.HasSuffix(pr.Packet.Topic, "/command"):
		handCommand(pr)
	}
	return true, nil
}
func handTask(pr paho.PublishReceived) {
	var data protocol.TaskMessage
	err := json.Unmarshal(pr.Packet.Payload, &data)
	if err != nil {
		log.Printf("处理MQTT消息失败: %+v", err)
	}
	switch data.Body.Action {
	case protocol.ActionMoveTo:
		handleMoveTo(data)
	}
}

// handCommand 处理接收到的MQTT命令消息
func handCommand(pr paho.PublishReceived) {
	var data protocol.CommandMessage
	err := json.Unmarshal(pr.Packet.Payload, &data)
	if err != nil {
		log.Printf("处理MQTT消息(command)失败：%+v", err)
	}
	switch data.Body.Action {
	case protocol.ActionStop:
		currentCancel()
	case protocol.ActionSetSpeed:
		selfBot.Speed = protocol.FloatParam(data.Body.Params, "speed", 1.0)
	}
}

func handleMoveTo(data protocol.TaskMessage) {
	stateMutex.Lock()

	// 如果有旧的任务 结束
	if currentCancel != nil {
		currentCancel()
	}
	selfBot.TaskID = data.Body.TaskID
	stateMutex.Unlock()

	log.Printf("开始处理移动指令，当前位置: x=%.3f, y=%.3f", selfBot.X, selfBot.Y)
	// 获取x和y的字符串值
	xStr := protocol.StringParam(data.Body.Params, "x", fmt.Sprintf("%.3f", selfBot.X))
	yStr := protocol.StringParam(data.Body.Params, "y", fmt.Sprintf("%.3f", selfBot.Y))
	log.Printf("目标位置: x=%s, y=%s", xStr, yStr)

	// 将字符串转换为float64
	x, err := strconv.ParseFloat(xStr, 64)
	if err != nil {
		log.Printf("解析x坐标失败: %v", err)
		return
	}

	y, err := strconv.ParseFloat(yStr, 64)
	if err != nil {
		log.Printf("解析y坐标失败: %v", err)
		return
	}

	ctx, can := context.WithCancel(context.Background())
	ctx = context.WithValue(ctx, "task_id", data.Body.TaskID)
	currentCancel = can

	// 调用MoveTo函数
	log.Printf("开始移动到目标位置: x=%.3f, y=%.3f", x, y)
	go func() {
		stateMutex.Lock()
		if selfBot.TaskID == data.Body.TaskID {
			selfBot.State = protocol.StateMoving
		}
		stateMutex.Unlock()

		robot.MoveTo(ctx, &selfBot.StatusBody, x, y)

		stateMutex.Lock()
		if selfBot.TaskID == data.Body.TaskID {
			selfBot.State = protocol.StateIdle
		}
		stateMutex.Unlock()
	}()
}
