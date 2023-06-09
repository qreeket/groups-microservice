package services

import (
	"context"
	"github.com/qcodelabsllc/qreeket/groups/clients"
	pb "github.com/qcodelabsllc/qreeket/groups/generated"
	"github.com/qcodelabsllc/qreeket/groups/utils"
	"go.mongodb.org/mongo-driver/bson"
	"go.mongodb.org/mongo-driver/bson/primitive"
	"go.mongodb.org/mongo-driver/mongo"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/status"
	"google.golang.org/protobuf/types/known/emptypb"
	"google.golang.org/protobuf/types/known/timestamppb"
	"google.golang.org/protobuf/types/known/wrapperspb"
	"log"
)

const (
	channelIconChunkSize = 4 * 1024 * 1024 // 4MB
	channelAudience      = "channel-audience"
	channelSubject       = "channel-subscription-requests"
)

// CreateChannel creates a new channel (completed)
func (s *QreeketGroupServer) CreateChannel(ctx context.Context, req *pb.CreateChannelRequest) (*pb.Channel, error) {
	var iconUrl string
	if len(req.GetIcon()) != 0 {
		mc := clients.CreateMediaClient()
		stream, err := mc.UploadLargeMedia(ctx)
		if err != nil {
			log.Printf("failed to upload media: %v", err)
			return nil, status.Errorf(codes.Internal, "failed to upload media")
		}

		// split the image data into chunks of 4MB
		for {
			if len(req.GetIcon()) == 0 {
				break
			}

			var chunk []byte
			if len(req.GetIcon()) > channelIconChunkSize {
				chunk = req.GetIcon()[:channelIconChunkSize]
				req.Icon = req.GetIcon()[channelIconChunkSize:]
			} else {
				chunk = req.GetIcon()
				req.Icon = nil
			}

			// upload the chunk
			name := req.GetName()
			owner := req.GetOwner()
			mediaReq := &pb.UploadMediaRequest{
				Media: chunk,
				Type:  pb.MediaType_IMAGE,
				Name:  &name,
				Owner: &owner,
			}

			err = stream.Send(mediaReq)
			if err != nil {
				return nil, status.Errorf(codes.Internal, "failed to upload media: %v", err)
			}
		}

		mediaResponse, err := stream.CloseAndRecv()
		if err != nil {
			return nil, status.Errorf(codes.Internal, "failed to upload media")
		}
		iconUrl = mediaResponse.GetUrl()
	}

	subscribers := make([]*pb.Subscriber, 0)
	for _, subscriber := range req.GetSubscribers() {
		subscribers = append(subscribers, subscriber)
	}

	// create the channel
	desc := req.GetDescription()
	icon := iconUrl
	channel := &pb.Channel{
		Id:          req.GetName(),
		Name:        req.GetName(),
		Description: &desc,
		Owner:       req.GetOwner(),
		Subscribers: subscribers,
		Muted:       []string{},
		Type:        req.GetType(),
		Created:     timestamppb.Now(),
		Updated:     timestamppb.Now(),
		Group:       req.GetGroup(),
		Icon:        &icon,
	}

	result, err := s.Channels.InsertOne(ctx, &channel)
	if err != nil {
		return nil, status.Errorf(codes.Internal, "failed to create channel: %v", err)
	}

	// update channel id
	channel.Id = result.InsertedID.(primitive.ObjectID).Hex()
	if _, err = s.Channels.UpdateOne(ctx, bson.M{"id": channel.GetName()}, bson.M{"$set": bson.M{"id": channel.GetId()}}); err != nil {
		return nil, status.Errorf(codes.Internal, "Unable to create channel at the moment. Please try again later.")
	}

	return channel, nil
}

