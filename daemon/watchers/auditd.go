package watchers

import (
	"bufio"
	"encoding/json"
	"fmt"
	"os"
	"regexp"
	"strconv"
	"strings"
	"time"

	"github.com/Shreehari-Acharya/Bannin/daemon/pkg/models"
)

var (
	auditMsgPattern  = regexp.MustCompile(`msg=audit\(([0-9]+(?:\.[0-9]+)?):([0-9]+)\):`)
	auditTypePattern = regexp.MustCompile(`type=([A-Z0-9_]+)`)
	auditKVPattern   = regexp.MustCompile(`([A-Za-z0-9_]+)=("([^"\\]|\\.)*"|[^\s]+)`)
)

func StartAuditLogTailer(logPath string, pollInterval time.Duration, eventQueue chan<- models.SecEvent) error {
	info, err := os.Stat(logPath)
	if err != nil {
		return fmt.Errorf("failed to stat audit log %s: %w", logPath, err)
	}

	offset := info.Size()
	ticker := time.NewTicker(pollInterval)
	defer ticker.Stop()

	for range ticker.C {
		nextOffset, err := shipAuditLogChunk(logPath, offset, eventQueue)
		if err != nil {
			return err
		}
		offset = nextOffset
	}

	return nil
}

func DescribeAuditLogTailer(logPath string) string {
	return fmt.Sprintf("auditd log tailer watching %s", logPath)
}

func shipAuditLogChunk(logPath string, offset int64, eventQueue chan<- models.SecEvent) (int64, error) {
	info, err := os.Stat(logPath)
	if err != nil {
		return offset, fmt.Errorf("failed to stat audit log %s: %w", logPath, err)
	}

	if info.Size() < offset {
		offset = 0
	}

	file, err := os.Open(logPath)
	if err != nil {
		return offset, fmt.Errorf("failed to open audit log %s: %w", logPath, err)
	}
	defer file.Close()

	if _, err := file.Seek(offset, 0); err != nil {
		return offset, fmt.Errorf("failed to seek audit log %s: %w", logPath, err)
	}

	scanner := bufio.NewScanner(file)
	for scanner.Scan() {
		line := strings.TrimSpace(scanner.Text())
		if line == "" {
			continue
		}

		event, ok := parseAuditLogLine(line)
		if !ok {
			continue
		}

		eventQueue <- event
	}

	if err := scanner.Err(); err != nil {
		return offset, fmt.Errorf("failed to read audit log %s: %w", logPath, err)
	}

	currentOffset, err := file.Seek(0, 1)
	if err != nil {
		return offset, fmt.Errorf("failed to capture audit log offset: %w", err)
	}

	return currentOffset, nil
}

func parseAuditLogLine(line string) (models.SecEvent, bool) {
	msgMatch := auditMsgPattern.FindStringSubmatch(line)
	if len(msgMatch) != 3 {
		return models.SecEvent{}, false
	}

	timestamp, err := parseAuditTimestamp(msgMatch[1])
	if err != nil {
		return models.SecEvent{}, false
	}

	eventType := "UNKNOWN"
	if typeMatch := auditTypePattern.FindStringSubmatch(line); len(typeMatch) == 2 {
		eventType = typeMatch[1]
	}

	fields := map[string]string{
		"type":   eventType,
		"serial": msgMatch[2],
	}
	for _, match := range auditKVPattern.FindAllStringSubmatch(line, -1) {
		if len(match) < 3 {
			continue
		}
		fields[match[1]] = strings.Trim(match[2], `"`)
	}

	rawBytes, err := json.Marshal(map[string]any{
		"line":   line,
		"type":   eventType,
		"serial": msgMatch[2],
		"fields": fields,
	})
	if err != nil {
		return models.SecEvent{}, false
	}

	return models.SecEvent{
		SourceTool:  models.AUDITD,
		Timestamp:   timestamp,
		Priority:    classifyAuditPriority(fields),
		Description: describeAuditEvent(fields),
		RawPayload:  json.RawMessage(rawBytes),
	}, true
}

func parseAuditTimestamp(raw string) (time.Time, error) {
	parts := strings.SplitN(raw, ".", 2)
	seconds, err := strconv.ParseInt(parts[0], 10, 64)
	if err != nil {
		return time.Time{}, err
	}

	nanos := int64(0)
	if len(parts) == 2 {
		fraction := parts[1]
		if len(fraction) > 9 {
			fraction = fraction[:9]
		}
		for len(fraction) < 9 {
			fraction += "0"
		}
		nanos, err = strconv.ParseInt(fraction, 10, 64)
		if err != nil {
			return time.Time{}, err
		}
	}

	return time.Unix(seconds, nanos).UTC(), nil
}

func classifyAuditPriority(fields map[string]string) string {
	if strings.EqualFold(fields["success"], "no") {
		return "Warning"
	}

	switch fields["type"] {
	case "ANOM_ABEND", "ANOM_EXEC", "AVC", "USER_AVC":
		return "High"
	default:
		return "Info"
	}
}

func describeAuditEvent(fields map[string]string) string {
	parts := []string{"auditd", fields["type"]}

	if key := fields["key"]; key != "" && key != "(null)" {
		parts = append(parts, "key="+key)
	}
	if exe := fields["exe"]; exe != "" {
		parts = append(parts, "exe="+exe)
	} else if comm := fields["comm"]; comm != "" {
		parts = append(parts, "comm="+comm)
	}
	if success := fields["success"]; success != "" {
		parts = append(parts, "success="+success)
	}

	return strings.Join(parts, " ")
}
