package ui

import (
	"fmt"
	"strings"
	"time"

	"github.com/Shreehari-Acharya/Bannin/daemon/internal/app"
	"github.com/Shreehari-Acharya/Bannin/daemon/internal/dispatcher"
	"github.com/Shreehari-Acharya/Bannin/daemon/internal/installers"
	"github.com/charmbracelet/bubbles/help"
	"github.com/charmbracelet/bubbles/key"
	"github.com/charmbracelet/bubbles/progress"
	"github.com/charmbracelet/bubbles/spinner"
	"github.com/charmbracelet/bubbles/textinput"
	"github.com/charmbracelet/bubbles/viewport"
	tea "github.com/charmbracelet/bubbletea"
	"github.com/charmbracelet/lipgloss"
)

const (
	colorInk     = lipgloss.Color("#E7F0EA")
	colorMuted   = lipgloss.Color("#8DA99B")
	colorPanel   = lipgloss.Color("#12211C")
	colorBorder  = lipgloss.Color("#3A6B56")
	colorAccent  = lipgloss.Color("#77D39F")
	colorAccent2 = lipgloss.Color("#D7F171")
	colorWarn    = lipgloss.Color("#FFB86C")
	colorError   = lipgloss.Color("#FF7B72")
)

var (
	docStyle = lipgloss.NewStyle().
			Border(lipgloss.RoundedBorder()).
			BorderForeground(colorBorder).
			Background(colorPanel).
			Padding(1, 2)

	titleStyle = lipgloss.NewStyle().
			Foreground(colorInk).
			Bold(true)

	subtleStyle = lipgloss.NewStyle().
			Foreground(colorMuted)

	accentStyle = lipgloss.NewStyle().
			Foreground(colorAccent).
			Bold(true)

	warnStyle = lipgloss.NewStyle().
			Foreground(colorWarn).
			Bold(true)

	errorStyle = lipgloss.NewStyle().
			Foreground(colorError).
			Bold(true)

	phaseStyle = lipgloss.NewStyle().
			Foreground(colorPanel).
			Background(colorAccent2).
			Bold(true).
			Padding(0, 1)

	commandStyle = lipgloss.NewStyle().
			Foreground(colorMuted)

	successStyle = lipgloss.NewStyle().
			Foreground(colorAccent).
			Bold(true)

	heroStyle = lipgloss.NewStyle().
			Foreground(colorAccent2).
			Bold(true)
)

const (
	stateIntro = iota
	stateInstalling
	stateProjectPath
	stateAskAnalyze
	stateAskRules
	stateGeneratingSummary
	stateGeneratingRules
	stateAskRestart
	stateRestarting
	stateDone
)

type summaryDoneMsg struct {
	text string
	err  error
}

type rulesDoneMsg struct {
	err error
}

type restartDoneMsg struct {
	err error
}

type keyMap struct {
	Enter key.Binding
	Up    key.Binding
	Down  key.Binding
	Left  key.Binding
	Right key.Binding
	Quit  key.Binding
}

func (k keyMap) ShortHelp() []key.Binding {
	return []key.Binding{k.Enter, k.Left, k.Right, k.Quit}
}

func (k keyMap) FullHelp() [][]key.Binding {
	return [][]key.Binding{{k.Enter, k.Left, k.Right, k.Up, k.Down, k.Quit}}
}

var defaultKeys = keyMap{
	Enter: key.NewBinding(key.WithKeys("enter"), key.WithHelp("enter", "continue")),
	Up:    key.NewBinding(key.WithKeys("up", "k"), key.WithHelp("up/k", "move")),
	Down:  key.NewBinding(key.WithKeys("down", "j"), key.WithHelp("down/j", "move")),
	Left:  key.NewBinding(key.WithKeys("left", "h", "y"), key.WithHelp("left/y", "yes")),
	Right: key.NewBinding(key.WithKeys("right", "l", "n"), key.WithHelp("right/n", "no")),
	Quit:  key.NewBinding(key.WithKeys("q", "ctrl+c"), key.WithHelp("q", "quit")),
}

