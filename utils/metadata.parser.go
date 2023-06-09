package utils

import (
	"context"
	"google.golang.org/grpc/metadata"
)

// CreateContextFromRequestMetadata creates a new context from the incoming request metadata
func CreateContextFromRequestMetadata(ctx context.Context) context.Context {
	md, _ := metadata.FromIncomingContext(ctx)
	return metadata.NewOutgoingContext(context.Background(), md)
}
