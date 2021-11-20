package main

import (
	"context"
	"flag"
	"log"
	"net"

	pb "github.com/CasperAntonPoulsen/disysminiproject3/proto"
	"google.golang.org/grpc"
)

type Request struct {
	user   *pb.User
	stream pb.Auction_RequestTokenServer
}

type Auction struct {
	amount   int32
	bidderID int32
}

type Server struct {
	pb.UnimplementedAuctionServer
	RequestQueue chan Request
	Release      chan *pb.Release
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

	return &pb.Acknowledgement{Status: "success"}, nil
}

func GrantToken(rqst Request) error {
	log.Printf("Granting token to: %v", rqst.user.Userid)
	err := rqst.stream.Send(&pb.Grant{User: rqst.user})
	return err
}

func main() {
	flag.Parse()
	grpcServer := grpc.NewServer()
	listener, err := net.Listen("tcp", ":8080")

	if err != nil {
		log.Fatalf("Error, couldn't create the server %v", err)
	}

	requestqueue := make(chan Request)
	releasequeue := make(chan *pb.Release)
	server := Server{
		RequestQueue: requestqueue,
		Release:      releasequeue,
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
	grpcServer.Serve(listener)
}
