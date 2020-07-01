package main

import (
	"context"
	"encoding/json"
	"flag"
	"fmt"
	"io/ioutil"
	"log"
	"os"
	"path/filepath"
	"strings"
	"time"

	topicToJSON "github.com/crossworth/vk-topic-to-json"
	vkapi "github.com/himidori/golang-vk-api"

	vgb "github.com/crossworth/vk-group-backup"
)

var (
	vkGroupID      int
	vkUserEmail    string
	vkUserPassword string
	vkDevice       string
	vkMode         string
	vkPullInterval = 500 * time.Millisecond
	vkOutputDir    string
)

func main() {
	var pullInterval int
	flag.StringVar(&vkUserEmail, "email", "", "VK Email")
	flag.StringVar(&vkUserPassword, "password", "", "VK Password")
	flag.IntVar(&vkGroupID, "group", 0, "VK Group ID")
	flag.StringVar(&vkMode, "mode", "all", "Mode (all or recents)")
	flag.StringVar(&vkDevice, "device", "iphone", "Device to use (android, wphone or iphone)")
	flag.IntVar(&pullInterval, "interval", 500, "PullInterval in milliseconds")
	flag.StringVar(&vkOutputDir, "output", "backup", "Output dir")

	flag.Parse()

	vkPullInterval = time.Duration(pullInterval) * time.Millisecond

	vkMode = strings.ToLower(vkMode)
	vkDevice = strings.ToLower(vkDevice)

	device := vkapi.DeviceIPhone

	switch vkDevice {
	case "android":
		device = vkapi.DeviceAndroid
	case "wphone":
		device = vkapi.DeviceWPhone
	default:
		device = vkapi.DeviceIPhone
	}

	client, err := vkapi.NewVKClient(device, vkUserEmail, vkUserPassword)
	if err != nil {
		log.Fatalf("Error creating VK client, %v", err)
	}

	ctx := context.Background()

	var idChan <-chan int
	var errorChan <-chan error

	if vkMode == "all" {
		idChan, errorChan = vgb.GetAllTopicIds(ctx, client, vkPullInterval, vkGroupID)
	} else {
		idChan, errorChan = vgb.GetRecentTopicsIDs(ctx, client, vkPullInterval, vkGroupID)
	}

	go func() {
		for err := range errorChan {
			log.Printf("Error getting Topic ID: %v\n", err)
		}
	}()

	_ = os.MkdirAll(vkOutputDir, os.ModePerm)

	for id := range idChan {
		log.Printf("Downloading topic with ID: %d\n", id)

		topic, err := topicToJSON.SaveTopic(client, vkGroupID, id)
		if err != nil {
			log.Printf("Error downloading Topic wiht ID: %d, %v\n", id, err)
			continue
		}

		fileName := fmt.Sprintf("%s/%d_%d_%d.json", vkOutputDir, vkGroupID, id, topic.UpdatedAt)

		if fileExists(fileName) {
			log.Printf("Topic %d already updated\n", id)
			continue
		}

		// delete older version
		files, err := filepath.Glob(fmt.Sprintf("%s/%d_%d_*.json", vkOutputDir, vkGroupID, id))
		for _, file := range files {
			err = os.Remove(file)
			if err != nil {
				log.Printf("Could not delete file %s, %v\n", file, err)
				continue
			}
		}

		data, err := json.Marshal(topic)
		if err != nil {
			log.Printf("Error converting Topic %d to json, %v\n", id, err)
			continue
		}

		err = ioutil.WriteFile(fileName, data, os.ModePerm)
		if err != nil {
			log.Printf("Error saving Topic %d to disc, %v\n", id, err)
			continue
		}
	}
}

func fileExists(filename string) bool {
	info, err := os.Stat(filename)
	if os.IsNotExist(err) {
		return false
	}
	return !info.IsDir()
}
