package dispatcher

import (
	"encoding/json"
	"io"
	"net/http"
	"strings"
	"testing"
	"time"

	"github.com/Shreehari-Acharya/Bannin/daemon/pkg/models"
)

type roundTripFunc func(*http.Request) (*http.Response, error)

func (f roundTripFunc) RoundTrip(req *http.Request) (*http.Response, error) {
	return f(req)
}

func newTestClient(t *testing.T, fn roundTripFunc) *Client {
	t.Helper()

	client, err := NewClient("https://backend.example")
	if err != nil {
		t.Fatalf("NewClient: %v", err)
	}

	client.httpClient = &http.Client{Transport: fn}
	client.now = func() time.Time { return time.Date(2026, 4, 24, 1, 2, 3, 0, time.UTC) }
	return client
}

func response(status int, body string) *http.Response {
	return &http.Response{
		StatusCode: status,
		Header:     make(http.Header),
		Body:       io.NopCloser(strings.NewReader(body)),
	}
}

func TestSendAlertsSuccess(t *testing.T) {
	var got BackendEventPayload
	client := newTestClient(t, func(req *http.Request) (*http.Response, error) {
		if req.URL.String() != "https://backend.example/events/new" {
			t.Fatalf("unexpected URL: %s", req.URL.String())
		}
		if err := json.NewDecoder(req.Body).Decode(&got); err != nil {
			t.Fatalf("decode request: %v", err)
		}
		return response(http.StatusAccepted, ""), nil
	})

	alert := models.SecEvent{
		SourceTool:  models.FALCO,
		Priority:    "Critical",
		Description: "shell spawned",
		RawPayload:  json.RawMessage(`{"rule":"terminal"}`),
	}

	if err := client.SendAlerts(alert); err != nil {
		t.Fatalf("SendAlerts: %v", err)
	}

	if got.SourceTool != models.FALCO || got.Priority != "Critical" || got.Description != "shell spawned" {
		t.Fatalf("unexpected payload: %#v", got)
	}
	if got.Timestamp.IsZero() {
		t.Fatal("expected timestamp to be set")
	}
}

func TestSendRuleReturnsBackendError(t *testing.T) {
	client := newTestClient(t, func(req *http.Request) (*http.Response, error) {
		if req.URL.Query().Get("toolname") != "falco" {
			t.Fatalf("unexpected query: %s", req.URL.RawQuery)
		}
		return response(http.StatusBadRequest, "bad rule"), nil
	})

	err := client.SendRule("falco", "rule text")
	if err == nil || !strings.Contains(err.Error(), "status 400") {
		t.Fatalf("expected backend error, got %v", err)
	}
}

func TestGenerateSummarySuccess(t *testing.T) {
	client := newTestClient(t, func(req *http.Request) (*http.Response, error) {
		body, _ := io.ReadAll(req.Body)
		if !strings.Contains(string(body), "/tmp/project") {
			t.Fatalf("unexpected request body: %s", string(body))
		}
		return response(http.StatusOK, "summary output\n"), nil
	})

	summary, err := client.GenerateSummary("/tmp/project")
	if err != nil {
		t.Fatalf("GenerateSummary: %v", err)
	}
	if summary != "summary output" {
		t.Fatalf("unexpected summary: %q", summary)
	}
}

func TestBackendEndpointURLRejectsEmptyBaseURL(t *testing.T) {
	_, err := backendEndpointURL("", "/events/new")
	if err == nil || !strings.Contains(err.Error(), "not configured") {
		t.Fatalf("expected configuration error, got %v", err)
	}
}
