package main

import (
	"context"
	"log"
	"net"
	"os"
	"strconv"
	"time"

	pb "github.com/CasperAntonPoulsen/disysminiproject3/proto"
	"google.golang.org/grpc"
)

var (
	client pb.AuctionClient
)

type Request struct {
	user   *pb.User
	stream pb.Auction_RequestTokenServer
}

type Auction struct {
	amount   int32
	bidderID int32
}

type Replica struct {
	id         int32
	port       string
	lamport    int32
	connection pb.AuctionClient
}

type Server struct {
	pb.UnimplementedAuctionServer
	RequestQueue chan Request
	Release      chan *pb.Release
	Leader       Replica
	Replicas          []Replica
	id           int32
	error        chan error
	lamport    int32
	auction Auction
}

func (s *Server) RequestToken(rqst *pb.Request, stream pb.Auction_RequestTokenServer) error {

	request := Request{user: rqst.User, stream: stream}

	go func() { s.RequestQueue <- request }()

	log.Printf("Request token added to queue from: %v", rqst.User.Userid)
	return <-s.error
}

func (s *Server) ReleaseToken(ctx context.Context, release *pb.Release) (*pb.Empty, error) {
	log.Printf("Recieved release token from: %v", release.User.Userid)

	go func() { s.Release <- &pb.Release{User: release.User} }()

	return &pb.Empty{}, nil
}

func (s *Server) RequestResult(ctx context.Context, user *pb.User) (*pb.Result, error) {

	return &pb.Result{Amount: s.auction.amount}, nil
}

func (s *Server) MakeBid(ctx context.Context, bid *pb.Bid) (*pb.Acknowledgement, error) {

	if bid.Amount <= s.auction.amount {
		return &pb.Acknowledgement{Status: "fail"}, nil
	}

	s.auction.amount = bid.Amount
	s.auction.bidderID = bid.Userid

	if s.id == s.Leader.id {
		s.broadcastBid(bid)
	}

	return &pb.Acknowledgement{Status: "success"}, nil
}

//write to other replicationmanagers (Servers)
func (s *Server) broadcastBid(bid *pb.Bid) {
	for _, rm := range s.Replicas {
		_, err := rm.connection.MakeBid(context.Background(), bid)
		if err != nil {
			log.Printf("could not connect to rm %v: %v", rm.id+1, err)
		}

	}
}

func (s *Server) Ping(ctx context.Context, empty *pb.Empty) (*pb.Empty, error) {
	return empty, nil
}

func GrantToken(rqst Request) error {
	log.Printf("Granting token to: %v", rqst.user.Userid)
	err := rqst.stream.Send(&pb.Grant{User: rqst.user})
	return err
}

//helper function to convert string to int32
func GetIntEnv(envvar string) int32 {
	envvarstring := os.Getenv(envvar)
	envvarint, err := strconv.Atoi(envvarstring)
	if err != nil {
		log.Fatalf("not a valid int: %v", err)
	}

	return int32(envvarint)
}

//Election Method-----------------------------------------------------------------------------
func CallElection(RmArr []Replica) {
	//ask other RM's for their lamport-time ¤
	max := CompareLamports(RmArr)
	

	//if lamport-time is less then self, do nothing ¤
	if max[0] >  Server.lamport{
	} else if max[0] <  Server.lamport{ //else if lamport-time more then all others, then make self leader, and message others that self is leader. ¤
		//tell all others that self is the new leader ¤
	} else { //else call election, to RM's with higher ID then self. ¤
	for _, rm := range RmArr {
		if rm.id > Server.id {
			//if response, do nothing ¤
			//if failure, make self leader, and broadcast to all others that self is leader ¤
		}
	}
	}
	
	
}

func GetReplicaLamports(RArr []Replica) {
	for _, rm := range RArr {
		//ask for lamport time from replica ¤
		//wait for response, and then update the replicas lamport time. ¤
		rm.lamport = //insert lamporttime from replicamanager ¤
	}
}

func CompareLamports(RmArr []Replica) []Replica {
	maxLamport := int32(0)
	managersWithMax := make([]Replica, len(RmArr))
	for _, rm := range RmArr {
		if rm.lamport > maxLamport {
			maxLamport = rm.lamport
		}
	}

	for _, rm := range RmArr {
		if rm.lamport == maxLamport {
			managersWithMax =  append(managersWithMax, rm)
		}
	}

	return managersWithMax
}

//main----------------------------------------------------------------------------------------
func main() {
	//setup server
	grpcServer := grpc.NewServer()
	listener, err := net.Listen("tcp", ":8080")

	if err != nil {
		log.Fatalf("Error, couldn't create the server %v", err)
	}
	//make queues
	requestqueue := make(chan Request)
	releasequeue := make(chan *pb.Release)
	//load variables from server.env
	leaderid := GetIntEnv("DEFAULTLEADER")

	id := GetIntEnv("ID")

	NumReplicas := GetIntEnv("NREPLICATIONMANAGERS")

	// construct list of all Replica managers and their ports
	var Replicas []Replica

	for i := 0; i < int(NumReplicas); i++ {

		// does not include itself
		if int32(i+1) == id {
			continue
		}
		portInt := 8080 + i
		port := ":" + strconv.Itoa(portInt)

		conn, err := grpc.Dial("localhost"+port, grpc.WithInsecure())
		if err != nil {
			log.Printf("could not connect to rm %v: %v", i+1, err)
		}

		rmClient := pb.NewAuctionClient(conn)

		Replicas = append(Replicas, Replica{id: int32(i + 1), port: port, connection: rmClient})
	}

	// construct server struct
	server := Server{
		RequestQueue: requestqueue,
		Release:      releasequeue,
		Leader:       Replica{id: leaderid, port: os.Getenv("DEFAULTLEADERPORT")},
		id:           id,
		Replicas:          Replicas,
	}

	// Initialize with a starting bid
	server.auction = Auction{amount: 50, bidderID: 0}

	pb.RegisterAuctionServer(grpcServer, &server)
	go func() {
		for {
			log.Print("Checking requests \n")
			rqst := <-server.RequestQueue
			err := GrantToken(rqst)
			if err != nil {
				log.Fatalf("Failed to send grant token: %v", err)
			}
			release := <-server.Release
			log.Printf("Release token recieved from: %v", release.User.Userid)
		}
	}()
	//ping leader if not self leader
	go func() {
		for {
			for _, rm := range server.Replicas {
				if rm.id == leaderid { //ping leader
					_, err := rm.connection.Ping(context.Background(), &pb.Empty{})
					//If we get a resonse error, we assume that the server has crash failure ¤
					if err != nil {
						// start election ¤
						CallElection(server.Replicas)
					}
				}
			}
			time.Sleep(5 * time.Second)
		}
	}()

	grpcServer.Serve(listener)
}
