package grpc_handler

import (
	"context"
	"log"
	myservice2 "service-template/cmd/grps_server/pb/myservice2/pb"
)

func (ms2 *MyService2) SayHello(ctx context.Context, req *myservice2.HelloRequest) (*myservice2.HelloReply, error) {
	log.Printf("Received: %v", req.Name)
	return &myservice2.HelloReply{Message: "Hello 2" + req.Name}, nil
}
