// Code generated by protoc-gen-go-grpc. DO NOT EDIT.
// versions:
// - protoc-gen-go-grpc v1.3.0
// - protoc             v3.15.8
// source: media_service.proto

package qreeket

import (
	context "context"
	grpc "google.golang.org/grpc"
	codes "google.golang.org/grpc/codes"
	status "google.golang.org/grpc/status"
	emptypb "google.golang.org/protobuf/types/known/emptypb"
	wrapperspb "google.golang.org/protobuf/types/known/wrapperspb"
)

// This is a compile-time assertion to ensure that this generated file
// is compatible with the grpc package it is being compiled against.
// Requires gRPC-Go v1.32.0 or later.
const _ = grpc.SupportPackageIsVersion7

const (
	MediaService_UploadMedia_FullMethodName      = "/qreeket.MediaService/upload_media"
	MediaService_UploadLargeMedia_FullMethodName = "/qreeket.MediaService/upload_large_media"
	MediaService_GetMedia_FullMethodName         = "/qreeket.MediaService/get_media"
	MediaService_DeleteMedia_FullMethodName      = "/qreeket.MediaService/delete_media"
)

// MediaServiceClient is the client API for MediaService service.
//
// For semantics around ctx use and closing/ending streaming RPCs, please refer to https://pkg.go.dev/google.golang.org/grpc/?tab=doc#ClientConn.NewStream.
type MediaServiceClient interface {
	// upload media takes a base64 encoded string and returns a media url
	UploadMedia(ctx context.Context, in *UploadMediaRequest, opts ...grpc.CallOption) (*UploadMediaResponse, error)
	UploadLargeMedia(ctx context.Context, opts ...grpc.CallOption) (MediaService_UploadLargeMediaClient, error)
	// get media takes a media url and returns a base64 encoded string
	GetMedia(ctx context.Context, in *wrapperspb.StringValue, opts ...grpc.CallOption) (*wrapperspb.StringValue, error)
	// delete media takes a media url and deletes the media
	DeleteMedia(ctx context.Context, in *wrapperspb.StringValue, opts ...grpc.CallOption) (*emptypb.Empty, error)
}

type mediaServiceClient struct {
	cc grpc.ClientConnInterface
}

func NewMediaServiceClient(cc grpc.ClientConnInterface) MediaServiceClient {
	return &mediaServiceClient{cc}
}

func (c *mediaServiceClient) UploadMedia(ctx context.Context, in *UploadMediaRequest, opts ...grpc.CallOption) (*UploadMediaResponse, error) {
	out := new(UploadMediaResponse)
	err := c.cc.Invoke(ctx, MediaService_UploadMedia_FullMethodName, in, out, opts...)
	if err != nil {
		return nil, err
	}
	return out, nil
}

func (c *mediaServiceClient) UploadLargeMedia(ctx context.Context, opts ...grpc.CallOption) (MediaService_UploadLargeMediaClient, error) {
	stream, err := c.cc.NewStream(ctx, &MediaService_ServiceDesc.Streams[0], MediaService_UploadLargeMedia_FullMethodName, opts...)
	if err != nil {
		return nil, err
	}
	x := &mediaServiceUploadLargeMediaClient{stream}
	return x, nil
}

type MediaService_UploadLargeMediaClient interface {
	Send(*UploadMediaRequest) error
	CloseAndRecv() (*UploadMediaResponse, error)
	grpc.ClientStream
}

type mediaServiceUploadLargeMediaClient struct {
	grpc.ClientStream
}

func (x *mediaServiceUploadLargeMediaClient) Send(m *UploadMediaRequest) error {
	return x.ClientStream.SendMsg(m)
}

func (x *mediaServiceUploadLargeMediaClient) CloseAndRecv() (*UploadMediaResponse, error) {
	if err := x.ClientStream.CloseSend(); err != nil {
		return nil, err
	}
	m := new(UploadMediaResponse)
	if err := x.ClientStream.RecvMsg(m); err != nil {
		return nil, err
	}
	return m, nil
}

func (c *mediaServiceClient) GetMedia(ctx context.Context, in *wrapperspb.StringValue, opts ...grpc.CallOption) (*wrapperspb.StringValue, error) {
	out := new(wrapperspb.StringValue)
	err := c.cc.Invoke(ctx, MediaService_GetMedia_FullMethodName, in, out, opts...)
	if err != nil {
		return nil, err
	}
	return out, nil
}

func (c *mediaServiceClient) DeleteMedia(ctx context.Context, in *wrapperspb.StringValue, opts ...grpc.CallOption) (*emptypb.Empty, error) {
	out := new(emptypb.Empty)
	err := c.cc.Invoke(ctx, MediaService_DeleteMedia_FullMethodName, in, out, opts...)
	if err != nil {
		return nil, err
	}
	return out, nil
}

// MediaServiceServer is the server API for MediaService service.
// All implementations must embed UnimplementedMediaServiceServer
// for forward compatibility
type MediaServiceServer interface {
	// upload media takes a base64 encoded string and returns a media url
	UploadMedia(context.Context, *UploadMediaRequest) (*UploadMediaResponse, error)
	UploadLargeMedia(MediaService_UploadLargeMediaServer) error
	// get media takes a media url and returns a base64 encoded string
	GetMedia(context.Context, *wrapperspb.StringValue) (*wrapperspb.StringValue, error)
	// delete media takes a media url and deletes the media
	DeleteMedia(context.Context, *wrapperspb.StringValue) (*emptypb.Empty, error)
	mustEmbedUnimplementedMediaServiceServer()
}

// UnimplementedMediaServiceServer must be embedded to have forward compatible implementations.
type UnimplementedMediaServiceServer struct {
}

