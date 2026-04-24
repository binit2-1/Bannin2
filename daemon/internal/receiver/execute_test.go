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
	output       []byte
	combinedErr  error
	runErr       error
	combinedArgs [][]string
	runArgs      [][]string
}

func (f fakeCommander) CombinedOutput(name string, args ...string) ([]byte, error) {
	f.combinedArgs = append(f.combinedArgs, append([]string{name}, args...))
	return f.output, f.combinedErr
}

func (f fakeCommander) Run(name string, args ...string) error {
	f.runArgs = append(f.runArgs, append([]string{name}, args...))
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

func TestHandleToolsReadRejectsMissingPath(t *testing.T) {
	req := httptest.NewRequest(http.MethodGet, "/tools/read", nil)
	rec := httptest.NewRecorder()

	NewHandler().HandleToolsRead(rec, req)

	if rec.Code != http.StatusBadRequest {
		t.Fatalf("expected 400, got %d", rec.Code)
	}
}

func TestHandleToolsWriteAndEditSuccess(t *testing.T) {
	dir := t.TempDir()
	target := filepath.Join(dir, "rules.yaml")

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

	data, err := os.ReadFile(target)
	if err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(string(data), "a: 2") {
		t.Fatalf("unexpected contents: %s", string(data))
	}
}

func TestHandleToolsEditRejectsMissingOldContents(t *testing.T) {
	body, _ := json.Marshal(EditRequest{Path: filepath.Join(t.TempDir(), "f.txt")})
	req := httptest.NewRequest(http.MethodPost, "/tools/edit", bytes.NewReader(body))
	rec := httptest.NewRecorder()

	NewHandler().HandleToolsEdit(rec, req)

	if rec.Code != http.StatusBadRequest {
		t.Fatalf("expected 400, got %d", rec.Code)
	}
}

func TestHandleToolsValidateSuccess(t *testing.T) {
	handler := &Handler{commander: fakeCommander{output: []byte("config ok")}}
	req := httptest.NewRequest(http.MethodGet, "/tools/validate", nil)
	rec := httptest.NewRecorder()

	handler.HandleToolsValidate(rec, req)

	if rec.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d", rec.Code)
	}
	if strings.TrimSpace(rec.Body.String()) != "config ok" {
		t.Fatalf("unexpected body: %q", rec.Body.String())
	}
}

func TestHandleToolsValidateFailure(t *testing.T) {
	handler := &Handler{commander: fakeCommander{output: []byte("bad config"), combinedErr: errors.New("exit 1")}}
	req := httptest.NewRequest(http.MethodGet, "/tools/validate", nil)
	rec := httptest.NewRecorder()

	handler.HandleToolsValidate(rec, req)

	if rec.Code != http.StatusBadRequest {
		t.Fatalf("expected 400, got %d", rec.Code)
	}
	if !strings.Contains(rec.Body.String(), "Falco validation failed for installed configuration:") || !strings.Contains(rec.Body.String(), "bad config") {
		t.Fatalf("unexpected body: %s", rec.Body.String())
	}
}

func TestHandleToolsValidateDraftRulesSuccess(t *testing.T) {
	handler := &Handler{commander: fakeCommander{output: []byte("draft ok")}}
	body, _ := json.Marshal(ValidateRequest{Rules: "- rule: ok", Toolname: "falco"})
	req := httptest.NewRequest(http.MethodPost, "/tools/validate?toolname=falco", bytes.NewReader(body))
	rec := httptest.NewRecorder()

	handler.HandleToolsValidate(rec, req)

	if rec.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d", rec.Code)
	}
	if strings.TrimSpace(rec.Body.String()) != "draft ok" {
		t.Fatalf("unexpected body: %q", rec.Body.String())
	}
}

func TestHandleToolsValidateDraftRulesFailure(t *testing.T) {
	handler := &Handler{commander: fakeCommander{output: []byte("syntax error at line 8"), combinedErr: errors.New("exit 1")}}
	body, _ := json.Marshal(ValidateRequest{Rules: "- rule: bad", Toolname: "falco"})
	req := httptest.NewRequest(http.MethodPost, "/tools/validate?toolname=falco", bytes.NewReader(body))
	rec := httptest.NewRecorder()

	handler.HandleToolsValidate(rec, req)

	if rec.Code != http.StatusBadRequest {
		t.Fatalf("expected 400, got %d", rec.Code)
	}
	if !strings.Contains(rec.Body.String(), "Falco validation failed for proposed rules:") || !strings.Contains(rec.Body.String(), "syntax error at line 8") {
		t.Fatalf("unexpected body: %s", rec.Body.String())
	}
}

