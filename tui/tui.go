package tui

import (
	"fmt"
	"os"
	"path/filepath"
	"slices"
	"strings"
	"time"

	"ra3d/torrent"

	"github.com/charmbracelet/bubbles/progress"
	"github.com/charmbracelet/bubbles/textinput"
	tea "github.com/charmbracelet/bubbletea"
	"github.com/charmbracelet/lipgloss"
)

var (
	accent = lipgloss.AdaptiveColor{Light: "#5A3FD9", Dark: "#9D8CFF"}
	green  = lipgloss.AdaptiveColor{Light: "#1A7F37", Dark: "#3FB950"}
	red    = lipgloss.AdaptiveColor{Light: "#CF222E", Dark: "#F85149"}

	logo   = lipgloss.NewStyle().Bold(true).Foreground(lipgloss.Color("#FFFFFF")).Background(accent).Padding(0, 1)
	title  = lipgloss.NewStyle().Bold(true)
	dim    = lipgloss.NewStyle().Faint(true)
	label  = lipgloss.NewStyle().Faint(true).Width(8)
	value  = lipgloss.NewStyle().Bold(true)
	have   = lipgloss.NewStyle().Foreground(accent)
	errSty = lipgloss.NewStyle().Foreground(red).Bold(true)
	okSty  = lipgloss.NewStyle().Foreground(green).Bold(true)
	box    = lipgloss.NewStyle().Border(lipgloss.RoundedBorder()).BorderForeground(accent).Padding(0, 2)
)

const (
	tickEvery   = 500 * time.Millisecond
	pieceRows   = 4
	maxWidth    = 80
	spinnerChar = "⣾⣽⣻⢿⡿⣟⣯⣷"
)

type tickMsg time.Time

func tick() tea.Cmd {
	return tea.Tick(tickEvery, func(t time.Time) tea.Msg { return tickMsg(t) })
}

type item struct {
	t     *torrent.Torrent
	stats torrent.Stats
	speed float64 // bytes/s, smoothed
	last  int
}

type model struct {
	items   []*item
	sel     int
	outDir  string
	input   textinput.Model
	adding  bool
	confirm bool   // waiting for y/n on delete
	note    string // last error or hint, shown above the help line
	bar     progress.Model
	width   int
	frame   int
	lastAt  time.Time
}

type deletedMsg struct {
	name string
	err  error
}

// Run opens the TUI with sources already added. More can be added from inside it.
func Run(sources []string, outDir string) error {
	in := textinput.New()
	in.Placeholder = "path/to/file.torrent or magnet:?xt=…"
	in.Prompt = "+ "
	m := &model{
		outDir: outDir,
		input:  in,
		bar:    progress.New(progress.WithGradient("#5A3FD9", "#3FB950")),
		width:  maxWidth,
		lastAt: time.Now(),
	}
	for _, src := range sources {
		m.add(src)
	}
	if len(m.items) == 0 {
		m.startAdding()
	}
	_, err := tea.NewProgram(m).Run()
	for _, it := range m.items {
		it.t.Stop()
	}
	return err
}

func (m *model) add(src string) {
	t := &torrent.Torrent{}
	go t.Run(src, m.outDir)
	m.items = append(m.items, &item{t: t})
	m.sel = len(m.items) - 1
}

func (m *model) startAdding() {
	m.adding = true
	m.input.Reset()
	m.input.Focus()
}

// cleanSource undoes what pasting or drag-and-drop does to a path: quotes,
// escaped spaces and a leading ~.
func cleanSource(s string) string {
	s = strings.Trim(strings.TrimSpace(s), `"'`)
	if strings.HasPrefix(s, "magnet:") {
		return s
	}
	s = strings.ReplaceAll(s, `\ `, " ")
	if rest, ok := strings.CutPrefix(s, "~/"); ok {
		if home, err := os.UserHomeDir(); err == nil {
			s = filepath.Join(home, rest)
		}
	}
	return s
}

func (m *model) Init() tea.Cmd { return tick() }