// GetChannelsForGroup gets a channel by id (completed)
func (s *QreeketGroupServer) GetChannelsForGroup(groupId *wrapperspb.StringValue, stream pb.GroupChannelService_GetChannelsForGroupServer) error {
	channels := &pb.ChannelList{}
	ctx := stream.Context()

	// get all channels for group
	cursor, err := s.Channels.Find(ctx, bson.M{"group": groupId.GetValue()})
	if err != nil {
		log.Printf("Failed to get channels for group: %v", err)
		return status.Errorf(codes.Internal, "Failed to get channels for group")
	}

	// iterate over the cursor and send each channel to the client
	for cursor.Next(ctx) {
		var channel *pb.Channel
		if err := cursor.Decode(&channel); err != nil {
			log.Printf("Failed to decode channel: %v", err)
			return status.Errorf(codes.Internal, "failed to decode channel: %v", err)
		}
		channels.Channels = append(channels.GetChannels(), channel)
	}
	_ = stream.Send(channels)

	// TODO: implement change stream for channels
	//channelsStream, err := s.Channels.Watch(ctx, bson.M{
	//	"$match": bson.M{
	//		"operationType": bson.M{
	//			"$in": bson.A{"insert", "delete", "replace"},
	//		},
	//	}},
	//)
	//if err != nil {
	//	log.Printf("Failed to get updates for channels under group: %v", err)
	//	return status.Errorf(codes.Internal, "Failed to get updates for channels under group")
	//}
	//
	//// iterate over the change stream and send each channel to the client
	//for channelsStream.Next(ctx) {
	//	var doc *utils.StreamDoc
	//	if err := channelsStream.Decode(&doc); err != nil {
	//		log.Printf("Failed to decode channel: %v", err)
	//		return status.Errorf(codes.Internal, "Failed to get updates for channel")
	//	}
	//
	//	var channel *pb.Channel
	//	if err = doc.GetFullDocument(&channel); err != nil {
	//		log.Printf("Failed to decode channel: %v", err)
	//		return status.Errorf(codes.Internal, "Failed to get updates for channel")
	//	}
	//	if channel.GetGroup() == groupId.GetValue() {
	//		// check operation type
	//		switch channelsStream.Current.Lookup("operationType").StringValue() {
	//		case "insert":
	//			channels.Channels = append(channels.GetChannels(), channel)
	//		case "delete":
	//			for i, c := range channels.GetChannels() {
	//				if c.GetId() == channel.GetId() {
	//					channels.Channels = append(channels.GetChannels()[:i], channels.GetChannels()[i+1:]...)
	//				}
	//			}
	//		case "replace":
	//		case "update":
	//			for i, c := range channels.GetChannels() {
	//				if c.GetId() == channel.GetId() {
	//					channels.Channels[i] = channel
	//				}
	//			}
	//		}
	//
	//		// send the updated channels to the client
	//		_ = stream.Send(channels)
	//	}
	//}

	return nil
}

// GetChannel returns a channel (completed)
func (s *QreeketGroupServer) GetChannel(ctx context.Context, channelId *wrapperspb.StringValue) (*pb.Channel, error) {
	var channel *pb.Channel
	if err := s.Channels.FindOne(ctx, bson.M{"id": channelId.GetValue()}).Decode(&channel); err != nil {
		return nil, status.Errorf(codes.NotFound, "Channel is no longer available")
	}

	return channel, nil
}

// UpdateChannel updates a channel (completed)
func (s *QreeketGroupServer) UpdateChannel(ctx context.Context, request *pb.Channel) (*pb.Channel, error) {
	// check if channel exists
	count, err := s.Channels.CountDocuments(ctx, bson.M{"id": request.GetId()})
	if err != nil {
		return nil, status.Errorf(codes.Internal, "Failed to update channel")
	}

	if count == 0 {
		return nil, status.Errorf(codes.NotFound, "Channel is no longer available")
	}

	// replace channel with new channel
	request.Updated = timestamppb.Now()
	if _, err = s.Channels.ReplaceOne(ctx, bson.M{"id": request.GetId()}, request); err != nil {
		return nil, status.Errorf(codes.Internal, "Failed to update channel")
	}

	return request, nil
}

// DeleteChannel deletes a channel (completed)
func (s *QreeketGroupServer) DeleteChannel(ctx context.Context, request *wrapperspb.StringValue) (*emptypb.Empty, error) {
	// find channel and delete it
	if err := s.Channels.FindOneAndDelete(ctx, bson.M{"id": request.GetValue()}).Decode(bson.D{}); err != nil {
		return nil, status.Errorf(codes.NotFound, "Channel is no longer available")
	}

	return &emptypb.Empty{}, nil
}

