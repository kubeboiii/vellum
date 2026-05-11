package ingest

import (
	"bytes"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	"github.com/gin-gonic/gin"

	"github.com/kubeboiii/vellum/internal/model"
)

type fakeSubmitter struct {
	accept bool
	calls  []model.Signal
}

func (f *fakeSubmitter) Submit(s model.Signal) bool {
	f.calls = append(f.calls, s)
	return f.accept
}

func newRouter(s Submitter) *gin.Engine {
	gin.SetMode(gin.TestMode)
	r := gin.New()
	frozen := time.Date(2026, 5, 12, 3, 14, 15, 0, time.UTC)
	r.POST("/v1/signals", NewHandler(s, func() time.Time { return frozen }))
	return r
}

func validBody() []byte {
	return []byte(`{
		"component_id":"RDBMS_PRIMARY_01",
		"component_type":"RDBMS",
		"severity":"P0",
		"source":"datadog",
		"payload":{"msg":"hi"}
	}`)
}

func TestHandler_202OnAccept(t *testing.T) {
	fake := &fakeSubmitter{accept: true}
	r := newRouter(fake)
	w := httptest.NewRecorder()
	req := httptest.NewRequest(http.MethodPost, "/v1/signals", bytes.NewReader(validBody()))
	req.Header.Set("Content-Type", "application/json")
	r.ServeHTTP(w, req)

	if w.Code != http.StatusAccepted {
		t.Fatalf("want 202, got %d: %s", w.Code, w.Body.String())
	}
	var body struct {
		SignalID string `json:"signal_id"`
	}
	_ = json.Unmarshal(w.Body.Bytes(), &body)
	if body.SignalID == "" {
		t.Fatal("expected server-generated signal_id in response")
	}
	if len(fake.calls) != 1 {
		t.Fatalf("want 1 submit, got %d", len(fake.calls))
	}
}

func TestHandler_503OnFull(t *testing.T) {
	fake := &fakeSubmitter{accept: false}
	r := newRouter(fake)
	w := httptest.NewRecorder()
	req := httptest.NewRequest(http.MethodPost, "/v1/signals", bytes.NewReader(validBody()))
	req.Header.Set("Content-Type", "application/json")
	r.ServeHTTP(w, req)

	if w.Code != http.StatusServiceUnavailable {
		t.Fatalf("want 503, got %d: %s", w.Code, w.Body.String())
	}
	if w.Header().Get("Retry-After") == "" {
		t.Fatal("expected Retry-After header on 503")
	}
}

func TestHandler_400OnBadJSON(t *testing.T) {
	r := newRouter(&fakeSubmitter{accept: true})
	w := httptest.NewRecorder()
	req := httptest.NewRequest(http.MethodPost, "/v1/signals", bytes.NewReader([]byte("{not json")))
	req.Header.Set("Content-Type", "application/json")
	r.ServeHTTP(w, req)

	if w.Code != http.StatusBadRequest {
		t.Fatalf("want 400, got %d", w.Code)
	}
}

func TestHandler_400OnValidation(t *testing.T) {
	r := newRouter(&fakeSubmitter{accept: true})
	w := httptest.NewRecorder()
	req := httptest.NewRequest(http.MethodPost, "/v1/signals", bytes.NewReader([]byte(`{
		"component_id":"X","component_type":"BAD","severity":"P0","source":"s","payload":{}
	}`)))
	req.Header.Set("Content-Type", "application/json")
	r.ServeHTTP(w, req)

	if w.Code != http.StatusBadRequest {
		t.Fatalf("want 400, got %d: %s", w.Code, w.Body.String())
	}
}
