package storage

import (
	topicToJSON "github.com/crossworth/vk-topic-to-json"
)

type Storage interface {
	Find(topicID int) (topicToJSON.Topic, error)
	Save(topic topicToJSON.Topic) error
}
