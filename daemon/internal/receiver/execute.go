package receiver

import (
	"encoding/json"
	"errors"
	"fmt"
	"io/fs"
	"log"
	"net/http"
	"os"
	"os/exec"
	"path/filepath"
	"strconv"
	"strings"
)

type WriteRequest struct {
	Contents string `json:"contents"`
	Path     string `json:"path"`
}

type EditRequest struct {
	OldContents string `json:"oldContents"`
	NewContents string `json:"newContents"`
	Path        string `json:"path"`
}

type Commander interface {
	CombinedOutput(name string, args ...string) ([]byte, error)
	Run(name string, args ...string) error
}

type OSCommander struct{}

func (OSCommander) CombinedOutput(name string, args ...string) ([]byte, error) {
	return exec.Command(name, args...).CombinedOutput()
}

func (OSCommander) Run(name string, args ...string) error {
	return exec.Command(name, args...).Run()
}

type Handler struct {
	commander Commander
}

func NewHandler() *Handler {
	return &Handler{commander: OSCommander{}}
}

func (h *Handler) commandRunner() Commander {
	if h.commander == nil {
		h.commander = OSCommander{}
	}
	return h.commander
}

func agentLog(format string, args ...any) {
	log.Printf("[agent-api] "+format, args...)
}

func resolvePath(input string) (string, error) {
	if strings.TrimSpace(input) == "" {
		return "", errors.New("empty path")
	}

	raw := strings.TrimSpace(input)
	if raw == "~" || strings.HasPrefix(raw, "~/") {
		homeDir, err := os.UserHomeDir()
		if err != nil {
			return "", fmt.Errorf("failed to resolve home directory: %w", err)
		}
		if raw == "~" {
			raw = homeDir
		} else {
			raw = filepath.Join(homeDir, strings.TrimPrefix(raw, "~/"))
		}
	}

	cleaned := filepath.Clean(raw)
	abs, err := filepath.Abs(cleaned)
	if err != nil {
		return "", fmt.Errorf("failed to resolve absolute path: %w", err)
	}

	return abs, nil
}

func HandleToolsRead(w http.ResponseWriter, r *http.Request) {
	NewHandler().HandleToolsRead(w, r)
}

func HandleToolsWrite(w http.ResponseWriter, r *http.Request) {
	NewHandler().HandleToolsWrite(w, r)
}

func HandleToolsEdit(w http.ResponseWriter, r *http.Request) {
	NewHandler().HandleToolsEdit(w, r)
}

func HandleToolsValidate(w http.ResponseWriter, r *http.Request) {
	NewHandler().HandleToolsValidate(w, r)
}

func HandleToolsRestart(w http.ResponseWriter, r *http.Request) {
	NewHandler().HandleToolsRestart(w, r)
}

func HandleDirEnum(w http.ResponseWriter, r *http.Request) {
	NewHandler().HandleDirEnum(w, r)
}

func (h *Handler) HandleToolsRead(w http.ResponseWriter, r *http.Request) {
	path, err := resolvePath(r.URL.Query().Get("path"))
	if err != nil {
		agentLog("read rejected: invalid path: %v", err)
		http.Error(w, "missing 'path' query parameter", http.StatusBadRequest)
		return
	}

	data, err := os.ReadFile(path)
	if err != nil {
		agentLog("read failed for %s: %v", path, err)
		http.Error(w, fmt.Sprintf("error reading file: %v", err), http.StatusInternalServerError)
		return
	}

	w.Header().Set("Content-Type", "application/json")
	_ = json.NewEncoder(w).Encode(map[string]string{
		"filepath": path,
		"contents": string(data),
	})
}

func (h *Handler) HandleToolsWrite(w http.ResponseWriter, r *http.Request) {
	var req WriteRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		agentLog("write rejected: invalid JSON body: %v", err)
		http.Error(w, "invalid JSON body", http.StatusBadRequest)
		return
	}

	path, err := resolveRequestPath(r, req.Path)
	if err != nil {
		agentLog("write rejected: invalid path: %v", err)
		http.Error(w, "missing 'path' query parameter", http.StatusBadRequest)
		return
	}

	if err := os.MkdirAll(filepath.Dir(path), 0755); err != nil {
		http.Error(w, fmt.Sprintf("error creating directory: %v", err), http.StatusInternalServerError)
		return
	}

	if shouldValidateFalcoPath(path) {
		if _, err := h.validateFalcoRuleContents(req.Contents); err != nil {
			http.Error(w, err.Error(), http.StatusBadRequest)
			return
		}
	}

	if err := os.WriteFile(path, []byte(req.Contents), 0644); err != nil {
		http.Error(w, fmt.Sprintf("error writing file: %v", err), http.StatusInternalServerError)
		return
	}

	w.WriteHeader(http.StatusOK)
	_, _ = w.Write([]byte("success"))
}

func (h *Handler) HandleToolsValidate(w http.ResponseWriter, r *http.Request) {
	result, err := h.validateFalcoBaseConfig()
	if err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}

	w.Header().Set("Content-Type", "text/plain; charset=utf-8")
	w.WriteHeader(http.StatusOK)
	_, _ = w.Write([]byte(result))
}

