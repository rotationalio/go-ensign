// Code generated by protoc-gen-go-grpc. DO NOT EDIT.
// versions:
// - protoc-gen-go-grpc v1.2.0
// - protoc             v3.21.12
// source: api/v1beta1/ensign.proto

package api

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

// EnsignClient is the client API for Ensign service.
//
// For semantics around ctx use and closing/ending streaming RPCs, please refer to https://pkg.go.dev/google.golang.org/grpc/?tab=doc#ClientConn.NewStream.
type EnsignClient interface {
	// Both the Publish and Subscribe RPCs are bidirectional streaming to allow for acks
	// and nacks of events to be sent between Ensign and the client. The Publish stream
	// is opened and the client sends events and receives acks/nacks -- when the client
	// closes the publish stream, the server sends back information about the current
	// state of the topic. When the Subscribe stream is opened, the client must send an
	// open stream message with the subscription info before receiving events. Once it
	// receives events it must send back acks/nacks up the stream so that Ensign
	// advances the topic offset for the rest of the clients in the group.
	Publish(ctx context.Context, opts ...grpc.CallOption) (Ensign_PublishClient, error)
	Subscribe(ctx context.Context, opts ...grpc.CallOption) (Ensign_SubscribeClient, error)
	// This is a simple topic management interface. Right now we assume that topics are
	// immutable, therefore there is no update topic RPC call. There are two ways to
	// delete a topic - archiving it makes the topic readonly so that no events can be
	// published to it, but it can still be read. Destroying the topic deletes it and
	// removes all of its data, freeing up the topic name to be used again.
	ListTopics(ctx context.Context, in *PageInfo, opts ...grpc.CallOption) (*TopicsPage, error)
	CreateTopic(ctx context.Context, in *Topic, opts ...grpc.CallOption) (*Topic, error)
	DeleteTopic(ctx context.Context, in *TopicMod, opts ...grpc.CallOption) (*TopicTombstone, error)
	// Implements a client-side heartbeat that can also be used by monitoring tools.
	Status(ctx context.Context, in *HealthCheck, opts ...grpc.CallOption) (*ServiceState, error)
}

type ensignClient struct {
	cc grpc.ClientConnInterface
}

func NewEnsignClient(cc grpc.ClientConnInterface) EnsignClient {
	return &ensignClient{cc}
}

func (c *ensignClient) Publish(ctx context.Context, opts ...grpc.CallOption) (Ensign_PublishClient, error) {
	stream, err := c.cc.NewStream(ctx, &Ensign_ServiceDesc.Streams[0], "/ensign.v1beta1.Ensign/Publish", opts...)
	if err != nil {
		return nil, err
	}
	x := &ensignPublishClient{stream}
	return x, nil
}

type Ensign_PublishClient interface {
	Send(*Event) error
	Recv() (*Publication, error)
	grpc.ClientStream
}

type ensignPublishClient struct {
	grpc.ClientStream
}

func (x *ensignPublishClient) Send(m *Event) error {
	return x.ClientStream.SendMsg(m)
}

func (x *ensignPublishClient) Recv() (*Publication, error) {
	m := new(Publication)
	if err := x.ClientStream.RecvMsg(m); err != nil {
		return nil, err
	}
	return m, nil
}

func (c *ensignClient) Subscribe(ctx context.Context, opts ...grpc.CallOption) (Ensign_SubscribeClient, error) {
	stream, err := c.cc.NewStream(ctx, &Ensign_ServiceDesc.Streams[1], "/ensign.v1beta1.Ensign/Subscribe", opts...)
	if err != nil {
		return nil, err
	}
	x := &ensignSubscribeClient{stream}
	return x, nil
}

type Ensign_SubscribeClient interface {
	Send(*Subscription) error
	Recv() (*Event, error)
	grpc.ClientStream
}

type ensignSubscribeClient struct {
	grpc.ClientStream
}

func (x *ensignSubscribeClient) Send(m *Subscription) error {
	return x.ClientStream.SendMsg(m)
}

func (x *ensignSubscribeClient) Recv() (*Event, error) {
	m := new(Event)
	if err := x.ClientStream.RecvMsg(m); err != nil {
		return nil, err
	}
	return m, nil
}

func (c *ensignClient) ListTopics(ctx context.Context, in *PageInfo, opts ...grpc.CallOption) (*TopicsPage, error) {
	out := new(TopicsPage)
	err := c.cc.Invoke(ctx, "/ensign.v1beta1.Ensign/ListTopics", in, out, opts...)
	if err != nil {
		return nil, err
	}
	return out, nil
}

func (c *ensignClient) CreateTopic(ctx context.Context, in *Topic, opts ...grpc.CallOption) (*Topic, error) {
	out := new(Topic)
	err := c.cc.Invoke(ctx, "/ensign.v1beta1.Ensign/CreateTopic", in, out, opts...)
	if err != nil {
		return nil, err
	}
	return out, nil
}