func (s *QreeketGroupServer) LeaveChannel(ctx context.Context, request *pb.ManageGroupOrChannel) (*emptypb.Empty, error) {

	// check if channel exists
	channel, err := s.GetChannel(ctx, &wrapperspb.StringValue{Value: request.GetChannel()})
	if err != nil {
		return nil, status.Errorf(codes.NotFound, "Channel is no longer available")
	}

	// check if user is a subscriber of the channel
	for i, subscriber := range channel.GetSubscribers() {
		// remove user from subscribers list
		if subscriber.GetId() == request.GetUser() {
			channel.Subscribers = append(channel.GetSubscribers()[:i], channel.GetSubscribers()[i+1:]...)
			break
		}
	}

	// update channel
	if _, err = s.Channels.ReplaceOne(ctx, bson.M{"id": channel.GetId()}, channel); err != nil {
		return nil, status.Errorf(codes.Internal, "Failed to update channel")
	}

	return &emptypb.Empty{}, nil
}

// SubscribeToChannel subscribes a user to a channel (completed)
func (s *QreeketGroupServer) SubscribeToChannel(ctx context.Context, request *pb.ChannelSubscriptionRequest) (*emptypb.Empty, error) {
	// check if channel exists
	channel, err := s.GetChannel(ctx, &wrapperspb.StringValue{Value: request.GetChannel()})
	if err != nil {
		return nil, status.Errorf(codes.NotFound, "Channel is no longer available")
	}

	// check if user is already subscribed
	for _, subscriber := range channel.GetSubscribers() {
		if subscriber.GetId() == request.GetUser() {
			return nil, status.Errorf(codes.AlreadyExists, "User is already subscribed to channel")
		}
	}

	// check if the user has already requested to subscribe to the channel
	count, err := s.Subscriptions.CountDocuments(ctx, bson.M{
		"channel": channel.GetId(),
		"user":    request.GetUser(),
	})
	if err != nil {
		return nil, status.Errorf(codes.Internal, "Failed to subscribe to channel")
	}

	if count > 0 {
		return nil, status.Errorf(codes.AlreadyExists, "User has already requested to subscribe to channel")
	}

	// create token
	token, err := utils.GenerateToken(channelAudience, channelSubject, map[string]string{
		"channel": channel.GetId(),
		"user":    request.GetUser(),
	})
	if err != nil {
		return nil, status.Errorf(codes.Internal, "Failed to subscribe to channel")
	}

	// create a new subscription request for the user
	subscription := &pb.ChannelSubscription{
		Channel: channel,
		User:    request.GetUser(),
		Token:   *token,
		Status:  pb.ChannelOrGroupInviteStatus_pending,
	}

	result, err := s.Subscriptions.InsertOne(ctx, subscription)
	if err != nil {
		return nil, status.Errorf(codes.Internal, "Failed to subscribe to channel")
	}

	subscription.Id = result.InsertedID.(primitive.ObjectID).Hex()

	// update subscription request in the channel
	_, err = s.Subscriptions.UpdateOne(ctx, bson.M{"_id": result.InsertedID}, bson.M{"$set": bson.M{"id": subscription.GetId()}})
	if err != nil {
		return nil, status.Errorf(codes.Internal, "Failed to subscribe to channel")
	}

	return &emptypb.Empty{}, nil
}