func (m *model) Update(msg tea.Msg) (tea.Model, tea.Cmd) {
	switch msg := msg.(type) {
	case tea.KeyMsg:
		if msg.String() == "ctrl+c" {
			return m, tea.Quit
		}
		if m.adding {
			return m, m.updateInput(msg)
		}
		if m.confirm {
			m.confirm, m.note = false, ""
			if msg.String() == "y" {
				it := m.remove()
				return m, func() tea.Msg { return deletedMsg{it.stats.Name, it.t.Delete()} }
			}
			return m, nil
		}
		m.note = ""
		if msg.Paste { // pasting a link opens the input with it
			m.startAdding()
			return m, m.updateInput(msg)
		}
		switch msg.String() {
		case "q", "esc":
			return m, tea.Quit
		case "a", "n":
			m.startAdding()
			return m, textinput.Blink
		case "up", "k":
			m.sel = max(m.sel-1, 0)
		case "down", "j":
			m.sel = min(m.sel+1, len(m.items)-1)
		}
		if len(m.items) == 0 {
			return m, nil
		}
		switch msg.String() {
		case "p", " ":
			m.items[m.sel].t.TogglePause()
		case "x":
			m.remove().t.Stop()
		case "d":
			m.confirm = true
			m.note = "Delete the files of " + nameOf(m.items[m.sel].stats) + "? y/n"
		}
	case deletedMsg:
		if msg.err != nil {
			m.note = "deleting " + msg.name + ": " + msg.err.Error()
		}
	case tea.WindowSizeMsg:
		m.width = min(msg.Width, maxWidth)
	case tickMsg:
		now := time.Time(msg)
		dt := now.Sub(m.lastAt).Seconds()
		m.frame++
		for _, it := range m.items {
			s := it.t.Stats()
			if dt > 0 {
				inst := float64(s.Fetched-it.last) / dt
				it.speed = 0.8*it.speed + 0.2*inst
			}
			it.stats, it.last = s, s.Fetched
		}
		m.lastAt = now
		return m, tick()
	}
	return m, nil
}

func (m *model) updateInput(msg tea.KeyMsg) tea.Cmd {
	switch msg.String() {
	case "esc":
		m.adding = false
	case "enter":
		m.adding = false
		if src := cleanSource(m.input.Value()); src != "" {
			m.add(src)
		}
	default:
		var cmd tea.Cmd
		m.input, cmd = m.input.Update(msg)
		return cmd
	}
	return nil
}

// remove takes the selected torrent out of the list and returns it.
func (m *model) remove() *item {
	it := m.items[m.sel]
	m.items = slices.Delete(m.items, m.sel, m.sel+1)
	m.sel = min(m.sel, max(len(m.items)-1, 0))
	return it
}

func (m *model) View() string {
	inner := m.width - 6 // border + padding
	if inner < 20 {
		inner = 20
	}
	var body string
	switch {
	case len(m.items) > 0:
		body = m.list(inner) + m.detail(m.items[m.sel], inner)
	case !m.adding:
		body = logo.Render("ra3d") + " " + dim.Render("no torrents, press a to add one")
	}
	if m.adding {
		m.input.Width = inner - 3
		if body != "" {
			body += "\n\n"
		}
		body += m.input.View()
	}

	help := "  enter add · esc cancel"
	if !m.adding {
		help = "  a add · p pause/resume · x remove · d delete files · q quit"
		if len(m.items) > 1 {
			help = "  ↑/↓ select · " + help[2:]
		}
	}
	out := box.Width(m.width-2).Render(body) + "\n"
	if m.note != "" {
		out += "  " + errSty.Render(m.note) + "\n"
	}
	return out + dim.Render(help) + "\n"
}

