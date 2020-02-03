package vgb

import (
	"fmt"
	"net/url"
	"strconv"
	"time"

	vkapi "github.com/himidori/golang-vk-api"
	"go.mongodb.org/mongo-driver/bson"
	"go.mongodb.org/mongo-driver/mongo"
)

type Profile struct {
	ID         int    `json:"id" bson:"id"`
	FirstName  string `json:"first_name" bson:"first_name"`
	LastName   string `json:"last_name" bson:"last_name"`
	ScreenName string `json:"screen_name" bson:"screen_name"`
	Photo      string `json:"photo" bson:"photo"`
}

type Comment struct {
	ID          int      `json:"id" bson:"id"`
	FromID      int      `json:"from_id" bson:"from_id"`
	Date        int64    `json:"date" bson:"date"`
	Text        string   `json:"text" bson:"text"`
	Likes       int      `json:"likes" bson:"likes"`
	ReplyToUID  int      `json:"reply_to_uid" bson:"reply_to_uid"`
	ReplyToCID  int      `json:"reply_to_cid" bson:"reply_to_cid"`
	Attachments []string `json:"attachments" bson:"attachments"`
}

type Poll struct {
	ID       int          `json:"id" bson:"id"`
	Question string       `json:"question" bson:"question"`
	Votes    int          `json:"votes" bson:"votes"`
	Answers  []PollAnswer `json:"answers" bson:"answers"`
	Multiple bool         `json:"multiple" bson:"multiple"`
	EndDate  int64        `json:"end_date" bson:"end_date"`
	Closed   bool         `json:"closed" bson:"closed"`
}

type PollAnswer struct {
	ID    int     `json:"id" bson:"id"`
	Text  string  `json:"text" bson:"text"`
	Votes int     `json:"votes" bson:"votes"`
	Rate  float64 `json:"rate" bson:"rate"`
}

type Topic struct {
	ID        int             `json:"id" bson:"id"`
	Title     string          `json:"title" bson:"title"`
	IsClosed  bool            `json:"is_closed" bson:"is_closed"`
	IsFixed   bool            `json:"is_fixed" bson:"is_fixed"`
	CreatedAt int64           `json:"created_at" bson:"created_at"`
	UpdatedAt int64           `json:"updated_at" bson:"updated_at"`
	CreatedBy Profile         `json:"created_by" bson:"created_by"`
	UpdatedBy Profile         `json:"updated_by" bson:"updated_by"`
	Profiles  map[int]Profile `json:"profiles" bson:"profiles"`
	Poll      Poll            `json:"poll" bson:"poll"`
	Comments  []Comment       `json:"comments" bson:"comments"`
}

func (v *VKGroupBackUp) GetRecentTopicsIDs() (<-chan int, <-chan error) {
	topicIDChan := make(chan int)
	errorChan := make(chan error)

	go func() {
		for {
			select {
			case <-time.After(v.pullRecentInterval):

				params := url.Values{}
				params.Set("order", "1")
				topics, err := v.client.BoardGetTopics(v.groupID, 100, params)
				if err != nil {
					errorChan <- err
					return
				}

				for i := range topics.Topics {
					topicIDChan <- topics.Topics[i].ID
				}

			case <-v.ctx.Done():
				close(topicIDChan)
				close(errorChan)
				return
			}
		}
	}()

	return topicIDChan, errorChan
}

