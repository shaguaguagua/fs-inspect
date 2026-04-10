// Package shell implements the `fs-inspect shell` subcommand: a
// bubbletea-powered interactive REPL over the cluster inventory.
//
// Design notes:
//
//   - The shell does not parse commands itself; it shells out to
//     cluster.Query and the caller-provided render functions. This keeps
//     the one-shot CLI and the interactive shell always in sync — they
//     literally call the same code.
//   - Command history is in-memory only. On-disk history is a later
//     feature, once I actually need it.
//   - Colors are forced on because the output gets written into a
//     bubbletea viewport, which is not the process stdout so the default
//     TTY check would otherwise turn them off.
package shell

import (
	"fmt"
	"strings"

	tea "github.com/charmbracelet/bubbletea"
	"github.com/charmbracelet/bubbles/textinput"
	"github.com/charmbracelet/bubbles/viewport"
	"github.com/charmbracelet/lipgloss"

	"github.com/shaguaguagua/fs-inspect/internal/cluster"
	"github.com/shaguaguagua/fs-inspect/internal/config"
	"github.com/shaguaguagua/fs-inspect/internal/display"
)

// Handlers wires the shell to the rendering functions defined in package
// main so the same code produces both one-shot CLI output and the
// interactive shell's scrollback.
type Handlers struct {
	Reg      func(ext string, results []cluster.Result) string
	Channels func(cfg *config.Config, results []cluster.Result) string
	Probe    func(addr, password, cmd string) (string, error)
}

// Run launches the interactive shell. It blocks until the user quits.
func Run(cfg *config.Config, h Handlers) error {
	display.ForceColor(true)

	ti := textinput.New()
	ti.Placeholder = "reg 1010  |  channels  |  probe <node> <cmd>  |  help  |  quit"
	ti.Focus()
	ti.Prompt = "› "
	ti.CharLimit = 256

	vp := viewport.New(80, 20)
	initial := banner(cfg)
	vp.SetContent(initial)

	m := model{
		cfg:        cfg,
		handlers:   h,
		input:      ti,
		output:     vp,
		scrollback: initial,
		histIdx:    0,
	}
	p := tea.NewProgram(m, tea.WithAltScreen())
	_, err := p.Run()
	return err
}

// model holds all shell state. Note the scrollback field is a plain
// string, not a strings.Builder: bubbletea passes the model by value
// through Update and View, and strings.Builder's copyCheck panics the
// moment a copied-by-value Builder is written to. A string field
// concatenates cheaply enough at shell-session scale and is copy-safe.
type model struct {
	cfg      *config.Config
	handlers Handlers

	input  textinput.Model
	output viewport.Model

	scrollback string
	history    []string
	histIdx    int
	width      int
	height     int
}

func (m model) Init() tea.Cmd { return textinput.Blink }

func (m model) Update(msg tea.Msg) (tea.Model, tea.Cmd) {
	switch msg := msg.(type) {
	case tea.WindowSizeMsg:
		m.width = msg.Width
		m.height = msg.Height
		m.output.Width = msg.Width - 2
		m.output.Height = msg.Height - 4
		m.output.SetContent(m.scrollback)
		m.output.GotoBottom()
		m.input.Width = msg.Width - 4
		return m, nil

	case tea.KeyMsg:
		switch msg.Type {
		case tea.KeyCtrlC, tea.KeyEsc:
			return m, tea.Quit
		case tea.KeyEnter:
			line := strings.TrimSpace(m.input.Value())
			m.input.Reset()
			if line == "" {
				return m, nil
			}
			m.history = append(m.history, line)
			m.histIdx = len(m.history)
			m.appendCommand(line)
			return m.dispatch(line)
		case tea.KeyUp:
			if len(m.history) > 0 && m.histIdx > 0 {
				m.histIdx--
				m.input.SetValue(m.history[m.histIdx])
				m.input.CursorEnd()
			}
			return m, nil
		case tea.KeyDown:
			if m.histIdx < len(m.history)-1 {
				m.histIdx++
				m.input.SetValue(m.history[m.histIdx])
				m.input.CursorEnd()
			} else {
				m.histIdx = len(m.history)
				m.input.SetValue("")
			}
			return m, nil
		}
	}

	var cmd tea.Cmd
	m.input, cmd = m.input.Update(msg)
	return m, cmd
}

