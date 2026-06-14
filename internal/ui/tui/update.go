package tui

import (
	"github.com/charmbracelet/bubbles/viewport"
	tea "github.com/charmbracelet/bubbletea"

	"github.com/vanducng/vd-cli/v2/internal/inventory"
)

func (m *model) Update(msg tea.Msg) (tea.Model, tea.Cmd) {
	switch msg := msg.(type) {
	case tea.WindowSizeMsg:
		m.width, m.height = msg.Width, msg.Height
		m.resize()
		return m, nil

	case tea.KeyMsg:
		if m.detail != nil {
			return m.updateDetail(msg)
		}
		return m.updateList(msg)
	}
	return m, nil
}

func (m *model) updateDetail(msg tea.KeyMsg) (tea.Model, tea.Cmd) {
	switch msg.String() {
	case "esc", "q", "backspace":
		m.detail = nil
		return m, nil
	case "ctrl+c":
		return m, tea.Quit
	}
	var cmd tea.Cmd
	m.vp, cmd = m.vp.Update(msg)
	return m, cmd
}

func (m *model) updateList(msg tea.KeyMsg) (tea.Model, tea.Cmd) {
	switch msg.String() {
	case "ctrl+c", "q":
		return m, tea.Quit
	case "tab", "right", "l":
		m.switchTab(1)
		return m, nil
	case "shift+tab", "left", "h":
		m.switchTab(-1)
		return m, nil
	case "a":
		if m.tab == tabInventory {
			m.cyclePlat()
			return m, nil
		}
	case "enter":
		if m.tab == tabInventory {
			m.openSelected()
			return m, nil
		}
	}
	var cmd tea.Cmd
	m.tables[m.tab], cmd = m.tables[m.tab].Update(msg)
	return m, cmd
}

func (m *model) switchTab(delta int) {
	m.tables[m.tab].Blur()
	m.tab = (m.tab + tab(delta) + numTabs) % numTabs
	m.tables[m.tab].Focus()
}

// openSelected loads detail for the highlighted inventory row if it is a skill.
func (m *model) openSelected() {
	idx := m.tables[tabInventory].Cursor()
	if idx < 0 || idx >= len(m.invRefs) {
		return
	}
	ref := m.invRefs[idx]
	if ref.typ != inventory.Skill {
		return
	}
	d, err := m.svc.SkillDetail(ref.name)
	if err != nil {
		m.err = err
		return
	}
	m.detail = d
	m.vp = viewport.New(m.contentWidth(), m.contentHeight())
	m.vp.SetContent(renderDetail(d))
}
