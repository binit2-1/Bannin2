package installers

import (
	"errors"
	"os"
	"path/filepath"
	"testing"
)

func TestAuditdInstallSkipsWhenAlreadyInstalled(t *testing.T) {
	runner := &fakeRunner{lookups: map[string]error{"auditctl": nil}}
	tool := &AuditdTool{runner: runner, fs: testFileSystem{root: t.TempDir()}}

	if err := tool.Install(); err != nil {
		t.Fatalf("Install: %v", err)
	}
	if len(runner.commands) != 0 || len(runner.scripts) != 0 {
		t.Fatal("expected no install commands when auditd is already present")
	}
}

func TestAuditdInstallPrefersAPT(t *testing.T) {
	runner := &fakeRunner{lookups: map[string]error{"apt": nil}}
	tool := &AuditdTool{runner: runner, fs: testFileSystem{root: t.TempDir()}}

	if err := tool.Install(); err != nil {
		t.Fatalf("Install: %v", err)
	}
	if len(runner.scripts) != 1 {
		t.Fatalf("expected apt script to run once, got %d", len(runner.scripts))
	}
}

func TestAuditdConfigureWritesBanninRules(t *testing.T) {
	root := t.TempDir()
	tool := &AuditdTool{runner: &fakeRunner{}, fs: testFileSystem{root: root}}

	if err := tool.Configure(); err != nil {
		t.Fatalf("Configure: %v", err)
	}

	data, err := os.ReadFile(filepath.Join(root, "/etc/audit/rules.d/bannin.rules"))
	if err != nil {
		t.Fatalf("read rules: %v", err)
	}
	if len(data) == 0 {
		t.Fatal("expected rules file contents")
	}
}

func TestAuditdStartReturnsError(t *testing.T) {
	tool := &AuditdTool{
		runner: &fakeRunner{runErr: errors.New("systemctl failed")},
		fs:     testFileSystem{root: t.TempDir()},
	}

	err := tool.Start()
	if err == nil || err.Error() == "" {
		t.Fatalf("expected start error, got %v", err)
	}
}