type model struct {
	tools          []installers.SecurityTools
	backendURL     string
	state          int
	spinner        spinner.Model
	progress       progress.Model
	viewport       viewport.Model
	help           help.Model
	keys           keyMap
	textInput      textinput.Model
	windowWidth    int
	windowHeight   int
	logs           []string
	totalSteps     int
	completedSteps int
	hadError       bool
	installEvents  <-chan tea.Msg
	projectPath    string
	runAnalysis    bool
	writeRules     bool
	yesSelected    bool
	summaryText    string
}

func InitialModel(tools []installers.SecurityTools, backendURL string) model {
	spin := spinner.New()
	spin.Spinner = spinner.Dot
	spin.Style = accentStyle

	prog := progress.New(
		progress.WithScaledGradient("#3A6B56", "#D7F171"),
		progress.WithWidth(48),
		progress.WithoutPercentage(),
	)

	vp := viewport.New(72, 14)
	vp.Style = lipgloss.NewStyle().
		Border(lipgloss.NormalBorder()).
		BorderForeground(colorBorder).
		Padding(0, 1)

	h := help.New()
	h.Styles.ShortKey = accentStyle
	h.Styles.ShortDesc = subtleStyle

	input := textinput.New()
	input.Prompt = "> "
	input.Placeholder = "/path/to/project"
	input.CharLimit = 512
	input.Width = 64
	input.Focus()

	return model{
		tools:       tools,
		backendURL:  backendURL,
		state:       stateIntro,
		spinner:     spin,
		progress:    prog,
		viewport:    vp,
		help:        h,
		keys:        defaultKeys,
		textInput:   input,
		yesSelected: true,
	}
}

func (m model) Init() tea.Cmd {
	return m.spinner.Tick
}

