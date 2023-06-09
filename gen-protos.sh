# bin/sh

PROTOS_DIR=protos
OUT_DIR=generated

# Create generated directories
mkdir -p $OUT_DIR

# Generate Go protobufs for announcements
protoc -I=$PROTOS_DIR --go_opt=paths=source_relative --go-grpc_opt=paths=source_relative -I=$OUT_DIR --go-grpc_out=$OUT_DIR --go_out=$OUT_DIR \
  auth.proto auth_service.proto \
  media.proto media_service.proto \
  group.proto group_service.proto