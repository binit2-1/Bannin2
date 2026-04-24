package receiver

import (
	"bytes"
	"encoding/json"
	"errors"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

type fakeCommander struct {
	output      []byte
	combinedErr error
	runErr      error
}

func (f fakeCommander) CombinedOutput(name string, args ...string) ([]byte, error) {
	return f.output, f.combinedErr
}

func (f fakeCommander) Run(name string, args ...string) error {
	return f.runErr
}

func TestHandleToolsReadSuccess(t *testing.T) {
	dir := t.TempDir()
	target := filepath.Join(dir, "sample.txt")
	if err := os.WriteFile(target, []byte("hello"), 0644); err != nil {
		t.Fatal(err)
	}

	req := httptest.NewRequest(http.MethodGet, "/tools/read?path="+target, nil)
	rec := httptest.NewRecorder()

	NewHandler().HandleToolsRead(rec, req)

	if rec.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d", rec.Code)
	}
	if !strings.Contains(rec.Body.String(), "hello") {
		t.Fatalf("unexpected body: %s", rec.Body.String())
	}
}

func TestHandleToolsWriteAndEditSuccess(t *testing.T) {
	dir := t.TempDir()
	target := filepath.Join(dir, "rules.txt")

	writeBody, _ := json.Marshal(WriteRequest{Path: target, Contents: "a: 1\n"})
	writeReq := httptest.NewRequest(http.MethodPost, "/tools/write", bytes.NewReader(writeBody))
	writeRec := httptest.NewRecorder()
	NewHandler().HandleToolsWrite(writeRec, writeReq)

	if writeRec.Code != http.StatusOK {
		t.Fatalf("write status: %d", writeRec.Code)
	}

	editBody, _ := json.Marshal(EditRequest{Path: target, OldContents: "a: 1", NewContents: "a: 2"})
	editReq := httptest.NewRequest(http.MethodPost, "/tools/edit", bytes.NewReader(editBody))
	editRec := httptest.NewRecorder()
	NewHandler().HandleToolsEdit(editRec, editReq)

	if editRec.Code != http.StatusOK {
		t.Fatalf("edit status: %d", editRec.Code)
	}
}

func TestHandleToolsValidateInstalledAuditdSuccess(t *testing.T) {
	handler := &Handler{commander: fakeCommander{output: []byte("No change")}}
	req := httptest.NewRequest(http.MethodGet, "/tools/validate?toolname=auditd", nil)
	rec := httptest.NewRecorder()

	handler.HandleToolsValidate(rec, req)

	if rec.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d", rec.Code)
	}
	if !strings.Contains(rec.Body.String(), "No change") {
		t.Fatalf("unexpected body: %s", rec.Body.String())
	}
}

func TestHandleToolsValidateDraftAuditdFailure(t *testing.T) {
	handler := NewHandler()
	body, _ := json.Marshal(ValidateRequest{Rules: "-a always,exit -F arch=b64", Toolname: "auditd"})
	req := httptest.NewRequest(http.MethodPost, "/tools/validate?toolname=auditd", bytes.NewReader(body))
	rec := httptest.NewRecorder()

	handler.HandleToolsValidate(rec, req)

	if rec.Code != http.StatusBadRequest {
		t.Fatalf("expected 400, got %d", rec.Code)
	}
	if !strings.Contains(rec.Body.String(), "auditd validation failed for proposed rules:") {
		t.Fatalf("unexpected body: %s", rec.Body.String())
	}
}

func TestHandleToolsWriteRejectsInvalidAuditdConfig(t *testing.T) {
	dir := t.TempDir()
	target := filepath.Join(dir, "etc", "audit", "rules.d", "bannin.rules")

	handler := NewHandler()
	body, _ := json.Marshal(WriteRequest{Path: target, Contents: "-a always,exit -F arch=b64"})
	req := httptest.NewRequest(http.MethodPost, "/tools/write", bytes.NewReader(body))
	rec := httptest.NewRecorder()

	handler.HandleToolsWrite(rec, req)

	if rec.Code != http.StatusBadRequest {
		t.Fatalf("expected 400, got %d", rec.Code)
	}
	if !strings.Contains(rec.Body.String(), "auditd validation failed for proposed rules:") {
		t.Fatalf("unexpected body: %s", rec.Body.String())
	}
}

func TestHandleToolsRestartBlocksAuditdOnInvalidValidation(t *testing.T) {
	handler := &Handler{commander: fakeCommander{output: []byte("syntax error"), combinedErr: errors.New("exit 1")}}
	req := httptest.NewRequest(http.MethodGet, "/tools/restart?toolname=auditd", nil)
	rec := httptest.NewRecorder()

	handler.HandleToolsRestart(rec, req)

	if rec.Code != http.StatusBadRequest {
		t.Fatalf("expected 400, got %d", rec.Code)
	}
	if !strings.Contains(rec.Body.String(), "auditd validation failed for installed configuration:") {
		t.Fatalf("unexpected body: %s", rec.Body.String())
	}
}

func TestValidateAuditdRuleLineRequiresKey(t *testing.T) {
	err := validateAuditdRuleLine("-w /etc/shadow -p wa", 1)
	if err == nil || !strings.Contains(err.Error(), "stable -k key") {
		t.Fatalf("expected key error, got %v", err)
	}
}
