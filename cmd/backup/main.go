package main

import (
	"io/ioutil"
	"log"
	"strings"
	"sync"

	topicToJSON "github.com/crossworth/vk-topic-to-json"
	"github.com/goccy/go-yaml"
	vkapi "github.com/himidori/golang-vk-api"

	vgb "github.com/crossworth/vk-group-backup"
	"github.com/crossworth/vk-group-backup/storage"
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
	ContinuousMode bool      `yaml:"continuousMode"`
}

const settingsFile = "settings.yml"

var wg sync.WaitGroup

func main() {
	settings := Settings{
		GroupID:        0,
		Mode:           "all",
		Output:         "file://backup",
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
		settings.Output = "file://backup"
	}

	if settings.GroupID == 0 {
		log.Fatalf("you must provide a valid group ID")
	}

	log.Printf("Mode: %s\n", settings.Mode)
	log.Printf("Output: %s\n", settings.Output)
	log.Printf("GroupID: %d\n", settings.GroupID)
	log.Printf("ContinuousMode: %t\n", settings.ContinuousMode)
	log.Printf("Number of accounts: %d\n", len(settings.Accounts))

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

	var topicStorage storage.Storage

	if strings.HasPrefix(settings.Output, "file://") {
		topicStorage, err = storage.NewFileStorage(settings.Output, settings.GroupID)
		if err != nil {
			log.Fatalf("could not create file storage, %v", err)
		}
	} else if strings.HasPrefix(settings.Output, "postgres://") {
		topicStorage, err = storage.NewPostgreSQL(settings.Output)
		if err != nil {
			log.Fatalf("could not create PostgreSQL storage, %v", err)
		}
	} else {
		log.Fatalf("output type not recognized, %s", settings.Output)
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
			log.Printf("error getting topic: %v\n", err)
		}
	}()

	wg.Add(len(accountsPoll) - 1)

	// skip the first account since its been used to get the posts
	for i, account := range accountsPoll[1:] {
		go work(i, topicStorage, topicChan, account, settings.GroupID)
	}

	wg.Wait()
	log.Println("done")
}

func work(workerID int, topicStorage storage.Storage, topicChan <-chan vkapi.Topic, client *vkapi.VKClient, groupID int) {
	for vkapiTopic := range topicChan {
		log.Printf("worker %d: checking topic %d\n", workerID, vkapiTopic.ID)

		updating := false

		topicFromStorage, err := topicStorage.Find(vkapiTopic.ID)
		if err != nil {
			log.Printf("worker %d:error reading topic %d from storage, %v\n", workerID, vkapiTopic.ID, err)
			continue
		}

		if topicFromStorage.ID != 0 && vkapiTopic.Updated == topicFromStorage.UpdatedAt {
			log.Printf("worker %d:topic %d already updated\n", workerID, topicFromStorage.ID)
			continue
		}

		if topicFromStorage.UpdatedAt > vkapiTopic.Updated {
			log.Printf("worker %d:topic %d is older than topic from database\n", workerID, topicFromStorage.ID)
			continue
		}

		if topicFromStorage.ID != 0 {
			updating = true
		}

		log.Printf("worker %d: downloading topic %d\n", workerID, vkapiTopic.ID)

		topic, err := topicToJSON.SaveTopic(client, groupID, vkapiTopic.ID)
		if err != nil {
			log.Printf("worker %d: error downloading topic %d, %v\n", workerID, vkapiTopic.ID, err)
			continue
		}

		err = topicStorage.Save(topic)
		if err != nil {
			log.Printf("worker %d: error saving topic %d on storage, %v\n", workerID, vkapiTopic.ID, err)
			continue
		}

		if updating {
			log.Printf("worker %d: topic %d updated\n", workerID, vkapiTopic.ID)
		} else {
			log.Printf("worker %d: topic %d created\n", workerID, vkapiTopic.ID)
		}
	}
	wg.Done()
}