func (m model) Update(msg tea.Msg) (tea.Model, tea.Cmd) {
	var cmds []tea.Cmd

	switch msg := msg.(type) {
	case app.InstallLogMsg:
		if msg.Line != "" {
			m.logs = append(m.logs, msg.Line)
			m.syncViewport()
		}
		if msg.Advance > 0 {
			m.completedSteps += msg.Advance
			if m.completedSteps > m.totalSteps {
				m.completedSteps = m.totalSteps
			}
		}
		if m.installEvents != nil {
			cmds = append(cmds, app.WaitForInstallMsg(m.installEvents))
		}

	case app.InstallDoneMsg:
		m.completedSteps = m.totalSteps
		m.installEvents = nil
		if msg.Err != nil {
			m.hadError = true
			m.state = stateDone
			m.logs = append(m.logs, m.Error(msg.Err.Error()))
			m.syncViewport()
			return m, nil
		}
		m.logs = append(m.logs, m.Success("auditd installation finished"))
		m.state = stateProjectPath
		m.textInput.SetValue("")
		m.textInput.Focus()
		m.syncViewport()
		return m, nil

	case summaryDoneMsg:
		if msg.err != nil {
			m.logs = append(m.logs, m.Error(msg.err.Error()))
			m.state = stateAskRestart
			m.yesSelected = true
			m.syncViewport()
			return m, nil
		}

		m.summaryText = msg.text
		if strings.TrimSpace(msg.text) != "" {
			m.logs = append(m.logs, m.Info("Project summary generated"))
			m.logs = append(m.logs, m.Command(truncateLine(msg.text, 140)))
		}
		m.logs = append(m.logs, m.Info("Generating auditd rules from backend"))
		m.state = stateGeneratingRules
		m.syncViewport()
		return m, generateRulesCmd(m.tools, m.summaryText, m.backendURL)

	case rulesDoneMsg:
		if msg.err != nil {
			m.logs = append(m.logs, m.Error(msg.err.Error()))
		} else {
			m.logs = append(m.logs, m.Success("Rule generation completed"))
			m.logs = append(m.logs, m.Info("auditd rules target: /etc/audit/rules.d/bannin.rules"))
			m.logs = append(m.logs, m.Info("auditd log source: /var/log/audit/audit.log"))
		}
		m.state = stateAskRestart
		m.yesSelected = true
		m.syncViewport()
		return m, nil

	case restartDoneMsg:
		if msg.err != nil {
			m.logs = append(m.logs, m.Error(msg.err.Error()))
		} else {
			m.logs = append(m.logs, m.Success("auditd service restarted"))
		}
		m.state = stateDone
		m.syncViewport()
		return m, nil

	case tea.WindowSizeMsg:
		m.windowWidth = msg.Width
		m.windowHeight = msg.Height
		m.syncLayout()

	case spinner.TickMsg:
		var cmd tea.Cmd
		m.spinner, cmd = m.spinner.Update(msg)
		cmds = append(cmds, cmd)

	case progress.FrameMsg:
		progressModel, cmd := m.progress.Update(msg)
		m.progress = progressModel.(progress.Model)
		cmds = append(cmds, cmd)

	case tea.KeyMsg:
		if key.Matches(msg, m.keys.Quit) {
			return m, tea.Quit
		}

		switch m.state {
		case stateIntro:
			if key.Matches(msg, m.keys.Enter) {
				return m.startInstall()
			}
		case stateProjectPath:
			if msg.Type == tea.KeyEnter {
				value := strings.TrimSpace(m.textInput.Value())
				m.projectPath = value
				if value == "" {
					m.logs = append(m.logs, m.Info("Project path skipped"))
				} else {
					m.logs = append(m.logs, m.Info("Project path set to "+value))
				}
				m.state = stateAskAnalyze
				m.yesSelected = true
				m.syncViewport()
				return m, nil
			}

			var cmd tea.Cmd
			m.textInput, cmd = m.textInput.Update(msg)
			return m, cmd

		case stateAskAnalyze:
			if next, handled := handleYesNo(msg, m.yesSelected); handled {
				m.yesSelected = next
				return m, nil
			}
			if msg.Type == tea.KeyEnter {
				m.runAnalysis = m.yesSelected
				if m.runAnalysis {
					m.logs = append(m.logs, m.Info("Project analysis enabled"))
				} else {
					m.logs = append(m.logs, m.Info("Project analysis skipped"))
				}
				m.state = stateAskRules
				m.yesSelected = true
				m.syncViewport()
				return m, nil
			}

		case stateAskRules:
			if next, handled := handleYesNo(msg, m.yesSelected); handled {
				m.yesSelected = next
				return m, nil
			}
			if msg.Type == tea.KeyEnter {
				m.writeRules = m.yesSelected
				if !m.writeRules {
					m.logs = append(m.logs, m.Info("Rule generation skipped"))
					m.state = stateAskRestart
					m.yesSelected = true
					m.syncViewport()
					return m, nil
				}

				if !m.runAnalysis || strings.TrimSpace(m.projectPath) == "" {
					m.logs = append(m.logs, m.Info("Skipping project summary and generating auditd rules directly"))
					m.state = stateGeneratingRules
					m.syncViewport()
					return m, generateRulesCmd(m.tools, "", m.backendURL)
				}

				m.logs = append(m.logs, m.Info("Requesting backend project summary"))
				m.state = stateGeneratingSummary
				m.syncViewport()
				return m, generateSummaryCmd(m.projectPath, m.backendURL)
			}

		case stateAskRestart:
			if next, handled := handleYesNo(msg, m.yesSelected); handled {
				m.yesSelected = next
				return m, nil
			}
			if msg.Type == tea.KeyEnter {
				if !m.yesSelected {
					m.logs = append(m.logs, m.Info("Service restart skipped"))
					m.state = stateDone
					m.syncViewport()
					return m, nil
				}

				m.logs = append(m.logs, m.Info("Restarting auditd service"))
				m.state = stateRestarting
				m.syncViewport()
				return m, restartServicesCmd(m.tools)
			}
		}
	}

	if m.state == stateInstalling || m.state == stateGeneratingSummary || m.state == stateGeneratingRules || m.state == stateRestarting {
		var cmd tea.Cmd
		m.spinner, cmd = m.spinner.Update(msg)
		if cmd != nil {
			cmds = append(cmds, cmd)
		}
	}

	return m, tea.Batch(cmds...)
}

