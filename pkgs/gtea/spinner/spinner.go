package spinner

import (
	"fmt"

	"github.com/charmbracelet/bubbles/spinner"
	tea "github.com/charmbracelet/bubbletea"
	"github.com/charmbracelet/lipgloss"
)

type errMsg error

type Spinner struct {
	spinner  spinner.Model
	quitting bool
	err      error
	fileName string
	title    string
}

func NewSpinner() *Spinner {
	s := spinner.New()
	s.Spinner = spinner.Line
	s.Style = lipgloss.NewStyle().Foreground(lipgloss.Color("205"))
	return &Spinner{spinner: s}
}

func (m *Spinner) SetFileName(fName string) {
	m.fileName = fName
}

func (m *Spinner) SetTitle(title string) {
	m.title = title
}

func (m *Spinner) Quit() {
	m.quitting = true
}

func (m *Spinner) Init() tea.Cmd {
	return m.spinner.Tick
}

func (m *Spinner) Update(msg tea.Msg) (tea.Model, tea.Cmd) {
	switch msg := msg.(type) {
	case tea.KeyMsg:
		switch msg.String() {
		case "q", "esc":
			m.quitting = true
			return m, tea.Quit
		default:
			return m, nil
		}
	case errMsg:
		m.err = msg
		return m, nil
	default:
		if m.quitting {
			return m, tea.Quit
		}
		var cmd tea.Cmd
		m.spinner, cmd = m.spinner.Update(msg)
		return m, cmd
	}
}

func (m *Spinner) View() string {
	if m.err != nil {
		return m.err.Error()
	}
	var str string
	if m.title == "" && m.fileName != "" {
		str = fmt.Sprintf("\n\n %s Downloading %s...\n\n", m.spinner.View(), m.fileName)
	} else {
		str = fmt.Sprintf("\n\n %s %s...\n\n", m.spinner.View(), m.title)
	}

	if m.quitting {
		return str + "\n"
	}
	return str
}

func (m *Spinner) Run() {
	p := tea.NewProgram(m)
	p.Run()
}
