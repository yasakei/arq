package tui

import (
	"github.com/yasakei/arq/internal/project"
	"fmt"
	"strings"

	"github.com/charmbracelet/bubbletea"
	"github.com/charmbracelet/lipgloss"
)

type model struct {
	dir      string
	options  []string
	selected int
	width    int
	height   int
	quitting bool
}

var (
	accent = lipgloss.NewStyle().Foreground(lipgloss.Color("205"))
	muted  = lipgloss.NewStyle().Foreground(lipgloss.Color("241"))
	active = lipgloss.NewStyle().Foreground(lipgloss.Color("230")).Background(lipgloss.Color("63")).Padding(0, 1)
	panel  = lipgloss.NewStyle().Border(lipgloss.RoundedBorder()).BorderForeground(lipgloss.Color("238")).Padding(1, 2)
)

func Run(dir string) (string, error) {
	options := []string{"empty"}
	seen := map[string]bool{"empty": true}
	detected := project.Detect(dir)
	for _, name := range detected {
		if !seen[name] {
			options = append(options, name)
			seen[name] = true
		}
	}
	for _, name := range project.Templates() {
		if !seen[name] {
			options = append(options, name)
			seen[name] = true
		}
	}
	selected := 0
	if suggested := project.SuggestTemplate(detected); suggested != "" {
		for i, name := range options {
			if name == suggested {
				selected = i
				break
			}
		}
	}
	p, err := tea.NewProgram(model{dir: dir, options: options, selected: selected, width: 88, height: 28}).Run()
	if err != nil {
		return "", err
	}
	result := p.(model)
	if result.quitting {
		return "", nil
	}
	return result.options[result.selected], nil
}

func (m model) Init() tea.Cmd { return nil }

func (m model) Update(msg tea.Msg) (tea.Model, tea.Cmd) {
	switch msg := msg.(type) {
	case tea.WindowSizeMsg:
		m.width, m.height = msg.Width, msg.Height
	case tea.KeyMsg:
		switch msg.String() {
		case "q", "esc":
			m.quitting = true
			return m, tea.Quit
		case "up", "k":
			if m.selected > 0 {
				m.selected--
			}
		case "down", "j":
			if m.selected < len(m.options)-1 {
				m.selected++
			}
		case "enter":
			return m, tea.Quit
		}
	}
	return m, nil
}

func (m model) View() string {
	if m.quitting {
		return ""
	}
	name := m.options[m.selected]
	header := lipgloss.JoinVertical(
		lipgloss.Left,
		accent.Bold(true).Render("✦ welcome to arq"),
		muted.Render("set up automation for "+m.dir),
	)

	start, end := visibleRange(m.selected, len(m.options), 8)
	left := []string{"what would you like to create?", ""}
	if start > 0 {
		left = append(left, muted.Render("↑ more starters"))
	}
	for i, option := range m.options[start:end] {
		i += start
		label := optionLabel(option)
		if i == m.selected {
			left = append(left, active.Render("❯ "+label))
		} else {
			left = append(left, "  "+label)
		}
	}
	if end < len(m.options) {
		left = append(left, muted.Render("↓ more starters"))
	}
	left = append(left, "", muted.Render("↑↓ navigate   enter select   esc cancel"))

	preview := project.Template(name)
	if preview == "" {
		preview = "nothing yet"
	}
	preview = strings.TrimSpace(preview)
	right := []string{
		accent.Render("preview · build.arq"),
		muted.Render("this is what arq will create"),
		"",
		preview,
	}

	columns := lipgloss.JoinHorizontal(lipgloss.Top,
		panel.Width(columnWidth(m.width, 0.42)).Render(strings.Join(left, "\n")),
		panel.Width(columnWidth(m.width, 0.52)).Render(strings.Join(right, "\n")),
	)
	footer := muted.Render(fmt.Sprintf("template %d/%d · arq init", m.selected+1, len(m.options)))
	return lipgloss.JoinVertical(lipgloss.Left, header, "", columns, "", footer) + "\n"
}

func visibleRange(selected, total, size int) (int, int) {
	if total <= size {
		return 0, total
	}
	start := selected - size/2
	if start < 0 {
		start = 0
	}
	if start+size > total {
		start = total - size
	}
	return start, start + size
}

func optionLabel(name string) string {
	if name == "empty" {
		return "start empty"
	}
	return "use " + name + " starter"
}

func columnWidth(total int, ratio float64) int {
	width := int(float64(total) * ratio)
	if width < 30 {
		return 30
	}
	return width
}
