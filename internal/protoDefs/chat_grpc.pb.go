// Code generated by protoc-gen-go-grpc. DO NOT EDIT.
// versions:
// - protoc-gen-go-grpc v1.2.0
// - protoc             v3.21.12
// source: internal/protoDefs/chat.proto

package __

import (
	context "context"
	grpc "google.golang.org/grpc"
	codes "google.golang.org/grpc/codes"
	status "google.golang.org/grpc/status"
)

// This is a compile-time assertion to ensure that this generated file
// is compatible with the grpc package it is being compiled against.
// Requires gRPC-Go v1.32.0 or later.
const _ = grpc.SupportPackageIsVersion7

// ChatClient is the client API for Chat service.
//
// For semantics around ctx use and closing/ending streaming RPCs, please refer to https://pkg.go.dev/google.golang.org/grpc/?tab=doc#ClientConn.NewStream.
type ChatClient interface {
	PutMsg(ctx context.Context, in *PutMsgReq, opts ...grpc.CallOption) (*PutMsgResp, error)
	GetMsgs(ctx context.Context, in *GetMsgsReq, opts ...grpc.CallOption) (Chat_GetMsgsClient, error)
}

type chatClient struct {
	cc grpc.ClientConnInterface
}

func NewChatClient(cc grpc.ClientConnInterface) ChatClient {
	return &chatClient{cc}
}

func (c *chatClient) PutMsg(ctx context.Context, in *PutMsgReq, opts ...grpc.CallOption) (*PutMsgResp, error) {
	out := new(PutMsgResp)
	err := c.cc.Invoke(ctx, "/chat.Chat/PutMsg", in, out, opts...)
	if err != nil {
		return nil, err
	}
	return out, nil
}

func (c *chatClient) GetMsgs(ctx context.Context, in *GetMsgsReq, opts ...grpc.CallOption) (Chat_GetMsgsClient, error) {
	stream, err := c.cc.NewStream(ctx, &Chat_ServiceDesc.Streams[0], "/chat.Chat/GetMsgs", opts...)
	if err != nil {
		return nil, err
	}
	x := &chatGetMsgsClient{stream}
	if err := x.ClientStream.SendMsg(in); err != nil {
		return nil, err
	}
	if err := x.ClientStream.CloseSend(); err != nil {
		return nil, err
	}
	return x, nil
}

type Chat_GetMsgsClient interface {
	Recv() (*GetMsgsResp, error)
	grpc.ClientStream
}

type chatGetMsgsClient struct {
	grpc.ClientStream
}

func (x *chatGetMsgsClient) Recv() (*GetMsgsResp, error) {
	m := new(GetMsgsResp)
	if err := x.ClientStream.RecvMsg(m); err != nil {
		return nil, err
	}
	return m, nil
}

// ChatServer is the server API for Chat service.
// All implementations must embed UnimplementedChatServer
// for forward compatibility
type ChatServer interface {
	PutMsg(context.Context, *PutMsgReq) (*PutMsgResp, error)
	GetMsgs(*GetMsgsReq, Chat_GetMsgsServer) error
	mustEmbedUnimplementedChatServer()
}

// UnimplementedChatServer must be embedded to have forward compatible implementations.
type UnimplementedChatServer struct {
}

func (UnimplementedChatServer) PutMsg(context.Context, *PutMsgReq) (*PutMsgResp, error) {
	return nil, status.Errorf(codes.Unimplemented, "method PutMsg not implemented")
}
func (UnimplementedChatServer) GetMsgs(*GetMsgsReq, Chat_GetMsgsServer) error {
	return status.Errorf(codes.Unimplemented, "method GetMsgs not implemented")
}
func (UnimplementedChatServer) mustEmbedUnimplementedChatServer() {}

// UnsafeChatServer may be embedded to opt out of forward compatibility for this service.
// Use of this interface is not recommended, as added methods to ChatServer will
// result in compilation errors.
type UnsafeChatServer interface {
	mustEmbedUnimplementedChatServer()
}

func RegisterChatServer(s grpc.ServiceRegistrar, srv ChatServer) {
	s.RegisterService(&Chat_ServiceDesc, srv)
}

func _Chat_PutMsg_Handler(srv interface{}, ctx context.Context, dec func(interface{}) error, interceptor grpc.UnaryServerInterceptor) (interface{}, error) {
	in := new(PutMsgReq)
	if err := dec(in); err != nil {
		return nil, err
	}
	if interceptor == nil {
		return srv.(ChatServer).PutMsg(ctx, in)
	}
	info := &grpc.UnaryServerInfo{
		Server:     srv,
		FullMethod: "/chat.Chat/PutMsg",
	}
	handler := func(ctx context.Context, req interface{}) (interface{}, error) {
		return srv.(ChatServer).PutMsg(ctx, req.(*PutMsgReq))
	}
	return interceptor(ctx, in, info, handler)
}

func _Chat_GetMsgs_Handler(srv interface{}, stream grpc.ServerStream) error {
	m := new(GetMsgsReq)
	if err := stream.RecvMsg(m); err != nil {
		return err
	}
	return srv.(ChatServer).GetMsgs(m, &chatGetMsgsServer{stream})
}

type Chat_GetMsgsServer interface {
	Send(*GetMsgsResp) error
	grpc.ServerStream
}

type chatGetMsgsServer struct {
	grpc.ServerStream
}

func (x *chatGetMsgsServer) Send(m *GetMsgsResp) error {
	return x.ServerStream.SendMsg(m)
}

// Chat_ServiceDesc is the grpc.ServiceDesc for Chat service.
// It's only intended for direct use with grpc.RegisterService,
// and not to be introspected or modified (even as a copy)
var Chat_ServiceDesc = grpc.ServiceDesc{
	ServiceName: "chat.Chat",
	HandlerType: (*ChatServer)(nil),
	Methods: []grpc.MethodDesc{
		{
			MethodName: "PutMsg",
			Handler:    _Chat_PutMsg_Handler,
		},
	},
	Streams: []grpc.StreamDesc{
		{
			StreamName:    "GetMsgs",
			Handler:       _Chat_GetMsgs_Handler,
			ServerStreams: true,
		},
	},
	Metadata: "internal/protoDefs/chat.proto",
}