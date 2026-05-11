// scripts/grpc-client.go — tiny client that streams N signals via
// gRPC and prints the acks. Used by the Phase 5 acceptance demo to
// prove the gRPC path is wired end-to-end.
//
// Usage:
//   cd backend && go run ../scripts/grpc-client.go --target localhost:9090 --n 10 --component DEMO_GRPC

//go:build phase5demo

package main

import (
	"context"
	"flag"
	"fmt"
	"io"
	"log"
	"time"

	"google.golang.org/grpc"
	"google.golang.org/grpc/credentials/insecure"
	"google.golang.org/protobuf/types/known/timestamppb"

	imsv1 "github.com/kubeboiii/ims/proto/ims/v1"
)

func main() {
	target := flag.String("target", "localhost:9090", "gRPC server address")
	component := flag.String("component", "GRPC_DEMO", "component_id to send")
	n := flag.Int("n", 10, "how many signals to stream")
	flag.Parse()

	conn, err := grpc.NewClient(*target, grpc.WithTransportCredentials(insecure.NewCredentials()))
	if err != nil {
		log.Fatalf("dial: %v", err)
	}
	defer conn.Close()

	client := imsv1.NewSignalServiceClient(conn)
	ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
	defer cancel()

	stream, err := client.IngestSignals(ctx)
	if err != nil {
		log.Fatalf("open stream: %v", err)
	}

	// Async ack reader.
	acks := make(chan *imsv1.Ack, *n)
	go func() {
		defer close(acks)
		for {
			a, err := stream.Recv()
			if err == io.EOF {
				return
			}
			if err != nil {
				log.Printf("recv: %v", err)
				return
			}
			acks <- a
		}
	}()

	for i := 0; i < *n; i++ {
		err := stream.Send(&imsv1.Signal{
			ComponentId:   *component,
			ComponentType: "CACHE",
			Severity:      "P0",
			Source:        "grpc-demo",
			Timestamp:     timestamppb.New(time.Now()),
			Payload:       []byte(fmt.Sprintf(`{"i":%d}`, i)),
		})
		if err != nil {
			log.Fatalf("send %d: %v", i, err)
		}
	}
	_ = stream.CloseSend()

	var accepted, rejected int
	for a := range acks {
		switch a.GetStatus() {
		case imsv1.Ack_ACK_STATUS_ACCEPTED:
			accepted++
		default:
			rejected++
			log.Printf("rejected: %s — %s", a.GetStatus(), a.GetError())
		}
	}
	fmt.Printf("accepted=%d rejected=%d\n", accepted, rejected)
}
