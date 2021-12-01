package main

import (
	"context"
	"flag"
	"log"
	"net"
	"os"
	"strconv"
	"time"

	pb "github.com/CasperAntonPoulsen/disysminiproject3/proto"
	"google.golang.org/grpc"
)

var port = flag.String("port", "8080", "The docker port of the server")

type Request struct {
	user   *pb.User
	stream pb.Auction_RequestTokenServer
}

type Auction struct {
	amount   int32
	bidderID int32
}

type Replica struct {
	user       *pb.User
	port       string
	connection pb.AuctionClient
}

type Server struct {
	pb.UnimplementedAuctionServer
	RequestQueue chan Request
	Release      chan *pb.Release
	Leader       Replica
	Replicas     []Replica
	id           int32
	error        chan error
	lamport      int32
	auction      Auction
	timeout      int32
	frontend     pb.AuctionClient
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

func (s *Server) RequestResult(ctx context.Context, empty *pb.Empty) (*pb.Result, error) {

	log.Print("Reading the result")
	return &pb.Result{Amount: s.auction.amount}, nil
}

func (s *Server) MakeBid(ctx context.Context, bid *pb.Bid) (*pb.Acknowledgement, error) {

	if bid.Amount <= s.auction.amount {
		return &pb.Acknowledgement{Status: "fail"}, nil
	}
	log.Print("Proccessing bid")
	if s.id == s.Leader.user.Userid {
		s.broadcastBid(bid)
	}

	s.auction.amount = bid.Amount
	s.auction.bidderID = bid.Userid

	s.lamport++
	log.Print("Bid complete")
	return &pb.Acknowledgement{Status: "success"}, nil
}

//write to other replicationmanagers (Servers)
func (s *Server) broadcastBid(bid *pb.Bid) {
	for _, rm := range s.Replicas {

		_, err := rm.connection.MakeBid(context.Background(), bid)
		if err != nil {
			log.Printf("could not connect to rm %v: %v", rm.user.Userid+1, err)
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

func (s *Server) RequestLamport(ctx context.Context, empty *pb.Empty) (*pb.User, error) {
	return &pb.User{Userid: s.id, Time: s.lamport}, nil
}

//Election Method-----------------------------------------------------------------------------
func (s *Server) CompareLamports() {
	for _, rm := range s.Replicas {
		usr, err := rm.connection.RequestLamport(context.Background(), &pb.Empty{})

		if err != nil {
			log.Printf("Replica %v did not respond within the given time %v", rm.user.Userid, err)
		} else {
			if usr.Time > s.lamport {
				log.Printf("Replica %v has newer version of the auction", rm.user.Userid)
				return
			} else if usr.Userid > s.id {
				log.Printf("Replica %v has higher id", rm.user.Userid)
				return
			}
		}

	}

	leaderState := &pb.State{User: &pb.User{Userid: s.id, Time: s.lamport}, Amount: s.auction.amount, Bidid: s.auction.bidderID}
	for _, rm := range s.Replicas {
		_, err := rm.connection.Coordinator(context.Background(), leaderState)
		if err != nil {
			log.Printf("Replica %v did not respond within the given time %v", rm.user.Userid, err)
		}
	}
	s.Leader = Replica{user: leaderState.User}
	log.Printf("Electing myself as leader")
	// tell frontend you're the new leader
	_, err := s.frontend.Coordinator(context.Background(), leaderState)
	if err != nil {
		log.Printf("Could not contanct frontend: %v", err)
	}
}

func (s *Server) Coordinator(ctx context.Context, leaderState *pb.State) (*pb.Empty, error) {
	s.Leader.user = leaderState.User
	for _, rm := range s.Replicas {
		if rm.user.Userid == s.Leader.user.Userid {
			s.Leader.port = rm.port
			s.Leader.connection = rm.connection
		}
	}
	s.auction = Auction{bidderID: leaderState.Bidid, amount: leaderState.Amount}
	log.Printf("Recieved new leader: %v", s.Leader.user.Userid)
	return &pb.Empty{}, nil
}

//main----------------------------------------------------------------------------------------
func main() {
	flag.Parse()

	//setup server
	grpcServer := grpc.NewServer()
	log.Print("Starting listener")
	listener, err := net.Listen("tcp", ":"+*port)

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

		conn, err := grpc.Dial("auctionserver"+strconv.Itoa(i+1)+port, grpc.WithInsecure())
		if err != nil {
			log.Printf("could not connect to rm %v: %v", i+1, err)
		}
		log.Printf("Connected to Replica: %v", i+1)
		rmClient := pb.NewAuctionClient(conn)

		Replicas = append(Replicas, Replica{&pb.User{Userid: int32(i + 1)}, port, rmClient})
	}

	//Connect to the frontend

	frontend, err := grpc.Dial("frontend:8001", grpc.WithInsecure())
	if err != nil {
		log.Printf("could not connect: %v", err)
	}

	// construct server struct
	server := Server{
		RequestQueue: requestqueue,
		Release:      releasequeue,
		Leader:       Replica{user: &pb.User{Userid: leaderid}, port: os.Getenv("DEFAULTLEADERPORT")},
		id:           id,
		Replicas:     Replicas,
		timeout:      GetIntEnv("GLOBALTIMEOUT"),
		frontend:     pb.NewAuctionClient(frontend),
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
				if rm.user.Userid == server.Leader.user.Userid { //ping leader
					time.Sleep(5 * time.Second)
					log.Printf("Pinging leader: %v", server.Leader.user.Userid)
					_, err := rm.connection.Ping(context.Background(), &pb.Empty{})

					//If we get a resonse error, we assume that the server has crash failure ¤
					if err != nil {
						// start election ¤
						log.Printf("Leader did not respond, starting comparison")
						server.CompareLamports()
					}
				}
			}
		}
	}()

	grpcServer.Serve(listener)
}
