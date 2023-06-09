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
	"google.golang.org/protobuf/types/known/wrapperspb"
	"log"
)

const (
	audience = "qreeket-group-invites"
	subject  = "invites"
)

// InviteToGroup invites a user to a group (completed)
func (s *QreeketGroupServer) InviteToGroup(ctx context.Context, req *pb.GroupInviteRequest) (*emptypb.Empty, error) {

	// get all channels from invite request
	channels := req.GetChannels()

	if len(channels) == 0 {
		return nil, status.Errorf(codes.InvalidArgument, "No channels specified")
	}

	// query the database for the channels, and make sure they exist
	channelIds := make([]primitive.ObjectID, len(channels))
	for i, channel := range channels {
		channelId, err := primitive.ObjectIDFromHex(channel)
		if err != nil {
			return nil, status.Errorf(codes.InvalidArgument, "Channel is no longer available")
		}

		// check if the channel exists
		_, err = s.Channels.FindOne(ctx, bson.M{"_id": channelId}).DecodeBytes()
		if err != nil {
			return nil, status.Errorf(codes.NotFound, "Channel is no longer available")
		}
		channelIds[i] = channelId
	}

	// check if the user is part of the admins list / part of the subscribers list for the group
	groupId, err := primitive.ObjectIDFromHex(req.GetGroup())
	if err != nil {
		return nil, status.Errorf(codes.InvalidArgument, "Group is no longer available")
	}

	// check if the user is part of the admins list
	count, err := s.Groups.CountDocuments(ctx, bson.M{"_id": groupId, "admins": bson.M{"$in": []string{req.GetUser()}}})
	if err != nil {
		return nil, status.Errorf(codes.Internal, "We are unable to process your request at this time")
	}

	if count != 0 {
		return nil, status.Errorf(codes.FailedPrecondition, "User is an admin of the group")
	}

	// check if the user is part of the subscribers list
	count, err = s.Groups.CountDocuments(ctx, bson.M{"_id": groupId, "subscribers": bson.M{"$elemMatch": bson.M{"id": req.GetUser()}}})
	if err != nil {
		return nil, status.Errorf(codes.Internal, "We are unable to process your request at this time")
	}

	if count != 0 {
		return nil, status.Errorf(codes.FailedPrecondition, "User is already a subscriber of the group")
	}

	// check if invite already exists for the user for the same group and status is pending
	count, err = s.Invites.CountDocuments(ctx, bson.M{
		"user":     req.GetUser(),
		"group.id": req.GetGroup(),
		"status":   pb.ChannelOrGroupInviteStatus_pending,
	})
	if err != nil {
		return nil, status.Errorf(codes.Internal, "We are unable to process your request at this time")
	}

	if count != 0 {
		return nil, status.Errorf(codes.FailedPrecondition, "Invite already exists for the user")
	}

	// generate token for the invite
	token, err := utils.GenerateToken(audience, subject, map[string]string{"user": req.GetUser(), "group": req.GetGroup()})
	if err != nil {
		return nil, status.Errorf(codes.Internal, "We are unable to process your request at this time")
	}

	// get group
	var group *pb.Group
	if err = s.Groups.FindOne(ctx, bson.M{"_id": groupId}).Decode(&group); err != nil {
		if err == mongo.ErrNoDocuments {
			return nil, status.Errorf(codes.NotFound, "Group is no longer available")
		}

		return nil, status.Errorf(codes.Internal, "We are unable to process your request at this time")
	}

	// create the invite
	invite := &pb.GroupInvite{
		Channels: channels,
		Admin:    req.GetAdmin(),
		Token:    *token, // generate a random token for the invite
		Status:   pb.ChannelOrGroupInviteStatus_pending,
		Group:    group,
		User:     req.GetUser(),
	}

	// insert the invite into the database
	result, err := s.Invites.InsertOne(ctx, invite)
	if err != nil {
		return nil, status.Errorf(codes.Internal, "Failed to create invite")
	}

	// set the invite id
	invite.Id = result.InsertedID.(primitive.ObjectID).Hex()

	// replace the invite in the database
	_, err = s.Invites.ReplaceOne(ctx, bson.M{"_id": result.InsertedID}, invite)
	if err != nil {
		return nil, status.Errorf(codes.Internal, "Failed to create invite")
	}

	// FIXME: send invite email / notification

	return &emptypb.Empty{}, nil
}

