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
	output []byte
	runErr error
}

func (f fakeCommander) CombinedOutput(name string, args ...string) ([]byte, error) {
	return f.output, f.runErr
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
	handler := &Handler{commander: fakeCommander{output: []byte("bad config"), runErr: errors.New("exit 1")}}
	req := httptest.NewRequest(http.MethodGet, "/tools/validate", nil)
	rec := httptest.NewRecorder()

	handler.HandleToolsValidate(rec, req)

	if rec.Code != http.StatusInternalServerError {
		t.Fatalf("expected 500, got %d", rec.Code)
	}
	if !strings.Contains(rec.Body.String(), "bad config") {
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