func (m model) View() string {
	switch m.state {
	case stateIntro:
		return m.viewIntro()
	case stateInstalling:
		return m.viewInstall()
	case stateProjectPath:
		return m.viewPrompt("Project Context", "Enter a project path to improve backend-generated auditd rules, or press enter to skip.", m.textInput.View())
	case stateAskAnalyze:
		return m.viewPrompt("Project Analysis", "Should the backend analyze the project before generating auditd rules?", renderYesNo(m.yesSelected))
	case stateAskRules:
		return m.viewPrompt("Rule Generation", "Should the backend generate auditd rules after setup?", renderYesNo(m.yesSelected))
	case stateGeneratingSummary:
		return m.viewWorking("Generating project summary")
	case stateGeneratingRules:
		return m.viewWorking("Generating auditd rules")
	case stateAskRestart:
		return m.viewPrompt("Restart auditd", "Restart auditd now to pick up the generated rules?", renderYesNo(m.yesSelected))
	case stateRestarting:
		return m.viewWorking("Restarting auditd")
	case stateDone:
		return m.viewDone()
	default:
		return "unknown state"
	}
}

func (m model) viewIntro() string {
	body := strings.Join([]string{
		heroStyle.Render("BANNIN"),
		"",
		titleStyle.Render("auditd-only setup"),
		" ",
		subtleStyle.Render("This guided flow installs and configures auditd, tails the local audit log, and can optionally ask the backend to generate auditd rules."),
		"",
		accentStyle.Render("What this setup does"),
		"  • installs auditd if it is missing",
		"  • writes `/etc/audit/rules.d/bannin.rules`",
		"  • can request rule generation from `BANNIN_BACKEND_URL`",
		"  • tails `/var/log/audit/audit.log` and forwards events",
		"  • can restart auditd after rule changes",
		"",
		"",
		subtleStyle.Render("Press enter to start."),
		"",
		"  " + m.help.View(m.keys),
	}, "\n")

	return docStyle.Render(body)
}

func (m model) viewInstall() string {
	pct := 0.0
	if m.totalSteps > 0 {
		pct = float64(m.completedSteps) / float64(m.totalSteps)
	}

	body := strings.Join([]string{
		titleStyle.Render("Installing auditd"),
		fmt.Sprintf("%s  %s", m.spinner.View(), subtleStyle.Render("streaming installer output")),
		"",
		m.progress.ViewAs(pct),
		fmt.Sprintf("%d/%d steps complete", m.completedSteps, m.totalSteps),
		"",
		m.viewport.View(),
	}, "\n")

	return docStyle.Render(body)
}

func (m model) viewPrompt(title, question, control string) string {
	body := strings.Join([]string{
		titleStyle.Render(title),
		subtleStyle.Render(question),
		"",
		control,
		"",
		m.viewport.View(),
	}, "\n")

	return docStyle.Render(body)
}

func (m model) viewWorking(label string) string {
	body := strings.Join([]string{
		titleStyle.Render(label),
		fmt.Sprintf("%s  %s", m.spinner.View(), subtleStyle.Render("please wait")),
		"",
		m.viewport.View(),
	}, "\n")

	return docStyle.Render(body)
}

func (m model) viewDone() string {
	header := successStyle.Render("Setup complete")
	if m.hadError {
		header = errorStyle.Render("Setup failed")
	}

	body := strings.Join([]string{
		header,
		"",
		m.viewport.View(),
		"",
		subtleStyle.Render("Press q to exit."),
	}, "\n")

	return docStyle.Render(body)
}

func (m model) startInstall() (tea.Model, tea.Cmd) {
	m.state = stateInstalling
	m.totalSteps = len(m.tools) * 3
	m.completedSteps = 0
	m.logs = []string{
		m.Info("Starting auditd setup"),
		m.Info("Installer output will stream below"),
	}
	m.syncViewport()

	eventCh := make(chan tea.Msg, 128)
	m.installEvents = eventCh
	go app.RunInstallPipeline(m.tools, m, eventCh)
	return m, app.WaitForInstallMsg(eventCh)
}

