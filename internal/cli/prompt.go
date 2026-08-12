package cli

import (
	"errors"
	"fmt"
	"os"
	"strings"

	tea "github.com/charmbracelet/bubbletea"
	"github.com/charmbracelet/lipgloss"

	"github.com/lvcas-dotcom/pgfathom/internal/report"
)

// errCancelled is the user pressing ctrl-c or esc. It is not a failure: they
// changed their mind, which a guide has to allow at every step.
var errCancelled = errors.New("cancelled")

// The prompts draw on stderr, never stdout.
//
// It is the same discipline the rest of the output follows: stdout carries the
// result meant for consumption — here, the composed command someone will copy —
// and everything else is diagnostic. Drawing the interface on stdout would put
// escape sequences in the middle of the one line the user wants to keep.
func runPrompt(m tea.Model) (tea.Model, error) {
	p := tea.NewProgram(m, tea.WithOutput(os.Stderr), tea.WithInput(os.Stdin))
	return p.Run()
}

// The guide is painted in the project's palette, and lipgloss is what makes
// that safe here: it knows whether the terminal is light or dark, so bone —
// which is paper on paper against a white background — can have a dark
// counterpart. The report cannot do this, because it must render identically
// whatever it is pointed at. Here there is a person looking, so it is worth it.
//
// The roles match the report's, so the same colour means the same thing all the
// way through: red is where attention goes, aqua is something settled, stone is
// what supports without competing.
var (
	ink   = lipgloss.AdaptiveColor{Light: report.BrandInk, Dark: report.BrandBone}
	red   = lipgloss.Color(report.BrandRed)
	aqua  = lipgloss.Color(report.BrandAqua)
	stone = lipgloss.Color(report.BrandStone)
)

var (
	titleStyle    = lipgloss.NewStyle().Bold(true).Foreground(ink)
	cursorStyle   = lipgloss.NewStyle().Bold(true).Foreground(red)
	selectedStyle = lipgloss.NewStyle().Bold(true).Foreground(ink)
	tickStyle     = lipgloss.NewStyle().Bold(true).Foreground(aqua)
	hintStyle     = lipgloss.NewStyle().Foreground(stone)
	detailStyle   = lipgloss.NewStyle().Foreground(stone)
	errorStyle    = lipgloss.NewStyle().Bold(true).Foreground(red)
	answerStyle   = lipgloss.NewStyle().Foreground(aqua)
)

// Option is one line of a chooser: what it is, and what it means.
type Option struct {
	Label string

	// Detail is shown beside the label, for the fact that makes the choice
	// decidable — a table count, or what a mode can and cannot conclude.
	Detail string
}

// chooser is a list of options with a cursor. Multi mode toggles with space and
// accepts with enter; single mode accepts with enter alone.
type chooser struct {
	title   string
	options []Option
	multi   bool

	cursor    int
	picked    map[int]bool
	cancelled bool
	done      bool

	// height is how many options fit on screen. A list longer than the terminal
	// scrolls its own top away, taking the title and the first options with it —
	// and on a server with sixty schemas the first options are the big ones,
	// which is precisely what the list exists to show.
	height int
}

// visibleRows is the fallback when the terminal has not said how tall it is.
// Small enough to fit anything, and the window keeps the cursor in view.
const visibleRows = 10

func (m chooser) Init() tea.Cmd { return nil }

func (m chooser) Update(msg tea.Msg) (tea.Model, tea.Cmd) {
	if size, ok := msg.(tea.WindowSizeMsg); ok {
		// Seven lines go to the title, the two "more" markers, the hint and the
		// blank lines around them; the eighth is slack, so the drawing never
		// reaches the bottom of the terminal and scrolls its own top away.
		m.height = max(3, size.Height-8)
		return m, nil
	}

	key, ok := msg.(tea.KeyMsg)
	if !ok {
		return m, nil
	}

	switch key.String() {
	case "ctrl+c", "esc", "q":
		m.cancelled = true
		return m, tea.Quit

	case "up", "k":
		if m.cursor > 0 {
			m.cursor--
		}

	case "down", "j":
		if m.cursor < len(m.options)-1 {
			m.cursor++
		}

	case "pgup":
		m.cursor = max(0, m.cursor-m.window())

	case "pgdown":
		m.cursor = min(len(m.options)-1, m.cursor+m.window())

	case "home", "g":
		m.cursor = 0

	case "end", "G":
		m.cursor = len(m.options) - 1

	case " ":
		if m.multi {
			m.picked[m.cursor] = !m.picked[m.cursor]
		}

	case "enter":
		// Enter takes what is ticked, and ticks what is under the cursor when
		// nothing is. Moving to an item and pressing enter is what people expect
		// to mean "this one", and a list that answered something else while the
		// cursor sat somewhere visible would be answering for them.
		if len(m.chosen()) == 0 {
			m.picked[m.cursor] = true
		}
		m.done = true
		return m, tea.Quit
	}

	return m, nil
}

// window is how many options are drawn at once.
func (m chooser) window() int {
	h := m.height
	if h <= 0 {
		h = visibleRows
	}
	return min(h, len(m.options))
}

// slice is the range of options to draw, kept around the cursor so it is always
// on screen.
func (m chooser) slice() (start, end int) {
	w := m.window()

	start = m.cursor - w/2
	start = max(0, start)
	if start+w > len(m.options) {
		start = len(m.options) - w
	}
	return start, start + w
}