func (c *ensignClient) DeleteTopic(ctx context.Context, in *TopicMod, opts ...grpc.CallOption) (*TopicTombstone, error) {
	out := new(TopicTombstone)
	err := c.cc.Invoke(ctx, "/ensign.v1beta1.Ensign/DeleteTopic", in, out, opts...)
	if err != nil {
		return nil, err
	}
	return out, nil
}

func (c *ensignClient) Status(ctx context.Context, in *HealthCheck, opts ...grpc.CallOption) (*ServiceState, error) {
	out := new(ServiceState)
	err := c.cc.Invoke(ctx, "/ensign.v1beta1.Ensign/Status", in, out, opts...)
	if err != nil {
		return nil, err
	}
	return out, nil
}

// EnsignServer is the server API for Ensign service.
// All implementations must embed UnimplementedEnsignServer
// for forward compatibility
type EnsignServer interface {
	// Both the Publish and Subscribe RPCs are bidirectional streaming to allow for acks
	// and nacks of events to be sent between Ensign and the client. The Publish stream
	// is opened and the client sends events and receives acks/nacks -- when the client
	// closes the publish stream, the server sends back information about the current
	// state of the topic. When the Subscribe stream is opened, the client must send an
	// open stream message with the subscription info before receiving events. Once it
	// receives events it must send back acks/nacks up the stream so that Ensign
	// advances the topic offset for the rest of the clients in the group.
	Publish(Ensign_PublishServer) error
	Subscribe(Ensign_SubscribeServer) error
	// This is a simple topic management interface. Right now we assume that topics are
	// immutable, therefore there is no update topic RPC call. There are two ways to
	// delete a topic - archiving it makes the topic readonly so that no events can be
	// published to it, but it can still be read. Destroying the topic deletes it and
	// removes all of its data, freeing up the topic name to be used again.
	ListTopics(context.Context, *PageInfo) (*TopicsPage, error)
	CreateTopic(context.Context, *Topic) (*Topic, error)
	DeleteTopic(context.Context, *TopicMod) (*TopicTombstone, error)
	// Implements a client-side heartbeat that can also be used by monitoring tools.
	Status(context.Context, *HealthCheck) (*ServiceState, error)
	mustEmbedUnimplementedEnsignServer()
}

// UnimplementedEnsignServer must be embedded to have forward compatible implementations.
type UnimplementedEnsignServer struct {
}

func (UnimplementedEnsignServer) Publish(Ensign_PublishServer) error {
	return status.Errorf(codes.Unimplemented, "method Publish not implemented")
}
func (UnimplementedEnsignServer) Subscribe(Ensign_SubscribeServer) error {
	return status.Errorf(codes.Unimplemented, "method Subscribe not implemented")
}
func (UnimplementedEnsignServer) ListTopics(context.Context, *PageInfo) (*TopicsPage, error) {
	return nil, status.Errorf(codes.Unimplemented, "method ListTopics not implemented")
}
func (UnimplementedEnsignServer) CreateTopic(context.Context, *Topic) (*Topic, error) {
	return nil, status.Errorf(codes.Unimplemented, "method CreateTopic not implemented")
}
func (UnimplementedEnsignServer) DeleteTopic(context.Context, *TopicMod) (*TopicTombstone, error) {
	return nil, status.Errorf(codes.Unimplemented, "method DeleteTopic not implemented")
}
func (UnimplementedEnsignServer) Status(context.Context, *HealthCheck) (*ServiceState, error) {
	return nil, status.Errorf(codes.Unimplemented, "method Status not implemented")
}
func (UnimplementedEnsignServer) mustEmbedUnimplementedEnsignServer() {}

// UnsafeEnsignServer may be embedded to opt out of forward compatibility for this service.
// Use of this interface is not recommended, as added methods to EnsignServer will
// result in compilation errors.
type UnsafeEnsignServer interface {
	mustEmbedUnimplementedEnsignServer()
}

func RegisterEnsignServer(s grpc.ServiceRegistrar, srv EnsignServer) {
	s.RegisterService(&Ensign_ServiceDesc, srv)
}

func _Ensign_Publish_Handler(srv interface{}, stream grpc.ServerStream) error {
	return srv.(EnsignServer).Publish(&ensignPublishServer{stream})
}

type Ensign_PublishServer interface {
	Send(*Publication) error
	Recv() (*Event, error)
	grpc.ServerStream
}

type ensignPublishServer struct {
	grpc.ServerStream
}

func (x *ensignPublishServer) Send(m *Publication) error {
	return x.ServerStream.SendMsg(m)
}

func (x *ensignPublishServer) Recv() (*Event, error) {
	m := new(Event)
	if err := x.ServerStream.RecvMsg(m); err != nil {
		return nil, err
	}
	return m, nil
}

func _Ensign_Subscribe_Handler(srv interface{}, stream grpc.ServerStream) error {
	return srv.(EnsignServer).Subscribe(&ensignSubscribeServer{stream})
}

