package ingest

import (
	"context"
	"encoding/json"
	"io"
	"net"
	"testing"
	"time"

	"github.com/google/uuid"
	"google.golang.org/grpc"
	"google.golang.org/grpc/credentials/insecure"
	"google.golang.org/grpc/test/bufconn"
	"google.golang.org/protobuf/types/known/timestamppb"

	"github.com/kubeboiii/vellum/internal/model"
	vellumv1 "github.com/kubeboiii/vellum/proto/vellum/v1"
)

func startGRPC(t *testing.T, pipe Submitter) vellumv1.SignalServiceClient {
	t.Helper()
	const bufSize = 1024 * 1024
	lis := bufconn.Listen(bufSize)

	srv := grpc.NewServer()
	vellumv1.RegisterSignalServiceServer(srv, NewSignalServiceServer(pipe))
	go func() { _ = srv.Serve(lis) }()
	t.Cleanup(srv.Stop)

	dialer := func(context.Context, string) (net.Conn, error) {
		return lis.Dial()
	}
	conn, err := grpc.NewClient("passthrough://bufnet",
		grpc.WithContextDialer(dialer),
		grpc.WithTransportCredentials(insecure.NewCredentials()),
	)
	if err != nil {
		t.Fatalf("grpc dial: %v", err)
	}
	t.Cleanup(func() { _ = conn.Close() })
	return vellumv1.NewSignalServiceClient(conn)
}

type recordingSubmitter struct {
	accept bool
	calls  []model.Signal
}

func (r *recordingSubmitter) Submit(s model.Signal) bool {
	r.calls = append(r.calls, s)
	return r.accept
}

func TestGRPC_IngestStream_Accepts(t *testing.T) {
	pipe := &recordingSubmitter{accept: true}
	client := startGRPC(t, pipe)

	ctx, cancel := context.WithTimeout(context.Background(), 2*time.Second)
	defer cancel()

	stream, err := client.IngestSignals(ctx)
	if err != nil {
		t.Fatalf("open stream: %v", err)
	}

	for i := 0; i < 3; i++ {
		err := stream.Send(&vellumv1.Signal{
			ComponentId:   "GRPC_TEST",
			ComponentType: string(model.ComponentCache),
			Severity:      string(model.SeverityP1),
			Source:        "grpc-test",
			Payload:       []byte(`{"i":` + string(rune('0'+i)) + `}`),
			Timestamp:     timestamppb.New(time.Now()),
		})
		if err != nil {
			t.Fatalf("send %d: %v", i, err)
		}
	}
	if err := stream.CloseSend(); err != nil {
		t.Fatalf("close send: %v", err)
	}

	var acks []*vellumv1.Ack
	for {
		ack, err := stream.Recv()
		if err == io.EOF {
			break
		}
		if err != nil {
			t.Fatalf("recv: %v", err)
		}
		acks = append(acks, ack)
	}
	if len(acks) != 3 {
		t.Fatalf("want 3 acks, got %d", len(acks))
	}
	for i, a := range acks {
		if a.GetStatus() != vellumv1.Ack_ACK_STATUS_ACCEPTED {
			t.Errorf("ack[%d]: want ACCEPTED, got %s (err=%q)", i, a.GetStatus(), a.GetError())
		}
		if a.GetSignalId() == "" {
			t.Errorf("ack[%d]: missing signal_id", i)
		}
	}
	if len(pipe.calls) != 3 {
		t.Errorf("pipeline received %d signals, want 3", len(pipe.calls))
	}
}

func TestGRPC_QueueFull_ReturnsRejectedAck(t *testing.T) {
	pipe := &recordingSubmitter{accept: false}
	client := startGRPC(t, pipe)

	ctx, cancel := context.WithTimeout(context.Background(), 2*time.Second)
	defer cancel()
	stream, _ := client.IngestSignals(ctx)
	_ = stream.Send(&vellumv1.Signal{
		ComponentId:   "FULL_TEST",
		ComponentType: string(model.ComponentAPI),
		Severity:      string(model.SeverityP0),
		Source:        "test",
	})
	_ = stream.CloseSend()

	ack, err := stream.Recv()
	if err != nil {
		t.Fatalf("recv: %v", err)
	}
	if ack.GetStatus() != vellumv1.Ack_ACK_STATUS_REJECTED_QUEUE_FULL {
		t.Errorf("status: want REJECTED_QUEUE_FULL, got %s", ack.GetStatus())
	}
	if ack.GetError() == "" {
		t.Error("error string should explain the rejection")
	}
}

func TestGRPC_InvalidSignal_ReturnsRejected(t *testing.T) {
	pipe := &recordingSubmitter{accept: true}
	client := startGRPC(t, pipe)

	ctx, cancel := context.WithTimeout(context.Background(), 2*time.Second)
	defer cancel()
	stream, _ := client.IngestSignals(ctx)
	_ = stream.Send(&vellumv1.Signal{

		ComponentType: string(model.ComponentAPI),
		Severity:      string(model.SeverityP0),
		Source:        "test",
	})
	_ = stream.CloseSend()

	ack, err := stream.Recv()
	if err != nil {
		t.Fatalf("recv: %v", err)
	}
	if ack.GetStatus() != vellumv1.Ack_ACK_STATUS_REJECTED_INVALID {
		t.Errorf("status: want REJECTED_INVALID, got %s (err=%q)", ack.GetStatus(), ack.GetError())
	}
	if len(pipe.calls) != 0 {
		t.Errorf("invalid signal must not reach pipeline, got %d", len(pipe.calls))
	}
}

func TestGRPC_HonoursProvidedSignalID(t *testing.T) {
	pipe := &recordingSubmitter{accept: true}
	client := startGRPC(t, pipe)
	ctx, cancel := context.WithTimeout(context.Background(), 2*time.Second)
	defer cancel()

	wanted := uuid.New()
	stream, _ := client.IngestSignals(ctx)
	_ = stream.Send(&vellumv1.Signal{
		SignalId:      wanted.String(),
		ComponentId:   "X",
		ComponentType: string(model.ComponentAPI),
		Severity:      string(model.SeverityP3),
		Source:        "test",
		Payload:       json.RawMessage(`{}`),
	})
	_ = stream.CloseSend()

	ack, _ := stream.Recv()
	if ack.GetSignalId() != wanted.String() {
		t.Errorf("client-supplied id not preserved: want %s, got %s", wanted, ack.GetSignalId())
	}
}
