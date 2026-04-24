package models

import (
	"encoding/json"
	"time"
)

type SecTool = string

const (
	AUDITD SecTool = "auditd"
)

type SecEvent struct {
	SourceTool  SecTool         `json:"sourceTool"`
	Timestamp   time.Time       `json:"timestamp"`
	Priority    string          `json:"severity"`
	Description string          `json:"description"`
	RawPayload  json.RawMessage `json:"rawPayload"`
}
