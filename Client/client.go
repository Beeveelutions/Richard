package main

import (
	proto "Richard/GRPC"
	"bufio"
	"context"
	"fmt"
	"log"
	"math/rand/v2"
	"net"
	"os"
	"strconv"
	"sync"
	"time"

	"google.golang.org/grpc"
	"google.golang.org/grpc/credentials/insecure"
)

var Queues = make(map[int64][]int64)
var approvalChannel map[int64]chan struct{}

type Message struct {
	timestamp int64
	nodeId    int64
}

type Richard_service struct {
	proto.UnimplementedRichardServer
	error      chan error
	grpc       *grpc.Server
	serverPort string
	highest    int
	//first int is client id, second is highest bid that that client has made

	ports             []string
	peers             map[string]proto.RichardClient //client pointing to other servers
	listener          net.Listener
	nodeId            int
	logicalTime       int64
	incriticalsection bool
	requesttimestamp  int
}

func main() {
	approvalChannel = make(map[int64]chan struct{})

	ports := []string{
		":5050",
		":5051",
		":5052",
	}

	server := &Richard_service{}

	log.Println("Enter the port of the server (A number from 0 to 2)")
	scanner := bufio.NewScanner(os.Stdin)
	scanner.Scan()
	Text := scanner.Text()

	if Text == "0" || Text == "1" || Text == "2" {
		port, _ := strconv.ParseInt(Text, 10, 64)
		go server.start_server(ports[port], ports, int(port))
		log.Println("Port selected: " + ports[port])
	} else {
		log.Println("Enter the correct port of the server (A number from 0 to 2)")
	}

	/*go server.start_server(":5050",ports)
	go server.start_server(":5051",ports)
	go server.start_server(":5052",ports)*/

	select {}
}

func (server *Richard_service) start_server(numberPort string, ports []string, nodeId int) {
	var q []int

	Queues[int64(nodeId)] = q
	server.grpc = grpc.NewServer()

	approvalChannel[int64(nodeId)] = make(chan struct{}, 10)

	listener, err := net.Listen("tcp", numberPort)

	if err != nil {
		log.Fatalf("Did not work 1")
	}

	server.peers = make(map[string]proto.RichardClient)
	server.listener = listener
	server.logicalTime = 0

	log.Println("the server has started")
	server.ports = ports

	for _, value := range server.ports {
		if value != numberPort {
			connection := "localhost" + value
			conn, err := grpc.NewClient(connection, grpc.WithTransportCredentials(insecure.NewCredentials()))
			if err != nil {
				log.Fatalf("connection failed")
			}

			client := proto.NewRichardClient(conn)
			server.peers[value] = client
		}
	}

	proto.RegisterRichardServer(server.grpc, server)
	port, _ := strconv.ParseInt(numberPort, 10, 64)

	go crit_call(int(server.logicalTime), port, server.peers)

	err = server.grpc.Serve(listener)

	if err != nil {
		log.Fatalf("Did not work 2")
	}

}

func crit_call(logicalTime int, nodeId int64, peers map[string]proto.RichardClient) {
	for {

		rng := rand.IntN(2)
		if rng == 0 {
			logicalTime++
			fmt.Println(nodeId, "is requesting access to Critical at logical time:", logicalTime)
			myRequestTimestamp := int(logicalTime)
			send(peers, nodeId, myRequestTimestamp)
		} else {
			time.Sleep(5 * time.Second)
		}

	}

}

func (server *Richard_service) SendRequest(ctx context.Context, in *proto.AskSend) (*proto.Proceed, error) {

	fmt.Println(in.NodeId, "recieved request")

	nodeTimeRecieve := in.TimeFormated

	if nodeTimeRecieve < int64(server.logicalTime) || server.logicalTime == -1 || (in.TimeFormated == int64(server.logicalTime) && in.NodeId < int64(server.nodeId)) {
		log.Println(server.nodeId, "is sending approval to", in.NodeId)
		log.Print(server.logicalTime, "-- current logical time... Request sent by", in.NodeId)

		server.logicalTime = max(server.logicalTime, in.TimeFormated) + 1

		return &proto.Proceed{
			Proceed: true,
			NodeId:  int64(server.nodeId),
		}, nil
	} else {
		server.logicalTime = max(server.logicalTime, in.TimeFormated) + 1

		log.Print(server.logicalTime, "-- current logical time... Request sent by", in.NodeId)

		log.Println("Request has been queued")
		Enqueue(server.nodeId, int(in.NodeId))
		return &proto.Proceed{
			Proceed: false,
			NodeId:  int64(server.nodeId),
		}, nil
	}
}

func send(clients map[string]proto.RichardClient, noId int64, logicaltime int) {
	var mu sync.Mutex
	count := 0
	for _, client := range clients {
		send, err := client.SendRequest(context.Background(),
			&proto.AskSend{
				TimeFormated: int64(logicaltime),
				NodeId:       int64(noId),
			},
		)
		if err != nil {
			log.Fatalf("client not sending messages")
		}

		if send.ProceedBool == true {
			count++
		}

	}

	// Wait for remaining approvals from queued nodes
	for count < len(clients) {
		select {
		case <-approvalChannel[noId]: // you need a channel per node
			count++
		}
	}

	mu.Lock()
	server.Critical_Section(noId)
	mu.Unlock()

	count = 0
}

func (server *Richard_service) Critical_Section(noId int64) {
	// Enter critical section
	server.incriticalsection = true
	log.Println(server.nodeId, "entered critical section at logical time", server.logicalTime)

	// Simulate CS work
	time.Sleep(2 * time.Second)

	// Leave critical section
	server.incriticalsection = false
	server.requesttimestamp = -1
	log.Println(server.nodeId, "leaving critical section")

	// Send approvals to queued requests
	leaveCriticalSection(server)
}

func leaveCriticalSection(server *Richard_service) {
	q := Queues[int64(server.nodeId)]
	Queues[int64(server.nodeId)] = []int64{}

	for _, target := range q {
		peer := server.peers[fmt.Sprintf(":%d", target)]
		peer.SendReply(context.Background(), &proto.Proceed{
			Proceed: true,
			NodeId:  int64(server.nodeId),
		})
	}
}

func (server *Richard_service) SendReply(ctx context.Context, in *proto.Proceed) (*proto.Empty, error) {

	if in.Proceed {
		approvalChannel[int64(in.NodeId)] <- struct{}{}
	}

	return &proto.Empty{}, nil
}

func Enqueue(noId int, nodeId int) {
	q := Queues[int64(noId)]
	q = append(q, int64(nodeId))
	Queues[int64(noId)] = q
	fmt.Println("Request is in queue")
}

func Dequeue(clients map[string]proto.RichardClient, noId int64, timestamp int) {
	q := Queues[noId]

	for len(q) > 0 {
		target := q[0]
		q = q[1:]

		client := clients[string(target)]
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

func (n *Richard_service) tick(received int64) int64 {
	n.logicalTime = max(n.logicalTime, received) + 1
	return n.logicalTime
}
