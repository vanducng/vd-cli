// Package tui is the terminal frontend under the internal/ui umbrella. Like
// internal/ui/web, it binds the transport-agnostic internal/inventory service —
// no HTTP, no SPA.
package tui

import (
	"github.com/charmbracelet/bubbles/table"
	"github.com/charmbracelet/bubbles/viewport"
	tea "github.com/charmbracelet/bubbletea"

	"github.com/vanducng/vd-cli/v2/internal/inventory"
)

type tab int

const (
	tabInventory tab = iota
	tabHooks
	tabDoctor
	numTabs
)

func (t tab) String() string {
	switch t {
	case tabInventory:
		return "Inventory"
	case tabHooks:
		return "Hooks"
	case tabDoctor:
		return "Doctor"
	default:
		return ""
	}
}

type assetRef struct {
	typ  inventory.AssetType
	name string
}

type model struct {
	svc           *inventory.Service
	inv           *inventory.Inventory
	tab           tab
	tables        [numTabs]table.Model
	invRefs       []assetRef // parallel to the inventory table rows
	plat          string     // active agent filter: filterAll | a platform
	detail        *inventory.SkillDetail
	vp            viewport.Model
	width, height int
	err           error
}

const filterAll = "all"

var platCycle = []string{filterAll, inventory.PlatformClaude, inventory.PlatformCodex, inventory.PlatformCursor}

func platShort(p string) string {
	switch p {
	case inventory.PlatformClaude:
		return "Claude"
	case inventory.PlatformCodex:
		return "Codex"
	case inventory.PlatformCursor:
		return "Cursor"
	case "":
		return "managed"
	default:
		return p
	}
}

// Run loads the inventory and starts the interactive program.
func Run(svc *inventory.Service) error {
	m, err := newModel(svc)
	if err != nil {
		return err
	}
	_, err = tea.NewProgram(m, tea.WithAltScreen()).Run()
	return err
}

func newModel(svc *inventory.Service) (*model, error) {
	inv, err := svc.Inventory()
	if err != nil {
		return nil, err
	}
	hooks, err := svc.Hooks()
	if err != nil {
		return nil, err
	}
	doc, err := svc.Doctor()
	if err != nil {
		return nil, err
	}

	m := &model{svc: svc, inv: inv, plat: filterAll}
	m.tables[tabInventory], m.invRefs = inventoryTable(inv, m.plat)
	m.tables[tabHooks] = hooksTable(hooks)
	m.tables[tabDoctor] = doctorTable(doc)
	m.tables[tabInventory].Focus()
	return m, nil
}

func (m *model) Init() tea.Cmd { return nil }

// inventoryTable builds the inventory rows, filtered to plat (filterAll shows
// managed + every agent; a specific platform shows only that agent's discovered
// assets — managed entries are repo-source and excluded).
func inventoryTable(inv *inventory.Inventory, plat string) (table.Model, []assetRef) {
	cols := []table.Column{
		{Title: "TYPE", Width: 7},
		{Title: "NAME", Width: 26},
		{Title: "AGENT", Width: 9},
		{Title: "DRIFT", Width: 9},
		{Title: "DESCRIPTION", Width: 56},
	}
	var rows []table.Row
	var refs []assetRef
	if plat == filterAll {
		for _, a := range inv.Managed {
			rows = append(rows, table.Row{string(a.Type), a.Name, "managed", a.Drift, oneLine(a.Description)})
			refs = append(refs, assetRef{a.Type, a.Name})
		}
	}
	for _, a := range inv.Discovered {
		if plat != filterAll && a.Platform != plat {
			continue
		}
		rows = append(rows, table.Row{string(a.Type), a.Name, platShort(a.Platform), "", oneLine(a.Description)})
		refs = append(refs, assetRef{a.Type, a.Name})
	}
	return newTable(cols, rows), refs
}

func hooksTable(hooks []inventory.Asset) table.Model {
	cols := []table.Column{
		{Title: "EVENT", Width: 42},
		{Title: "VD", Width: 4},
		{Title: "COMMAND", Width: 60},
	}
	var rows []table.Row
	for _, h := range hooks {
		vd := ""
		if managed, _ := h.Frontmatter["managedByVd"].(bool); managed {
			vd = "✓"
		}
		rows = append(rows, table.Row{h.Name, vd, oneLine(h.Description)})
	}
	return newTable(cols, rows)
}

func doctorTable(rep *inventory.DoctorReport) table.Model {
	cols := []table.Column{
		{Title: "SKILL", Width: 30},
		{Title: "STATUS", Width: 12},
		{Title: "DETAIL", Width: 50},
	}
	var rows []table.Row
	for _, e := range rep.Entries {
		rows = append(rows, table.Row{e.Skill, e.Status, e.Detail})
	}
	return newTable(cols, rows)
}

func newTable(cols []table.Column, rows []table.Row) table.Model {
	t := table.New(table.WithColumns(cols), table.WithRows(rows), table.WithHeight(20))
	t.SetStyles(tableStyles())
	return t
}

// cyclePlat advances the agent filter and rebuilds the inventory table.
func (m *model) cyclePlat() {
	i := 0
	for j, p := range platCycle {
		if p == m.plat {
			i = j
			break
		}
	}
	m.plat = platCycle[(i+1)%len(platCycle)]
	focused := m.tables[tabInventory].Focused()
	m.tables[tabInventory], m.invRefs = inventoryTable(m.inv, m.plat)
	if focused {
		m.tables[tabInventory].Focus()
	}
	m.resize()
}

func (m *model) resize() {
	h := m.contentHeight()
	for i := range m.tables {
		m.tables[i].SetHeight(h)
	}
	m.vp.Width = m.contentWidth()
	m.vp.Height = h
}

func (m *model) contentWidth() int {
	if m.width > 0 {
		return m.width
	}
	return 80
}

func (m *model) contentHeight() int {
	h := m.height - 4 // tabs row + footer + padding
	if h < 3 {
		return 3
	}
	return h
}
