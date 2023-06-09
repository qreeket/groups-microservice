package services

import (
	pb "github.com/qcodelabsllc/qreeket/groups/generated"
	"go.mongodb.org/mongo-driver/mongo"
)

// QreeketGroupServer is the implementation of the GroupChannelServiceServer interface
type QreeketGroupServer struct {
	pb.UnimplementedGroupChannelServiceServer
	Groups        *mongo.Collection
	Channels      *mongo.Collection
	Invites       *mongo.Collection
	Subscriptions *mongo.Collection
}

func NewQreeketGroupServer(groupsCollection *mongo.Collection, channelsCollection *mongo.Collection, invitesCollection *mongo.Collection, subscriptionCollection *mongo.Collection) *QreeketGroupServer {
	return &QreeketGroupServer{
		Groups:        groupsCollection,
		Channels:      channelsCollection,
		Invites:       invitesCollection,
		Subscriptions: subscriptionCollection,
	}
}
