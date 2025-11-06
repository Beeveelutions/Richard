package main

import (
	proto "Richard/GRPC"
	"context"

	"log"
	"net"
	"os/user"

	"google.golang.org/grpc"
	"google.golang.org/grpc/credentials/insecure"
)

type richard_service struct{

	logicalTime int64
	grpc *grpc.Server
}


func main() {
	server := &richard_service{
		logicalTime: 0,
	}

	server.start_server()
	
	conn, err := grpc.NewClient("localhost:5050", grpc.WithTransportCredentials(insecure.NewCredentials()))

	if err != nil {
		log.Fatalf("Not working client 1")
	}
	log.Println("Client has connected to server")
	currentUser, err := user.Current()
	if err != nil {
		log.Fatalf(err.Error())
	}

	client := proto.NewRichardClient(conn)

}


func (server *richard_service) start_server() {
	server.grpc = grpc.NewServer()
	listener, err := net.Listen("tcp", ":5050")
	
	if err != nil {
		log.Fatalf("Did not work 1")
	}

	log.Println("the server has started")

	proto.RegisterRichardServer(server.grpc, server)

	err = server.grpc.Serve(listener)

	if err != nil {
		log.Fatalf("Did not work 2")
	}

}

func (server *richard_service) send_request(ctx context.Context, in *proto.AskSend) (*proto.Empty, error) {
	
	


	return &proto.Empty{}, nil
}

func (server *richard_service) send_reply(ctx context.Context, in *proto.Proceed) (*proto.Empty, error) {
	
	return &proto.Empty{}, nil
}
