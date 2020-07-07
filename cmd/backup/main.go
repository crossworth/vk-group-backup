package main

import (
	"encoding/json"
	"fmt"
	"io/ioutil"
	"log"
	"os"
	"path/filepath"
	"strings"
	"sync"

	topicToJSON "github.com/crossworth/vk-topic-to-json"
	"github.com/goccy/go-yaml"
	vkapi "github.com/himidori/golang-vk-api"

	vgb "github.com/crossworth/vk-group-backup"
)

type Account struct {
	Email    string `yaml:"email"`
	Password string `yaml:"password"`
}

type Settings struct {
	GroupID        int       `yaml:"groupID"`
	Mode           string    `yaml:"mode"`
	Output         string    `yaml:"output"`
	Accounts       []Account `yaml:"accounts"`
	ContinuousMode bool      `yaml: "continuousMode"`
}

const settingsFile = "settings.yml"

func main() {
	settings := Settings{
		GroupID:        0,
		Mode:           "all",
		Output:         "backup",
		Accounts:       nil,
		ContinuousMode: false,
	}

	output, err := ioutil.ReadFile(settingsFile)
	if err != nil {
		log.Fatalf("could not read settings file, %v", err)
	}

	err = yaml.Unmarshal(output, &settings)
	if err != nil {
		log.Fatalf("could not decode the settings file, %v", err)
	}

	settings.Mode = strings.ToLower(settings.Mode)

	if settings.Accounts == nil || len(settings.Accounts) == 0 {
		log.Fatalf("you must provide at least one account")
	}

	for i, account := range settings.Accounts {
		if account.Email == "" {
			log.Fatalf("you must provide the email for the %d account", i)
		}

		if account.Password == "" {
			log.Fatalf("you must provide the password for the %d account", i)
		}
	}

	if settings.Mode != "all" && settings.Mode != "recents" {
		settings.Mode = "all"
	}

	if settings.Output == "" {
		settings.Output = "backup"
	}

	if settings.GroupID == 0 {
		log.Fatalf("you must provide a valid group ID")
	}

	var accountsPoll []*vkapi.VKClient

	for _, account := range settings.Accounts {

		androidClient, err := vkapi.NewVKClient(vkapi.DeviceAndroid, account.Email, account.Password)
		if err != nil {
			log.Fatalf("error creating Android VK client, %v", err)
		}

		iPhoneClient, err := vkapi.NewVKClient(vkapi.DeviceIPhone, account.Email, account.Password)
		if err != nil {
			log.Fatalf("error creating iPhone VK client, %v", err)
		}

		WPhoneClient, err := vkapi.NewVKClient(vkapi.DeviceWPhone, account.Email, account.Password)
		if err != nil {
			log.Fatalf("error creating WPhone VK client, %v", err)
		}

		accountsPoll = append(accountsPoll, androidClient, iPhoneClient, WPhoneClient)
	}

	// NOTE(Pedro): Sanity check
	if accountsPoll == nil || len(accountsPoll) == 1 {
		log.Fatalf("to run we need at least two clients on the poll (1 account = 3 clients)")
	}

	var topicChan <-chan vkapi.Topic
	var errorChan <-chan error

	if settings.Mode == "all" {
		topicChan, errorChan = vgb.GetAllTopicIds(accountsPoll[0], settings.GroupID, settings.ContinuousMode)
	} else {
		topicChan, errorChan = vgb.GetRecentTopicsIDs(accountsPoll[0], settings.GroupID, settings.ContinuousMode)
	}

	go func() {
		for err := range errorChan {
			log.Printf("error getting Topic ID: %v\n", err)
		}
	}()

	_ = os.MkdirAll(settings.Output, os.ModePerm)

	var wg sync.WaitGroup

	wg.Add(len(accountsPoll) - 1)

	// skip the first account since its been used to get the posts
	for i, account := range accountsPoll[1:] {
		go work(i, topicChan, account, settings.GroupID, settings.Output)
	}

	wg.Wait()
}

func work(workerID int, topicChan <-chan vkapi.Topic, client *vkapi.VKClient, groupID int, outputDir string) {
	for vkapiTopic := range topicChan {
		log.Printf("worker %d: downloading topic with ID: %d\n", workerID, vkapiTopic.ID)

		fileName := fmt.Sprintf("%s/%d_%d_%d.json", outputDir, groupID, vkapiTopic.ID, vkapiTopic.Updated)

		if fileExists(fileName) {
			log.Printf("worker %d: topic %d already updated\n", workerID, vkapiTopic.ID)
			continue
		}

		topic, err := topicToJSON.SaveTopic(client, groupID, vkapiTopic.ID)
		if err != nil {
			log.Printf("worker %d: error downloading Topic wiht ID: %d, %v\n", workerID, vkapiTopic.ID, err)
			continue
		}

		// delete older version
		files, err := filepath.Glob(fmt.Sprintf("%s/%d_%d_*.json", outputDir, groupID, vkapiTopic.ID))
		for _, file := range files {
			err = os.Remove(file)
			if err != nil {
				log.Printf("worker %d: could not delete file %s, %v\n", workerID, file, err)
				continue
			}
		}

		data, err := json.Marshal(topic)
		if err != nil {
			log.Printf("worker %d: error encoding Topic %d to json, %v\n", workerID, vkapiTopic.ID, err)
			continue
		}

		err = ioutil.WriteFile(fileName, data, os.ModePerm)
		if err != nil {
			log.Printf("worker %d: error saving Topic %d to disc, %v\n", workerID, vkapiTopic.ID, err)
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
