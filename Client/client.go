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

var channels = make(map[int64]chan Message)
var approval = make(map[int64]chan bool)

type Node struct {
	proto.UnimplementedRichardServer
	nodeId      int64
	logicalTime int64
	grpc        *grpc.Server
	
}

type Message struct {
	timestamp int64
	nodeId int64
	
}

func main() {

	

	for i := 1; i < 4; i++ {
		go start_client(i)

	}

}

func (server *Node) start_server(noId int64) {
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

func (server *Node) send_request(ctx context.Context, in *proto.AskSend) (*proto.Empty, error) {


	Message := Message{
		timestamp: in.TimeFormated,
		nodeId: in.NodeId,
	}

	for key, value := range channels{
		if (key != in.NodeId){
			value <- Message
		}
	}

	server.logicalTime = max(server.logicalTime, in.TimeFormated) + 1
	log.Print(server.logicalTime, "-- current logical time")

	return &proto.Empty{}, nil
}

func (server *Node) send_reply(ctx context.Context, in *proto.Proceed) (*proto.Empty, error) {

	return &proto.Empty{}, nil
}

func Critical_Section() {

}

func start_client(noId int64) {

	channels[noId] = make(chan int)

	server := &Node{
		logicalTime: 0,
	}

	server.start_server(noId)

	conn, err := grpc.NewClient("localhost:5050", grpc.WithTransportCredentials(insecure.NewCredentials()))

	if err != nil {
		log.Fatalf("Not working client 1")
	}
	log.Println("Client", noId, "has connected to server")
	if err != nil {
		log.Fatalf(err.Error())
	}


	client := proto.NewRichardClient(conn)

	go recieve(client, noId)

	send(client, noId, 0)

}

func send(client proto.RichardClient, noId int64, logicaltime int64) {

	send, err := client.SendRequest(context.Background(),
		&proto.AskSend{
			TimeFormated: 0,
			NodeId:       noId,
		},
	)
	if err != nil {
		log.Fatalf("client not sending messages")
	}

	log.Println(send)

	
}

func recieve(client proto.RichardClient, noId int64) {
	for {
		nodeMessage := <- channels[noId]
		nodeIdRecieve := nodeMessage.nodeId
		nodeTimeRecieve := nodeMessage.timestamp
		


		if()

	}
}