func (m chooser) View() string {
	var b strings.Builder

	fmt.Fprintf(&b, "\n  %s\n\n", titleStyle.Render(m.title))

	start, end := m.slice()
	if start > 0 {
		fmt.Fprintf(&b, "    %s\n", detailStyle.Render(fmt.Sprintf("↑ %d more", start)))
	}

	for i := start; i < end; i++ {
		o := m.options[i]
		cursor := "  "
		if i == m.cursor {
			cursor = cursorStyle.Render("❯ ")
		}

		mark := ""
		if m.multi {
			mark = detailStyle.Render("[ ] ")
			if m.picked[i] {
				mark = tickStyle.Render("[x] ")
			}
		}

		label := o.Label
		if i == m.cursor {
			label = selectedStyle.Render(label)
		}

		fmt.Fprintf(&b, "  %s%s%s", cursor, mark, label)
		if o.Detail != "" {
			fmt.Fprintf(&b, "  %s", detailStyle.Render(o.Detail))
		}
		b.WriteString("\n")
	}

	if end < len(m.options) {
		fmt.Fprintf(&b, "    %s\n", detailStyle.Render(fmt.Sprintf("↓ %d more", len(m.options)-end)))
	}

	hint := "↑↓ move · enter choose · esc cancel"
	if m.multi {
		hint = "↑↓ move · space toggle · enter confirm · esc cancel"
	}
	if len(m.options) > m.window() {
		hint += " · pgup/pgdn page"
	}
	fmt.Fprintf(&b, "\n  %s\n", hintStyle.Render(hint))

	return b.String()
}

func (m chooser) chosen() []int {
	var out []int
	for i := range m.options {
		if m.picked[i] {
			out = append(out, i)
		}
	}
	return out
}

// selectOne asks for exactly one option and returns its index.
func selectOne(title string, options []Option) (int, error) {
	picked, err := choose(title, options, false)
	if err != nil {
		return 0, err
	}
	return picked[0], nil
}

// selectMany asks for one or more options and returns their indices, with the
// given ones already ticked.
func selectMany(title string, options []Option, preselected ...int) ([]int, error) {
	picked := make(map[int]bool, len(preselected))
	for _, i := range preselected {
		picked[i] = true
	}
	return chooseWith(title, options, true, picked)
}

func choose(title string, options []Option, multi bool) ([]int, error) {
	return chooseWith(title, options, multi, map[int]bool{})
}

func chooseWith(title string, options []Option, multi bool, picked map[int]bool) ([]int, error) {
	if len(options) == 0 {
		return nil, fmt.Errorf("nothing to choose from")
	}

	final, err := runPrompt(chooser{title: title, options: options, multi: multi, picked: picked})
	if err != nil {
		return nil, err
	}

	m, ok := final.(chooser)
	if !ok || m.cancelled || !m.done {
		return nil, errCancelled
	}
	return m.chosen(), nil
}

// line is a one-line text field. Enough for a host, a port, a directory — and
// deliberately not enough for a password, which this guide never asks for.
type line struct {
	title       string
	hint        string
	value       string
	placeholder string

	cancelled bool
	done      bool
}

func (m line) Init() tea.Cmd { return nil }

func (m line) Update(msg tea.Msg) (tea.Model, tea.Cmd) {
	key, ok := msg.(tea.KeyMsg)
	if !ok {
		return m, nil
	}

	switch key.Type {
	case tea.KeyCtrlC, tea.KeyEsc:
		m.cancelled = true
		return m, tea.Quit

	case tea.KeyEnter:
		m.done = true
		return m, tea.Quit

	case tea.KeyBackspace:
		if r := []rune(m.value); len(r) > 0 {
			m.value = string(r[:len(r)-1])
		}

	case tea.KeyRunes, tea.KeySpace:
		m.value += string(key.Runes)
		if key.Type == tea.KeySpace {
			m.value += " "
		}
	}

	return m, nil
}

func (m line) View() string {
	var b strings.Builder

	fmt.Fprintf(&b, "\n  %s\n\n", titleStyle.Render(m.title))

	shown := answerStyle.Render(m.value)
	if m.value == "" && m.placeholder != "" {
		shown = detailStyle.Render(m.placeholder)
	}
	fmt.Fprintf(&b, "  %s%s%s\n", cursorStyle.Render("❯ "), shown, cursorStyle.Render("█"))

	if m.hint != "" {
		fmt.Fprintf(&b, "\n  %s\n", hintStyle.Render(m.hint))
	}
	return b.String()
}

// askLine reads one line. An empty answer falls back to fallback, which is what
// makes an optional question optional.
func askLine(title, hint, placeholder, fallback string) (string, error) {
	final, err := runPrompt(line{title: title, hint: hint, placeholder: placeholder})
	if err != nil {
		return "", err
	}

	m, ok := final.(line)
	if !ok || m.cancelled || !m.done {
		return "", errCancelled
	}

	answer := strings.TrimSpace(m.value)
	if answer == "" {
		return fallback, nil
	}
	return answer, nil
}

// confirm asks a yes-or-no question, defaulting the cursor to no.
//
// The default matters: this is the last gate before the tool touches somebody's
// database, and an accidental enter should decline.
func confirm(title string) (bool, error) {
	picked, err := selectOne(title, []Option{
		{Label: "No", Detail: "print the command and stop"},
		{Label: "Yes", Detail: "run it now"},
	})
	if err != nil {
		return false, err
	}
	return picked == 1, nil
}