func (UnimplementedMediaServiceServer) UploadMedia(context.Context, *UploadMediaRequest) (*UploadMediaResponse, error) {
	return nil, status.Errorf(codes.Unimplemented, "method UploadMedia not implemented")
}
func (UnimplementedMediaServiceServer) UploadLargeMedia(MediaService_UploadLargeMediaServer) error {
	return status.Errorf(codes.Unimplemented, "method UploadLargeMedia not implemented")
}
func (UnimplementedMediaServiceServer) GetMedia(context.Context, *wrapperspb.StringValue) (*wrapperspb.StringValue, error) {
	return nil, status.Errorf(codes.Unimplemented, "method GetMedia not implemented")
}
func (UnimplementedMediaServiceServer) DeleteMedia(context.Context, *wrapperspb.StringValue) (*emptypb.Empty, error) {
	return nil, status.Errorf(codes.Unimplemented, "method DeleteMedia not implemented")
}
func (UnimplementedMediaServiceServer) mustEmbedUnimplementedMediaServiceServer() {}

// UnsafeMediaServiceServer may be embedded to opt out of forward compatibility for this service.
// Use of this interface is not recommended, as added methods to MediaServiceServer will
// result in compilation errors.
type UnsafeMediaServiceServer interface {
	mustEmbedUnimplementedMediaServiceServer()
}

func RegisterMediaServiceServer(s grpc.ServiceRegistrar, srv MediaServiceServer) {
	s.RegisterService(&MediaService_ServiceDesc, srv)
}

func _MediaService_UploadMedia_Handler(srv interface{}, ctx context.Context, dec func(interface{}) error, interceptor grpc.UnaryServerInterceptor) (interface{}, error) {
	in := new(UploadMediaRequest)
	if err := dec(in); err != nil {
		return nil, err
	}
	if interceptor == nil {
		return srv.(MediaServiceServer).UploadMedia(ctx, in)
	}
	info := &grpc.UnaryServerInfo{
		Server:     srv,
		FullMethod: MediaService_UploadMedia_FullMethodName,
	}
	handler := func(ctx context.Context, req interface{}) (interface{}, error) {
		return srv.(MediaServiceServer).UploadMedia(ctx, req.(*UploadMediaRequest))
	}
	return interceptor(ctx, in, info, handler)
}

func _MediaService_UploadLargeMedia_Handler(srv interface{}, stream grpc.ServerStream) error {
	return srv.(MediaServiceServer).UploadLargeMedia(&mediaServiceUploadLargeMediaServer{stream})
}

type MediaService_UploadLargeMediaServer interface {
	SendAndClose(*UploadMediaResponse) error
	Recv() (*UploadMediaRequest, error)
	grpc.ServerStream
}

type mediaServiceUploadLargeMediaServer struct {
	grpc.ServerStream
}

func (x *mediaServiceUploadLargeMediaServer) SendAndClose(m *UploadMediaResponse) error {
	return x.ServerStream.SendMsg(m)
}

func (x *mediaServiceUploadLargeMediaServer) Recv() (*UploadMediaRequest, error) {
	m := new(UploadMediaRequest)
	if err := x.ServerStream.RecvMsg(m); err != nil {
		return nil, err
	}
	return m, nil
}

func _MediaService_GetMedia_Handler(srv interface{}, ctx context.Context, dec func(interface{}) error, interceptor grpc.UnaryServerInterceptor) (interface{}, error) {
	in := new(wrapperspb.StringValue)
	if err := dec(in); err != nil {
		return nil, err
	}
	if interceptor == nil {
		return srv.(MediaServiceServer).GetMedia(ctx, in)
	}
	info := &grpc.UnaryServerInfo{
		Server:     srv,
		FullMethod: MediaService_GetMedia_FullMethodName,
	}
	handler := func(ctx context.Context, req interface{}) (interface{}, error) {
		return srv.(MediaServiceServer).GetMedia(ctx, req.(*wrapperspb.StringValue))
	}
	return interceptor(ctx, in, info, handler)
}

func _MediaService_DeleteMedia_Handler(srv interface{}, ctx context.Context, dec func(interface{}) error, interceptor grpc.UnaryServerInterceptor) (interface{}, error) {
	in := new(wrapperspb.StringValue)
	if err := dec(in); err != nil {
		return nil, err
	}
	if interceptor == nil {
		return srv.(MediaServiceServer).DeleteMedia(ctx, in)
	}
	info := &grpc.UnaryServerInfo{
		Server:     srv,
		FullMethod: MediaService_DeleteMedia_FullMethodName,
	}
	handler := func(ctx context.Context, req interface{}) (interface{}, error) {
		return srv.(MediaServiceServer).DeleteMedia(ctx, req.(*wrapperspb.StringValue))
	}
	return interceptor(ctx, in, info, handler)
}

// MediaService_ServiceDesc is the grpc.ServiceDesc for MediaService service.
// It's only intended for direct use with grpc.RegisterService,
// and not to be introspected or modified (even as a copy)
var MediaService_ServiceDesc = grpc.ServiceDesc{
	ServiceName: "qreeket.MediaService",
	HandlerType: (*MediaServiceServer)(nil),
	Methods: []grpc.MethodDesc{
		{
			MethodName: "upload_media",
			Handler:    _MediaService_UploadMedia_Handler,
		},
		{
			MethodName: "get_media",
			Handler:    _MediaService_GetMedia_Handler,
		},
		{
			MethodName: "delete_media",
			Handler:    _MediaService_DeleteMedia_Handler,
		},
	},
	Streams: []grpc.StreamDesc{
		{
			StreamName:    "upload_large_media",
			Handler:       _MediaService_UploadLargeMedia_Handler,
			ClientStreams: true,
		},
	},
	Metadata: "media_service.proto",
}
