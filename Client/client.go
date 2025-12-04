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
var approvalChannel chan int64

var mu sync.Mutex
var muState sync.Mutex
var muQ sync.Mutex

type Richard_service struct {
	proto.UnimplementedRichardServer
	grpc       *grpc.Server
	//first int is client id, second is highest bid that that client has made

	ports             []string
	peers             map[string]proto.RichardClient //client pointing to other servers
	listener          net.Listener
	nodeId            int
	logicalTime       int64
	state			  string
}

func main() {

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

	select {}
}

func (server *Richard_service) start_server(numberPort string, ports []string, nodeId int) {
	var q []int64
	server.nodeId = nodeId
	Queues[int64(nodeId)] = q
	server.grpc = grpc.NewServer()

	approvalChannel = make(chan int64)

	listener, err := net.Listen("tcp", numberPort)

	if err != nil {
		log.Fatalf("Did not work 1")
	}

	server.peers = make(map[string]proto.RichardClient)
	server.listener = listener
	server.logicalTime = 0
	server.state = "FREE"

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
	

	go crit_call(int(server.logicalTime), int64(nodeId), server.peers, server)

	err = server.grpc.Serve(listener)

	if err != nil {
		log.Fatalf("Did not work 2")
	}

}

func crit_call(logicalTime int, nodeId int64, peers map[string]proto.RichardClient, server *Richard_service) {
				time.Sleep(30 * time.Second)

	for {
		log.Println("Figuring out behaviour")
		rng := rand.IntN(2)
		if rng == 0 {
			send(peers, nodeId, server)
		} else {
			time.Sleep(5 * time.Second)
		}

	}

}

func (server *Richard_service) SendRequest(ctx context.Context, in *proto.AskSend) (*proto.Proceed, error) {

	fmt.Println(in.NodeId, "has sent request")

	if server.state == "HELD" {
		//Increment logical time two times because one is for receiving the message, and the second one is for replying to the message
		mu.Lock()
		server.logicalTime = max(server.logicalTime, in.TimeFormated) + 1
		server.logicalTime++;
		mu.Unlock()
		log.Println("Request has been queued because", server.nodeId, "is in critical")
		muQ.Lock()
		Enqueue(server.nodeId, int(in.NodeId))
		muQ.Unlock()
		return &proto.Proceed{
			ProceedBool: false,
			NodeId:  int64(server.nodeId),
		}, nil

	}
	if server.state == "WANTED" && (int64(server.logicalTime) < in.TimeFormated){
		mu.Lock()
		server.logicalTime = max(server.logicalTime, in.TimeFormated) + 1
		server.logicalTime++;
		mu.Unlock()
		log.Println("Request has been queued because", server.nodeId, "is wanted and has smaller timestamp")
		muQ.Lock()
		Enqueue(server.nodeId, int(in.NodeId))
		muQ.Unlock()
		return &proto.Proceed{
			ProceedBool: false,
			NodeId:  int64(server.nodeId),
		}, nil
	// if node is wanted and both timestamps are same, defer if node recieving message is smaller then node sending message
	} else if server.state == "WANTED" && (int64(server.logicalTime) == in.TimeFormated) && (int64(server.nodeId) < in.NodeId)  {
		mu.Lock()
		server.logicalTime = max(server.logicalTime, in.TimeFormated) + 1
		server.logicalTime++;
		mu.Unlock()
		log.Println("Request has been queued because", server.nodeId, "is wanted and has smaller nodeid due to having same timestamp")
		muQ.Lock()
		Enqueue(server.nodeId, int(in.NodeId))
		muQ.Unlock()
		return &proto.Proceed{
			ProceedBool: false,
			NodeId:  int64(server.nodeId),
		}, nil
	} else {
		log.Println(server.nodeId, "is sending approval to", in.NodeId)
		mu.Lock()
		server.logicalTime = max(server.logicalTime, in.TimeFormated) + 1
		server.logicalTime++;
		mu.Unlock()
		return &proto.Proceed{
			ProceedBool: true,
			NodeId:  int64(server.nodeId),
		}, nil
	}

}

//this is where a node sends a request to other nodes
func send(clients map[string]proto.RichardClient, noId int64, server *Richard_service) {
	//node is requesting thus setting state to Wanted
	muState.Lock()
	server.state = "WANTED"
	muState.Unlock()
	count := 0
	for _, client := range clients {
		mu.Lock()
		server.logicalTime++
		mu.Unlock()
		fmt.Println(noId, "is requesting access to Critical at logical time:", server.logicalTime)
		send, err := client.SendRequest(context.Background(),
			&proto.AskSend{
				TimeFormated: int64(server.logicalTime),
				NodeId:       int64(noId),
			},
		)
		if err != nil {
			log.Fatalf("client not sending messages")
		}

		if send.ProceedBool {
			count++
		}
		mu.Lock()
		server.logicalTime = max(server.logicalTime, send.TimeFormated) + 1
		mu.Unlock()


	}

	log.Println("Waiting for approval")

	//Waits for approval (if didn't get approval from all (and was therefore set in a queue))
	for count < len(server.peers) {
		select {
		case <-approvalChannel: 
			count++
		}
	}
	log.Println("approved")

	Critical_Section(noId, server)

	count = 0
}

func  Critical_Section(noId int64, server *Richard_service) {
	//Node is in critical thus state is Held
	muState.Lock()
	server.state = "HELD"
	muState.Unlock()
	// Enter critical section
	log.Println(server.nodeId, "entered critical section at logical time", server.logicalTime)

	// Simulate CS work
	time.Sleep(2 * time.Second)

	// Leave critical section
	log.Println(server.nodeId, "leaving critical section")

	// State til free
	muState.Lock()
	server.state = "FREE"
	muState.Unlock()

	//will now dequeue and send approval to all requests in the queue
	leaveCriticalSection(server)
}

func leaveCriticalSection(server *Richard_service) {
	muQ.Lock()
	q := Queues[int64(server.nodeId)]
	Queues[int64(server.nodeId)] = []int64{}
	muQ.Unlock()

	for _, target := range q {
		if target == 0{
			peer := server.peers[":5050"]
			peer.SendReply(context.Background(), &proto.Proceed{
			ProceedBool: true,
			NodeId:  int64(server.nodeId),
		})
		} else if target == 1 {
			peer := server.peers[":5051"]
			peer.SendReply(context.Background(), &proto.Proceed{
			ProceedBool: true,
			NodeId:  int64(server.nodeId),
			})
		} else {
			peer := server.peers[":5052"]
			peer.SendReply(context.Background(), &proto.Proceed{
			ProceedBool: true,
			NodeId:  int64(server.nodeId),
			})
		}
		
	}
	
}

func (server *Richard_service) SendReply(ctx context.Context, in *proto.Proceed) (*proto.Empty, error) {
	// send approval into the nodes approval channel to unlock the node
	approvalChannel <- 1
	return &proto.Empty{}, nil
}

//queues in the node
func Enqueue(noId int, nodeId int) {
	q := Queues[int64(noId)]
	q = append(q, int64(nodeId))
	Queues[int64(noId)] = q
	fmt.Println("Request is in queue")
}



