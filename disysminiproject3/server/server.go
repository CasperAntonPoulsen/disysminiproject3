package main

import (
	"flag"
	"log"
	"net"

	pb "github.com/CasperAntonPoulsen/disysminiproject3/proto"
	"google.golang.org/grpc"
)

type Server struct {
	pb.UnimplementedAuctionServer
	auction Auction
}

type Auction struct {
	amount   int32
	bidderID int32
}

func main() {
	flag.Parse()
	grpcServer := grpc.NewServer()
	listener, err := net.Listen("tcp", "localhost:8080")

	if err != nil {
		log.Fatalf("Error, couldn't create the server %v", err)
	}

	server := Server{auction: Auction{amount: 50, bidderID: 0}}

	pb.RegisterAuctionServer(grpcServer, &server)
	grpcServer.Serve(listener)
}
