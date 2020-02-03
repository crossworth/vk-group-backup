package main

import (
	"fmt"
	"log"
	"os"
	"strconv"
	"strings"

	vkapi "github.com/himidori/golang-vk-api"
	"github.com/streadway/amqp"

	vgb "github.com/crossworth/vk-group-backup"
)

var (
	rabbitmqServer    string
	rabbitmqQueueName string
	mongoServer       string
	mongoDatabase     string
	vkGroupID         string
	vkUserEmail       string
	vkUserPassword    string
	vkGroupIDInt      int
	vkDevice          string
)

func init() {
	rabbitmqServer = os.Getenv("RABBITMQ_SERVER")
	rabbitmqQueueName = os.Getenv("RABBITMQ_QUEUE_NAME")
	mongoServer = os.Getenv("MONGO_SERVER")
	mongoDatabase = os.Getenv("MONGO_DATABASE")
	vkGroupID = os.Getenv("VK_GROUP_ID")
	vkUserEmail = os.Getenv("VK_EMAIL")
	vkUserPassword = os.Getenv("VK_PASSWORD")
	vkDevice = os.Getenv("VK_DEVICE")

	if rabbitmqServer == "" {
		log.Fatal("RABBITMQ_SERVER env var not set")
	}

	if rabbitmqQueueName == "" {
		log.Fatal("RABBITMQ_QUEUE_NAME env var not set")
	}

	if mongoServer == "" {
		log.Fatal("RABBITMQ_SERVER env var not set")
	}

	if mongoDatabase == "" {
		log.Fatal("MONGO_SERVER env var not set")
	}

	if vkGroupID == "" {
		log.Fatal("VK_GROUP_ID env var not set")
	}

	if vkUserEmail == "" {
		log.Fatal("VK_EMAIL env var not set")
	}

	if vkUserPassword == "" {
		log.Fatal("VK_PASSWORD env var not set")
	}

	if vkDevice == "" {
		vkDevice = "android"
	}

	vkDevice = strings.ToLower(vkDevice)

	var err error
	vkGroupIDInt, err = strconv.Atoi(vkGroupID)
	if err != nil {
		log.Fatal(err)
	}
}

func main() {
	conn, err := amqp.Dial(rabbitmqServer)
	if err != nil {
		log.Fatalf("failed to open connection to rabbitmq: %v\n", err)
	}

	ch, err := conn.Channel()
	if err != nil {
		log.Fatalf("failed to open a channel: %v\n", err)
	}
	defer ch.Close()

	queue, err := ch.QueueDeclare(
		rabbitmqQueueName,
		false,
		false,
		false,
		false,
		nil,
	)
	if err != nil {
		log.Fatalf("failed to declare a queue: %v\n", err)
	}

	err = ch.Qos(1, 0, false)
	if err != nil {
		log.Fatal(err)
	}

	device := vkapi.DeviceIPhone

	switch vkDevice {
	case "android":
		device = vkapi.DeviceAndroid
	case "wphone":
		device = vkapi.DeviceWPhone
	default:
		device = vkapi.DeviceIPhone
	}

	client, err := vgb.New(vkGroupIDInt, vkUserEmail, vkUserPassword,
		vgb.WithMongo(mongoServer, mongoDatabase), vgb.WithDevice(device))
	if err != nil {
		log.Fatal(err)
	}

	msgs, err := ch.Consume(
		queue.Name,
		"",
		false,
		false,
		false,
		false,
		nil,
	)
	if err != nil {
		log.Fatal(err)
	}

	for d := range msgs {
		_ = d.Ack(false)

		fmt.Printf("Got payload %s from rabbitmq\n", string(d.Body))

		id, err := strconv.Atoi(string(d.Body))
		if err != nil {
			log.Printf("failed to convert id %s to int\n", string(d.Body))
			continue
		}

		_, err = client.SaveTopic(id)
		if err != nil {
			log.Printf("failed to save topic %d error: %v\n", id, err)
		}
	}
}
