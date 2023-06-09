package services

import (
	"context"
	"encoding/base64"
	"errors"
	"fmt"
	"github.com/qcodelabsllc/qreeket/groups/clients"
	pb "github.com/qcodelabsllc/qreeket/groups/generated"
	"github.com/qcodelabsllc/qreeket/groups/utils"
	"go.mongodb.org/mongo-driver/bson"
	"go.mongodb.org/mongo-driver/bson/primitive"
	"go.mongodb.org/mongo-driver/mongo/options"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/status"
	"google.golang.org/protobuf/types/known/emptypb"
	"google.golang.org/protobuf/types/known/timestamppb"
	"google.golang.org/protobuf/types/known/wrapperspb"
	"io/ioutil"
	"log"
	"math/rand"
	"path/filepath"
	"strings"
	"time"
)

const groupIconChunkSize = 4 * 1024 * 1024 // 4MB

// CreateGroup creates a new group (completed)
func (s *QreeketGroupServer) CreateGroup(ctx context.Context, req *pb.CreateGroupRequest) (*pb.Group, error) {
	// check if group name already exists for the admin
	if count, err := s.Groups.CountDocuments(ctx, bson.M{
		"name": req.GetName(),
		"admins": bson.M{
			"$in": []string{req.GetAdmin()},
		},
	}); err != nil {
		return nil, status.Errorf(codes.Internal, "Unable to create group at the moment. Please try again later.")
	} else if count != 0 {
		return nil, status.Errorf(codes.AlreadyExists, "Group already exists")
	}

	// get admin account details
	var subscriber *pb.Subscriber
	ac := clients.CreateAuthClient()
	if account, err := ac.GetAccountById(utils.CreateContextFromRequestMetadata(ctx), &wrapperspb.StringValue{
		Value: req.GetAdmin(),
	}); err != nil {
		log.Printf("Unable to create group at the moment. Please try again later. %v", err)
		return nil, status.Errorf(codes.Internal, "Unable to create group at the moment. Please try again later.")
	} else {
		subscriber = &pb.Subscriber{
			Id:     account.GetId(),
			Name:   account.GetUsername(),
			Avatar: account.GetAvatarUrl(),
			Online: account.GetIsVisible(),
		}
	}

	name := req.GetName()
	// upload media file
	mc := clients.CreateMediaClient()
	stream, err := mc.UploadLargeMedia(ctx)
	if err != nil {
		// connection error
		return nil, status.Errorf(codes.Internal, "Unable to create group at the moment. Please try again later.")
	}

	// upload the group icon
	if len(req.GetIcon()) == 0 {
		// randomize the group icon from a folder of icons
		if icon, err := createIconFromFolder(); err != nil {
			return nil, status.Errorf(codes.Internal, "Unable to create group at the moment. Please try again later.")
		} else {
			// upload the chunk
			mediaReq := &pb.UploadMediaRequest{
				Media: *icon,
				Type:  pb.MediaType_IMAGE,
				Name:  &name,
			}

			if err = stream.Send(mediaReq); err != nil {
				return nil, status.Errorf(codes.Internal, "failed to upload media: %v", err)
			}
		}
	} else {
		for {
			if len(req.GetIcon()) == 0 {
				break
			}

			var chunk []byte
			if len(req.GetIcon()) > groupIconChunkSize {
				chunk = req.GetIcon()[:groupIconChunkSize]
				req.Icon = req.GetIcon()[groupIconChunkSize:]
			} else {
				chunk = req.GetIcon()
				req.Icon = nil
			}

			// upload the chunk
			mediaReq := &pb.UploadMediaRequest{
				Media: chunk,
				Type:  pb.MediaType_IMAGE,
				Name:  &name,
			}

			err = stream.Send(mediaReq)
			if err != nil {
				return nil, status.Errorf(codes.Internal, "We are unable to create your group at the moment. Please try again later.")
			}
		}
	}

	mediaResponse, err := stream.CloseAndRecv()
	if err != nil {
		return nil, status.Errorf(codes.Internal, "We are unable to create your group at the moment. Please try again later.")
	}

	// create the group
	desc := req.GetDescription()
	icon := mediaResponse.GetUrl()

	// add the admin to the list of admins
	var admins []string
	admins = append(admins, req.GetAdmin())

	// subscribe the admin to the group
	subscribers := make([]*pb.Subscriber, 0)
	subscribers = append(subscribers, subscriber)

	tags := make([]string, 0)
	tags = append(tags, req.GetTags()...)

	muted := make([]string, 0)
	banned := make([]string, 0)
	channels := make([]*pb.Channel, 0)

	// create the group
	group := &pb.Group{
		Id:          name,
		Name:        name,
		Description: &desc,
		Icon:        &icon,
		Admins:      admins,
		Subscribers: subscribers,
		Muted:       muted,
		Banned:      banned,
		Channels:    channels,
		Tags:        req.GetTags(),
		Created:     timestamppb.Now(),
		Updated:     timestamppb.Now(),
	}

	// save the group to the database
	result, err := s.Groups.InsertOne(ctx, &group)
	if err != nil {
		return nil, err
	}

	group.Id = result.InsertedID.(primitive.ObjectID).Hex()

	// convert group id to base64 string
	encodedId := base64.StdEncoding.EncodeToString([]byte(group.Id))

	// update the group id
	if _, err = s.Groups.UpdateOne(ctx, bson.M{"id": name}, bson.M{"$set": bson.M{"id": group.GetId(), "link": fmt.Sprintf("qreeket://profiles/groups/%s", encodedId)}}); err != nil {
		return nil, status.Errorf(codes.Internal, "Unable to create group at the moment. Please try again later.")
	}

	// create a channel in the background for the group
	go func(grp *pb.Group) {
		ctx = context.Background()

		// create a channel for the group
		channelDesc := fmt.Sprintf("General channel for group %s", grp.GetName())
		channel, err := s.CreateChannel(ctx, &pb.CreateChannelRequest{
			Name:        "general",
			Description: &channelDesc,
			Tags:        []string{"general"},
			Subscribers: grp.GetSubscribers(),
			Owner:       grp.GetAdmins()[0],
			Group:       grp.GetId(),
		})
		if err != nil {
			log.Printf("Unable to create channel for group %s: %v", grp.GetName(), err)
		}

		// add the channel to the group
		if _, err = s.Groups.UpdateOne(ctx, bson.M{"id": grp.GetId()}, bson.M{"$set": bson.M{"channels": []*pb.Channel{channel}}}); err != nil {
			log.Printf("Unable to create channel for group %s: %v", grp.GetName(), err)
		}
	}(group)

	return group, nil
}

