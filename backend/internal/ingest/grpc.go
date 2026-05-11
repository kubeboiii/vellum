package ingest

import (
	"errors"
	"fmt"
	"io"
	"log"
	"time"

	"github.com/google/uuid"

	"github.com/kubeboiii/vellum/internal/model"
	vellumv1 "github.com/kubeboiii/vellum/proto/vellum/v1"
)

type SignalServiceServer struct {
	vellumv1.UnimplementedSignalServiceServer

	pipe Submitter
	now  func() time.Time
}

func NewSignalServiceServer(pipe Submitter) *SignalServiceServer {
	return &SignalServiceServer{pipe: pipe, now: time.Now}
}

func (s *SignalServiceServer) IngestSignals(stream vellumv1.SignalService_IngestSignalsServer) error {
	for {
		in, err := stream.Recv()
		if errors.Is(err, io.EOF) {

			return nil
		}
		if err != nil {

			return err
		}

		ack := s.handleOne(in)
		if err := stream.Send(ack); err != nil {

			log.Printf("ingest grpc: send ack failed: %v", err)
			return err
		}
	}
}

func (s *SignalServiceServer) handleOne(in *vellumv1.Signal) *vellumv1.Ack {
	sig := model.Signal{
		ComponentID:   in.GetComponentId(),
		ComponentType: model.ComponentType(in.GetComponentType()),
		Severity:      model.Severity(in.GetSeverity()),
		Source:        in.GetSource(),
		Payload:       in.GetPayload(),
	}
	if t := in.GetTimestamp(); t != nil {
		sig.Timestamp = t.AsTime()
	}
	if raw := in.GetSignalId(); raw != "" {

		if id, err := uuid.Parse(raw); err == nil {
			sig.SignalID = id
		}
	}

	if err := sig.Validate(); err != nil {
		return &vellumv1.Ack{
			SignalId: in.GetSignalId(),
			Status:   vellumv1.Ack_ACK_STATUS_REJECTED_INVALID,
			Error:    err.Error(),
		}
	}
	sig.ApplyDefaults(s.now())

	if !s.pipe.Submit(sig) {
		return &vellumv1.Ack{
			SignalId: sig.SignalID.String(),
			Status:   vellumv1.Ack_ACK_STATUS_REJECTED_QUEUE_FULL,
			Error:    fmt.Sprintf("queue full; retry after %dms", retryAfterMillis),
		}
	}
	return &vellumv1.Ack{
		SignalId: sig.SignalID.String(),
		Status:   vellumv1.Ack_ACK_STATUS_ACCEPTED,
	}
}
