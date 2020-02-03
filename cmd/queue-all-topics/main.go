package main

import (
	"fmt"
	"log"
	"os"
	"strconv"

	"github.com/streadway/amqp"

	vgb "github.com/crossworth/vk-group-backup"
)

var (
	rabbitmqServer    string
	rabbitmqQueueName string
	vkGroupID         string
	vkUserEmail       string
	vkUserPassword    string
	vkGroupIDInt      int
)

func init() {
	rabbitmqServer = os.Getenv("RABBITMQ_SERVER")
	rabbitmqQueueName = os.Getenv("RABBITMQ_QUEUE_NAME")
	vkGroupID = os.Getenv("VK_GROUP_ID")
	vkUserEmail = os.Getenv("VK_EMAIL")
	vkUserPassword = os.Getenv("VK_PASSWORD")

	if rabbitmqServer == "" {
		log.Fatal("RABBITMQ_SERVER env var not set")
	}

	if rabbitmqQueueName == "" {
		log.Fatal("RABBITMQ_QUEUE_NAME env var not set")
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

	client, err := vgb.New(vkGroupIDInt, vkUserEmail, vkUserPassword)
	if err != nil {
		log.Fatal(err)
	}

	idChan, errorChan := client.GetAllTopicIds()

	go func() {
		for err := range errorChan {
			log.Printf("error getting recent topics ids: %v\n", err)
		}
	}()

	for id := range idChan {
		err := ch.Publish(
			"",
			queue.Name,
			false,
			false,
			amqp.Publishing{
				ContentType: "text/plain",
				Body:        []byte(strconv.Itoa(id)),
			},
		)

		fmt.Printf("Sending ID=%d to rabbitmq\n", id)

		if err != nil {
			log.Printf("failed to publish id %d to queue, err: %v\n", id, err)
		}
	}
}
