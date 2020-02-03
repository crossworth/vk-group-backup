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
	mongoDSN           string
	mongoDatabase      string
	device             int
}

type Option interface {
	apply(*VKGroupBackUp)
}

type mongoOptions struct {
	mongoDSN string
	database string
}

func WithMongo(mongoDSN string, database string) Option {
	return mongoOptions{
		mongoDSN: mongoDSN,
		database: database,
	}
}

func (m mongoOptions) apply(v *VKGroupBackUp) {
	v.mongoDSN = m.mongoDSN
	v.mongoDatabase = m.database
}

type deviceOption int

func WithDevice(option int) Option {
	return deviceOption(option)
}

func (d deviceOption) apply(v *VKGroupBackUp) {
	v.device = int(d)
}

func New(groupID int, userEmail string, userPassword string, opts ...Option) (*VKGroupBackUp, error) {
	vgb := &VKGroupBackUp{
		groupID:            groupID,
		pullRecentInterval: 2 * time.Second,
		ctx:                context.Background(),
		device:             vkapi.DeviceIPhone,
	}

	for _, o := range opts {
		o.apply(vgb)
	}

	var err error
	vgb.client, err = vkapi.NewVKClient(vgb.device, userEmail, userPassword)
	if err != nil {
		return vgb, err
	}

	if vgb.mongoDSN != "" {
		vgb.mongo, err = mongo.Connect(vgb.ctx, options.Client().ApplyURI(vgb.mongoDSN))
		if err != nil {
			return vgb, err
		}

		err = vgb.mongo.Ping(vgb.ctx, nil)
		if err != nil {
			return vgb, err
		}
	}

	return vgb, nil
}
