package network

import (
	pb "github.com/qcodelabsllc/qreeket/groups/generated"
	"github.com/qcodelabsllc/qreeket/groups/services"
	"go.mongodb.org/mongo-driver/mongo"
	"google.golang.org/grpc"
	"google.golang.org/grpc/reflection"
	"log"
	"net"
	"os"
	"strconv"
)

func InitServer(groupsCollection *mongo.Collection, channelsCollection *mongo.Collection, invitesCollection *mongo.Collection, subscriptionCollection *mongo.Collection) {
	// create a new grpc server
	s := grpc.NewServer(
		grpc.UnaryInterceptor(AuthUnaryInterceptor),
		grpc.StreamInterceptor(AuthStreamInterceptor),
	)

	// register the grpc server
	pb.RegisterGroupChannelServiceServer(s, services.NewQreeketGroupServer(groupsCollection, channelsCollection, invitesCollection, subscriptionCollection))

	// register the grpc server for reflection
	reflection.Register(s)

	// get the port number from .env file
	port, _ := strconv.Atoi(os.Getenv("PORT"))

	// listen on the port
	if lis, err := net.Listen("tcp", ":"+strconv.Itoa(port)); err == nil {
		log.Printf("groups server started on %v\n", lis.Addr())
		if err := s.Serve(lis); err != nil {
			log.Fatalf("unable to start grpc server: %+v\n", err)
		}
	}
}
