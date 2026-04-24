package installers

import (
	"io"
	"os"
	"os/exec"
	"sync"
)

// SecurityTools describes the lifecycle required by the setup workflow.
type SecurityTools interface {
	Name() string
	Description() string
	Install() error
	Configure() error
	Start() error
}

// CommandRunner abstracts command execution so installers can be tested
// without invoking package managers or systemd.
type CommandRunner interface {
	LookPath(name string) error
	Run(name string, args ...string) error
	RunScript(script string) error
}

// FileSystem abstracts filesystem mutations needed by installers.
type FileSystem interface {
	MkdirAll(path string, perm os.FileMode) error
	WriteFile(name string, data []byte, perm os.FileMode) error
	Stat(name string) (os.FileInfo, error)
}

type OSCommandRunner struct{}

func (OSCommandRunner) LookPath(name string) error {
	_, err := exec.LookPath(name)
	return err
}

func (OSCommandRunner) Run(name string, args ...string) error {
	cmd := exec.Command(name, args...)
	cmd.Stdout = getCommandOutput()
	cmd.Stderr = getCommandOutput()
	return cmd.Run()
}

func (OSCommandRunner) RunScript(script string) error {
	cmd := exec.Command("bash", "-c", script)
	cmd.Stdout = getCommandOutput()
	cmd.Stderr = getCommandOutput()
	return cmd.Run()
}

type OSFileSystem struct{}

func (OSFileSystem) MkdirAll(path string, perm os.FileMode) error {
	return os.MkdirAll(path, perm)
}

func (OSFileSystem) WriteFile(name string, data []byte, perm os.FileMode) error {
	return os.WriteFile(name, data, perm)
}

func (OSFileSystem) Stat(name string) (os.FileInfo, error) {
	return os.Stat(name)
}

var (
	commandOutputMu sync.RWMutex
	commandOutput   io.Writer = os.Stdout
)

func SetCommandOutput(w io.Writer) {
	commandOutputMu.Lock()
	defer commandOutputMu.Unlock()

	if w == nil {
		commandOutput = os.Stdout
		return
	}

	commandOutput = w
}

func getCommandOutput() io.Writer {
	commandOutputMu.RLock()
	defer commandOutputMu.RUnlock()
	return commandOutput
}
