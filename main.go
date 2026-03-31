package main

import (
	_ "embed"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"time"

	"charm.land/bubbles/v2/help"
	"charm.land/bubbles/v2/key"
	"charm.land/bubbles/v2/spinner"
	"charm.land/bubbles/v2/viewport"
	tea "charm.land/bubbletea/v2"
	"charm.land/glamour/v2"
	"charm.land/lipgloss/v2"
	"github.com/spf13/cobra"
)

//go:embed README.md
var readme string

//go:embed LICENSE
var license string

// Messages

type commandOutputMsg struct {
	output string
	err    error
}

type tickMsg time.Time

// Key map

type keyMap struct {
	Quit       key.Binding
	Suspend    key.Binding
	Refresh    key.Binding
	Faster     key.Binding
	Slower     key.Binding
	ToggleDiff key.Binding
	ToggleWrap key.Binding
	Fullscreen key.Binding
	Help       key.Binding
}

func (k keyMap) ShortHelp() []key.Binding {
	return []key.Binding{k.Quit, k.Refresh, k.Faster, k.Slower, k.ToggleDiff, k.ToggleWrap, k.Fullscreen, k.Help}
}

func (k keyMap) FullHelp() [][]key.Binding {
	return [][]key.Binding{
		{k.Quit, k.Suspend},
		{k.Refresh, k.Faster, k.Slower},
		{k.ToggleDiff, k.ToggleWrap, k.Fullscreen, k.Help},
	}
}

var keys = keyMap{
	Quit: key.NewBinding(
		key.WithKeys("q", "ctrl+c"),
		key.WithHelp("q", "quit"),
	),
	Suspend: key.NewBinding(
		key.WithKeys("ctrl+z"),
		key.WithHelp("ctrl+z", "suspend"),
	),
	Refresh: key.NewBinding(
		key.WithKeys("space"),
		key.WithHelp("space", "refresh"),
	),
	Faster: key.NewBinding(
		key.WithKeys("+", "="),
		key.WithHelp("+", "faster"),
	),
	Slower: key.NewBinding(
		key.WithKeys("-"),
		key.WithHelp("-", "slower"),
	),
	ToggleDiff: key.NewBinding(
		key.WithKeys("d"),
		key.WithHelp("d", "diff"),
	),
	ToggleWrap: key.NewBinding(
		key.WithKeys("w"),
		key.WithHelp("w", "no-wrap"),
	),
	Fullscreen: key.NewBinding(
		key.WithKeys("f"),
		key.WithHelp("f", "fullscreen"),
	),
	Help: key.NewBinding(
		key.WithKeys("?"),
		key.WithHelp("?", "help"),
	),
}

// Model

type model struct {
	command  string
	args     []string
	interval time.Duration
	diffMode bool
	noWrap   bool

	output     string
	prevOutput string
	lastRun    time.Time
	err        error
	width      int
	height     int
	executing  bool
	ready      bool
	fullscreen bool

	viewport viewport.Model
	spinner  spinner.Model
	help     help.Model
	keys     keyMap
}

func initialModel(command string, args []string, interval time.Duration, diffMode bool, noWrap bool) model {
	s := spinner.New()
	s.Spinner = spinner.Dot
	s.Style = lipgloss.NewStyle().Foreground(lipgloss.Color("#7D56F4"))

	h := help.New()
	h.Styles.ShortKey = lipgloss.NewStyle().Foreground(lipgloss.Color("#FAFAFA"))
	h.Styles.ShortDesc = lipgloss.NewStyle().Foreground(lipgloss.Color("#626262"))
	h.Styles.ShortSeparator = lipgloss.NewStyle().Foreground(lipgloss.Color("#3C3C3C"))
	h.Styles.FullKey = h.Styles.ShortKey
	h.Styles.FullDesc = h.Styles.ShortDesc
	h.Styles.FullSeparator = h.Styles.ShortSeparator

	return model{
		command:   command,
		args:      args,
		interval:  interval,
		diffMode:  diffMode,
		noWrap:    noWrap,
		executing: true,
		spinner:   s,
		help:      h,
		keys:      keys,
	}
}

func (m model) Init() tea.Cmd {
	return tea.Batch(runCommand(m.command, m.args), tickCmd(m.interval), m.spinner.Tick)
}