func (v *VKGroupBackUp) GetAllTopicIds() (<-chan int, <-chan error) {
	topicIDChan := make(chan int)
	errorChan := make(chan error)

	go func() {
		params := url.Values{}
		params.Set("order", "-2")
		skip := 0
		total := 0

		for {
			params.Set("offset", strconv.Itoa(skip))
			topics, err := v.client.BoardGetTopics(v.groupID, 100, params)
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

func (v *VKGroupBackUp) SaveTopic(topicID int) (bool, error) {
	topicsCollection := v.mongo.Database(v.mongoDatabase).Collection("topics")

	params := url.Values{}
	params.Set("topic_ids", strconv.Itoa(topicID))
	params.Set("extended", "1")
	topicResult, err := v.client.BoardGetTopics(v.groupID, 1, params)
	if err != nil {
		return false, err
	}

	if len(topicResult.Topics) < 0 {
		panic("len(topicResult.Topics) < 0")
	}

	topicFilter := bson.D{{
		"id", topicID,
	}}

	var topic Topic
	var updating bool

	err = topicsCollection.FindOne(v.ctx, topicFilter).Decode(&topic)
	if err != nil && err != mongo.ErrNoDocuments {
		return false, err
	}

	// NOTE(Pedro): Topic updated
	if topic.ID != 0 && topic.UpdatedAt == topicResult.Topics[0].Updated {
		fmt.Printf("Topic ID = %d already updated\n", topic.ID)
		return true, nil
	}

	if topic.ID != 0 {
		updating = true
	}

	profilesUsers := mapUsers(topicResult.Profiles)

	topic.ID = topicID
	topic.Title = topicResult.Topics[0].Title
	topic.IsClosed = intToBool(topicResult.Topics[0].IsClosed)
	topic.IsFixed = intToBool(topicResult.Topics[0].IsFixed)
	topic.CreatedAt = topicResult.Topics[0].Created
	topic.CreatedBy = vkUserToProfile(profilesUsers[topicResult.Topics[0].CreatedBy])
	topic.UpdatedAt = topicResult.Topics[0].Updated
	topic.UpdatedBy = vkUserToProfile(profilesUsers[topicResult.Topics[0].UpdatedBy])

	if topic.Profiles == nil {
		topic.Profiles = make(map[int]Profile)
	}

	commentsParams := url.Values{}
	commentsParams.Set("extended", "1")
	commentsParams.Set("need_likes", "1")

	if len(topic.Comments) > 0 {
		params.Set("start_comment_id", strconv.Itoa(len(topic.Comments)))
	}

	for {
		comments, err := v.client.BoardGetComments(v.groupID, topicID, 100, commentsParams)
		if err != nil {
			return false, err
		}

		if comments.Poll != nil {
			topic.Poll = Poll{
				ID:       comments.Poll.ID,
				Question: comments.Poll.Question,
				Votes:    comments.Poll.Votes,
				Answers:  vkPollAnswerToPollAnswer(comments.Poll.Answers),
				Multiple: comments.Poll.Multiple,
				EndDate:  comments.Poll.EndDate,
				Closed:   comments.Poll.Closed,
			}
		}

		// NOTE(Pedro): This save the profiles without duplicating it
		for i := range comments.Profiles {
			topic.Profiles[comments.Profiles[i].UID] = vkUserToProfile(*comments.Profiles[i])
		}

		for i := range comments.Comments {
			topic.Comments = append(topic.Comments, vkCommentToComment(*comments.Comments[i]))
		}

		fmt.Printf("%d len(topic.Comments) >=  %d comments.Count\n", len(topic.Comments), comments.Count)
		if len(topic.Comments) >= comments.Count {
			break
		}
	}

	if updating {
		_, err = topicsCollection.UpdateOne(v.ctx, topicFilter, topic)
	} else {
		_, err = topicsCollection.InsertOne(v.ctx, topic)
	}

	if err != nil {
		return false, err
	}

	return true, nil
}

func intToBool(i int) bool {
	if i > 0 {
		return true
	}

	return false
}

func vkUserToProfile(user vkapi.User) Profile {
	return Profile{
		ID:         user.UID,
		FirstName:  user.FirstName,
		LastName:   user.LastName,
		ScreenName: user.ScreenName,
		Photo:      user.Photo100,
	}
}

func mapUsers(profiles []*vkapi.User) map[int]vkapi.User {
	users := make(map[int]vkapi.User)

	for i := range profiles {
		users[profiles[i].UID] = *profiles[i]
	}

	return users
}

func vkCommentToComment(comment vkapi.TopicComment) Comment {
	cmt := Comment{
		ID:         comment.ID,
		FromID:     comment.FromID,
		Date:       comment.Date,
		Text:       comment.Text,
		ReplyToUID: comment.ReplyToUID,
		ReplyToCID: comment.ReplyToUID,
	}

	if comment.Likes != nil {
		cmt.Likes = comment.Likes.Count
	}

	for i := range comment.Attachments {
		switch comment.Attachments[i].Type {
		case "photo":
			cmt.Attachments = append(cmt.Attachments, getBestPhoto(*comment.Attachments[i].Photo))
		case "sticker":
			cmt.Attachments = append(cmt.Attachments, getBestSticker(*comment.Attachments[i].Sticker))
		case "video":
			cmt.Attachments = append(cmt.Attachments, fmt.Sprintf("https://vk.com/video?z=video%d_%d%%2F%s", comment.Attachments[i].Video.OwnerID, comment.Attachments[i].Video.ID, comment.Attachments[i].Video.AccessKey))
		case "audio":
			cmt.Attachments = append(cmt.Attachments, comment.Attachments[0].Audio.Url)
		}
	}

	return cmt
}

func getBestPhoto(attachment vkapi.AttachmentPhoto) string {
	best := attachment.Sizes[0]

	for i := range attachment.Sizes {
		s := attachment.Sizes[i].Width * attachment.Sizes[i].Height
		b := best.Width * best.Height
		if s > b {
			best = attachment.Sizes[i]
		}
	}

	return best.Url
}

func getBestSticker(attachment vkapi.AttachmentSticker) string {
	best := attachment.Images[0]

	for i := range attachment.Images {
		s := attachment.Images[i].Width * attachment.Images[i].Height
		b := best.Width * best.Height
		if s > b {
			best = attachment.Images[i]
		}
	}

	return best.Url
}

func vkPollAnswerToPollAnswer(answers []*vkapi.PollAnswer) []PollAnswer {
	var result []PollAnswer

	for i := range answers {
		result = append(result, PollAnswer{
			ID:    answers[i].ID,
			Text:  answers[i].Text,
			Votes: answers[i].Votes,
			Rate:  answers[i].Rate,
		})
	}

	return result
}
