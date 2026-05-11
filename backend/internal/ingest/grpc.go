package ingest

import (
	"errors"
	"fmt"
	"io"
	"log"
	"time"

	"github.com/google/uuid"

	"github.com/kubeboiii/ims/internal/model"
	imsv1 "github.com/kubeboiii/ims/proto/ims/v1"
)

// SignalServiceServer is the gRPC implementation of
// imsv1.SignalServiceServer. It shares the same pipeline.Submitter
// as the HTTP handler — FR-1.3 mandates "exactly one downstream path
// regardless of protocol." All the work (debounce, fan-out, alerts)
// happens in the existing processor; this file is pure protocol
// adaptation.
//
// We deliberately do NOT register a rate limiter here for v1:
//   - Ingest's per-source HTTP limiter keys on ClientIP. A gRPC peer
//     is typically a long-lived process emitting many signals on one
//     connection — different semantics from HTTP-per-request limits.
//   - gRPC's natural backpressure (the server controls when it reads
//     from the stream) plus the pipeline's queue-full → REJECTED ack
//     already serves as backpressure.
//   - A proper gRPC interceptor-based limiter is Phase 6+ territory.
type SignalServiceServer struct {
	imsv1.UnimplementedSignalServiceServer // require_unimplemented_servers=false in buf.gen.yaml means this is a no-op, but keep the embed for forward-compat

	pipe Submitter
	now  func() time.Time
}

// NewSignalServiceServer wires the gRPC server to the shared pipeline.
// `now` is injectable for tests; production uses time.Now.
func NewSignalServiceServer(pipe Submitter) *SignalServiceServer {
	return &SignalServiceServer{pipe: pipe, now: time.Now}
}

// IngestSignals is the bidi-stream RPC. Loop: read one Signal, attempt
// to enqueue, send an Ack with the resulting status, repeat. Closes
// when the client half-closes (io.EOF on Recv) or the stream errors.
//
// Order guarantee: we ack each signal BEFORE reading the next one, so
// clients see Ack[i] before Signal[i+1] is processed. That matches
// HTTP's request/response semantics and makes debugging trivial.
func (s *SignalServiceServer) IngestSignals(stream imsv1.SignalService_IngestSignalsServer) error {
	for {
		in, err := stream.Recv()
		if errors.Is(err, io.EOF) {
			// Normal end-of-stream from the client.
			return nil
		}
		if err != nil {
			// Stream broken (client disconnect, ctx cancelled).
			return err
		}

		ack := s.handleOne(in)
		if err := stream.Send(ack); err != nil {
			// Failure to ack means we can't tell the client what
			// happened; log and bail. The signal we already enqueued
			// is fine — the pipeline doesn't know about acks.
			log.Printf("ingest grpc: send ack failed: %v", err)
			return err
		}
	}
}

// handleOne converts one inbound proto Signal into model.Signal,
// validates it, and submits to the pipeline. Returns an Ack
// describing the outcome.
func (s *SignalServiceServer) handleOne(in *imsv1.Signal) *imsv1.Ack {
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
		// Client supplied an ID; honour it. If it doesn't parse as a
		// UUID we'd ideally reject — but to keep the gRPC contract
		// resilient against minor format differences, we let an
		// invalid one fall through to uuid.Nil and the server
		// generates a fresh one in ApplyDefaults.
		if id, err := uuid.Parse(raw); err == nil {
			sig.SignalID = id
		}
	}

	if err := sig.Validate(); err != nil {
		return &imsv1.Ack{
			SignalId: in.GetSignalId(),
			Status:   imsv1.Ack_ACK_STATUS_REJECTED_INVALID,
			Error:    err.Error(),
		}
	}
	sig.ApplyDefaults(s.now())

	if !s.pipe.Submit(sig) {
		return &imsv1.Ack{
			SignalId: sig.SignalID.String(),
			Status:   imsv1.Ack_ACK_STATUS_REJECTED_QUEUE_FULL,
			Error:    fmt.Sprintf("queue full; retry after %dms", retryAfterMillis),
		}
	}
	return &imsv1.Ack{
		SignalId: sig.SignalID.String(),
		Status:   imsv1.Ack_ACK_STATUS_ACCEPTED,
	}
}