func (m *model) syncLayout() {
	width := 72
	if m.windowWidth > 0 {
		width = maxInt(48, minInt(m.windowWidth-8, 96))
	}
	height := 14
	if m.windowHeight > 0 {
		height = maxInt(8, minInt(m.windowHeight-14, 18))
	}

	m.progress.Width = width - 6
	m.textInput.Width = width - 6
	m.viewport.Width = width - 6
	m.viewport.Height = height
	m.syncViewport()
}

func (m *model) syncViewport() {
	m.viewport.SetContent(strings.Join(m.logs, "\n"))
	m.viewport.GotoBottom()
}

func (m model) Phase(tool, phase, detail string) string {
	return fmt.Sprintf("%s  %s  %s  %s", timestamp(), phaseStyle.Render(strings.ToUpper(phase)), accentStyle.Render(tool), detail)
}

func (m model) Success(msg string) string {
	return fmt.Sprintf("%s  %s", timestamp(), successStyle.Render("OK  "+msg))
}

func (m model) Error(msg string) string {
	return fmt.Sprintf("%s  %s", timestamp(), errorStyle.Render("ERR "+msg))
}

func (m model) Command(msg string) string {
	return fmt.Sprintf("%s  %s", timestamp(), commandStyle.Render(msg))
}

func (m model) Info(msg string) string {
	return fmt.Sprintf("%s  %s", timestamp(), subtleStyle.Render(msg))
}

func generateSummaryCmd(projectPath, backendURL string) tea.Cmd {
	return func() tea.Msg {
		if strings.TrimSpace(backendURL) == "" {
			return summaryDoneMsg{err: fmt.Errorf("BANNIN_BACKEND_URL is not configured")}
		}
		text, err := dispatcher.GenerateSummary(projectPath, backendURL)
		return summaryDoneMsg{text: text, err: err}
	}
}

func generateRulesCmd(tools []installers.SecurityTools, contents, backendURL string) tea.Cmd {
	return func() tea.Msg {
		if strings.TrimSpace(backendURL) == "" {
			return rulesDoneMsg{err: fmt.Errorf("BANNIN_BACKEND_URL is not configured")}
		}
		for _, tool := range tools {
			if err := dispatcher.SendRule(strings.ToLower(tool.Name()), contents, backendURL); err != nil {
				return rulesDoneMsg{err: fmt.Errorf("[%s] rule generation failed: %w", tool.Name(), err)}
			}
		}
		return rulesDoneMsg{}
	}
}

func restartServicesCmd(tools []installers.SecurityTools) tea.Cmd {
	return func() tea.Msg {
		for _, tool := range tools {
			if err := tool.Start(); err != nil {
				return restartDoneMsg{err: fmt.Errorf("[%s] restart failed: %w", tool.Name(), err)}
			}
		}
		return restartDoneMsg{}
	}
}

func handleYesNo(msg tea.KeyMsg, current bool) (bool, bool) {
	switch msg.String() {
	case "left", "h", "y", "Y":
		return true, true
	case "right", "l", "n", "N":
		return false, true
	default:
		return current, false
	}
}

func renderYesNo(yes bool) string {
	selected := lipgloss.NewStyle().Foreground(colorPanel).Background(colorAccent).Bold(true).Padding(0, 1)
	idle := subtleStyle
	if yes {
		return selected.Render("YES") + "   " + idle.Render("NO")
	}
	return idle.Render("YES") + "   " + selected.Render("NO")
}

func timestamp() string {
	return subtleStyle.Render(time.Now().Format("15:04:05"))
}

func truncateLine(line string, limit int) string {
	if len(line) <= limit {
		return line
	}
	return line[:limit-3] + "..."
}

func minInt(a, b int) int {
	if a < b {
		return a
	}
	return b
}

func maxInt(a, b int) int {
	if a > b {
		return a
	}
	return b
}
