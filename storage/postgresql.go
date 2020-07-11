package storage

import (
	"database/sql"
	"fmt"
	"strings"

	topicToJSON "github.com/crossworth/vk-topic-to-json"
	_ "github.com/lib/pq"
	"github.com/travelaudience/go-sx"
)

type PostgreSQLStorage struct {
	db *sql.DB
}

func NewPostgreSQL(dsn string) (*PostgreSQLStorage, error) {
	postgreSQLStorage := &PostgreSQLStorage{}

	var err error
	postgreSQLStorage.db, err = sql.Open("postgres", dsn)
	if err != nil {
		return postgreSQLStorage, fmt.Errorf("could not create the PostgreSQL client, %v", err)
	}

	err = postgreSQLStorage.db.Ping()
	if err != nil {
		return postgreSQLStorage, fmt.Errorf("could not connect to PostgreSQL, %v", err)
	}

	err = postgreSQLStorage.Migrate()
	if err != nil {
		return postgreSQLStorage, fmt.Errorf("could not migrate the PostgreSQL database, %v", err)
	}

	return postgreSQLStorage, nil
}

func (p *PostgreSQLStorage) Find(topicID int) (topicToJSON.Topic, error) {
	var topic topicToJSON.Topic
	result := p.db.QueryRow(`SELECT id, updated_at FROM topics WHERE id = $1`, topicID)
	err := result.Scan(&topic.ID, &topic.UpdatedAt)

	if err == sql.ErrNoRows {
		return topic, nil
	}

	return topic, err
}

func (p *PostgreSQLStorage) Save(topic topicToJSON.Topic) error {
	// NOTE(Pedro): try insert profile always
	err := sx.Do(p.db, func(tx *sx.Tx) {
		profileQuery := `INSERT INTO profiles VALUES ($1, $2, $3, $4, $5) ON CONFLICT (id) DO UPDATE SET first_name = $2, last_name = $3, screen_name = $4, photo = $5`

		// NOTE(Pedro): This fix an odd behaviour where the profile is not listed
		topic.Profiles[topic.CreatedBy.ID] = topic.CreatedBy
		topic.Profiles[topic.UpdatedBy.ID] = topic.UpdatedBy

		for _, profile := range topic.Profiles {
			tx.MustExec(profileQuery, profile.ID, profile.FirstName, profile.LastName, profile.ScreenName, profile.Photo)
		}
	})

	if err != nil {
		return err
	}

	err = sx.Do(p.db, func(tx *sx.Tx) {
		topicQuery := `INSERT INTO topics VALUES ($1, $2, $3, $4, $5, $6, $7, $8, $9) ON CONFLICT (id) DO UPDATE SET title = $2, is_closed = $3, is_fixed = $4, created_at = $5, updated_at = $6, created_by = $7, updated_by = $8`
		tx.MustExec(topicQuery, topic.ID, topic.Title, topic.IsClosed, topic.IsFixed, topic.CreatedAt, topic.UpdatedAt, topic.CreatedBy.ID, topic.UpdatedBy.ID, false)

		commentQuery := `INSERT INTO comments VALUES ($1, $2, $3, $4, $5, $6, $7, $8, $9) ON CONFLICT (id) DO UPDATE SET date = $3, text = $4, likes = $5, reply_to_uid = $6, reply_to_cid = $7`
		attachmentQuery := `INSERT INTO attachments  VALUES($1, $2) ON CONFLICT (comment_id, content) DO UPDATE SET content = $1, comment_id = $2`

		for _, comment := range topic.Comments {
			tx.MustExec(commentQuery, comment.ID, comment.FromID, comment.Date, comment.Text, comment.Likes, comment.ReplyToUID, comment.ReplyToCID, topic.ID, comment.FromID)

			for _, attachment := range comment.Attachments {
				tx.MustExec(attachmentQuery, attachment, comment.ID)
			}
		}

		pollQuery := `INSERT INTO polls VALUES ($1, $2, $3, $4, $5, $6, $7) ON CONFLICT (id) DO UPDATE SET question = $2, votes = $3, multiple = $4, end_date = $5, closed = $6`
		tx.MustExec(pollQuery, topic.Poll.ID, topic.Poll.Question, topic.Poll.Votes, topic.Poll.Multiple, topic.Poll.EndDate, topic.Poll.Closed, topic.ID)

		pollAnswerQuery := `INSERT INTO poll_answers VALUES ($1, $2, $3, $4, $5) ON CONFLICT (id) DO UPDATE SET text = $2, votes = $3, rate = $4`
		for _, answer := range topic.Poll.Answers {
			tx.MustExec(pollAnswerQuery, answer.ID, answer.Text, answer.Votes, answer.Votes, topic.Poll.ID)
		}
	})

	return err
}