// GetGroupInvitesForUser gets all invites for a user (TODO: implement change stream)
func (s *QreeketGroupServer) GetGroupInvitesForUser(userId *wrapperspb.StringValue, stream pb.GroupChannelService_GetGroupInvitesForUserServer) error {
	ctx := stream.Context()
	invites := &pb.GroupInviteList{}

	// get user id
	userIdStr := userId.GetValue()

	// get all invites for the user & status is pending
	cursor, err := s.Invites.Find(context.Background(), bson.M{"user": userIdStr, "status": pb.ChannelOrGroupInviteStatus_pending.String()})
	if err != nil {
		return status.Errorf(codes.Internal, "We are unable to process your request at this time")
	}

	defer func(cursor *mongo.Cursor, ctx context.Context) {
		_ = cursor.Close(ctx)
	}(cursor, ctx)

	// iterate over the cursor
	for cursor.Next(ctx) {
		var invite *pb.GroupInvite
		// decode the invite
		err := cursor.Decode(&invite)
		if err != nil {
			return status.Errorf(codes.Internal, "We are unable to process your request at this time")
		}

		// append the invite to the list
		invites.Invites = append(invites.GetInvites(), invite)
	}

	// send the invite to the client
	if err = stream.Send(invites); err != nil {
		return status.Errorf(codes.Internal, "We are unable to process your request at this time")
	}

	// TODO: implement change stream
	// watch for changes to the invites collection
	//changeStream, err := s.Invites.Watch(ctx, mongo.Pipeline{
	//	{{"$match", bson.M{
	//		"operationType": bson.M{
	//			"$in": []string{"delete", "replace", "insert"},
	//		},
	//	}}},
	//}, options.ChangeStream().SetFullDocument(options.UpdateLookup))
	//if err != nil {
	//	return status.Errorf(codes.Internal, "We are unable to process your request at this time")
	//}
	//
	//defer func(changeStream *mongo.ChangeStream, ctx context.Context) {
	//	_ = changeStream.Close(ctx)
	//}(changeStream, ctx)
	//
	//// iterate over the change stream
	//for changeStream.Next(ctx) {
	//	var doc *utils.StreamDoc
	//	if err := cursor.Decode(&doc); err != nil {
	//		if err == io.EOF {
	//			break
	//		}
	//		return status.Errorf(codes.Internal, "We are unable to process your request at this time")
	//	}
	//
	//	// send the invite to the client
	//	var invite *pb.GroupInvite
	//	if err = doc.GetFullDocument(&invite); err != nil {
	//		return status.Errorf(codes.Internal, "We are unable to process your request at this time")
	//	}
	//
	//	// check if the document was deleted
	//	if doc.OperationType == "delete" {
	//		// remove the invite from the list
	//		for i, v := range invites.Invites {
	//			if v.Id == changeStream.Current.Lookup("_id").ObjectID().Hex() {
	//				invites.Invites = append(invites.Invites[:i], invites.Invites[i+1:]...)
	//				break
	//			}
	//		}
	//
	//		if err = stream.Send(invites); err != nil {
	//			return status.Errorf(codes.Internal, "We are unable to process your request at this time")
	//		}
	//
	//		continue
	//	}
	//
	//	// check if the invite is for the user from the stream lookup result
	//	if invite.GetUser() != userIdStr {
	//		continue
	//	}
	//
	//	switch changeStream.Current.Lookup("operationType").StringValue() {
	//	case "insert":
	//		invites.Invites = append(invites.GetInvites(), invite)
	//	case "update":
	//	case "replace":
	//		for i, v := range invites.Invites {
	//			if v.Id == invite.Id {
	//				invites.Invites[i] = invite
	//				break
	//			}
	//		}
	//	}
	//
	//	if err = stream.Send(invites); err != nil {
	//		return status.Errorf(codes.Internal, "We are unable to process your request at this time")
	//	}
	//}

	return nil
}