func (m model) Update(msg tea.Msg) (tea.Model, tea.Cmd) {
	var cmds []tea.Cmd

	switch msg := msg.(type) {
	case tea.WindowSizeMsg:
		m.width = msg.Width
		m.height = msg.Height
		m.help.SetWidth(msg.Width)

		if !m.ready {
			m.viewport = viewport.New(
				viewport.WithWidth(msg.Width),
				viewport.WithHeight(msg.Height),
			)
			m.viewport.SoftWrap = !m.noWrap
			m.ready = true
		}
		m.resizeViewport()
		m.updateViewportContent()
		return m, nil

	case tea.KeyPressMsg:
		switch {
		case key.Matches(msg, m.keys.Quit):
			return m, tea.Quit
		case key.Matches(msg, m.keys.Suspend):
			return m, tea.Suspend
		case key.Matches(msg, m.keys.Refresh):
			m.executing = true
			return m, runCommand(m.command, m.args)
		case key.Matches(msg, m.keys.Faster):
			if m.interval > 200*time.Millisecond {
				m.interval -= 200 * time.Millisecond
			}
			return m, nil
		case key.Matches(msg, m.keys.Slower):
			m.interval += 200 * time.Millisecond
			return m, nil
		case key.Matches(msg, m.keys.ToggleDiff):
			m.diffMode = !m.diffMode
			m.updateViewportContent()
			return m, nil
		case key.Matches(msg, m.keys.ToggleWrap):
			m.noWrap = !m.noWrap
			m.viewport.SoftWrap = !m.noWrap
			m.updateViewportContent()
			return m, nil
		case key.Matches(msg, m.keys.Fullscreen):
			m.fullscreen = !m.fullscreen
			m.resizeViewport()
			return m, nil
		case key.Matches(msg, m.keys.Help):
			m.help.ShowAll = !m.help.ShowAll
			m.resizeViewport()
			return m, nil
		}

	case commandOutputMsg:
		m.executing = false
		m.prevOutput = m.output
		m.output = msg.output
		if msg.err != nil {
			m.err = msg.err
		} else {
			m.err = nil
		}
		m.lastRun = time.Now()
		m.updateViewportContent()
		return m, nil

	case tickMsg:
		m.executing = true
		return m, tea.Batch(runCommand(m.command, m.args), tickCmd(m.interval))
	}

	// Update spinner
	var cmd tea.Cmd
	m.spinner, cmd = m.spinner.Update(msg)
	cmds = append(cmds, cmd)

	// Update viewport (handles scroll keys and mouse wheel)
	m.viewport, cmd = m.viewport.Update(msg)
	cmds = append(cmds, cmd)

	return m, tea.Batch(cmds...)
}

func (m *model) resizeViewport() {
	if !m.ready {
		return
	}
	if m.fullscreen {
		m.viewport.SetWidth(m.width)
		m.viewport.SetHeight(m.height)
	} else {
		headerH := lipgloss.Height(m.renderHeader())
		footerH := lipgloss.Height(m.renderFooter())
		// 2 for top/bottom border
		m.viewport.SetWidth(m.width - 2)
		m.viewport.SetHeight(m.height - headerH - footerH - 2)
	}
}

func (m *model) updateViewportContent() {
	if !m.ready {
		return
	}
	m.viewport.SetContent(m.buildContent())
}

func (m model) View() tea.View {
	var v tea.View
	v.AltScreen = true
	v.MouseMode = tea.MouseModeCellMotion

	if !m.ready {
		v.SetContent("  Initializing...")
		return v
	}

	if m.fullscreen {
		v.SetContent(m.viewport.View())
		return v
	}

	header := m.renderHeader()
	vpView := lipgloss.NewStyle().
		Border(lipgloss.RoundedBorder()).
		BorderForeground(lipgloss.Color("#3C3C3C")).
		Render(m.viewport.View())
	footer := m.renderFooter()
	v.SetContent(header + "\n" + vpView + "\n" + footer)
	return v
}

// Diff types

type diffOp int

const (
	diffEqual  diffOp = iota
	diffInsert        // line exists only in new output
	diffDelete        // line existed only in old output
	diffChange        // line content changed
)

type diffLine struct {
	op   diffOp
	text string // the new (or deleted) line text
}