// list draws one line per torrent; it is skipped when there is only one.
func (m *model) list(inner int) string {
	if len(m.items) < 2 {
		return ""
	}
	var b strings.Builder
	for i, it := range m.items {
		s := it.stats
		cursor := "  "
		if i == m.sel {
			cursor = have.Render("▸ ")
		}
		pct := 0
		if s.Length > 0 {
			pct = s.Downloaded * 100 / s.Length
		}
		state := bytes(int(it.speed)) + "/s"
		switch {
		case s.Err != nil:
			state = errSty.Render("error")
		case s.Done:
			state = okSty.Render("done")
		case s.Paused:
			state = dim.Render("paused")
		}
		right := fmt.Sprintf(" %3d%% %s", pct, state)
		b.WriteString(cursor + truncate(nameOf(s), inner-2-lipgloss.Width(right)) + right + "\n")
	}
	return b.String() + "\n"
}

func nameOf(s torrent.Stats) string {
	if s.Name == "" {
		return "resolving…"
	}
	return s.Name
}

// detail draws the full progress view for one torrent.
func (m *model) detail(it *item, inner int) string {
	s := it.stats
	var b strings.Builder
	b.WriteString(logo.Render("ra3d") + " " + title.Render(truncate(nameOf(s), inner-7)) + "\n\n")

	pct := 0.0
	if s.Length > 0 {
		pct = float64(s.Downloaded) / float64(s.Length)
	}
	m.bar.Width = inner
	b.WriteString(m.bar.ViewAs(pct) + "\n\n")

	eta := "–"
	if it.speed > 1 && s.Length > s.Downloaded {
		eta = (time.Duration(float64(s.Length-s.Downloaded)/it.speed) * time.Second).Round(time.Second).String()
	}
	if s.Done {
		eta = "0s"
	}
	b.WriteString(row("Size", fmt.Sprintf("%s / %s", bytes(s.Downloaded), bytes(s.Length))) + "\n")
	b.WriteString(row("Speed", bytes(int(it.speed))+"/s") + dim.Render("   ETA ") + value.Render(eta) + "\n")
	b.WriteString(row("Peers", fmt.Sprintf("%d", s.Peers)) + dim.Render(fmt.Sprintf(" connected of %d known", s.KnownPeers)) + "\n")
	b.WriteString(row("Pieces", fmt.Sprintf("%d / %d", s.DonePieces, s.NumPieces)) + "\n\n")

	if s.NumPieces > 0 {
		b.WriteString(pieceMap(s, inner) + "\n\n")
	}

	switch {
	case s.Err != nil:
		b.WriteString(errSty.Render("✗ " + s.Err.Error()))
	case s.Done:
		b.WriteString(okSty.Render("✓ Download complete"))
	case s.Paused:
		b.WriteString(dim.Render("❚❚ Paused"))
	default:
		spin := []rune(spinnerChar)
		b.WriteString(have.Render(string(spin[m.frame%len(spin)])) + " " + s.Status)
	}

	return b.String()
}

// pieceMap draws a grid where each cell shades by how many of its pieces are done.
func pieceMap(s torrent.Stats, width int) string {
	cells := min(width*pieceRows, s.NumPieces)
	shades := []string{"░", "▒", "▓", "█"}
	var b strings.Builder
	for c := 0; c < cells; c++ {
		from, to := c*s.NumPieces/cells, (c+1)*s.NumPieces/cells
		done := 0
		for i := from; i < to; i++ {
			if s.Pieces.HasPiece(i) {
				done++
			}
		}
		switch {
		case done == 0:
			b.WriteString(dim.Render("·"))
		case done == to-from:
			b.WriteString(have.Render("█"))
		default:
			b.WriteString(have.Render(shades[done*3/(to-from)]))
		}
		if (c+1)%width == 0 && c+1 < cells {
			b.WriteString("\n")
		}
	}
	return b.String()
}

func row(name, v string) string {
	return label.Render(name) + value.Render(v)
}

func truncate(s string, n int) string {
	r := []rune(s)
	if len(r) <= n || n < 2 {
		return s
	}
	return string(r[:n-1]) + "…"
}

func bytes(n int) string {
	const unit = 1024
	if n < unit {
		return fmt.Sprintf("%d B", n)
	}
	div, exp := unit, 0
	for x := n / unit; x >= unit; x /= unit {
		div *= unit
		exp++
	}
	return fmt.Sprintf("%.1f %ciB", float64(n)/float64(div), "KMGTPE"[exp])
}