// GetGroupInvitesForGroup gets all invites for a group (TODO: implement change stream)
func (s *QreeketGroupServer) GetGroupInvitesForGroup(groupId *wrapperspb.StringValue, stream pb.GroupChannelService_GetGroupInvitesForGroupServer) error {
	ctx := stream.Context()
	invites := &pb.GroupInviteList{}

	// get group id
	groupIdStr := groupId.GetValue()

	// get all invites for the group
	cursor, err := s.Invites.Find(context.Background(), bson.M{"group.id": groupIdStr})
	if err != nil {
		return status.Errorf(codes.Internal, "We are unable to process your request at this time")
	}

	defer func(cursor *mongo.Cursor, ctx context.Context) {
		_ = cursor.Close(ctx)
	}(cursor, ctx)

	// iterate over the cursor
	for cursor.Next(ctx) {
		var invite *pb.GroupInvite
		// decode the invite
		err := cursor.Decode(&invite)
		if err != nil {
			return status.Errorf(codes.Internal, "We are unable to process your request at this time")
		}

		// append the invite to the list
		invites.Invites = append(invites.GetInvites(), invite)
	}

	// send the invite to the client
	if err = stream.Send(invites); err != nil {
		return status.Errorf(codes.Internal, "We are unable to process your request at this time")
	}

	// TODO: implement change stream
	// watch for changes to the invites collection
	//changeStream, err := s.Invites.Watch(ctx, mongo.Pipeline{
	//	{{"$match", bson.M{
	//		"operationType": bson.M{
	//			"$in": []string{"delete", "replace", "insert"},
	//		},
	//	}}},
	//}, options.ChangeStream().SetFullDocument(options.UpdateLookup))
	//if err != nil {
	//	return status.Errorf(codes.Internal, "We are unable to process your request at this time")
	//}
	//
	//defer func(changeStream *mongo.ChangeStream, ctx context.Context) {
	//	_ = changeStream.Close(ctx)
	//}(changeStream, ctx)
	//
	//// iterate over the change stream
	//for changeStream.Next(ctx) {
	//	var doc *utils.StreamDoc
	//	if err := cursor.Decode(&doc); err != nil {
	//		if err == io.EOF {
	//			break
	//		}
	//		return status.Errorf(codes.Internal, "We are unable to process your request at this time")
	//	}
	//
	//	// send the invite to the client
	//	var invite *pb.GroupInvite
	//	if err = doc.GetFullDocument(&invite); err != nil {
	//		return status.Errorf(codes.Internal, "We are unable to process your request at this time")
	//	}
	//
	//	// check if the document was deleted
	//	if doc.OperationType == "delete" {
	//		// remove the invite from the list
	//		for i, v := range invites.Invites {
	//			if v.Id == changeStream.Current.Lookup("_id").ObjectID().Hex() {
	//				invites.Invites = append(invites.Invites[:i], invites.Invites[i+1:]...)
	//				break
	//			}
	//		}
	//
	//		if err = stream.Send(invites); err != nil {
	//			return status.Errorf(codes.Internal, "We are unable to process your request at this time")
	//		}
	//
	//		continue
	//	}
	//
	//	// check if the invite is for the group
	//	if invite.Group.GetId() != groupIdStr {
	//		continue
	//	}
	//
	//	switch changeStream.Current.Lookup("operationType").StringValue() {
	//	case "insert":
	//		invites.Invites = append(invites.GetInvites(), invite)
	//	case "update":
	//	case "replace":
	//		for i, v := range invites.Invites {
	//			if v.Id == invite.Id {
	//				invites.Invites[i] = invite
	//				break
	//			}
	//		}
	//	case "delete":
	//		for i, v := range invites.Invites {
	//			if v.Id == invite.Id {
	//				invites.Invites = append(invites.Invites[:i], invites.Invites[i+1:]...)
	//				break
	//			}
	//		}
	//	}
	//
	//	if err = stream.Send(invites); err != nil {
	//		return status.Errorf(codes.Internal, "We are unable to process your request at this time")
	//	}
	//}

	return nil
}

