package mqtt

import (
	"context"
	"fmt"
	"log"
	"net"

	"github.com/eclipse/paho.golang/paho"
)

// MQTT代理服务器的连接信息
type MQTTBrokerInfo struct {
	// ===== 连接参数 =====
	Schema   string // 连接协议，如 "tcp"、"ssl" 等
	Host     string // MQTT代理服务器的主机名或IP地址
	Port     int    // MQTT代理服务器的端口号
	ClientId string // 客户端唯一标识符

	// ===== 会话参数 =====
	CleanStart     bool   // 是否清除历史会话
	KeepAlive      uint16 // 客户端与服务器的心跳间隔（秒）
	SessionExpiry  uint32 // 会话过期时间
	WillMessage    *paho.WillMessage
	WillProperties *paho.WillProperties

	// ===== 认证与安全 =====
	Auth     bool // 是否认证
	UserName string
	Password []byte

	// === 消息默认策略 (仅作为全局兜底值) ===
	DefaultQoS    byte   // 默认 QoS
	MaxPacketSize uint32 // 限制最大报文大小，防止内存溢出

	// === 重连策略 ===
	ConnRetryMax  int // 最大重试次数
	ConnRetryBase int // 初始等待时间(秒)
}

func NewClient(info MQTTBrokerInfo) (*paho.Client, error) {
	var client *paho.Client
	host := fmt.Sprintf("%s:%d", info.Host, info.Port)

	conn, err := net.Dial(info.Schema, host)
	if err != nil {
		return nil, fmt.Errorf("连接到 %s://%s 时，发生错误：%w", info.Schema, host, err)
	}

	client = paho.NewClient(paho.ClientConfig{
		ClientID: info.ClientId,
		Conn:     conn,
	})

	cp := &paho.Connect{
		Username:     info.UserName,
		Password:     info.Password,
		UsernameFlag: info.Auth,
		PasswordFlag: info.Auth,

		ClientID:   info.ClientId,
		KeepAlive:  info.KeepAlive,
		CleanStart: info.CleanStart,

		WillMessage:    info.WillMessage,
		WillProperties: info.WillProperties,

		Properties: &paho.ConnectProperties{
			MaximumPacketSize: &info.MaxPacketSize,
		},
	}
	for i := 0; i < info.ConnRetryMax; i++ {
		ack, err := client.Connect(context.Background(), cp)
		if err == nil && ack.ReasonCode < 0x80 {
			log.Printf("连接成功 (尝试 %d/%d)\n", i+1, info.ConnRetryMax)
			break
		} else if err != nil {
			log.Printf("连接失败 (尝试 %d/%d): %s\n", i+1, info.ConnRetryMax, err)
		} else if ack.ReasonCode >= 0x80 {
			log.Printf("连接失败 (尝试 %d/%d): 服务器返回错误码 %d - %s\n", i+1, info.ConnRetryMax, ack.ReasonCode, ack.Properties.ReasonString)
		}
		
		// 如果是最后一次重试，返回错误
		if i == info.ConnRetryMax-1 {
			return nil, fmt.Errorf("连接失败，已达到最大重试次数 %d", info.ConnRetryMax)
		}
	}
	return client, nil
}