// computeDiff produces a sequence of diffLine entries using longest common
// subsequence so that inserts/deletes don't cause every subsequent line to
// appear changed.
func computeDiff(old, new []string) []diffLine {
	n, m := len(old), len(new)

	// LCS table
	dp := make([][]int, n+1)
	for i := range dp {
		dp[i] = make([]int, m+1)
	}
	for i := 1; i <= n; i++ {
		for j := 1; j <= m; j++ {
			if old[i-1] == new[j-1] {
				dp[i][j] = dp[i-1][j-1] + 1
			} else {
				dp[i][j] = max(dp[i-1][j], dp[i][j-1])
			}
		}
	}

	// Back-track to produce edit script
	var result []diffLine
	i, j := n, m
	for i > 0 || j > 0 {
		if i > 0 && j > 0 && old[i-1] == new[j-1] {
			result = append(result, diffLine{op: diffEqual, text: new[j-1]})
			i--
			j--
		} else if j > 0 && (i == 0 || dp[i][j-1] >= dp[i-1][j]) {
			result = append(result, diffLine{op: diffInsert, text: new[j-1]})
			j--
		} else {
			result = append(result, diffLine{op: diffDelete, text: old[i-1]})
			i--
		}
	}

	// Reverse (we built it back-to-front)
	for l, r := 0, len(result)-1; l < r; l, r = l+1, r-1 {
		result[l], result[r] = result[r], result[l]
	}

	return result
}

// Styles
var (
	headerStyle = lipgloss.NewStyle().
			Bold(true).
			Foreground(lipgloss.Color("#FAFAFA")).
			Background(lipgloss.Color("#7D56F4")).
			Padding(0, 1)

	headerInfoStyle = lipgloss.NewStyle().
			Foreground(lipgloss.Color("#FAFAFA")).
			Background(lipgloss.Color("#5A3DB5")).
			Padding(0, 1)

	diffAddStyle = lipgloss.NewStyle().
			Background(lipgloss.Color("#2D4F2D")).
			Foreground(lipgloss.Color("#A8D8A8"))

	diffDelStyle = lipgloss.NewStyle().
			Background(lipgloss.Color("#4F2D2D")).
			Foreground(lipgloss.Color("#D8A8A8"))

	errStyle = lipgloss.NewStyle().
			Foreground(lipgloss.Color("#FF6B6B")).
			Bold(true)

	diffInsertMarker = lipgloss.NewStyle().
				Foreground(lipgloss.Color("#6BCB77")).
				Bold(true)

	diffDeleteMarker = lipgloss.NewStyle().
				Foreground(lipgloss.Color("#FF6B6B")).
				Bold(true)

	diffChangeMarker = lipgloss.NewStyle().
				Foreground(lipgloss.Color("#FFD93D")).
				Bold(true)

	statusBarBg = lipgloss.NewStyle().
			Background(lipgloss.Color("#5A3DB5"))
)

func (m model) renderHeader() string {
	cmd := m.command
	if len(m.args) > 0 {
		cmd += " " + strings.Join(m.args, " ")
	}

	var spinnerStr string
	if m.executing {
		spinnerStr = " " + m.spinner.View()
	}

	title := headerStyle.Render(fmt.Sprintf(" Every %.1fs: %s%s", m.interval.Seconds(), cmd, spinnerStr))

	var rightParts []string
	if m.err != nil {
		rightParts = append(rightParts, errStyle.Render(" ERR "))
	}
	if m.diffMode {
		rightParts = append(rightParts, headerInfoStyle.Render(" DIFF "))
	}
	if !m.lastRun.IsZero() {
		rightParts = append(rightParts, headerInfoStyle.Render(m.lastRun.Format("15:04:05")))
	}
	rightSide := strings.Join(rightParts, "")

	// Fill gap with background color
	spacerWidth := max(0, m.width-lipgloss.Width(title)-lipgloss.Width(rightSide))
	spacer := statusBarBg.Render(strings.Repeat(" ", spacerWidth))

	return title + spacer + rightSide
}

func (m model) buildContent() string {
	lines := strings.Split(m.output, "\n")

	var content string

	if !m.diffMode {
		content = m.output
	} else if m.prevOutput == "" {
		// No previous output yet — just indent all lines to match diff markers
		var b strings.Builder
		for i, l := range lines {
			if i > 0 {
				b.WriteString("\n")
			}
			b.WriteString("  " + l)
		}
		content = b.String()
	} else {
		prevLines := strings.Split(m.prevOutput, "\n")
		diffs := computeDiff(prevLines, lines)

		var b strings.Builder
		first := true
		for idx := 0; idx < len(diffs); idx++ {
			if !first {
				b.WriteString("\n")
			}
			first = false

			d := diffs[idx]
			switch d.op {
			case diffEqual:
				b.WriteString("  " + d.text)
			case diffInsert:
				b.WriteString(diffInsertMarker.Render("+ ") + diffAddStyle.Render(d.text))
			case diffDelete:
				// Single delete followed by single insert = replacement, show as change
				if idx+1 < len(diffs) && diffs[idx+1].op == diffInsert {
					if idx+2 >= len(diffs) || diffs[idx+2].op != diffInsert {
						idx++
						b.WriteString(diffChangeMarker.Render("~ ") + diffAddStyle.Render(diffs[idx].text))
						continue
					}
				}
				count := 1
				for idx+1 < len(diffs) && diffs[idx+1].op == diffDelete {
					count++
					idx++
				}
				label := fmt.Sprintf("(%d line removed)", count)
				if count != 1 {
					label = fmt.Sprintf("(%d lines removed)", count)
				}
				b.WriteString(diffDeleteMarker.Render("- ") + diffDelStyle.Render(label))
			case diffChange:
				b.WriteString(diffChangeMarker.Render("~ ") + diffAddStyle.Render(d.text))
			}
		}
		content = b.String()
	}

	return content
}

