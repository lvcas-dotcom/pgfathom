package cli

import (
	"errors"
	"fmt"
	"os"
	"strings"

	tea "github.com/charmbracelet/bubbletea"
	"github.com/charmbracelet/lipgloss"
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

var (
	titleStyle    = lipgloss.NewStyle().Bold(true)
	cursorStyle   = lipgloss.NewStyle().Bold(true)
	selectedStyle = lipgloss.NewStyle().Bold(true)
	hintStyle     = lipgloss.NewStyle().Faint(true)
	detailStyle   = lipgloss.NewStyle().Faint(true)
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
}

func (m chooser) Init() tea.Cmd { return nil }

func (m chooser) Update(msg tea.Msg) (tea.Model, tea.Cmd) {
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

	case " ":
		if m.multi {
			m.picked[m.cursor] = !m.picked[m.cursor]
		}

	case "enter":
		if !m.multi {
			m.picked = map[int]bool{m.cursor: true}
		}
		// Accepting nothing in multi mode would compose a command with an empty
		// scope, which fails later with a worse message than this one.
		if len(m.chosen()) == 0 {
			return m, nil
		}
		m.done = true
		return m, tea.Quit
	}

	return m, nil
}

func (m chooser) View() string {
	var b strings.Builder

	fmt.Fprintf(&b, "\n  %s\n\n", titleStyle.Render(m.title))

	for i, o := range m.options {
		cursor := "  "
		if i == m.cursor {
			cursor = cursorStyle.Render("❯ ")
		}

		mark := ""
		if m.multi {
			mark = "[ ] "
			if m.picked[i] {
				mark = selectedStyle.Render("[x] ")
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

	hint := "↑↓ move · enter choose · esc cancel"
	if m.multi {
		hint = "↑↓ move · space toggle · enter confirm · esc cancel"
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

	shown := m.value
	if shown == "" && m.placeholder != "" {
		shown = detailStyle.Render(m.placeholder)
	}
	fmt.Fprintf(&b, "  ❯ %s█\n", shown)

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