func (m model) View() string {
	var b strings.Builder
	b.WriteString(m.output.View())
	b.WriteString("\n")
	b.WriteString(lipgloss.NewStyle().Foreground(lipgloss.Color("86")).Render("fs-inspect ") + m.input.View())
	return b.String()
}

// appendCommand writes the user's command line into the scrollback as a
// labelled prompt line so the transcript reads like a real shell.
func (m *model) appendCommand(line string) {
	m.scrollback += display.Gray("› ") + line + "\n"
}

// appendOutput writes a chunk of command output into the scrollback.
func (m *model) appendOutput(s string) {
	if !strings.HasSuffix(s, "\n") {
		s += "\n"
	}
	m.scrollback += s
	m.output.SetContent(m.scrollback)
	m.output.GotoBottom()
}

// dispatch parses one line and calls the matching handler. All output
// lands in the scrollback via appendOutput — no writes to process stdout.
func (m model) dispatch(line string) (tea.Model, tea.Cmd) {
	fields := strings.Fields(line)
	cmd := fields[0]
	args := fields[1:]

	switch cmd {
	case "quit", "exit", "q":
		return m, tea.Quit

	case "help", "?":
		m.appendOutput(helpText())
		return m, nil

	case "nodes":
		m.appendOutput(m.renderNodes())
		return m, nil

	case "reg":
		if len(args) < 1 {
			m.appendOutput(display.Red("usage: reg <ext>"))
			return m, nil
		}
		results := cluster.Query(m.cfg, "show registrations as json")
		m.appendOutput(m.handlers.Reg(args[0], results))
		return m, nil

	case "channels":
		results := cluster.Query(m.cfg, "show channels as json")
		m.appendOutput(m.handlers.Channels(m.cfg, results))
		return m, nil

	case "probe":
		if len(args) < 2 {
			m.appendOutput(display.Red("usage: probe <node-name> <api-command...>"))
			return m, nil
		}
		nodeName := args[0]
		apiCmd := strings.Join(args[1:], " ")
		node, ok := findNode(m.cfg, nodeName)
		if !ok {
			m.appendOutput(display.Red(fmt.Sprintf("no node named %q in config", nodeName)))
			return m, nil
		}
		body, err := m.handlers.Probe(node.Addr, node.Password, apiCmd)
		if err != nil {
			m.appendOutput(display.Red("error: ") + err.Error())
			return m, nil
		}
		m.appendOutput(display.PrettyJSON(body))
		return m, nil

	default:
		m.appendOutput(display.Red(fmt.Sprintf("unknown command: %q (try `help`)", cmd)))
		return m, nil
	}
}

func findNode(cfg *config.Config, name string) (config.Node, bool) {
	for _, n := range cfg.Nodes {
		if n.Name == name {
			return n, true
		}
	}
	return config.Node{}, false
}

func (m model) renderNodes() string {
	var b strings.Builder
	b.WriteString(display.Bold("NODES") + "\n")
	for _, n := range m.cfg.Nodes {
		fmt.Fprintf(&b, "  %s  %s\n", display.Cyan(n.Name), display.Gray(n.Addr))
	}
	return b.String()
}

func helpText() string {
	return display.Bold("commands") + `
  ` + display.Cyan("reg <ext>") + `              find which node an extension is registered on
  ` + display.Cyan("channels") + `               list active channels across every node
  ` + display.Cyan("probe <node> <cmd...>") + `  run any FS API command on one specific node
  ` + display.Cyan("nodes") + `                  show the loaded cluster inventory
  ` + display.Cyan("help") + `                   this text
  ` + display.Cyan("quit") + `                   exit (or Ctrl+C, Esc)

` + display.Gray("↑/↓ browse command history") + `
`
}

func banner(cfg *config.Config) string {
	title := display.Bold(display.Cyan("fs-inspect")) + display.Gray(" — interactive shell")
	sub := display.Gray(fmt.Sprintf("%d node(s) loaded. type `help` for commands, `quit` to exit.", len(cfg.Nodes)))
	return title + "\n" + sub + "\n\n"
}