// GetGroup returns a group by id (completed)
func (s *QreeketGroupServer) GetGroup(ctx context.Context, groupId *wrapperspb.StringValue) (*pb.Group, error) {
	var group *pb.Group
	if err := s.Groups.FindOne(ctx, bson.M{"id": groupId.GetValue()}).Decode(&group); err != nil {
		return nil, status.Errorf(codes.Internal, "Unable to get group at the moment. Please try again later.")
	}

	// get the channels for the group
	cursor, err := s.Channels.Find(ctx, bson.M{"group": group.GetId()})
	if err != nil {
		return nil, status.Errorf(codes.Internal, "Unable to get group at the moment. Please try again later.")
	}

	channels := make([]*pb.Channel, 0)
	for cursor.Next(ctx) {
		channel := &pb.Channel{}
		if err := cursor.Decode(&channel); err != nil {
			return nil, status.Errorf(codes.Internal, "Unable to get group at the moment. Please try again later.")
		}

		channels = append(channels, channel)
	}

	group.Channels = channels
	return group, nil
}

// GetGroups returns a list of groups where the user is an admin or subscriber (completed)
func (s *QreeketGroupServer) GetGroups(userId *wrapperspb.StringValue, stream pb.GroupChannelService_GetGroupsServer) error {
	ctx := stream.Context()
	groups := &pb.GroupList{}

	// filter for groups where user is an admin or part of the list of subscribers
	// (an object whose id is the user id) and is not in the banned list
	filter := bson.M{
		"$or": []bson.M{
			{"admins": bson.M{"$in": []string{userId.GetValue()}}},
			{"subscribers": bson.M{"$elemMatch": bson.M{"id": userId.GetValue()}}},
		},
		"banned": bson.M{"$ne": userId.GetValue()},
	}

	// get all groups where the user is an admin or subscriber
	cursor, err := s.Groups.Find(ctx, filter)
	if err != nil {
		return status.Errorf(codes.Internal, "Unable to get groups at the moment. Please try again later.")
	}

	// iterate through the groups and send them to the client
	for cursor.Next(ctx) {
		var group *pb.Group
		if err := cursor.Decode(&group); err != nil {
			return status.Errorf(codes.Internal, "Unable to get groups at the moment. Please try again later.")
		}

		// get all channels with group equal to the group id and subscribers containing the user id
		channelCursor, err := s.Channels.Find(ctx, bson.M{"group": group.GetId(), "subscribers": bson.M{"$elemMatch": bson.M{"id": userId.GetValue()}}})
		if err != nil {
			return status.Errorf(codes.Internal, "Unable to get groups at the moment. Please try again later.")
		}

		// map all channel ids to channel list
		var channels []*pb.Channel
		for channelCursor.Next(ctx) {
			var channel *pb.Channel
			if err := channelCursor.Decode(&channel); err != nil {
				return status.Errorf(codes.Internal, "Unable to get groups at the moment. Please try again later.")
			}

			channels = append(channels, channel)
		}

		group.Channels = channels
		groups.Groups = append(groups.GetGroups(), group)
	}
	if err = stream.Send(groups); err != nil {
		return status.Errorf(codes.Internal, "Unable to get groups at the moment. Please try again later.")
	}

	// TODO: implement change stream
	// watch for changes in the groups collection
	//changeStream, err := s.Groups.Watch(ctx, mongo.Pipeline{
	//	//bson.D{{"$match", filter}},
	//	bson.D{{"$match", bson.M{"operationType": bson.M{"$in": []string{"insert", "replace", "delete"}}}}},
	//})
	//if err != nil {
	//	log.Printf("Failed to create change stream: %v", err)
	//	return status.Errorf(codes.Internal, "Unable to get groups at the moment. Please try again later.")
	//}
	//
	//defer func(changeStream *mongo.ChangeStream, ctx context.Context) {
	//	_ = changeStream.Close(ctx)
	//}(changeStream, context.Background())
	//
	//// iterate through the change stream and send the changes to the client
	//for changeStream.Next(ctx) {
	//	var doc *utils.StreamDoc
	//	if err := changeStream.Decode(&doc); err != nil {
	//		log.Printf("Failed to decode group: %v", err)
	//		return status.Errorf(codes.Internal, "Failed to get updates for group")
	//	}
	//
	//	var group *pb.Group
	//	if err = doc.GetFullDocument(&group); err != nil {
	//		log.Printf("Failed to decode group: %v", err)
	//		return status.Errorf(codes.Internal, "Failed to get updates for group")
	//	}
	//
	//	// check operation type
	//	switch changeStream.Current.Lookup("operationType").StringValue() {
	//	case "insert":
	//		groups.Groups = append(groups.GetGroups(), group)
	//	case "delete":
	//		for i, c := range groups.GetGroups() {
	//			if c.GetId() == group.GetId() {
	//				groups.Groups = append(groups.GetGroups()[:i], groups.GetGroups()[i+1:]...)
	//			}
	//		}
	//	case "replace":
	//		for i, c := range groups.GetGroups() {
	//			if c.GetId() == group.GetId() {
	//				groups.Groups[i] = group
	//			}
	//		}
	//	}
	//
	//	// send the updated groups to the client
	//	_ = stream.Send(groups)
	//}

	return nil
}

