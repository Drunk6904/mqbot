package mqtt

import (
	"context"
	"fmt"
	"log"
	"net"
	"time"

	"github.com/eclipse/paho.golang/paho"
)

type WillMessage struct {
	Retain  bool
	QoS     byte
	Topic   string
	Payload []byte
}

// MQTT代理服务器的连接信息
type MQTTBrokerInfo struct {
	// ===== 连接参数 =====
	Schema   string // 连接协议，如 "tcp"、"ssl" 等
	Host     string // MQTT代理服务器的主机名或IP地址
	Port     int    // MQTT代理服务器的端口号
	ClientId string // 客户端唯一标识符

	// ===== 会话参数 =====
	CleanStart    bool   // 是否清除历史会话
	KeepAlive     uint16 // 客户端与服务器的心跳间隔（秒）
	SessionExpiry uint32 // 会话过期时间
	Will          WillMessage

	// ===== 认证与安全 =====
	Auth     bool // 是否认证
	UserName string
	Password []byte

	// ===== 消息默认策略 =====
	DefaultQoS    byte   // 默认 QoS
	MaxPacketSize uint32 // 限制最大报文大小，防止内存溢出

	// ===== 重连策略 =====
	ConnRetryMax  int // 最大重试次数
	ConnRetryBase int // 初始等待时间(秒)

	// ===== 消息回调函数 =====
	OnPublishReceived func(paho.PublishReceived) (bool, error)
}

func NewClient(info *MQTTBrokerInfo) (*paho.Client, error) {
	var client *paho.Client
	host := net.JoinHostPort(info.Host, fmt.Sprintf("%d", info.Port))
	if info.Schema == "" {
		info.Schema = "tcp"
	}
	if info.ConnRetryMax == 0 {
		info.ConnRetryMax = 3
	}
	if info.ConnRetryBase == 0 {
		info.ConnRetryBase = 1
	}
	for i := 0; i <= info.ConnRetryMax; i++ {
		// 如果是最后一次重试，返回错误(0,1,2 完毕后 3在这里直接返回 不会进行第四次连接)
		if i == info.ConnRetryMax {
			return nil, fmt.Errorf("连接失败，已达到最大重试次数 %d", info.ConnRetryMax)
		}

		cp := &paho.Connect{
			Username:     info.UserName,
			Password:     info.Password,
			UsernameFlag: info.Auth,
			PasswordFlag: info.Auth,

			ClientID:   info.ClientId,
			KeepAlive:  info.KeepAlive,
			CleanStart: info.CleanStart,

			Properties: &paho.ConnectProperties{
				MaximumPacketSize: &info.MaxPacketSize,
			},
		}
		if info.Will.Topic != "" {
			cp.WillMessage = &paho.WillMessage{
				Topic:   info.Will.Topic,
				QoS:     info.Will.QoS,
				Retain:  info.Will.Retain,
				Payload: info.Will.Payload,
			}
		}
		if info.SessionExpiry > 0 {
			cp.Properties.SessionExpiryInterval = &info.SessionExpiry
		}

		conn, err := net.Dial(info.Schema, host)
		if err != nil {
			log.Printf("连接失败 (尝试 %d/%d): %s\n", i+1, info.ConnRetryMax, err)
			continue
		}

		client = paho.NewClient(paho.ClientConfig{
			ClientID: info.ClientId,
			Conn:     conn,
		})

		cxt, cancel := context.WithTimeout(context.Background(), time.Second*10)
		ack, err := client.Connect(cxt, cp)
		cancel()

		if err == nil && ack.ReasonCode < 0x80 {
			log.Printf("连接成功 (尝试 %d/%d)\n", i+1, info.ConnRetryMax)
			break
		} else if err != nil {
			log.Printf("连接失败 (尝试 %d/%d): %s\n", i+1, info.ConnRetryMax, err)
		} else if ack.ReasonCode >= 0x80 {
			log.Printf("连接失败 (尝试 %d/%d): 服务器返回错误码 %d - %s\n", i+1, info.ConnRetryMax, ack.ReasonCode, ack.Properties.ReasonString)
		}
		// 关闭 TCP 连接
		conn.Close()
		// 等待后再重试，防止触发限流
		if i < info.ConnRetryMax-1 {
			waitTime := time.Duration(info.ConnRetryBase) * time.Second
			if waitTime == 0 {
				waitTime = 1 * time.Second
			}
			log.Printf("等待 %v 后重试...\n", waitTime)
			time.Sleep(waitTime)
		}
	}
	return client, nil
}

func SubscribeTopic(c *paho.Client, topic string, qos byte) error {

	t := time.NewTicker(3 * time.Second)
	defer t.Stop()

	max := 3
	i := 1

	for range t.C {
		ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)

		ps, err := c.Subscribe(ctx, &paho.Subscribe{
			Subscriptions: []paho.SubscribeOptions{
				{Topic: topic, QoS: qos},
			},
		})

		if err != nil {
			log.Printf("订阅 %s 失败 (尝试 %d/%d): %v", topic, i, max, err)
		} else if len(ps.Reasons) == 0 {
			log.Printf("订阅 %s 失败 (尝试 %d/%d): 未收到响应", topic, i, max)
		} else if ps.Reasons[0] < 0x80 {
			log.Printf("订阅 %s 成功 (尝试 %d/%d)", topic, i, max)
			cancel()
			return nil
		} else {
			reason := "未知错误"
			if ps.Properties != nil && ps.Properties.ReasonString != "" {
				reason = ps.Properties.ReasonString
			}
			log.Printf("订阅 %s 被拒绝 (尝试 %d/%d): [0x%02X] %s", topic, i, max, ps.Reasons[0], reason)
			err = fmt.Errorf("reason code 0x%02X", ps.Reasons[0])
		}

		if i >= max {
			cancel()
			return fmt.Errorf("订阅 %s 失败: 已达到最大重试次数 %d", topic, max)
		}
		cancel()
		i++
	}
	return fmt.Errorf("订阅 %s 失败: ticker已停止", topic)
}
