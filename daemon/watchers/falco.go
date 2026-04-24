package watchers

import (
	"encoding/json"
	"fmt"
	"net/http"
	"time"

	"github.com/Shreehari-Acharya/Bannin/daemon/pkg/models"
)

type FalcoPayload struct {
	Time         string         `json:"time"`
	Rule         string         `json:"rule"`
	Priority     string         `json:"priority"`
	Output       string         `json:"output"`
	OutputFields map[string]any `json:"output_fields"`
}

func StartFalcoHTTP(port string, eventQueue chan<- models.SecEvent) error {
	return http.ListenAndServe(":"+port, NewFalcoHandler(eventQueue))
}

func NewFalcoHandler(eventQueue chan<- models.SecEvent) http.Handler {
	mux := http.NewServeMux()
	mux.HandleFunc("/falco", func(w http.ResponseWriter, r *http.Request) {
		if r.Method != http.MethodPost {
			http.Error(w, "method not allowed", http.StatusMethodNotAllowed)
			return
		}

		defer r.Body.Close()

		var payload FalcoPayload
		if err := json.NewDecoder(r.Body).Decode(&payload); err != nil {
			http.Error(w, "invalid payload", http.StatusBadRequest)
			return
		}

		parsedTime, err := time.Parse(time.RFC3339, payload.Time)
		if err != nil {
			http.Error(w, "invalid timestamp", http.StatusBadRequest)
			return
		}

		rawBytes, err := json.Marshal(payload)
		if err != nil {
			http.Error(w, "failed to serialize payload", http.StatusInternalServerError)
			return
		}

		alert := models.SecEvent{
			SourceTool:  models.FALCO,
			Timestamp:   parsedTime,
			Priority:    payload.Priority,
			Description: payload.Output,
			RawPayload:  json.RawMessage(rawBytes),
		}

		select {
		case eventQueue <- alert:
			w.WriteHeader(http.StatusOK)
			_, _ = w.Write([]byte("ok"))
		default:
			http.Error(w, "event queue is full", http.StatusServiceUnavailable)
		}
	})

	return mux
}

func DescribeFalcoListener(port string) string {
	return fmt.Sprintf("Falco event listener ready on :%s/falco", port)
}
