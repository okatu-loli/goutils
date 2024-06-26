package gtable

import (
	"github.com/atotto/clipboard"
	tea "github.com/charmbracelet/bubbletea"
	"github.com/charmbracelet/lipgloss"
)

var baseStyle = lipgloss.NewStyle().
	BorderStyle(lipgloss.HiddenBorder()).
	BorderForeground(lipgloss.Color("240"))

type TableModel struct {
	table  *Model
	toCopy bool
}

func (that *TableModel) Init() tea.Cmd { return nil }

func (that *TableModel) Update(msg tea.Msg) (tea.Model, tea.Cmd) {
	var cmd tea.Cmd
	switch msg := msg.(type) {
	case tea.KeyMsg:
		switch msg.String() {
		case "esc", "tab":
			if that.table.Focused() {
				that.table.Blur()
			} else {
				that.table.Focus()
			}
		case "q", "enter", "ctrl+c":
			if that.toCopy {
				row := that.table.SelectedRow()
				if len(row) > 0 {
					clipboard.WriteAll(row[0])
					// if err := clipboard.WriteAll(row[0]); err == nil {
					// 	gprint.PrintInfo("%s is copied to clipboard.", row[0])
					// }
				}
			}
			return that, tea.Quit
			// case "enter":
			// 	return that, tea.Batch(
			// 		tea.Printf("Let's go to %s!", that.table.SelectedRow()[1]),
			// 	)
		}
	}
	var tModel Model
	tModel, cmd = that.table.Update(msg)
	that.table = &tModel
	return that, cmd
}

func (that *TableModel) View() string {
	helpStyle := lipgloss.NewStyle().Foreground(lipgloss.Color("#FFA500"))
	helpStr := `Press "↑/k" to move up, "↓/j" to move down, "q" to quit.`
	return baseStyle.Render(that.table.View()) + "\n" + helpStyle.Render(helpStr) + "\n"
}

func (that *TableModel) SetCopyToClipBoard(toCopy bool) {
	that.toCopy = toCopy
}