// RevokeGroupInvite allows any admin of a group to revoke a group invite (completed)
func (s *QreeketGroupServer) RevokeGroupInvite(ctx context.Context, request *pb.RevokeGroupInviteRequest) (*emptypb.Empty, error) {

	// check if the invite exists
	var invite *pb.GroupInvite
	if err := s.Invites.FindOne(ctx, bson.M{"id": request.GetInviteId()}).Decode(&invite); err != nil {
		if err == mongo.ErrNoDocuments {
			return nil, status.Errorf(codes.NotFound, "Invite not found")
		}

		return nil, status.Errorf(codes.Internal, "We are unable to process your request at this time")
	}

	// check if the token is valid
	if invite.GetToken() != request.GetToken() {
		return nil, status.Errorf(codes.InvalidArgument, "Invalid token")
	}

	// decrypt the token (this returns a map[string]string containing the `group` id)
	groupId, err := utils.DecryptToken(request.GetToken(), "group")
	if err != nil {
		return nil, status.Errorf(codes.InvalidArgument, "Invalid token")
	}

	// check if the admin is one of the admins of the group
	count, err := s.Groups.CountDocuments(ctx, bson.M{"id": groupId, "admins": bson.M{"$in": []string{request.GetAdmin()}}})
	if err != nil {
		return nil, status.Errorf(codes.Internal, "We are unable to process your request at this time")
	}

	if count == 0 {
		return nil, status.Errorf(codes.PermissionDenied, "You do not have permission to revoke this invite")
	}

	// delete the invite
	_, err = s.Invites.DeleteOne(ctx, bson.M{"id": request.GetInviteId()})
	if err != nil {
		return nil, status.Errorf(codes.Internal, "We are unable to process your request at this time")
	}

	return &emptypb.Empty{}, nil
}

