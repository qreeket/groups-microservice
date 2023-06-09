package clients

import (
	pb "github.com/qcodelabsllc/qreeket/groups/generated"
	"google.golang.org/grpc"
	"google.golang.org/grpc/credentials/insecure"
	"log"
	"os"
)

// CreateAuthClient creates a new gRPC client for the auth service
func CreateAuthClient() pb.AuthServiceClient {
	serverAddress := os.Getenv("AUTH_CLIENT_URL")
	conn, err := grpc.Dial(serverAddress, grpc.WithTransportCredentials(insecure.NewCredentials()))
	if err != nil {
		log.Fatal("cannot dial server: ", err)
	}

	return pb.NewAuthServiceClient(conn)
}
