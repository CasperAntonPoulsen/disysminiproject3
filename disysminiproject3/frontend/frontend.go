package main

import (
	"context"
	"fmt"
	"log"
	"net/http"
	"strconv"

	pb "github.com/CasperAntonPoulsen/disysminiproject3/proto"
	"github.com/gin-gonic/gin"
)

var (
	client pb.AuctionClient
)

func MakeBid(c *gin.Context) {

	userid, err := strconv.Atoi(c.PostForm("userid"))
	if err != nil {
		log.Fatalf("Could not convert userid to int, %v", err)
	}

	amount, err := strconv.Atoi(c.PostForm("amount"))
	if err != nil {
		log.Fatalf("Could not convert amount to int, %v", err)
	}

	rqst := &pb.Request{User: &pb.User{Userid: int32(userid)}}
	bid := &pb.Bid{Userid: int32(userid), Amount: int32(amount)}

	stream, err := client.RequestToken(context.Background(), rqst)
	if err != nil {
		log.Fatalf("connection has failed: %v", err)
	}

	//recieve grant token

	for {
		_, err := stream.Recv()
		if err != nil {
			log.Printf("error recieving grant token: %v", err)
			break
		} else {
			log.Print("Grant token recieved, accessing critical section")
			// make the bid

			ack, err := client.MakeBid(context.Background(), bid)
			if err != nil {
				// Throw and internal server error message and send the exception in the ack
				c.JSON(500, gin.H{
					"ack": fmt.Sprintln(err),
				})
			}
			c.JSON(200, gin.H{
				"ack": ack.Status,
			})

			log.Print("Finished, sending release token")
			// then release
			client.ReleaseToken(context.Background(), &pb.Release{User: rqst.User})
			break
		}
	}

}

func RequestResult(c *gin.Context) {

	userid, err := strconv.Atoi(c.Query("userid"))
	if err != nil {
		log.Fatalf("Could not convert userid to int, %v", err)
	}

	rqst := &pb.Request{User: &pb.User{Userid: int32(userid)}}

	stream, err := client.RequestToken(context.Background(), rqst)
	if err != nil {
		log.Fatalf("connection has failed:, %v", err)
	}

	//recieve grant token

	for {
		_, err := stream.Recv()
		if err != nil {
			log.Printf("Error recieving message: %v", err)
			break
		} else {
			log.Print("Grant token recieved, accessing critical section")
			// access critical section

			result, err := client.RequestResult(context.Background(), rqst.User)
			if err != nil {
				log.Fatalf("Could not get result: %v", err)
			}

			c.String(http.StatusOK, strconv.Itoa(int(result.Amount)))

			log.Print("Finished, sending release token")
			// then release
			client.ReleaseToken(context.Background(), &pb.Release{User: rqst.User})
			break
		}
	}

}

func main() {
	r := gin.Default()

	r.GET("/result", RequestResult)
	r.POST("/bid", MakeBid)

	r.Run(":8080")
}
