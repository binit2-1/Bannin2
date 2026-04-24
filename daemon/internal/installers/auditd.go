package installers

import "fmt"

type AuditdTool struct {
	runner CommandRunner
	fs     FileSystem
}

func NewAuditdTool() *AuditdTool {
	return &AuditdTool{
		runner: OSCommandRunner{},
		fs:     OSFileSystem{},
	}
}

func (a *AuditdTool) Name() string {
	return "Auditd"
}

func (a *AuditdTool) Description() string {
	return "Linux audit subsystem"
}

func (a *AuditdTool) Install() error {
	if err := a.commandRunner().LookPath("auditctl"); err == nil {
		return nil
	}

	if err := a.commandRunner().LookPath("dnf"); err == nil {
		return a.InstallRPM("dnf")
	}

	if err := a.commandRunner().LookPath("apt"); err == nil {
		return a.InstallAPT()
	}

	if err := a.commandRunner().LookPath("yum"); err == nil {
		return a.InstallRPM("yum")
	}

	return fmt.Errorf("unsupported operating system: no known package manager found")
}

func (a *AuditdTool) InstallRPM(packageManager string) error {
	if err := a.commandRunner().Run(packageManager, "install", "-y", "audit"); err != nil {
		return fmt.Errorf("failed to install audit package: %w", err)
	}
	return nil
}

func (a *AuditdTool) InstallAPT() error {
	script := `
set -e
apt-get update -y
DEBIAN_FRONTEND=noninteractive apt-get install -y auditd audispd-plugins
`

	return a.commandRunner().RunScript(script)
}

func (a *AuditdTool) Configure() error {
	rulesDir := "/etc/audit/rules.d"
	if err := a.fileSystem().MkdirAll(rulesDir, 0755); err != nil {
		return fmt.Errorf("failed to create audit rules directory: %w", err)
	}

	rules := []byte(`# Bannin auditd custom rules
# Track privileged execution by interactive users.
-a always,exit -F arch=b64 -S execve -F path=/usr/bin/sudo -F auid>=1000 -F auid!=unset -k bannin_priv_esc

# Track writes to common persistence locations.
-w /etc/systemd/system -p wa -k bannin_persistence
`)

	if err := a.fileSystem().WriteFile(rulesDir+"/bannin.rules", rules, 0640); err != nil {
		return fmt.Errorf("failed to write audit rules file: %w", err)
	}

	return nil
}

func (a *AuditdTool) Start() error {
	if err := a.commandRunner().Run("systemctl", "enable", "--now", "auditd"); err != nil {
		return fmt.Errorf("failed to enable auditd: %w", err)
	}

	if err := a.commandRunner().Run("augenrules", "--load"); err != nil {
		return fmt.Errorf("failed to load auditd rules: %w", err)
	}

	if err := a.commandRunner().Run("systemctl", "restart", "auditd"); err != nil {
		return fmt.Errorf("failed to restart auditd: %w", err)
	}

	return nil
}

func (a *AuditdTool) commandRunner() CommandRunner {
	if a.runner == nil {
		a.runner = OSCommandRunner{}
	}
	return a.runner
}

func (a *AuditdTool) fileSystem() FileSystem {
	if a.fs == nil {
		a.fs = OSFileSystem{}
	}
	return a.fs
}
