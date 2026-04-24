package installers

import (
	"errors"
	"os"
	"path/filepath"
)

type fakeRunner struct {
	lookups  map[string]error
	commands [][]string
	runErr   error
	scripts  []string
}

func (f *fakeRunner) LookPath(name string) error {
	if err, ok := f.lookups[name]; ok {
		return err
	}
	return errors.New("not found")
}

func (f *fakeRunner) Run(name string, args ...string) error {
	cmd := append([]string{name}, args...)
	f.commands = append(f.commands, cmd)
	return f.runErr
}

func (f *fakeRunner) RunScript(script string) error {
	f.scripts = append(f.scripts, script)
	return f.runErr
}

type testFileSystem struct {
	root string
}

func (t testFileSystem) MkdirAll(path string, perm os.FileMode) error {
	return os.MkdirAll(filepath.Join(t.root, path), perm)
}

func (t testFileSystem) WriteFile(name string, data []byte, perm os.FileMode) error {
	full := filepath.Join(t.root, name)
	if err := os.MkdirAll(filepath.Dir(full), 0755); err != nil {
		return err
	}
	return os.WriteFile(full, data, perm)
}

func (t testFileSystem) Stat(name string) (os.FileInfo, error) {
	return os.Stat(filepath.Join(t.root, name))
}
