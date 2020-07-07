package vgb

import (
	"net/url"
	"strconv"
	"time"

	vkapi "github.com/himidori/golang-vk-api"
)

func GetRecentTopicsIDs(client *vkapi.VKClient, groupID int, continuousMode bool) (<-chan vkapi.Topic, <-chan error) {
	topicChan := make(chan vkapi.Topic)
	errorChan := make(chan error)

	go func() {
		params := url.Values{}
		params.Set("order", "1")

		for {
			topics, err := client.BoardGetTopics(groupID, 100, params)
			if err != nil {
				errorChan <- err
				return
			}

			for i := range topics.Topics {
				topicChan <- *topics.Topics[i]
			}

			if !continuousMode {
				break
			}

			time.Sleep(100 * time.Millisecond)
		}

		close(topicChan)
		close(errorChan)
	}()

	return topicChan, errorChan
}

func GetAllTopicIds(client *vkapi.VKClient, groupID int, continuousMode bool) (<-chan vkapi.Topic, <-chan error) {
	topicChan := make(chan vkapi.Topic)
	errorChan := make(chan error)

	go func() {
		params := url.Values{}
		params.Set("order", "-2")

		for {
			skip := 0
			total := 0

			for {
				params.Set("offset", strconv.Itoa(skip))
				topics, err := client.BoardGetTopics(groupID, 100, params)
				if err != nil {
					errorChan <- err
					continue
				}

				for i := range topics.Topics {
					topicChan <- *topics.Topics[i]
				}

				total += len(topics.Topics)
				if total >= topics.Count {
					break
				}

				skip += 100
			}

			if !continuousMode {
				break
			}

			time.Sleep(100 * time.Millisecond)
		}

		close(topicChan)
		close(errorChan)
	}()

	return topicChan, errorChan
}