func (m model) renderFooter() string {
	scrollPct := ""
	if m.ready {
		pct := m.viewport.ScrollPercent()
		if pct > 0 {
			scrollPct = fmt.Sprintf(" %3.0f%%", pct*100)
		}
	}

	info := lipgloss.NewStyle().
		Foreground(lipgloss.Color("#FAFAFA")).
		Background(lipgloss.Color("#5A3DB5")).
		Padding(0, 1).
		Render(fmt.Sprintf("%.1fs%s", m.interval.Seconds(), scrollPct))

	helpView := m.help.View(m.keys)
	spacerW := max(0, m.width-lipgloss.Width(helpView)-lipgloss.Width(info))

	return helpView + strings.Repeat(" ", spacerW) + info
}

// Commands

func runCommand(command string, args []string) tea.Cmd {
	return func() tea.Msg {
		fullCmd := command
		if len(args) > 0 {
			fullCmd += " " + strings.Join(args, " ")
		}
		cmd := exec.Command("sh", "-c", fullCmd)
		cmd.Env = os.Environ()
		out, err := cmd.CombinedOutput()
		return commandOutputMsg{output: string(out), err: err}
	}
}

func tickCmd(d time.Duration) tea.Cmd {
	return tea.Tick(d, func(t time.Time) tea.Msg {
		return tickMsg(t)
	})
}

func renderDoc(doc string) {
	r, err := glamour.NewTermRenderer(
		glamour.WithStandardStyle("dark"),
		glamour.WithWordWrap(120),
	)
	if err != nil {
		fmt.Fprint(os.Stderr, doc)
		return
	}
	out, err := r.Render(doc)
	if err != nil {
		fmt.Fprint(os.Stderr, doc)
		return
	}
	fmt.Print(out)
}

func printHelp() {
	exeName := filepath.Base(os.Args[0])
	doc := strings.ReplaceAll(readme, "gowatch", exeName)
	doc = strings.ReplaceAll(doc, "GOWATCH", strings.ToUpper(exeName))
	renderDoc(doc)
}

func printLicense() {
	fmt.Print(license)
}

func main() {
	var interval float64
	var diffMode bool
	var noWrap bool
	var showLicense bool

	rootCmd := &cobra.Command{
		Use:   "gowatch [flags] command [args...]",
		Short: "Execute a program periodically, showing output fullscreen",
		Args: func(cmd *cobra.Command, args []string) error {
			if showLicense {
				return nil
			}
			return cobra.MinimumNArgs(1)(cmd, args)
		},
		Run: func(cmd *cobra.Command, args []string) {
			if showLicense {
				printLicense()
				return
			}
			dur := time.Duration(float64(time.Second) * interval)
			m := initialModel(args[0], args[1:], dur, diffMode, noWrap)
			p := tea.NewProgram(m)
			if _, err := p.Run(); err != nil {
				fmt.Fprintf(os.Stderr, "Error: %v\n", err)
				os.Exit(1)
			}
		},
	}

	rootCmd.Flags().Float64VarP(&interval, "interval", "n", 2.0, "seconds between updates")
	rootCmd.Flags().BoolVarP(&diffMode, "differences", "d", false, "highlight differences")
	rootCmd.Flags().BoolVarP(&noWrap, "no-wrap", "w", false, "turn off line wrapping")
	rootCmd.Flags().BoolVar(&showLicense, "license", false, "display the license")
	rootCmd.Flags().SetInterspersed(false)

	rootCmd.SetHelpFunc(func(cmd *cobra.Command, args []string) {
		printHelp()
	})

	if err := rootCmd.Execute(); err != nil {
		os.Exit(1)
	}
}