// UpdateGroup updates a group (completed)
func (s *QreeketGroupServer) UpdateGroup(ctx context.Context, req *pb.Group) (*pb.Group, error) {
	// find the group
	var group *pb.Group
	if err := s.Groups.FindOne(ctx, bson.M{"id": req.GetId()}).Decode(&group); err != nil {
		return nil, status.Errorf(codes.Internal, "Unable to update group at the moment. Please try again later.")
	}

	// replace the group with the updated group
	req.Updated = timestamppb.Now()
	if _, err := s.Groups.ReplaceOne(ctx, bson.M{"id": req.GetId()}, &req); err != nil {
		return nil, status.Errorf(codes.Internal, "Unable to update group at the moment. Please try again later.")
	}

	return req, nil
}

// DeleteGroup deletes a group by id (completed)
func (s *QreeketGroupServer) DeleteGroup(ctx context.Context, request *pb.DeleteGroupRequest) (*emptypb.Empty, error) {

	// check if group exists
	var group *pb.Group
	if err := s.Groups.FindOne(ctx, bson.M{"id": request.GetGroup()}).Decode(&group); err != nil {
		return nil, status.Errorf(codes.Internal, "Unable to delete group at the moment. Please try again later.")
	}

	// check if admin is in the group
	var adminExists bool
	for _, admin := range group.GetAdmins() {
		if admin == request.GetAdmin() {
			adminExists = true
			break
		}
	}

	if !adminExists {
		return nil, status.Errorf(codes.PermissionDenied, "You are not an admin in this group.")
	}

	// check if admins are more than one
	if len(group.GetAdmins()) == 1 {
		return nil, status.Errorf(codes.PermissionDenied, "You cannot delete the group as you are the only admin. Please add another admin and try again.")
	}

	if _, err := s.Groups.DeleteOne(ctx, bson.M{"id": request.GetGroup()}); err != nil {
		return nil, status.Errorf(codes.Internal, "Unable to delete group at the moment. Please try again later.")
	}

	// delete all channels in the group
	if _, err := s.Channels.DeleteMany(ctx, bson.M{"group": request.GetGroup()}); err != nil {
		return nil, status.Errorf(codes.Internal, "Unable to delete group at the moment. Please try again later.")
	}

	return &emptypb.Empty{}, nil
}