func (p *PostgreSQLStorage) Migrate() error {
	err := sx.Do(p.db, func(tx *sx.Tx) {
		queries := strings.Split(postgreSQLSchema, "\n\n")

		for _, q := range queries {
			q = strings.TrimSpace(q)
			if len(q) > 0 {
				tx.MustExec(q)
			}
		}
	})
	return err
}

const postgreSQLSchema = `
CREATE TABLE IF NOT EXISTS topics (
	id int8 NOT NULL,
	title varchar(500) NOT NULL,
	is_closed bool NOT NULL,
	is_fixed bool NOT NULL,
	created_at int8 NOT NULL,
	updated_at int8 NOT NULL,
	created_by int8 NOT NULL,
	updated_by int8 NOT NULL,
	deleted bool NOT NULL,
	CONSTRAINT "PK_topics" PRIMARY KEY (id)
);

CREATE TABLE IF NOT EXISTS profiles (
	id int8 NOT NULL,
	first_name varchar(250) NOT NULL,
	last_name varchar(250) NOT NULL,
	screen_name varchar(250) NOT NULL,
	photo varchar(250) NOT NULL,
	CONSTRAINT "PK_profiles" PRIMARY KEY (id)
);

CREATE TABLE IF NOT EXISTS profile_names (
	profile_id int8 NOT NULL,
	first_name varchar(250) NOT NULL,
	last_name varchar(250) NOT NULL,
	screen_name varchar(250) NOT NULL,
	photo varchar(250) NOT NULL,
	"date" int8 NOT NULL,
	CONSTRAINT profile_names_pk PRIMARY KEY (profile_id, first_name, last_name, screen_name, photo)
);

CREATE TABLE IF NOT EXISTS polls (
	id int8 NOT NULL,
	question varchar(500) NOT NULL,
	votes int4 NOT NULL,
	multiple bool NOT NULL,
	end_date int8 NOT NULL,
	closed bool NOT NULL,
	topic_id int8 NOT NULL,
	CONSTRAINT "PK_poll" PRIMARY KEY (id)
);

CREATE TABLE IF NOT EXISTS poll_answers (
	id int8 NOT NULL,
	"text" varchar(500) NOT NULL,
	votes int4 NOT NULL,
	rate float4 NOT NULL,
	poll_id int8 NOT NULL,
	CONSTRAINT "PK_poll_answers" PRIMARY KEY (id)
);

CREATE TABLE IF NOT EXISTS "comments" (
	id int8 NOT NULL,
	from_id int8 NOT NULL,
	"date" int8 NOT NULL,
	"text" text NOT NULL,
	likes int4 NOT NULL,
	reply_to_uid int8 NOT NULL,
	reply_to_cid int8 NOT NULL,
	topic_id int8 NOT NULL,
	profile_id int8 NOT NULL,
	CONSTRAINT "PK_comments" PRIMARY KEY (id)
);

CREATE TABLE IF NOT EXISTS attachments (
	"content" text NOT NULL,
	comment_id int8 NOT NULL,
	CONSTRAINT attachments_pkey PRIMARY KEY (comment_id, content)
);

CREATE OR REPLACE FUNCTION insert_profile() RETURNS trigger AS
$$
  BEGIN
    INSERT INTO profile_names 
     (profile_id, first_name, last_name, screen_name, photo, date) 
    VALUES
      (NEW.id, NEW.first_name, NEW.last_name, NEW.screen_name, NEW.photo, extract(epoch from now()))
    ON CONFLICT DO NOTHING;
    RETURN NEW;
  END;
$$
LANGUAGE plpgsql;

DROP TRIGGER IF EXISTS insert_profile_trigger
  ON profiles;

CREATE TRIGGER insert_profile_trigger
AFTER INSERT ON profiles
  FOR EACH ROW EXECUTE PROCEDURE insert_profile();

CREATE OR REPLACE FUNCTION update_profile() RETURNS trigger AS
$$
  BEGIN
    INSERT INTO profile_names 
     (profile_id, first_name, last_name, screen_name, photo, date) 
    VALUES
      (NEW.id, NEW.first_name, NEW.last_name, NEW.screen_name, NEW.photo, extract(epoch from now()))
    ON CONFLICT DO NOTHING;
    RETURN NEW;
  END;
$$
LANGUAGE plpgsql;

DROP TRIGGER IF EXISTS update_profile_trigger
  ON profiles;

CREATE TRIGGER update_profile_trigger
AFTER UPDATE ON profiles
  FOR EACH ROW EXECUTE PROCEDURE update_profile();
`