// UnsubscribeFromChannel unsubscribes a user from a channel (completed)
func (s *QreeketGroupServer) UnsubscribeFromChannel(ctx context.Context, request *pb.ChannelSubscriptionRequest) (*emptypb.Empty, error) {
	// check if channel exists
	count, err := s.Channels.CountDocuments(ctx, bson.M{"id": request.GetChannel()})
	if err != nil {
		return nil, status.Errorf(codes.Internal, "Failed to unsubscribe from channel")
	}

	if count == 0 {
		return nil, status.Errorf(codes.NotFound, "Channel is no longer available")
	}

	// check if user is subscribed to channel
	channel, err := s.GetChannel(ctx, &wrapperspb.StringValue{Value: request.GetChannel()})
	if err != nil {
		return nil, status.Errorf(codes.NotFound, "Channel is no longer available")
	}

	// remove user from subscribers list
	for i, subscriber := range channel.GetSubscribers() {
		if subscriber.GetId() == request.GetUser() {
			channel.Subscribers = append(channel.GetSubscribers()[:i], channel.GetSubscribers()[i+1:]...)
			break
		}
	}

	// update channel
	if _, err = s.Channels.UpdateOne(ctx, bson.M{"id": channel.GetId()}, bson.M{"$set": bson.M{"subscribers": channel.GetSubscribers()}}); err != nil {
		return nil, status.Errorf(codes.Internal, "Failed to unsubscribe from channel")
	}

	return &emptypb.Empty{}, nil
}

// ManageChannelSubscription manages a channel subscription (completed)
func (s *QreeketGroupServer) ManageChannelSubscription(ctx context.Context, request *pb.ManageChannelSubscriptionRequest) (*emptypb.Empty, error) {
	// check token validity
	valid, err := utils.ValidateToken(request.GetToken())
	if err != nil {
		return nil, status.Errorf(codes.InvalidArgument, "Failed to manage channel subscription")
	}

	if !valid {
		return nil, status.Errorf(codes.InvalidArgument, "Your request has expired. Make a new request to subscribe to channel")
	}

	// check if the user has already requested to subscribe to the channel
	count, err := s.Subscriptions.CountDocuments(ctx, bson.M{
		"channel.id": request.GetChannel(),
		"user":       request.GetUser(),
	})

	if err != nil {
		return nil, status.Errorf(codes.Internal, "Failed to manage channel subscription")
	}

	if count == 0 {
		return nil, status.Errorf(codes.NotFound, "Make a request to subscribe to channel first")
	}

	// check if channel exists
	count, err = s.Channels.CountDocuments(ctx, bson.M{"id": request.GetChannel()})
	if err != nil {
		return nil, status.Errorf(codes.Internal, "Failed to manage channel subscription")
	}

	if count == 0 {
		return nil, status.Errorf(codes.NotFound, "Channel is no longer available")
	}

	// get user and channel from token
	userId, err := utils.DecryptToken(request.GetToken(), "user")
	if err != nil {
		return nil, status.Errorf(codes.InvalidArgument, "Failed to manage channel subscription")
	}
	channelId, err := utils.DecryptToken(request.GetToken(), "channel")
	if err != nil {
		return nil, status.Errorf(codes.InvalidArgument, "Failed to manage channel subscription")
	}

	// check if token is for the right user
	if *userId != request.GetUser() || *channelId != request.GetChannel() {
		return nil, status.Errorf(codes.InvalidArgument, "Your request has expired. Make a new request to subscribe to channel")
	}

	// check if token is for the right channel
	if !valid {
		return nil, status.Errorf(codes.InvalidArgument, "Your request has expired. Make a new request to subscribe to channel")
	}

	// check if user is subscribed to channel
	channel, err := s.GetChannel(ctx, &wrapperspb.StringValue{Value: request.GetChannel()})
	if err != nil {
		return nil, status.Errorf(codes.NotFound, "Channel is no longer available")
	}

	// check if user is subscribed to channel
	for _, subscriber := range channel.GetSubscribers() {
		if subscriber.GetId() == request.GetUser() {
			return nil, status.Errorf(codes.AlreadyExists, "User is already subscribed to channel")
		}
	}

	// update subscription request in the channel
	if request.GetAccept() {
		// get user account
		ac := clients.CreateAuthClient()
		account, err := ac.GetAccountById(utils.CreateContextFromRequestMetadata(ctx), &wrapperspb.StringValue{Value: request.GetUser()})
		if err != nil {
			return nil, status.Errorf(codes.Internal, "Failed to manage channel subscription")
		}

		subscriber := &pb.Subscriber{
			Id:     account.GetId(),
			Name:   account.GetUsername(),
			Avatar: account.GetAvatarUrl(),
		}

		// add user to subscribers list
		go func(groupId string, subscriber *pb.Subscriber) {
			ctx = context.Background()
			_, err = s.Channels.UpdateOne(ctx, bson.M{"id": request.GetChannel()}, bson.M{"$push": bson.M{"subscribers": subscriber}})
			if err != nil {
				log.Println(err)
			}

			// check if subscriber is already in the group
			count, err = s.Groups.CountDocuments(ctx, bson.M{"id": groupId, "subscribers": bson.M{"$elemMatch": bson.M{"id": subscriber.GetId()}}})
			if err != nil {
				log.Println(err)
			}

			// add subscriber to group if not already in group
			if count == 0 {
				_, err = s.Groups.UpdateOne(ctx, bson.M{"id": groupId}, bson.M{"$push": bson.M{"subscribers": subscriber}})
				if err != nil {
					log.Println(err)
				}
			}

		}(channel.GetGroup(), subscriber)

		// update subscription request in the channel status to accepted
		_, err = s.Subscriptions.UpdateOne(ctx, bson.M{"channel.id": request.GetChannel(), "user": request.GetUser()}, bson.M{"$set": bson.M{"status": pb.ChannelOrGroupInviteStatus_accepted}})
		if err != nil {
			return nil, status.Errorf(codes.Internal, "Failed to manage channel subscription")
		}

	} else {
		// decline subscription request
		_, err = s.Subscriptions.UpdateOne(ctx, bson.M{"channel": request.GetChannel(), "user": request.GetUser()}, bson.M{"$set": bson.M{"status": pb.ChannelOrGroupInviteStatus_declined}})
		if err != nil {
			return nil, status.Errorf(codes.Internal, "Failed to manage channel subscription")
		}
	}

	return &emptypb.Empty{}, nil
}