// MuteGroup mutes a group (completed)
func (s *QreeketGroupServer) MuteGroup(ctx context.Context, request *pb.ManageGroupOrChannelRequest) (*emptypb.Empty, error) {

	// check if group exists
	var group *pb.Group
	if err := s.Groups.FindOne(ctx, bson.M{"id": request.GetGroup()}).Decode(&group); err != nil {
		return nil, status.Errorf(codes.Internal, "Unable to mute group at the moment. Please try again later.")
	}

	var user *pb.Subscriber
	// check if user is a subscriber
	for _, subscriber := range group.GetSubscribers() {
		if subscriber.GetId() == request.GetUser() {
			user = subscriber
			break
		}
	}

	if user == nil {
		return nil, status.Errorf(codes.PermissionDenied, "You are not a subscriber of this group.")
	}

	// add user to muted list
	group.Muted = append(group.GetMuted(), request.GetUser())

	// update the group
	if _, err := s.Groups.UpdateOne(ctx, bson.M{"id": group.GetId()}, bson.M{"$set": group}, options.Update().SetUpsert(true)); err != nil {
		return nil, status.Errorf(codes.Internal, "Unable to mute group at the moment. Please try again later.")
	}

	return &emptypb.Empty{}, nil
}

// UnmuteGroup unmutes a group (completed)
func (s *QreeketGroupServer) UnmuteGroup(ctx context.Context, request *pb.ManageGroupOrChannelRequest) (*emptypb.Empty, error) {
	// check if group exists
	var group *pb.Group
	if err := s.Groups.FindOne(ctx, bson.M{"id": request.GetGroup()}).Decode(&group); err != nil {
		return nil, status.Errorf(codes.Internal, "Unable to unmute group at the moment. Please try again later.")
	}

	var user *pb.Subscriber
	// check if user is a subscriber
	for _, subscriber := range group.GetSubscribers() {
		if subscriber.GetId() == request.GetUser() {
			user = subscriber
			break
		}
	}

	if user == nil {
		return nil, status.Errorf(codes.PermissionDenied, "You are not a subscriber of this group.")
	}

	// remove from muted list
	for i, mutedUser := range group.GetMuted() {
		if mutedUser == request.GetUser() {
			group.Muted = append(group.GetMuted()[:i], group.GetMuted()[i+1:]...)
		}
	}

	// update the group
	if _, err := s.Groups.UpdateOne(ctx, bson.M{"id": group.GetId()}, bson.M{"$set": group}, options.Update().SetUpsert(true)); err != nil {
		return nil, status.Errorf(codes.Internal, "Unable to mute group at the moment. Please try again later.")
	}

	return &emptypb.Empty{}, nil
}

