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

type ReplicationManager struct {
	id         int32
	port       string
	connection pb.AuctionClient
}

type Server struct {
	pb.UnimplementedAuctionServer
	RequestQueue chan Request
	Release      chan *pb.Release
	Leader       ReplicationManager
	RMs          []ReplicationManager
	id           int32
	error        chan error

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
	for _, rm := range s.RMs {
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

	NumRms := GetIntEnv("NREPLICATIONMANAGERS")

	// construct list of all replication managers and their ports
	var Rms []ReplicationManager

	for i := 0; i < int(NumRms); i++ {

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

		Rms = append(Rms, ReplicationManager{id: int32(i + 1), port: port, connection: rmClient})
	}

	// construct server struct
	server := Server{
		RequestQueue: requestqueue,
		Release:      releasequeue,
		Leader:       ReplicationManager{id: leaderid, port: os.Getenv("DEFAULTLEADERPORT")},
		id:           id,
		RMs:          Rms,
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
			for _, rm := range server.RMs {
				if rm.id == leaderid { //ping leader
					_, err := rm.connection.Ping(context.Background(), &pb.Empty{})
					if err != nil {
						// start election
					}
				}
			}
			time.Sleep(5 * time.Second)
		}
	}()

	grpcServer.Serve(listener)
}
