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

type ValidateRequest struct {
	Rules    string `json:"rules"`
	Path     string `json:"path"`
	Toolname string `json:"toolname"`
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

	if shouldValidateAuditdPath(path) {
		if _, err := h.validateAuditdRuleContents(req.Contents); err != nil {
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
	result, status, err := h.validateRequest(r)
	if err != nil {
		http.Error(w, err.Error(), status)
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
	if shouldValidateAuditdPath(path) {
		if _, err := h.validateAuditdRuleContents(updated); err != nil {
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

	switch strings.ToLower(toolname) {
	case "auditd":
		if _, err := h.validateAuditdBaseConfig(); err != nil {
			http.Error(w, err.Error(), http.StatusBadRequest)
			return
		}
		if result, err := h.loadAuditdRules(); err != nil {
			http.Error(w, err.Error(), http.StatusBadRequest)
			return
		} else if strings.TrimSpace(result) != "" {
			agentLog("auditd reload output: %s", result)
		}
	default:
		http.Error(w, fmt.Sprintf("restart for tool %q is not supported", toolname), http.StatusBadRequest)
		return
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
			fmt.Fprintf(&sb, "%sDIR %s/\n", indent, info.Name())
		} else {
			fmt.Fprintf(&sb, "%sFILE %s\n", indent, info.Name())
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

func shouldValidateAuditdPath(path string) bool {
	cleanPath := filepath.ToSlash(strings.ToLower(path))
	return strings.Contains(cleanPath, "/audit/") &&
		(strings.Contains(cleanPath, "/rules.d/") || strings.HasSuffix(cleanPath, "/audit.rules"))
}

func (h *Handler) validateRequest(r *http.Request) (string, int, error) {
	toolname := strings.TrimSpace(r.URL.Query().Get("toolname"))

	if r.Method == http.MethodPost {
		var req ValidateRequest
		if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
			return "", http.StatusBadRequest, fmt.Errorf("invalid JSON body")
		}

		if toolname == "" {
			toolname = strings.TrimSpace(req.Toolname)
		}
		if toolname == "" && shouldValidateAuditdPath(req.Path) {
			toolname = "auditd"
		}
		if strings.TrimSpace(req.Rules) == "" {
			return "", http.StatusBadRequest, fmt.Errorf("missing rules in request body")
		}

		switch strings.ToLower(toolname) {
		case "auditd":
			result, err := h.validateAuditdRuleContents(req.Rules)
			if err != nil {
				return result, http.StatusBadRequest, err
			}
			return result, http.StatusOK, nil
		case "":
			return "", http.StatusBadRequest, fmt.Errorf("missing 'toolname' query parameter")
		default:
			return "", http.StatusBadRequest, fmt.Errorf("validation for tool %q is not supported", toolname)
		}
	}

	switch strings.ToLower(toolname) {
	case "", "auditd":
		result, err := h.validateAuditdBaseConfig()
		if err != nil {
			return result, http.StatusBadRequest, err
		}
		return result, http.StatusOK, nil
	default:
		return "", http.StatusBadRequest, fmt.Errorf("validation for tool %q is not supported", toolname)
	}
}

func (h *Handler) validateAuditdBaseConfig() (string, error) {
	output, err := h.commandRunner().CombinedOutput("sudo", "augenrules", "--check")
	result := strings.TrimSpace(string(output))
	if result == "" {
		result = "auditd validation completed with no output"
	}

	if err != nil {
		return result, fmt.Errorf("auditd validation failed for installed configuration:\n%s", result)
	}

	return result, nil
}

func (h *Handler) validateAuditdRuleContents(contents string) (string, error) {
	lines := strings.Split(contents, "\n")
	for index, rawLine := range lines {
		line := strings.TrimSpace(rawLine)
		if line == "" || strings.HasPrefix(line, "#") {
			continue
		}

		if err := validateAuditdRuleLine(line, index+1); err != nil {
			return err.Error(), fmt.Errorf("auditd validation failed for proposed rules:\n%s", err.Error())
		}
	}

	return "auditd draft passed static validation", nil
}

func validateAuditdRuleLine(line string, lineNumber int) error {
	allowedPrefixes := []string{
		"-a ", "-A ", "-w ", "-W ", "-b ", "-f ", "-e ", "-D", "--loginuid-immutable",
	}
	for _, prefix := range allowedPrefixes {
		if strings.HasPrefix(line, prefix) {
			if strings.HasPrefix(line, "-a ") || strings.HasPrefix(line, "-A ") {
				if !strings.Contains(line, "-S ") {
					return fmt.Errorf("line %d: syscall rule must include at least one -S filter: %s", lineNumber, line)
				}
			}
			if (strings.HasPrefix(line, "-a ") || strings.HasPrefix(line, "-A ") || strings.HasPrefix(line, "-w ")) && !strings.Contains(line, "-k ") {
				return fmt.Errorf("line %d: rule must include a stable -k key: %s", lineNumber, line)
			}
			return nil
		}
	}

	return fmt.Errorf("line %d: unsupported auditd rule directive: %s", lineNumber, line)
}

func (h *Handler) loadAuditdRules() (string, error) {
	output, err := h.commandRunner().CombinedOutput("sudo", "augenrules", "--load")
	result := strings.TrimSpace(string(output))
	if result == "" {
		result = "auditd rules loaded with no output"
	}

	if err != nil {
		return result, fmt.Errorf("auditd validation failed while loading rules:\n%s", result)
	}

	return result, nil
}
