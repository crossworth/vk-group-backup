package storage

import (
	"encoding/json"
	"fmt"
	"io/ioutil"
	"os"
	"strings"

	topicToJSON "github.com/crossworth/vk-topic-to-json"
)

type FileStorage struct {
	output  string
	groupID int
}

func NewFileStorage(output string, groupID int) (*FileStorage, error) {
	output = strings.ReplaceAll(output, "file://", "")

	fileStorage := &FileStorage{
		output:  output,
		groupID: groupID,
	}

	_ = os.MkdirAll(output, os.ModePerm)
	return fileStorage, nil
}

func (f *FileStorage) Find(topicID int) (topicToJSON.Topic, error) {
	var topic topicToJSON.Topic
	fileName := fmt.Sprintf("%s/%d_%d.json", f.output, f.groupID, topicID)

	if !fileExists(fileName) {
		return topic, nil
	}

	data, err := ioutil.ReadFile(fileName)
	if err != nil {
		return topic, fmt.Errorf("could not read file %s, %v", fileName, err)
	}

	err = json.Unmarshal(data, &topic)
	if err != nil {
		return topic, fmt.Errorf("could not decode JSON file %s, %v", fileName, err)
	}

	return topic, nil
}

func (f *FileStorage) Save(topic topicToJSON.Topic) error {
	fileName := fmt.Sprintf("%s/%d_%d.json", f.output, f.groupID, topic.ID)

	data, err := json.Marshal(topic)
	if err != nil {
		return fmt.Errorf("could not encode topic %d, %v", topic.ID, err)
	}

	err = ioutil.WriteFile(fileName, data, os.ModePerm)
	if err != nil {
		return fmt.Errorf("could not save topic %d, %v", topic.ID, err)
	}

	return nil
}

func fileExists(filename string) bool {
	info, err := os.Stat(filename)
	if os.IsNotExist(err) {
		return false
	}
	return !info.IsDir()
}