// PromoteGroupAdmin promotes a channel admin (completed)
func (s *QreeketGroupServer) PromoteGroupAdmin(ctx context.Context, request *pb.ManageAdminRequest) (*emptypb.Empty, error) {
	// check if group exists
	var group *pb.Group
	if err := s.Groups.FindOne(ctx, bson.M{"id": request.GetGroup()}).Decode(&group); err != nil {
		return nil, status.Errorf(codes.Internal, "Unable to promote group admin at the moment. Please try again later.")
	}

	// check if admin exists in group
	var adminExists bool
	for _, admin := range group.GetAdmins() {
		if admin == request.GetAdmin() {
			adminExists = true
			break
		}
	}

	if !adminExists {
		return nil, status.Errorf(codes.PermissionDenied, "The admin is not a member of this group.")
	}

	// check if user is a subscriber
	var user *pb.Subscriber
	for _, subscriber := range group.GetSubscribers() {
		if subscriber.GetId() == request.GetUser() {
			user = subscriber
			break
		}
	}

	if user == nil {
		return nil, status.Errorf(codes.PermissionDenied, "The user is not a subscriber of this group.")
	}

	// check if user is already an admin
	for _, admin := range group.GetAdmins() {
		if admin == request.GetUser() {
			return nil, status.Errorf(codes.PermissionDenied, "The user is already an admin of this group.")
		}
	}

	// add user to admin list
	group.Admins = append(group.GetAdmins(), request.GetUser())

	// update the group
	if _, err := s.Groups.UpdateOne(ctx, bson.M{"id": group.GetId()}, bson.M{"$set": group}, options.Update().SetUpsert(true)); err != nil {
		return nil, status.Errorf(codes.Internal, "Unable to promote group admin at the moment. Please try again later.")
	}

	return &emptypb.Empty{}, nil
}

// DemoteGroupAdmin demotes a channel admin (completed)
func (s *QreeketGroupServer) DemoteGroupAdmin(ctx context.Context, request *pb.ManageAdminRequest) (*emptypb.Empty, error) {
	// check if group exists
	var group *pb.Group
	if err := s.Groups.FindOne(ctx, bson.M{"id": request.GetGroup()}).Decode(&group); err != nil {
		return nil, status.Errorf(codes.Internal, "Unable to demote group admin at the moment. Please try again later.")
	}

	// check if admin exists in group
	var adminExists bool
	for _, admin := range group.GetAdmins() {
		if admin == request.GetAdmin() {
			adminExists = true
			break
		}
	}

	if !adminExists {
		return nil, status.Errorf(codes.PermissionDenied, "The admin is not a member of this group.")
	}

	// check if user is a subscriber
	var user *pb.Subscriber
	for _, subscriber := range group.GetSubscribers() {
		if subscriber.GetId() == request.GetUser() {
			user = subscriber
			break
		}
	}

	if user == nil {
		return nil, status.Errorf(codes.PermissionDenied, "The user is not a subscriber of this group.")
	}

	// check if user is already an admin
	var userIsAdmin bool
	for _, admin := range group.GetAdmins() {
		if admin == request.GetUser() {
			userIsAdmin = true
			break
		}
	}

	if !userIsAdmin {
		return nil, status.Errorf(codes.PermissionDenied, "The user is not an admin of this group.")
	}

	// remove user from admin list
	for i, admin := range group.GetAdmins() {
		if admin == request.GetUser() {
			group.Admins = append(group.GetAdmins()[:i], group.GetAdmins()[i+1:]...)
		}
	}

	// update the group
	if _, err := s.Groups.UpdateOne(ctx, bson.M{"id": group.GetId()}, bson.M{"$set": group}, options.Update().SetUpsert(true)); err != nil {
		return nil, status.Errorf(codes.Internal, "Unable to demote group admin at the moment. Please try again later.")
	}

	return &emptypb.Empty{}, nil
}

