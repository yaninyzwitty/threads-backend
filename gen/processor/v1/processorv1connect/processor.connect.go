// Code generated by protoc-gen-connect-go. DO NOT EDIT.
//
// Source: processor/v1/processor.proto

package processorv1connect

import (
	connect "connectrpc.com/connect"
	context "context"
	errors "errors"
	v1 "github.com/yaninyzwitty/threads-go-backend/gen/processor/v1"
	http "net/http"
	strings "strings"
)

// This is a compile-time assertion to ensure that this generated file and the connect package are
// compatible. If you get a compiler error that this constant is not defined, this code was
// generated with a version of connect newer than the one compiled into your binary. You can fix the
// problem by either regenerating this code with an older version of connect or updating the connect
// version compiled into your binary.
const _ = connect.IsAtLeastVersion1_13_0

const (
	// ProcessorServiceName is the fully-qualified name of the ProcessorService service.
	ProcessorServiceName = "processor.v1.ProcessorService"
)

// These constants are the fully-qualified names of the RPCs defined in this package. They're
// exposed at runtime as Spec.Procedure and as the final two segments of the HTTP route.
//
// Note that these are different from the fully-qualified method names used by
// google.golang.org/protobuf/reflect/protoreflect. To convert from these constants to
// reflection-formatted method names, remove the leading slash and convert the remaining slash to a
// period.
const (
	// ProcessorServiceProcessOutboxMessageProcedure is the fully-qualified name of the
	// ProcessorService's ProcessOutboxMessage RPC.
	ProcessorServiceProcessOutboxMessageProcedure = "/processor.v1.ProcessorService/ProcessOutboxMessage"
)

// ProcessorServiceClient is a client for the processor.v1.ProcessorService service.
type ProcessorServiceClient interface {
	ProcessOutboxMessage(context.Context, *connect.Request[v1.ProcessOutboxMessageRequest]) (*connect.Response[v1.ProcessOutboxMessageResponse], error)
}

// NewProcessorServiceClient constructs a client for the processor.v1.ProcessorService service. By
// default, it uses the Connect protocol with the binary Protobuf Codec, asks for gzipped responses,
// and sends uncompressed requests. To use the gRPC or gRPC-Web protocols, supply the
// connect.WithGRPC() or connect.WithGRPCWeb() options.
//
// The URL supplied here should be the base URL for the Connect or gRPC server (for example,
// http://api.acme.com or https://acme.com/grpc).
func NewProcessorServiceClient(httpClient connect.HTTPClient, baseURL string, opts ...connect.ClientOption) ProcessorServiceClient {
	baseURL = strings.TrimRight(baseURL, "/")
	processorServiceMethods := v1.File_processor_v1_processor_proto.Services().ByName("ProcessorService").Methods()
	return &processorServiceClient{
		processOutboxMessage: connect.NewClient[v1.ProcessOutboxMessageRequest, v1.ProcessOutboxMessageResponse](
			httpClient,
			baseURL+ProcessorServiceProcessOutboxMessageProcedure,
			connect.WithSchema(processorServiceMethods.ByName("ProcessOutboxMessage")),
			connect.WithClientOptions(opts...),
		),
	}
}

// processorServiceClient implements ProcessorServiceClient.
type processorServiceClient struct {
	processOutboxMessage *connect.Client[v1.ProcessOutboxMessageRequest, v1.ProcessOutboxMessageResponse]
}

// ProcessOutboxMessage calls processor.v1.ProcessorService.ProcessOutboxMessage.
func (c *processorServiceClient) ProcessOutboxMessage(ctx context.Context, req *connect.Request[v1.ProcessOutboxMessageRequest]) (*connect.Response[v1.ProcessOutboxMessageResponse], error) {
	return c.processOutboxMessage.CallUnary(ctx, req)
}

// ProcessorServiceHandler is an implementation of the processor.v1.ProcessorService service.
type ProcessorServiceHandler interface {
	ProcessOutboxMessage(context.Context, *connect.Request[v1.ProcessOutboxMessageRequest]) (*connect.Response[v1.ProcessOutboxMessageResponse], error)
}

// NewProcessorServiceHandler builds an HTTP handler from the service implementation. It returns the
// path on which to mount the handler and the handler itself.
//
// By default, handlers support the Connect, gRPC, and gRPC-Web protocols with the binary Protobuf
// and JSON codecs. They also support gzip compression.
func NewProcessorServiceHandler(svc ProcessorServiceHandler, opts ...connect.HandlerOption) (string, http.Handler) {
	processorServiceMethods := v1.File_processor_v1_processor_proto.Services().ByName("ProcessorService").Methods()
	processorServiceProcessOutboxMessageHandler := connect.NewUnaryHandler(
		ProcessorServiceProcessOutboxMessageProcedure,
		svc.ProcessOutboxMessage,
		connect.WithSchema(processorServiceMethods.ByName("ProcessOutboxMessage")),
		connect.WithHandlerOptions(opts...),
	)
	return "/processor.v1.ProcessorService/", http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		switch r.URL.Path {
		case ProcessorServiceProcessOutboxMessageProcedure:
			processorServiceProcessOutboxMessageHandler.ServeHTTP(w, r)
		default:
			http.NotFound(w, r)
		}
	})
}

// UnimplementedProcessorServiceHandler returns CodeUnimplemented from all methods.
type UnimplementedProcessorServiceHandler struct{}

func (UnimplementedProcessorServiceHandler) ProcessOutboxMessage(context.Context, *connect.Request[v1.ProcessOutboxMessageRequest]) (*connect.Response[v1.ProcessOutboxMessageResponse], error) {
	return nil, connect.NewError(connect.CodeUnimplemented, errors.New("processor.v1.ProcessorService.ProcessOutboxMessage is not implemented"))
}
