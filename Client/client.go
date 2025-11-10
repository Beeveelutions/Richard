package main

import (
	proto "Richard/GRPC"
	"context"
	"log"
	"net"
	"strconv"
	"sync"

	"google.golang.org/grpc"
	"google.golang.org/grpc/credentials/insecure"
)

// send requests
var channels = make(map[int]chan Message)

// send approval
var approval = make(map[int]chan Message)

var Queues = make(map[int] []int)

// int for making dif local hosts
var serverId = 0
var port = 0

type Node struct {
	proto.UnimplementedRichardServer
	nodeId      int
	logicalTime int64

	grpc *grpc.Server
}

type Message struct {
	timestamp int64
	nodeId    int64
}

func main() {

	for i := 1; i < 4; i++ {
		go start_client(i)
	}

}

func (server *Node) start_server(noId int) {
	server.grpc = grpc.NewServer()
	serverId++
	baseport := 5000
	port = baseport + serverId
	serverName := ":" + strconv.Itoa(port)
	listener, err := net.Listen("tcp", serverName)

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

func start_client(noId int) {
	var q []int
	channels[noId] = make(chan Message)
	approval[noId] = make(chan Message)
	Queues[noId] = q

	server := &Node{
		logicalTime: 0,
	}

	server.start_server(noId)

	clientServer := "localhost:" + strconv.Itoa(port)
	conn, err := grpc.NewClient(clientServer, grpc.WithTransportCredentials(insecure.NewCredentials()))

	if err != nil {
		log.Fatalf("Not working client 1")
	}
	log.Println("Client", noId, "has connected to server")
	if err != nil {
		log.Fatalf(err.Error())
	}

	client := proto.NewRichardClient(conn)

	go recieveM(client, noId, int(server.logicalTime))

	send(client, noId, 0)

}

func (server *Node) send_request(ctx context.Context, in *proto.AskSend) (*proto.Empty, error) {

	Message := Message{
		timestamp: in.TimeFormated,
		nodeId:    in.NodeId,
	}

	for key, value := range channels {
		if key != int(in.NodeId) {
			value <- Message
		}
	}

	server.logicalTime = max(server.logicalTime, int64(in.TimeFormated)) + 1
	log.Print(server.logicalTime, "-- current logical time")

	return &proto.Empty{}, nil
}

func send(client proto.RichardClient, noId int, logicaltime int) {

	send, err := client.SendRequest(context.Background(),
		&proto.AskSend{
			TimeFormated: 0,
			NodeId:       int64(noId),
		},
	)
	if err != nil {
		log.Fatalf("client not sending messages")
	}
	go recieveA(client, noId)

	log.Println(send)

}

// send the approval to a node
func (server *Node) send_reply(ctx context.Context, in *proto.Proceed) (*proto.Empty, error) {

	return &proto.Empty{}, nil
}

// RECEIVE APPROVAL
func recieveA(client proto.RichardClient, noId int) {
	var yes int
	var mu sync.Mutex

	for {
		nodeMessage := <-approval[noId]
		yes++
		//this is to avoid the "declared and not used" error message
		_ = nodeMessage

		//check if enough aprovals to access critical section
		if yes == len(approval) {
			mu.Lock()
			Critical_Section(noId)
			mu.Unlock()
			yes = 0
			Dequeue(client, noId, Queues[noId])
		}
	}
}

// receive mesages (request for approval from another client), this is also where we will decide if we want to send an approval to a another clients request
func recieveM(client proto.RichardClient, noId int, timestamp int) {

	for {
		nodeMessage := <-approval[noId]
		nodeTimeRecieve := nodeMessage.timestamp

		if nodeTimeRecieve < int64(timestamp) {
			_, err := client.SendReply(context.Background())
			if err != nil {
				log.Fatalf("client not sending messages")
			}
		} else {
			Enqueue(int(nodeMessage.nodeId),  Queues[noId])
		}
	}
}

func Critical_Section(noId int) {
	log.Println("I've accessed the critical section :)", noId)
}

func Enqueue(noId int, q []int) {
	q = append(q, noId)
}

func Dequeue(client proto.RichardClient, noId int, q []int) {
	if IsEmpty(q) {
		return
	}

	for i := 0; i < len(q); i++ {
		_, err := client.SendReply(context.Background(),
			&proto.Proceed{
				Proceed: true,
				NodeId:  int64(i),
			},
		)
		if err != nil {
			log.Fatalf("client not sending reply")
		}
	}

}

func IsEmpty(q []int) bool {
	return len(q) == 0
}