// GetChannelSubscriptionsForUser gets channel subscriptions for a user (completed)
func (s *QreeketGroupServer) GetChannelSubscriptionsForUser(userId *wrapperspb.StringValue, stream pb.GroupChannelService_GetChannelSubscriptionsForUserServer) error {
	ctx := stream.Context()

	// check if user exists
	ac := clients.CreateAuthClient()
	_, err := ac.GetAccountById(utils.CreateContextFromRequestMetadata(ctx), userId)
	if err != nil {
		return status.Errorf(codes.PermissionDenied, "You have no subscriptions available")
	}

	// get subscriptions
	cursor, err := s.Subscriptions.Find(ctx, bson.M{"user": userId.GetValue()})
	if err != nil {
		return status.Errorf(codes.Internal, "Failed to get subscriptions")
	}

	defer func(cursor *mongo.Cursor, ctx context.Context) {
		_ = cursor.Close(ctx)
	}(cursor, ctx)

	response := &pb.ChannelSubscriptionList{}
	for cursor.Next(ctx) {
		var subscription *pb.ChannelSubscription
		if err := cursor.Decode(&subscription); err != nil {
			return status.Errorf(codes.Internal, "Failed to get subscriptions")
		}

		// get channel
		channel, err := s.GetChannel(ctx, &wrapperspb.StringValue{Value: subscription.GetChannel().GetId()})
		if err != nil {
			return status.Errorf(codes.Internal, "Failed to get subscriptions")
		}
		subscription.Channel = channel

		response.Subscriptions = append(response.GetSubscriptions(), subscription)
	}

	if err := stream.Send(response); err != nil {
		return status.Errorf(codes.Internal, "Failed to get subscriptions")
	}

	// TODO implement change stream to update subscriptions in real time

	return nil
}

// MuteChannel mutes a group
// todo
func (s *QreeketGroupServer) MuteChannel(ctx context.Context, request *pb.ManageGroupOrChannelRequest) (*emptypb.Empty, error) {

	// todo implement this
	return &emptypb.Empty{}, nil
}

// UnmuteChannel unmutes a group
// todo
func (s *QreeketGroupServer) UnmuteChannel(ctx context.Context, request *pb.ManageGroupOrChannelRequest) (*emptypb.Empty, error) {

	// todo implement this
	return &emptypb.Empty{}, nil
}