type Ensign_SubscribeServer interface {
	Send(*Event) error
	Recv() (*Subscription, error)
	grpc.ServerStream
}

type ensignSubscribeServer struct {
	grpc.ServerStream
}

func (x *ensignSubscribeServer) Send(m *Event) error {
	return x.ServerStream.SendMsg(m)
}

func (x *ensignSubscribeServer) Recv() (*Subscription, error) {
	m := new(Subscription)
	if err := x.ServerStream.RecvMsg(m); err != nil {
		return nil, err
	}
	return m, nil
}

func _Ensign_ListTopics_Handler(srv interface{}, ctx context.Context, dec func(interface{}) error, interceptor grpc.UnaryServerInterceptor) (interface{}, error) {
	in := new(PageInfo)
	if err := dec(in); err != nil {
		return nil, err
	}
	if interceptor == nil {
		return srv.(EnsignServer).ListTopics(ctx, in)
	}
	info := &grpc.UnaryServerInfo{
		Server:     srv,
		FullMethod: "/ensign.v1beta1.Ensign/ListTopics",
	}
	handler := func(ctx context.Context, req interface{}) (interface{}, error) {
		return srv.(EnsignServer).ListTopics(ctx, req.(*PageInfo))
	}
	return interceptor(ctx, in, info, handler)
}

func _Ensign_CreateTopic_Handler(srv interface{}, ctx context.Context, dec func(interface{}) error, interceptor grpc.UnaryServerInterceptor) (interface{}, error) {
	in := new(Topic)
	if err := dec(in); err != nil {
		return nil, err
	}
	if interceptor == nil {
		return srv.(EnsignServer).CreateTopic(ctx, in)
	}
	info := &grpc.UnaryServerInfo{
		Server:     srv,
		FullMethod: "/ensign.v1beta1.Ensign/CreateTopic",
	}
	handler := func(ctx context.Context, req interface{}) (interface{}, error) {
		return srv.(EnsignServer).CreateTopic(ctx, req.(*Topic))
	}
	return interceptor(ctx, in, info, handler)
}

func _Ensign_DeleteTopic_Handler(srv interface{}, ctx context.Context, dec func(interface{}) error, interceptor grpc.UnaryServerInterceptor) (interface{}, error) {
	in := new(TopicMod)
	if err := dec(in); err != nil {
		return nil, err
	}
	if interceptor == nil {
		return srv.(EnsignServer).DeleteTopic(ctx, in)
	}
	info := &grpc.UnaryServerInfo{
		Server:     srv,
		FullMethod: "/ensign.v1beta1.Ensign/DeleteTopic",
	}
	handler := func(ctx context.Context, req interface{}) (interface{}, error) {
		return srv.(EnsignServer).DeleteTopic(ctx, req.(*TopicMod))
	}
	return interceptor(ctx, in, info, handler)
}

func _Ensign_Status_Handler(srv interface{}, ctx context.Context, dec func(interface{}) error, interceptor grpc.UnaryServerInterceptor) (interface{}, error) {
	in := new(HealthCheck)
	if err := dec(in); err != nil {
		return nil, err
	}
	if interceptor == nil {
		return srv.(EnsignServer).Status(ctx, in)
	}
	info := &grpc.UnaryServerInfo{
		Server:     srv,
		FullMethod: "/ensign.v1beta1.Ensign/Status",
	}
	handler := func(ctx context.Context, req interface{}) (interface{}, error) {
		return srv.(EnsignServer).Status(ctx, req.(*HealthCheck))
	}
	return interceptor(ctx, in, info, handler)
}

// Ensign_ServiceDesc is the grpc.ServiceDesc for Ensign service.
// It's only intended for direct use with grpc.RegisterService,
// and not to be introspected or modified (even as a copy)
var Ensign_ServiceDesc = grpc.ServiceDesc{
	ServiceName: "ensign.v1beta1.Ensign",
	HandlerType: (*EnsignServer)(nil),
	Methods: []grpc.MethodDesc{
		{
			MethodName: "ListTopics",
			Handler:    _Ensign_ListTopics_Handler,
		},
		{
			MethodName: "CreateTopic",
			Handler:    _Ensign_CreateTopic_Handler,
		},
		{
			MethodName: "DeleteTopic",
			Handler:    _Ensign_DeleteTopic_Handler,
		},
		{
			MethodName: "Status",
			Handler:    _Ensign_Status_Handler,
		},
	},
	Streams: []grpc.StreamDesc{
		{
			StreamName:    "Publish",
			Handler:       _Ensign_Publish_Handler,
			ServerStreams: true,
			ClientStreams: true,
		},
		{
			StreamName:    "Subscribe",
			Handler:       _Ensign_Subscribe_Handler,
			ServerStreams: true,
			ClientStreams: true,
		},
	},
	Metadata: "api/v1beta1/ensign.proto",
}
