package main

import (
	proto "Richard/GRPC"
	"context"
	"fmt"
	"log"
	"math/rand/v2"
	"net"
	"strconv"
	"sync"
	"time"

	"google.golang.org/grpc"
	"google.golang.org/grpc/credentials/insecure"
)

// send requests
var channels = make(map[int]chan Message)

// send approval
var approval = make(map[int]chan Message)

var Queues = make(map[int][]int)

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
		fmt.Println("made client", i)
	}

	time.Sleep(5 * time.Minute)
}

func (server *Node) start_server(noId int, ready chan<- bool) {
	server.grpc = grpc.NewServer()
	baseport := 5000
	port := baseport + noId
	serverName := ":" + strconv.Itoa(port)
	listener, err := net.Listen("tcp", serverName)

	if err != nil {
		log.Fatalf("Did not work 1")
	}

	server.logicalTime = 0

	log.Println("the server has started")

	
	proto.RegisterRichardServer(server.grpc, server)
	ready <- true
	err = server.grpc.Serve(listener)

	if err != nil {
		log.Fatalf("Did not work 2")
	}

}

func start_client(noId int) {
	var q []int
	channels[noId] = make(chan Message, 10)
	approval[noId] = make(chan Message, 10)
	Queues[noId] = q
	myRequestTimestamp := -1
	ready := make(chan bool)

	server := &Node{
		logicalTime: 0,
	}

	fmt.Println("making server")

	go server.start_server(noId, ready)

	<-ready

	clients := make(map[int]proto.RichardClient)

	for i := 1; i <= 3; i++ {
		if i == noId {
			continue
		}
		portNumber := 5000 + i
		clientServer := "localhost:" + strconv.Itoa(portNumber)
		conn, err := grpc.NewClient(clientServer, grpc.WithTransportCredentials(insecure.NewCredentials()))

		if err != nil {
			log.Fatalf("Not working client 1")
		}
		clients[i] = proto.NewRichardClient(conn)

		
		if err != nil {
			log.Fatalf(err.Error())
		}
	}
	log.Println("Client", noId, "has connected to server")
	server.logicalTime++

	go recieveM(clients, noId, int(server.logicalTime), myRequestTimestamp)

	for {

		rng := rand.IntN(2)
		if rng == 0 {
			server.logicalTime++
			fmt.Println(noId, "is requesting access to Critical at logical time:", server.logicalTime)
			myRequestTimestamp = int(server.logicalTime)
			send(clients, noId, myRequestTimestamp)
			break
		} else {
			time.Sleep(5 * time.Second)
		}

	}

	fmt.Println(noId, "has sent a message to critical.")

}

func (server *Node) SendRequest(ctx context.Context, in *proto.AskSend) (*proto.Empty, error) {

	Message := Message{
		timestamp: in.TimeFormated,
		nodeId:    in.NodeId,
	}

	for key, value := range channels {
		if key != int(in.NodeId) {
			value <- Message
		}
	}
	server.logicalTime = server.tick(in.TimeFormated)
	log.Print(server.logicalTime, "-- current logical time... Request sent by", in.NodeId)

	return &proto.Empty{}, nil
}

func send(clients map[int]proto.RichardClient, noId int, logicaltime int) {
	for _, client := range clients {
		_, err := client.SendRequest(context.Background(),
			&proto.AskSend{
				TimeFormated: int64(logicaltime),
				NodeId:       int64(noId),
			},
		)
		if err != nil {
			log.Fatalf("client not sending messages")
		}
	}

	log.Println(noId, "has sent message to other clients")

	recieveA(clients, noId, logicaltime)


}

// send the approval to the node named in in.nodeId
func (server *Node) SendReply(ctx context.Context, in *proto.Proceed) (*proto.Empty, error) {
	Message := Message{
		timestamp: server.logicalTime,
		nodeId:    in.NodeId,
	}
	log.Println(in.NodeId, "'s request has been approved by unknown client")
	approval[int(in.NodeId)] <- Message

	return &proto.Empty{}, nil
}

// RECEIVE APPROVAL
func recieveA(clients map[int]proto.RichardClient, noId int, timestamp int) {
	var yes int
	var mu sync.Mutex
	fmt.Println(noId, " is waiting on approval")
	for {
		nodeMessage := <-approval[noId]

		timestamp = max(timestamp, int(nodeMessage.timestamp)) + 1
		yes++
		//this is to avoid the "declared and not used" error message
		_ = nodeMessage

		//check if enough aprovals to access critical section
		if yes == len(approval)-1 {
			mu.Lock()
			Critical_Section(noId)
			mu.Unlock()
			timestamp++
			yes = 0
			Dequeue(clients, noId, timestamp)
			break
		}
	}
}

// receive mesages (request for approval from another client), this is also where we will decide if we want to send an approval to a another clients request
func recieveM(clients map[int]proto.RichardClient, noId int, timestamp int, localTimestamp int) {

	for {
		nodeMessage := <-channels[noId]
		fmt.Println(noId, "recieved request")
		timestamp = max(timestamp, int(nodeMessage.timestamp)) + 1

		nodeTimeRecieve := nodeMessage.timestamp

		if nodeTimeRecieve < int64(localTimestamp) || localTimestamp == -1 || (nodeMessage.timestamp == int64(localTimestamp) && nodeMessage.nodeId < int64(noId)) {
			client := clients[int(nodeMessage.nodeId)]
			_, err := client.SendReply(context.Background(), &proto.Proceed{
				Proceed: true,
				NodeId:  int64(nodeMessage.nodeId),
			})
			if err != nil {
				log.Fatalf("client not sending messages")
			}
			log.Println(noId, "is sending approval to", nodeMessage.nodeId)
		} else {
			log.Println("Request has been queued")
			Enqueue(noId, int(nodeMessage.nodeId))

		}

	}
}

func Critical_Section(noId int) {
	log.Println("I've accessed the critical section :)", noId)
}

func Enqueue(noId int, nodeId int) {
	q := Queues[noId]
	q = append(q, nodeId)
	Queues[noId] = q
	fmt.Println("Request is in queue")
}

func Dequeue(clients map[int]proto.RichardClient, noId int, timestamp int) {
	q := Queues[noId]

	for len(q) > 0 {
		target := q[0]
		q = q[1:]

		client := clients[target]
		_, err := client.SendReply(context.Background(),
			&proto.Proceed{
				Proceed: true,
				NodeId:  int64(noId),
			},
		)
		if err != nil {
			log.Fatalf("client not sending reply")
		}
		timestamp++
		log.Println(noId, "is sending approval to", target)
	}

	Queues[noId] = q

}

func IsEmpty(q []int) bool {
	return len(q) == 0
}

func (n *Node) tick(received int64) int64 {
	n.logicalTime = max(n.logicalTime, received) + 1
	return n.logicalTime
}