func (h *Handler) HandleToolsEdit(w http.ResponseWriter, r *http.Request) {
	var req EditRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		http.Error(w, "invalid JSON body", http.StatusBadRequest)
		return
	}

	path, err := resolveRequestPath(r, req.Path)
	if err != nil {
		http.Error(w, "missing 'path' query parameter", http.StatusBadRequest)
		return
	}

	if req.OldContents == "" {
		http.Error(w, "missing old content in request body", http.StatusBadRequest)
		return
	}

	data, err := os.ReadFile(path)
	if err != nil {
		http.Error(w, fmt.Sprintf("error reading file for edit: %v", err), http.StatusInternalServerError)
		return
	}

	content := string(data)
	if !strings.Contains(content, req.OldContents) {
		http.Error(w, "oldContents not found in file", http.StatusBadRequest)
		return
	}

	updated := strings.Replace(content, req.OldContents, req.NewContents, 1)
	if shouldValidateFalcoPath(path) {
		if _, err := h.validateFalcoRuleContents(updated); err != nil {
			http.Error(w, err.Error(), http.StatusBadRequest)
			return
		}
	}

	if err := os.WriteFile(path, []byte(updated), 0644); err != nil {
		http.Error(w, fmt.Sprintf("error saving edited file: %v", err), http.StatusInternalServerError)
		return
	}

	w.WriteHeader(http.StatusOK)
	_, _ = w.Write([]byte("success"))
}

func (h *Handler) HandleToolsRestart(w http.ResponseWriter, r *http.Request) {
	toolname := strings.TrimSpace(r.URL.Query().Get("toolname"))
	if toolname == "" {
		http.Error(w, "missing 'toolname' query parameter", http.StatusBadRequest)
		return
	}

	if strings.EqualFold(toolname, "falco") {
		if _, err := h.validateFalcoBaseConfig(); err != nil {
			http.Error(w, err.Error(), http.StatusBadRequest)
			return
		}
	}

	if err := h.commandRunner().Run("systemctl", "restart", toolname); err != nil {
		http.Error(w, fmt.Sprintf("failed to restart %s: %v", toolname, err), http.StatusInternalServerError)
		return
	}

	w.WriteHeader(http.StatusOK)
	_, _ = w.Write([]byte("success"))
}

func (h *Handler) HandleDirEnum(w http.ResponseWriter, r *http.Request) {
	targetPath := r.URL.Query().Get("path")
	if targetPath == "" {
		targetPath = "."
	}

	resolvedPath, err := resolvePath(targetPath)
	if err != nil {
		http.Error(w, "invalid 'path' query parameter", http.StatusBadRequest)
		return
	}

	maxDepth := 1
	if levelStr := r.URL.Query().Get("level"); levelStr != "" {
		if parsed, parseErr := strconv.Atoi(levelStr); parseErr == nil && parsed > 0 {
			maxDepth = parsed
		}
	}

	var sb strings.Builder
	sb.WriteString(fmt.Sprintf("Directory listing for: %s (Max Depth: %d)\n", resolvedPath, maxDepth))

	_ = filepath.Walk(resolvedPath, func(path string, info fs.FileInfo, walkErr error) error {
		if walkErr != nil {
			return nil
		}

		relPath, _ := filepath.Rel(resolvedPath, path)
		depth := strings.Count(relPath, string(os.PathSeparator))
		if relPath == "." {
			depth = 0
		}

		if depth > maxDepth {
			if info.IsDir() {
				return filepath.SkipDir
			}
			return nil
		}

		indent := strings.Repeat("  ", depth)
		if info.IsDir() {
			fmt.Fprintf(&sb, "%s📁 %s/\n", indent, info.Name())
		} else {
			fmt.Fprintf(&sb, "%s📄 %s\n", indent, info.Name())
		}
		return nil
	})

	w.Header().Set("Content-Type", "application/json")
	_ = json.NewEncoder(w).Encode(map[string]string{
		"contents": sb.String(),
	})
}

func resolveRequestPath(r *http.Request, fallback string) (string, error) {
	rawPath := strings.TrimSpace(r.URL.Query().Get("path"))
	if rawPath == "" {
		rawPath = fallback
	}
	return resolvePath(rawPath)
}

func shouldValidateFalcoPath(path string) bool {
	cleanPath := filepath.ToSlash(strings.ToLower(path))
	return strings.Contains(cleanPath, "/falco/")
}

func (h *Handler) validateFalcoBaseConfig() (string, error) {
	output, err := h.commandRunner().CombinedOutput("sudo", "falco", "-c", "/etc/falco/falco.yaml", "--dry-run")
	result := strings.TrimSpace(string(output))
	if result == "" {
		result = "validation completed with no output"
	}

	if err != nil {
		return result, fmt.Errorf("Validation failed:\n%s", result)
	}

	return result, nil
}

func (h *Handler) validateFalcoRuleContents(contents string) (string, error) {
	tmpFile, err := os.CreateTemp("", "falco-validate-*.yaml")
	if err != nil {
		return "", fmt.Errorf("failed to create validation temp file: %w", err)
	}
	tmpPath := tmpFile.Name()
	defer os.Remove(tmpPath)

	if _, err := tmpFile.WriteString(contents); err != nil {
		_ = tmpFile.Close()
		return "", fmt.Errorf("failed to write validation temp file: %w", err)
	}

	if err := tmpFile.Close(); err != nil {
		return "", fmt.Errorf("failed to close validation temp file: %w", err)
	}

	output, err := h.commandRunner().CombinedOutput(
		"sudo", "falco",
		"-c", "/etc/falco/falco.yaml",
		"-r", tmpPath,
		"--dry-run",
	)
	result := strings.TrimSpace(string(output))
	if result == "" {
		result = "validation completed with no output"
	}

	if err != nil {
		return result, fmt.Errorf("Validation failed:\n%s", result)
	}

	return result, nil
}
