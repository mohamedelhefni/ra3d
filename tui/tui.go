package tui

import (
	"fmt"
	"strings"
	"time"

	"ra3d/torrent"

	"github.com/charmbracelet/bubbles/progress"
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

type model struct {
	t      *torrent.Torrent
	stats  torrent.Stats
	bar    progress.Model
	width  int
	frame  int
	speed  float64 // bytes/s, smoothed
	last   int
	lastAt time.Time
}

// Run shows the download's progress until it finishes or the user quits.
func Run(t *torrent.Torrent) error {
	m := model{
		t:      t,
		bar:    progress.New(progress.WithGradient("#5A3FD9", "#3FB950")),
		width:  maxWidth,
		lastAt: time.Now(),
	}
	_, err := tea.NewProgram(m).Run()
	return err
}

func (m model) Init() tea.Cmd { return tick() }

func (m model) Update(msg tea.Msg) (tea.Model, tea.Cmd) {
	switch msg := msg.(type) {
	case tea.KeyMsg:
		switch msg.String() {
		case "q", "ctrl+c", "esc":
			return m, tea.Quit
		}
	case tea.WindowSizeMsg:
		m.width = min(msg.Width, maxWidth)
	case tickMsg:
		now := time.Time(msg)
		m.stats = m.t.Stats()
		m.frame++
		if dt := now.Sub(m.lastAt).Seconds(); dt > 0 {
			inst := float64(m.stats.Fetched-m.last) / dt
			m.speed = 0.8*m.speed + 0.2*inst
		}
		m.last, m.lastAt = m.stats.Fetched, now
		if m.stats.Done || m.stats.Err != nil {
			return m, tea.Quit
		}
		return m, tick()
	}
	return m, nil
}

func (m model) View() string {
	s := m.stats
	inner := m.width - 6 // border + padding
	if inner < 20 {
		inner = 20
	}
	var b strings.Builder

	name := s.Name
	if name == "" {
		name = "resolving…"
	}
	b.WriteString(logo.Render("ra3d") + " " + title.Render(truncate(name, inner-7)) + "\n\n")

	pct := 0.0
	if s.Length > 0 {
		pct = float64(s.Downloaded) / float64(s.Length)
	}
	m.bar.Width = inner
	b.WriteString(m.bar.ViewAs(pct) + "\n\n")

	eta := "–"
	if m.speed > 1 && s.Length > s.Downloaded {
		eta = (time.Duration(float64(s.Length-s.Downloaded)/m.speed) * time.Second).Round(time.Second).String()
	}
	if s.Done {
		eta = "0s"
	}
	b.WriteString(row("Size", fmt.Sprintf("%s / %s", bytes(s.Downloaded), bytes(s.Length))) + "\n")
	b.WriteString(row("Speed", bytes(int(m.speed))+"/s") + dim.Render("   ETA ") + value.Render(eta) + "\n")
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
	default:
		spin := []rune(spinnerChar)
		b.WriteString(have.Render(string(spin[m.frame%len(spin)])) + " " + s.Status)
	}

	return box.Width(m.width-2).Render(b.String()) + "\n" + dim.Render("  q quit · progress is kept, rerun to resume") + "\n"
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
