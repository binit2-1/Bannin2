package watchers

import (
	"strings"
	"testing"

	"github.com/Shreehari-Acharya/Bannin/daemon/pkg/models"
)

func TestParseAuditLogLineAcceptsValidEvent(t *testing.T) {
	line := `type=SYSCALL msg=audit(1713953472.123:77): arch=c000003e syscall=59 success=no exe="/usr/bin/sudo" key="bannin_priv_esc"`

	event, ok := parseAuditLogLine(line)
	if !ok {
		t.Fatal("expected line to parse")
	}
	if event.SourceTool != models.AUDITD {
		t.Fatalf("unexpected source tool: %s", event.SourceTool)
	}
	if event.Priority != "Warning" {
		t.Fatalf("unexpected priority: %s", event.Priority)
	}
	if !strings.Contains(event.Description, "SYSCALL") || !strings.Contains(event.Description, "bannin_priv_esc") {
		t.Fatalf("unexpected description: %s", event.Description)
	}
	if len(event.RawPayload) == 0 {
		t.Fatal("expected raw payload")
	}
}

func TestParseAuditLogLineRejectsInvalidTimestamp(t *testing.T) {
	if _, ok := parseAuditLogLine(`type=SYSCALL msg=audit(nope:77): exe="/usr/bin/sudo"`); ok {
		t.Fatal("expected invalid timestamp to be rejected")
	}
}

func TestParseAuditLogLineRejectsMissingAuditEnvelope(t *testing.T) {
	if _, ok := parseAuditLogLine(`type=SYSCALL exe="/usr/bin/sudo"`); ok {
		t.Fatal("expected line without audit envelope to be rejected")
	}
}