// BanFromGroup bans a user from a group (completed)
func (s *QreeketGroupServer) BanFromGroup(ctx context.Context, request *pb.ManageGroupOrChannelRequest) (*emptypb.Empty, error) {
	// check if user is in the subscribers list
	var group *pb.Group
	if err := s.Groups.FindOne(ctx,
		bson.M{"id": request.GetGroup(), "subscribers": bson.M{"$elemMatch": bson.M{"id": request.GetUser()}}}).Decode(&group); err != nil {
		return nil, status.Errorf(codes.NotFound, "The subscriber you are trying to ban is not in this group")
	}

	// check if admin is in the group
	var adminExists bool
	for _, admin := range group.GetAdmins() {
		if admin == request.GetAdmin() {
			adminExists = true
			break
		}
	}

	if !adminExists {
		return nil, status.Errorf(codes.PermissionDenied, "You are not an admin of this group.")
	}

	// check if user is in the group
	var userExists bool
	for _, subscriber := range group.GetSubscribers() {
		if subscriber.GetId() == request.GetUser() {
			userExists = true
			break
		}
	}

	if !userExists {
		return nil, status.Errorf(codes.PermissionDenied, "The user is not a subscriber of this group.")
	}

	// add the user to the banned list
	if _, err := s.Groups.UpdateOne(ctx, bson.M{"id": request.GetGroup()}, bson.M{"$addToSet": bson.M{"banned": request.GetUser()}}, options.Update().SetUpsert(true)); err != nil {
		return nil, status.Errorf(codes.Internal, "Unable to ban user at the moment. Please try again later.")
	}

	return &emptypb.Empty{}, nil
}

// UnbanFromGroup unbans a user from a group (completed)
func (s *QreeketGroupServer) UnbanFromGroup(ctx context.Context, request *pb.ManageGroupOrChannelRequest) (*emptypb.Empty, error) {
	// check if user is in the subscribers list
	var group *pb.Group
	if err := s.Groups.FindOne(ctx,
		bson.M{"id": request.GetGroup(), "subscribers": bson.M{"$elemMatch": bson.M{"id": request.GetUser()}}}).Decode(&group); err != nil {
		return nil, status.Errorf(codes.NotFound, "The subscriber you are trying to ban is not in this group")
	}

	// check if admin is in the group
	var adminExists bool
	for _, admin := range group.GetAdmins() {
		if admin == request.GetAdmin() {
			adminExists = true
			break
		}
	}

	if !adminExists {
		return nil, status.Errorf(codes.PermissionDenied, "You are not an admin of this group.")
	}

	// check if user is in the group
	var userExists bool
	for _, subscriber := range group.GetSubscribers() {
		if subscriber.GetId() == request.GetUser() {
			userExists = true
			break
		}
	}

	if !userExists {
		return nil, status.Errorf(codes.PermissionDenied, "The user is not a subscriber of this group.")
	}

	// remove the user from the banned list
	if _, err := s.Groups.UpdateOne(ctx, bson.M{"id": request.GetGroup()}, bson.M{"$pull": bson.M{"banned": request.GetUser()}}, options.Update().SetUpsert(true)); err != nil {
		return nil, status.Errorf(codes.Internal, "Unable to unban user at the moment. Please try again later.")
	}

	return &emptypb.Empty{}, nil
}

func createIconFromFolder() (*[]byte, error) {
	imageFolderPath := "icons"

	// read all the images in the folder
	files, err := ioutil.ReadDir(imageFolderPath)
	if err != nil {
		fmt.Println("Error reading directory:", err)
		return nil, err
	}

	imageFiles := make([]string, 0)
	for _, file := range files {
		if !file.IsDir() && isImageFile(file.Name()) {
			imageFiles = append(imageFiles, file.Name())
		}
	}

	if len(imageFiles) == 0 {
		fmt.Println("No image files found in the folder.")
		return nil, errors.New("no image files found in the folder")
	}

	// randomly pick one of the images
	rand.Seed(time.Now().UnixNano())
	randomIndex := rand.Intn(len(imageFiles))
	randomImage := imageFiles[randomIndex]

	// read the image file
	imagePath := filepath.Join(imageFolderPath, randomImage)
	if imageBytes, err := ioutil.ReadFile(imagePath); err != nil {
		fmt.Println("Error reading image file:", err)
		return nil, err
	} else {
		return &imageBytes, nil
	}
}

func isImageFile(filename string) bool {
	extensions := []string{".jpg", ".jpeg", ".png", ".gif"} // Add more extensions if needed

	ext := strings.ToLower(filepath.Ext(filename))
	for _, validExt := range extensions {
		if ext == validExt {
			return true
		}
	}
	return false
}
