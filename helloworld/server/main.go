package main

import (
	"context"
	"log"
	"net"
	"time"
	"os"

	"google.golang.org/api/option"
	"google.golang.org/grpc"
	pb "google.golang.org/grpc/examples/helloworld/helloworld"
	"github.com/apigee/registry/gapic"
	emptypb "github.com/golang/protobuf/ptypes/empty"
	"google.golang.org/grpc/reflection"
)

const (
	port = ":8081"
)

// server is used to implement helloworld.GreeterServer.
type server struct {
	pb.UnimplementedGreeterServer
}

// SayHello implements helloworld.GreeterServer
func (s *server) SayHello(ctx context.Context, in *pb.HelloRequest) (*pb.HelloReply, error) {
	log.Printf("Received: %v", in.GetName())
	registry_hostname := os.Getenv("REGISTRY_HOSTNAME")
	if len(registry_hostname) == 0 {
		registry_hostname = "registry-backend.registry.svc.cluster.local"
	}
	registry_port := os.Getenv("REGISTRY_PORT")
	if len(registry_port) == 0 {
		registry_port = "8080"
	}
	registry_address := registry_hostname + ":" + registry_port
	log.Printf("Listening on: %v", registry_address)

	// Set up a connection to the server.
	var opts []option.ClientOption
	conn, err := grpc.Dial(registry_address, grpc.WithInsecure())
	if err != nil {
		return nil, err
	}

	opts = append(opts, option.WithGRPCConn(conn))
	opts = append(opts, option.WithEndpoint(registry_address))
	ctx, cancel := context.WithTimeout(context.Background(), time.Second)
	defer cancel()
	c, _ := gapic.NewRegistryClient(ctx, opts...)

	req := &emptypb.Empty{}
	resp, _ := c.GetStatus(ctx, req)

	return &pb.HelloReply{Message: "Hello " + resp.GetMessage()}, nil
}

func main() {
	lis, err := net.Listen("tcp", port)
	log.Print("Starting")
	if err != nil {
		log.Fatalf("failed to listen: %v", err)
	}
	s := grpc.NewServer()
	reflection.Register(s)
	pb.RegisterGreeterServer(s, &server{})
	if err := s.Serve(lis); err != nil {
		log.Fatalf("failed to serve: %v", err)
	}
}