package installers

import "fmt"

type FalcoTool struct {
	runner CommandRunner
	fs     FileSystem
}

func NewFalcoTool() *FalcoTool {
	return &FalcoTool{
		runner: OSCommandRunner{},
		fs:     OSFileSystem{},
	}
}

func (f *FalcoTool) Name() string {
	return "Falco"
}

func (f *FalcoTool) Description() string {
	return "Cloud Native Runtime Security"
}

func (f *FalcoTool) Install() error {
	if err := f.commandRunner().LookPath("falco"); err == nil {
		return nil
	}

	if err := f.commandRunner().LookPath("dnf"); err == nil {
		return f.InstallRPM("dnf")
	}

	if err := f.commandRunner().LookPath("apt"); err == nil {
		return f.InstallAPT("apt")
	}

	if err := f.commandRunner().LookPath("yum"); err == nil {
		return f.InstallRPM("yum")
	}

	return fmt.Errorf("unsupported operating system: no known package manager found")
}

func (f *FalcoTool) InstallRPM(packageManager string) error {
	cmds := [][]string{
		{"rpm", "--import", "https://falco.org/repo/falcosecurity-packages.asc"},
		{"curl", "-s", "-o", "/etc/yum.repos.d/falcosecurity.repo", "https://falco.org/repo/falcosecurity-rpm.repo"},
		{"yum", "update", "-y"},
		{packageManager, "install", "-y", "falco"},
	}

	for _, cmdArgs := range cmds {
		if err := f.commandRunner().Run(cmdArgs[0], cmdArgs[1:]...); err != nil {
			return fmt.Errorf("failed to run %s: %w", cmdArgs[0], err)
		}
	}

	return nil
}

func (f *FalcoTool) InstallAPT(_ string) error {
	script := `
set -e
curl -fsSL https://falco.org/repo/falcosecurity-packages.asc | gpg --dearmor --yes -o /usr/share/keyrings/falco-archive-keyring.gpg
echo "deb [signed-by=/usr/share/keyrings/falco-archive-keyring.gpg] https://download.falco.org/packages/deb stable main" | tee /etc/apt/sources.list.d/falcosecurity.list
apt-get update -y
apt-get install -y falco
`

	return f.commandRunner().RunScript(script)
}

func (f *FalcoTool) Configure() error {
	configDir := "/etc/falco/config.d"
	if err := f.fileSystem().MkdirAll(configDir, 0755); err != nil {
		return fmt.Errorf("failed to create config directory: %w", err)
	}

	BANNINConfig := []byte(`
# bannin Auto-Generated Override Config
json_output: true
json_include_output_property: true
json_include_output_fields_property: true
json_include_tags_property: true

http_output:
  enabled: true
  url: "http://127.0.0.1:8081/falco"
`)

	filePath := configDir + "/bannin.yaml"
	if err := f.fileSystem().WriteFile(filePath, BANNINConfig, 0644); err != nil {
		return fmt.Errorf("failed to write config file: %w", err)
	}

	return nil
}

func (f *FalcoTool) Start() error {
	if err := f.commandRunner().Run("systemctl", "daemon-reload"); err != nil {
		return fmt.Errorf("failed to reload daemon: %w", err)
	}

	if err := f.commandRunner().Run("systemctl", "restart", "falco"); err != nil {
		return fmt.Errorf("failed to start falco: %w", err)
	}

	return nil
}

func (f *FalcoTool) commandRunner() CommandRunner {
	if f.runner == nil {
		f.runner = OSCommandRunner{}
	}
	return f.runner
}

func (f *FalcoTool) fileSystem() FileSystem {
	if f.fs == nil {
		f.fs = OSFileSystem{}
	}
	return f.fs
}
