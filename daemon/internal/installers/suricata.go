package installers

import "fmt"

type SuricataTool struct {
	runner CommandRunner
	fs     FileSystem
}

func NewSuricataTool() *SuricataTool {
	return &SuricataTool{
		runner: OSCommandRunner{},
		fs:     OSFileSystem{},
	}
}

func (s *SuricataTool) Name() string {
	return "Suricata"
}

func (s *SuricataTool) Description() string {
	return "High Performance Network IDS, IPS, and Network Security Monitoring Engine"
}

func (s *SuricataTool) Install() error {
	if err := s.commandRunner().LookPath("suricata"); err == nil {
		return nil
	}

	if err := s.commandRunner().LookPath("dnf"); err == nil {
		return s.InstallRPM("dnf")
	}

	if err := s.commandRunner().LookPath("apt"); err == nil {
		return s.InstallAPT("apt")
	}

	if err := s.commandRunner().LookPath("yum"); err == nil {
		return s.InstallRPM("yum")
	}

	return fmt.Errorf("unsupported operating system: no known package manager found")
}

func (s *SuricataTool) InstallRPM(packageManager string) error {
	if packageManager != "dnf" {
		return fmt.Errorf("rpm installation requires dnf for the OISF COPR repository")
	}

	cmds := [][]string{
		{"dnf", "install", "-y", "epel-release", "dnf-plugins-core"},
		{"dnf", "copr", "enable", "-y", "@oisf/suricata-8.0"},
		{"dnf", "install", "-y", "suricata", "jq"},
	}

	for _, cmdArgs := range cmds {
		if err := s.commandRunner().Run(cmdArgs[0], cmdArgs[1:]...); err != nil {
			return fmt.Errorf("failed to install suricata with rpm packages: %w", err)
		}
	}

	return nil
}

func (s *SuricataTool) InstallAPT(_ string) error {
	script := `
set -e
apt-get update -y
apt-get install -y software-properties-common
add-apt-repository -y ppa:oisf/suricata-stable
apt-get update -y
apt-get install -y suricata jq
`

	if err := s.commandRunner().RunScript(script); err != nil {
		return fmt.Errorf("failed to install suricata with apt packages: %w", err)
	}

	return nil
}

func (s *SuricataTool) Configure() error {
	dirs := []string{
		"/etc/suricata",
		"/var/log/suricata",
		"/var/lib/suricata",
	}

	for _, dir := range dirs {
		if err := s.fileSystem().MkdirAll(dir, 0755); err != nil {
			return fmt.Errorf("failed to create suricata directory %s: %w", dir, err)
		}
	}

	if err := s.commandRunner().LookPath("suricata-update"); err == nil {
		if err := s.commandRunner().Run("suricata-update"); err != nil {
			return fmt.Errorf("failed to update suricata rules: %w", err)
		}
	}

	return nil
}

func (s *SuricataTool) Start() error {
	if err := s.commandRunner().Run("systemctl", "daemon-reload"); err != nil {
		return fmt.Errorf("failed to reload systemd: %w", err)
	}

	if err := s.commandRunner().Run("systemctl", "enable", "--now", "suricata"); err != nil {
		return fmt.Errorf("failed to enable and start suricata: %w", err)
	}

	return nil
}

func (s *SuricataTool) commandRunner() CommandRunner {
	if s.runner == nil {
		s.runner = OSCommandRunner{}
	}
	return s.runner
}

func (s *SuricataTool) fileSystem() FileSystem {
	if s.fs == nil {
		s.fs = OSFileSystem{}
	}
	return s.fs
}