// ManageGroupInvite allows the user of the invite to accept or reject a group invite (completed)
func (s *QreeketGroupServer) ManageGroupInvite(ctx context.Context, req *pb.ManageGroupInviteRequest) (*emptypb.Empty, error) {
	// check if token has expired
	valid, err := utils.ValidateToken(req.GetToken())
	if err != nil {
		return nil, status.Errorf(codes.InvalidArgument, "Invalid token")
	}

	if !valid {
		return nil, status.Errorf(codes.InvalidArgument, "This invite has expired. Please request a new invite")
	}

	// check if the invite exists
	count, err := s.Invites.CountDocuments(ctx, bson.M{"id": req.GetInviteId()})
	if err != nil {
		return nil, status.Errorf(codes.Internal, "We are unable to process your request at this time")
	}

	if count == 0 {
		return nil, status.Errorf(codes.NotFound, "Invite not found")
	}

	// check if all channels in request exist
	for _, v := range req.GetChannels() {
		count, err = s.Channels.CountDocuments(ctx, bson.M{"id": v})
		if err != nil {
			return nil, status.Errorf(codes.Internal, "We are unable to process your request at this time")
		}

		if count == 0 {
			return nil, status.Errorf(codes.NotFound, "Channel not found")
		}
	}

	// check if the token is valid
	groupId, err := utils.DecryptToken(req.GetToken(), "group")
	if err != nil {
		return nil, status.Errorf(codes.InvalidArgument, "Invalid token")
	}

	userId, err := utils.DecryptToken(req.GetToken(), "user")
	if err != nil {
		return nil, status.Errorf(codes.InvalidArgument, "Invalid token")
	}

	// check if the user is already a member of the group
	count, err = s.Groups.CountDocuments(ctx, bson.M{"id": groupId, "subscribers": bson.M{"$elemMatch": bson.M{"id": userId}}})
	if err != nil {
		return nil, status.Errorf(codes.Internal, "We are unable to process your request at this time")
	}

	if count > 0 {
		return nil, status.Errorf(codes.AlreadyExists, "You are already a member of this group")
	}

	// get the user information
	ac := clients.CreateAuthClient()
	account, err := ac.GetAccountById(utils.CreateContextFromRequestMetadata(ctx), &wrapperspb.StringValue{
		Value: *userId,
	})
	if err != nil {
		log.Printf("Error getting user account: %v", err)
		return nil, status.Errorf(codes.Internal, "We are unable to process your request at this time")
	}

	// create a new subscriber
	subscriber := &pb.Subscriber{
		Id:     *userId,
		Name:   account.GetUsername(),
		Avatar: account.GetAvatarUrl(),
		Online: account.GetIsVisible(),
	}

	// allow user to accept or decline the invite
	if req.GetAccept() {

		count, err = s.Groups.CountDocuments(ctx, bson.M{"id": groupId})
		if err != nil {
			return nil, status.Errorf(codes.Internal, "We are unable to process your request at this time")
		}

		if count == 0 {
			return nil, status.Errorf(codes.NotFound, "Group not found")
		}

		// add the user to the group
		_, err = s.Groups.UpdateOne(ctx, bson.M{"id": groupId}, bson.M{"$push": bson.M{"subscribers": &subscriber}})
		if err != nil {
			return nil, status.Errorf(codes.Internal, "We are unable to process your request at this time")
		}

		// update invite status
		go func(id string) {
			ctx = context.Background()
			_, err = s.Invites.UpdateOne(ctx, bson.M{"id": id}, bson.M{"$set": bson.M{"status": pb.ChannelOrGroupInviteStatus_accepted}})
		}(req.GetInviteId())

		// update channels with new subscriber
		go func(id *string, channels []string, subscriber *pb.Subscriber) {
			ctx = context.Background()
			// get the channels
			cursor, err := s.Channels.Find(ctx, bson.M{"group": id, "id": bson.M{"$in": channels}})
			if err != nil {
				log.Printf("Error getting channels: %v", err)
				return
			}

			defer func(cursor *mongo.Cursor, ctx context.Context) {
				_ = cursor.Close(ctx)
			}(cursor, ctx)

			for cursor.Next(ctx) {
				var channel *pb.Channel
				if err = cursor.Decode(&channel); err != nil {
					log.Printf("Error decoding channel: %v", err)
					continue
				}

				// check if the user is already a member of the channel
				count, err = s.Channels.CountDocuments(ctx, bson.M{"id": channel.GetId(), "subscribers": bson.M{"$elemMatch": bson.M{"id": userId}}})
				if err != nil {
					log.Printf("Error checking if user is a member of the channel: %v", err)
					continue
				}

				if count > 0 {
					continue
				}

				// add the user to the channel
				_, err = s.Channels.UpdateOne(ctx, bson.M{"id": channel.GetId()}, bson.M{"$push": bson.M{"subscribers": &subscriber}})
				if err != nil {
					log.Printf("Error adding user to channel: %v", err)
					continue
				}
			}
		}(groupId, req.GetChannels(), subscriber)

	} else {
		// update invite status
		_, err = s.Invites.UpdateOne(ctx, bson.M{"id": req.GetInviteId()}, bson.M{"$set": bson.M{"status": pb.ChannelOrGroupInviteStatus_declined}})
		if err != nil {
			return nil, status.Errorf(codes.Internal, "We are unable to process your request at this time")
		}
	}

	return &emptypb.Empty{}, nil
}
