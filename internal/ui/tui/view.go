package tui

import (
	"fmt"
	"sort"
	"strings"

	"github.com/charmbracelet/bubbles/table"
	"github.com/charmbracelet/lipgloss"

	"github.com/vanducng/vd-cli/v2/internal/inventory"
)

var (
	activeTab = lipgloss.NewStyle().Bold(true).Foreground(lipgloss.Color("39")).
			Padding(0, 2).Border(lipgloss.RoundedBorder(), false, false, true, false).BorderForeground(lipgloss.Color("39"))
	inactiveTab = lipgloss.NewStyle().Foreground(lipgloss.Color("245")).Padding(0, 2)
	titleStyle  = lipgloss.NewStyle().Bold(true).Foreground(lipgloss.Color("39"))
	mutedStyle  = lipgloss.NewStyle().Foreground(lipgloss.Color("245"))
	keyStyle    = lipgloss.NewStyle().Foreground(lipgloss.Color("245"))
	errStyle    = lipgloss.NewStyle().Foreground(lipgloss.Color("203"))
	footerStyle = lipgloss.NewStyle().Foreground(lipgloss.Color("240")).MarginTop(1)
)

func tableStyles() table.Styles {
	s := table.DefaultStyles()
	s.Header = s.Header.Bold(true).Foreground(lipgloss.Color("245")).
		BorderStyle(lipgloss.NormalBorder()).BorderBottom(true).BorderForeground(lipgloss.Color("240"))
	s.Selected = s.Selected.Foreground(lipgloss.Color("231")).Background(lipgloss.Color("24")).Bold(false)
	return s
}

func driftColor(drift string) lipgloss.Style {
	switch drift {
	case "none":
		return lipgloss.NewStyle().Foreground(lipgloss.Color("42"))
	case "local":
		return lipgloss.NewStyle().Foreground(lipgloss.Color("214"))
	case "missing":
		return lipgloss.NewStyle().Foreground(lipgloss.Color("203"))
	default:
		return mutedStyle
	}
}

func (m *model) View() string {
	if m.err != nil {
		return errStyle.Render("error: "+m.err.Error()) + "\n\npress q to quit\n"
	}
	body := m.tables[m.tab].View()
	if m.detail != nil {
		body = m.vp.View()
	}
	return lipgloss.JoinVertical(lipgloss.Left, m.renderTabs(), body, m.renderFooter())
}

func (m *model) renderTabs() string {
	var parts []string
	for t := tabInventory; t < numTabs; t++ {
		if t == m.tab {
			parts = append(parts, activeTab.Render(t.String()))
		} else {
			parts = append(parts, inactiveTab.Render(t.String()))
		}
	}
	return lipgloss.NewStyle().MarginBottom(1).Render(lipgloss.JoinHorizontal(lipgloss.Bottom, parts...))
}

func (m *model) renderFooter() string {
	hint := "tab: switch · ↑/↓: move · enter: open skill · a: agent · q: quit"
	if m.detail != nil {
		hint = "↑/↓: scroll · esc: back · q: back · ctrl+c: quit"
	} else if m.tab == tabInventory && m.plat != filterAll {
		hint = "agent: " + platShort(m.plat) + " (a to cycle) · " + hint
	}
	return footerStyle.Render(hint)
}

func renderDetail(d *inventory.SkillDetail) string {
	var b strings.Builder
	b.WriteString(titleStyle.Render(d.Name))
	if d.Drift != "" {
		b.WriteString("  " + driftColor(d.Drift).Render("["+d.Drift+"]"))
	}
	b.WriteString("\n" + mutedStyle.Render(d.Path) + "\n\n")
	for _, k := range sortedKeys(d.Frontmatter) {
		b.WriteString(keyStyle.Render(k+": ") + fmt.Sprint(d.Frontmatter[k]) + "\n")
	}
	b.WriteString("\n" + mutedStyle.Render(strings.Repeat("─", 50)) + "\n\n")
	b.WriteString(d.Body)
	return b.String()
}

func sortedKeys(m map[string]any) []string {
	out := make([]string, 0, len(m))
	for k := range m {
		out = append(out, k)
	}
	sort.Strings(out)
	return out
}

// oneLine collapses runs of whitespace (incl. newlines) into single spaces.
func oneLine(s string) string {
	return strings.Join(strings.Fields(s), " ")
}
