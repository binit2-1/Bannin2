package app

import (
	"errors"
	"strings"
	"testing"

	"github.com/Shreehari-Acharya/Bannin/daemon/internal/installers"
	tea "github.com/charmbracelet/bubbletea"
)

type fakeFormatter struct{}

func (fakeFormatter) Phase(tool, phase, detail string) string {
	return tool + ":" + phase + ":" + detail
}
func (fakeFormatter) Success(msg string) string { return "ok:" + msg }
func (fakeFormatter) Error(msg string) string   { return "err:" + msg }
func (fakeFormatter) Command(msg string) string { return "cmd:" + msg }

type fakeTool struct {
	name         string
	installErr   error
	configureErr error
	startErr     error
}

func (f fakeTool) Name() string        { return f.name }
func (f fakeTool) Description() string { return "test tool" }
func (f fakeTool) Install() error      { return f.installErr }
func (f fakeTool) Configure() error    { return f.configureErr }
func (f fakeTool) Start() error        { return f.startErr }

func collectMessages(ch <-chan tea.Msg) []tea.Msg {
	var out []tea.Msg
	for msg := range ch {
		out = append(out, msg)
	}
	return out
}

func TestRunInstallPipelineSuccess(t *testing.T) {
	ch := make(chan tea.Msg, 32)
	tools := []installers.SecurityTools{fakeTool{name: "Auditd"}}

	RunInstallPipeline(tools, fakeFormatter{}, ch)
	msgs := collectMessages(ch)

	if len(msgs) == 0 {
		t.Fatal("expected messages")
	}

	var advances int
	var done InstallDoneMsg
	for _, msg := range msgs {
		switch typed := msg.(type) {
		case InstallLogMsg:
			advances += typed.Advance
		case InstallDoneMsg:
			done = typed
		}
	}

	if done.Err != nil {
		t.Fatalf("unexpected error: %v", done.Err)
	}

	if advances != 3 {
		t.Fatalf("expected 3 progress advances, got %d", advances)
	}
}

func TestRunInstallPipelineStopsOnConfigureError(t *testing.T) {
	ch := make(chan tea.Msg, 32)
	tools := []installers.SecurityTools{fakeTool{name: "Auditd", configureErr: errors.New("bad config")}}

	RunInstallPipeline(tools, fakeFormatter{}, ch)
	msgs := collectMessages(ch)

	var done InstallDoneMsg
	found := false
	for _, msg := range msgs {
		if typed, ok := msg.(InstallDoneMsg); ok {
			done = typed
			found = true
		}
	}

	if !found {
		t.Fatal("expected done message")
	}
	if done.Err == nil || !strings.Contains(done.Err.Error(), "configure failed") {
		t.Fatalf("expected configure error, got %v", done.Err)
	}
}

func TestWaitForInstallMsgReturnsNilWhenChannelCloses(t *testing.T) {
	ch := make(chan tea.Msg)
	close(ch)

	if msg := WaitForInstallMsg(ch)(); msg != nil {
		t.Fatalf("expected nil message, got %#v", msg)
	}
}
