package grpc_handler

import (
	"context"
	"fmt"
	"log"
	myservice "service-template/cmd/grps_server/pb/myservice/pb"
)

func (ms *MyService) SayHello(ctx context.Context, req *myservice.HelloRequest) (*myservice.HelloReply, error) {
	log.Printf("Received: %v", req.Name)
	srv := ms.srv
	postgres := ms.postgres

	fmt.Println(srv)

	row := postgres.GetDB().QueryRow("insert into user_p(id) values (1);")
	err := row.Err()
	fmt.Println(err)
	row1 := postgres.GetDB().QueryRow("select count(*) from user_p")
	err1 := row1.Err()
	fmt.Println(err1)
	a := 0
	row1.Scan(&a)
	fmt.Println(a)

	return &myservice.HelloReply{Message: "Hello 1 " + string(rune(a))}, nil
}
