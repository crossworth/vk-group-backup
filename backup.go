package vgb

import (
	"context"
	"time"

	vkapi "github.com/himidori/golang-vk-api"
	"go.mongodb.org/mongo-driver/mongo"
	"go.mongodb.org/mongo-driver/mongo/options"
)

type VKGroupBackUp struct {
	groupID            int
	client             *vkapi.VKClient
	pullRecentInterval time.Duration
	mongo              *mongo.Client
	ctx                context.Context
	mongoDatabase      string
}

func New(groupID int, userEmail string, userPassword string) (*VKGroupBackUp, error) {
	vgb := &VKGroupBackUp{
		groupID:            groupID,
		pullRecentInterval: 2 * time.Second,
		ctx:                context.Background(),
	}

	var err error
	vgb.client, err = vkapi.NewVKClient(vkapi.DeviceIPhone, userEmail, userPassword)
	if err != nil {
		return vgb, err
	}

	return vgb, nil
}

func (v *VKGroupBackUp) WithMongo(mongoDSN string, database string) error {
	var err error
	v.mongoDatabase = database
	v.mongo, err = mongo.Connect(v.ctx, options.Client().ApplyURI(mongoDSN))
	if err != nil {
		return err
	}

	return err
}
