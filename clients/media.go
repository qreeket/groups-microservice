package clients

import (
	pb "github.com/qcodelabsllc/qreeket/groups/generated"
	"google.golang.org/grpc"
	"google.golang.org/grpc/credentials/insecure"
	"log"
	"os"
)

// CreateMediaClient creates a new gRPC client for the media service
func CreateMediaClient() pb.MediaServiceClient {
	serverAddress := os.Getenv("MEDIA_CLIENT_URL")
	conn, err := grpc.Dial(serverAddress, grpc.WithTransportCredentials(insecure.NewCredentials()))
	if err != nil {
		log.Fatal("cannot dial server: ", err)
	}

	return pb.NewMediaServiceClient(conn)
}
