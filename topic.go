package vgb

import (
	"context"
	"net/url"
	"strconv"
	"time"

	vkapi "github.com/himidori/golang-vk-api"
)

func GetRecentTopicsIDs(ctx context.Context, client *vkapi.VKClient, pullRecentInterval time.Duration, groupID int) (<-chan int, <-chan error) {
	topicIDChan := make(chan int)
	errorChan := make(chan error)

	go func() {
		for {
			select {
			case <-time.After(pullRecentInterval):
				params := url.Values{}
				params.Set("order", "1")
				topics, err := client.BoardGetTopics(groupID, 100, params)
				if err != nil {
					errorChan <- err
					return
				}

				for i := range topics.Topics {
					topicIDChan <- topics.Topics[i].ID
				}

			case <-ctx.Done():
				close(topicIDChan)
				close(errorChan)
				return
			}
		}
	}()

	return topicIDChan, errorChan
}

func GetAllTopicIds(ctx context.Context, client *vkapi.VKClient, pullRecentInterval time.Duration, groupID int) (<-chan int, <-chan error) {
	topicIDChan := make(chan int)
	errorChan := make(chan error)

	go func() {
		params := url.Values{}
		params.Set("order", "-2")
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
				topicIDChan <- topics.Topics[i].ID
			}

			total += len(topics.Topics)
			if total >= topics.Count {
				break
			}

			skip += 100
		}

		close(topicIDChan)
		close(errorChan)
	}()

	return topicIDChan, errorChan
}
