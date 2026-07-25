package main

import (
	"bufio"
	"context"
	"flag"
	"fmt"
	"io"
	"log"
	"net"
	"os"
	"os/signal"
	"syscall"

	"github.com/eclipse/paho.golang/paho"
)

func main() {
	stdin := bufio.NewReader(os.Stdin)
	hostname, _ := os.Hostname()

	server := flag.String("server", "127.0.0.1:1883", "Broker 地址")
	topic := flag.String("topic", "a", "订阅的主题")
	qos := flag.Int("qos", 0, "消息质量")
	clientId := flag.String("client-id", "", "客户端Id")
	name := flag.String("chatname", hostname, "The name to attach to your messages")
	username := flag.String("username", "", "A username to authenticate to the MQTT server")
	password := flag.String("password", "", "Password to match username")
	flag.Parse()

	conn, err := net.Dial("tcp", *server)
	if err != nil {
		log.Fatalf("Failed connent to %s: %s", *server, err)
	}

	c := paho.NewClient(paho.ClientConfig{
		ClientID: *clientId,
		Conn:     conn,
		OnPublishReceived: []func(paho.PublishReceived) (bool, error){
			func(pr paho.PublishReceived) (bool, error) {
				content := string(pr.Packet.Payload)
				log.Printf("消息: %s", content)
				return true, nil
			},
		},
	})

	cp := &paho.Connect{
		KeepAlive:  30,
		ClientID:   *clientId,
		CleanStart: true,
		Username:   *username,
		Password:   []byte(*password),
	}

	if *username != "" {
		cp.UsernameFlag = true
	}
	if *password != "" {
		cp.PasswordFlag = true
	}

	ca, err := c.Connect(context.Background(), cp)
	if err != nil {
		log.Fatalln(err)
	}

	if ca.ReasonCode != 0 {
		log.Fatalf("连接到 %s 发生错误：%d - %s", *server, ca.ReasonCode, ca.Properties.ReasonString)
	}

	fmt.Printf("连接到 %s\n", *server)

	ic := make(chan os.Signal, 1)
	signal.Notify(ic, os.Interrupt, syscall.SIGTERM)

	go func() {
		<-ic
		fmt.Println("signal received, exiting")
		if c != nil {
			d := &paho.Disconnect{ReasonCode: 0}
			err := c.Disconnect(d)
			if err != nil {
				log.Fatalf("failed to send Disconnect: %s", err)
			}
		}
		os.Exit(0)
	}()

	if _, err := c.Subscribe(context.Background(), &paho.Subscribe{
		Subscriptions: []paho.SubscribeOptions{
			{Topic: *topic, QoS: byte(*qos), NoLocal: true},
		},
	}); err != nil {
		log.Fatalln(err)
	}

	log.Printf("Subscribed to %s", *topic)

	for {
		message, err := stdin.ReadString('\n')
		if err == io.EOF {
			os.Exit(0)
		}

		props := &paho.PublishProperties{}
		props.User.Add("chatname", *name)

		pb := &paho.Publish{
			Topic:      *topic,
			QoS:        byte(*qos),
			Payload:    []byte(message),
			Properties: props,
		}

		if _, err = c.Publish(context.Background(), pb); err != nil {
			log.Println(err)
		}
	}

}