func TestHandleToolsWriteRejectsInvalidFalcoConfig(t *testing.T) {
	dir := t.TempDir()
	target := filepath.Join(dir, "etc", "falco", "rules.yaml")

	handler := &Handler{commander: fakeCommander{output: []byte("syntax error at line 3"), combinedErr: errors.New("exit 1")}}
	body, _ := json.Marshal(WriteRequest{Path: target, Contents: "- rule: invalid"})
	req := httptest.NewRequest(http.MethodPost, "/tools/write", bytes.NewReader(body))
	rec := httptest.NewRecorder()

	handler.HandleToolsWrite(rec, req)

	if rec.Code != http.StatusBadRequest {
		t.Fatalf("expected 400, got %d", rec.Code)
	}
	if !strings.Contains(rec.Body.String(), "Falco validation failed for proposed rules:") || !strings.Contains(rec.Body.String(), "syntax error") {
		t.Fatalf("unexpected body: %s", rec.Body.String())
	}
	if _, err := os.Stat(target); !errors.Is(err, os.ErrNotExist) {
		t.Fatalf("expected file to be absent after failed validation, stat err=%v", err)
	}
}

func TestHandleToolsEditRejectsInvalidFalcoConfig(t *testing.T) {
	dir := t.TempDir()
	target := filepath.Join(dir, "etc", "falco", "rules.yaml")
	if err := os.MkdirAll(filepath.Dir(target), 0755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(target, []byte("a: 1\n"), 0644); err != nil {
		t.Fatal(err)
	}

	handler := &Handler{commander: fakeCommander{output: []byte("invalid rule"), combinedErr: errors.New("exit 1")}}
	body, _ := json.Marshal(EditRequest{Path: target, OldContents: "a: 1", NewContents: "a: ["})
	req := httptest.NewRequest(http.MethodPost, "/tools/edit", bytes.NewReader(body))
	rec := httptest.NewRecorder()

	handler.HandleToolsEdit(rec, req)

	if rec.Code != http.StatusBadRequest {
		t.Fatalf("expected 400, got %d", rec.Code)
	}
	if !strings.Contains(rec.Body.String(), "Falco validation failed for proposed rules:") || !strings.Contains(rec.Body.String(), "invalid rule") {
		t.Fatalf("unexpected body: %s", rec.Body.String())
	}

	data, err := os.ReadFile(target)
	if err != nil {
		t.Fatal(err)
	}
	if string(data) != "a: 1\n" {
		t.Fatalf("expected original contents unchanged, got: %s", string(data))
	}
}

func TestHandleToolsRestartBlocksFalcoOnInvalidValidation(t *testing.T) {
	handler := &Handler{commander: fakeCommander{output: []byte("bad config"), combinedErr: errors.New("exit 1")}}
	req := httptest.NewRequest(http.MethodGet, "/tools/restart?toolname=falco", nil)
	rec := httptest.NewRecorder()

	handler.HandleToolsRestart(rec, req)

	if rec.Code != http.StatusBadRequest {
		t.Fatalf("expected 400, got %d", rec.Code)
	}
	if !strings.Contains(rec.Body.String(), "Falco validation failed for installed configuration:") {
		t.Fatalf("unexpected body: %s", rec.Body.String())
	}
}

func TestHandleToolsRestartRequiresToolname(t *testing.T) {
	req := httptest.NewRequest(http.MethodGet, "/tools/restart", nil)
	rec := httptest.NewRecorder()

	NewHandler().HandleToolsRestart(rec, req)

	if rec.Code != http.StatusBadRequest {
		t.Fatalf("expected 400, got %d", rec.Code)
	}
}

func TestHandleDirEnumSuccess(t *testing.T) {
	dir := t.TempDir()
	if err := os.WriteFile(filepath.Join(dir, "a.txt"), []byte("x"), 0644); err != nil {
		t.Fatal(err)
	}
	if err := os.Mkdir(filepath.Join(dir, "nested"), 0755); err != nil {
		t.Fatal(err)
	}

	req := httptest.NewRequest(http.MethodGet, "/tools/direnum?path="+dir+"&level=2", nil)
	rec := httptest.NewRecorder()

	NewHandler().HandleDirEnum(rec, req)

	if rec.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d", rec.Code)
	}
	if !strings.Contains(rec.Body.String(), "a.txt") || !strings.Contains(rec.Body.String(), "nested") {
		t.Fatalf("unexpected body: %s", rec.Body.String())
	}
}
