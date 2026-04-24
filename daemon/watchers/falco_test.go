package watchers

import (
	"bytes"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	"github.com/Shreehari-Acharya/Bannin/daemon/pkg/models"
)

func TestNewFalcoHandlerAcceptsValidEvent(t *testing.T) {
	queue := make(chan models.SecEvent, 1)
	handler := NewFalcoHandler(queue)

	payload := FalcoPayload{
		Time:     "2026-04-24T10:11:12Z",
		Priority: "Warning",
		Output:   "shell execution detected",
	}
	body, _ := json.Marshal(payload)

	req := httptest.NewRequest(http.MethodPost, "/falco", bytes.NewReader(body))
	rec := httptest.NewRecorder()
	handler.ServeHTTP(rec, req)

	if rec.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d", rec.Code)
	}

	select {
	case event := <-queue:
		if event.SourceTool != models.FALCO {
			t.Fatalf("unexpected source tool: %s", event.SourceTool)
		}
		if event.Priority != "Warning" || event.Description != "shell execution detected" {
			t.Fatalf("unexpected event: %#v", event)
		}
		if event.Timestamp.Equal(time.Time{}) {
			t.Fatal("expected parsed timestamp")
		}
	default:
		t.Fatal("expected event in queue")
	}
}

func TestNewFalcoHandlerRejectsInvalidJSON(t *testing.T) {
	queue := make(chan models.SecEvent, 1)
	handler := NewFalcoHandler(queue)

	req := httptest.NewRequest(http.MethodPost, "/falco", bytes.NewBufferString("{"))
	rec := httptest.NewRecorder()
	handler.ServeHTTP(rec, req)

	if rec.Code != http.StatusBadRequest {
		t.Fatalf("expected 400, got %d", rec.Code)
	}
}

func TestNewFalcoHandlerRejectsInvalidTimestamp(t *testing.T) {
	queue := make(chan models.SecEvent, 1)
	handler := NewFalcoHandler(queue)

	body := bytes.NewBufferString(`{"time":"nope","priority":"High","output":"bad time"}`)
	req := httptest.NewRequest(http.MethodPost, "/falco", body)
	rec := httptest.NewRecorder()
	handler.ServeHTTP(rec, req)

	if rec.Code != http.StatusBadRequest {
		t.Fatalf("expected 400, got %d", rec.Code)
	}
}

func TestNewFalcoHandlerRejectsWhenQueueIsFull(t *testing.T) {
	queue := make(chan models.SecEvent, 1)
	queue <- models.SecEvent{}
	handler := NewFalcoHandler(queue)

	body := bytes.NewBufferString(`{"time":"2026-04-24T10:11:12Z","priority":"High","output":"full queue"}`)
	req := httptest.NewRequest(http.MethodPost, "/falco", body)
	rec := httptest.NewRecorder()
	handler.ServeHTTP(rec, req)

	if rec.Code != http.StatusServiceUnavailable {
		t.Fatalf("expected 503, got %d", rec.Code)
	}
}
