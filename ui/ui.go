package ui

import (
	"fmt"
	"strings"

	"github.com/charmbracelet/bubbles/progress"
	"github.com/charmbracelet/bubbles/textinput"
	tea "github.com/charmbracelet/bubbletea"
	"github.com/charmbracelet/lipgloss"
	lsmdb "github.com/justsurfingit/lsm-db/db"
)

// --- STYLES ---
var (
	// The left pane for stats
	statsBoxStyle = lipgloss.NewStyle().
			Border(lipgloss.RoundedBorder()).
			BorderForeground(lipgloss.Color("63")).
			Padding(1, 2).
			Width(45).
			Height(12)

	// The right pane for logs
	logsBoxStyle = lipgloss.NewStyle().
			Border(lipgloss.RoundedBorder()).
			BorderForeground(lipgloss.Color("86")).
			Padding(1, 2).
			Width(65).
			Height(12)

	titleStyle = lipgloss.NewStyle().
			Bold(true).
			Foreground(lipgloss.Color("86"))

	logPutStyle = lipgloss.NewStyle().Foreground(lipgloss.Color("42")).Bold(true) // Green
	logGetStyle = lipgloss.NewStyle().Foreground(lipgloss.Color("39")).Bold(true) // Blue
	logErrStyle = lipgloss.NewStyle().Foreground(lipgloss.Color("196")).Bold(true) // Red
)

// 1. THE MODEL
type model struct {
	db          *lsmdb.Db
	textInput   textinput.Model
	progressBar progress.Model
	logs        []string
	history     []string
	historyIdx  int
}

// 2. INIT
func (m model) Init() tea.Cmd {
	return textinput.Blink
}

// 3. UPDATE
func (m model) Update(msg tea.Msg) (tea.Model, tea.Cmd) {
	var cmd tea.Cmd

	switch msg := msg.(type) {
	case tea.KeyMsg:
		switch msg.Type {
		case tea.KeyCtrlC, tea.KeyEsc:
			return m, tea.Quit
			
		case tea.KeyUp:
			// Command History!
			if len(m.history) > 0 && m.historyIdx > 0 {
				m.historyIdx--
				m.textInput.SetValue(m.history[m.historyIdx])
			}
			return m, nil
			
		case tea.KeyDown:
			// Command History!
			if len(m.history) > 0 && m.historyIdx < len(m.history)-1 {
				m.historyIdx++
				m.textInput.SetValue(m.history[m.historyIdx])
			} else {
				m.historyIdx = len(m.history)
				m.textInput.SetValue("")
			}
			return m, nil

		case tea.KeyEnter:
			rawInput := strings.TrimSpace(m.textInput.Value())
			if rawInput == "" {
				return m, nil
			}

			// Add to history
			if len(m.history) == 0 || m.history[len(m.history)-1] != rawInput {
				m.history = append(m.history, rawInput)
			}
			m.historyIdx = len(m.history)
			m.textInput.Reset()

			parts := strings.Fields(rawInput)
			command := strings.ToUpper(parts[0])

			switch command {
			case "PUT":
				if len(parts) < 3 {
					m.logs = append(m.logs, logErrStyle.Render("[ERR] ")+"PUT requires a key and value")
					break
				}
				value := strings.Join(parts[2:], " ")
				err := m.db.Put(parts[1], []byte(value))
				if err != nil {
					m.logs = append(m.logs, logErrStyle.Render("[ERR] ")+err.Error())
				} else {
					m.logs = append(m.logs, logPutStyle.Render("[PUT] ")+fmt.Sprintf("Key '%s' inserted", parts[1]))
				}

			case "GET":
				if len(parts) < 2 {
					m.logs = append(m.logs, logErrStyle.Render("[ERR] ")+"GET requires a key")
					break
				}
				val, found, err := m.db.Get(parts[1])
				if err != nil {
					m.logs = append(m.logs, logErrStyle.Render("[ERR] ")+err.Error())
				} else if !found {
					m.logs = append(m.logs, logGetStyle.Render("[GET] ")+"Key not found")
				} else {
					m.logs = append(m.logs, logGetStyle.Render("[GET] ")+fmt.Sprintf("Found: %s", string(val)))
				}

			case "COMPACT":
				m.logs = append(m.logs, "⚙️ Forcing manual compaction...")
				compacted, err := m.db.CompactionManual()
				if err != nil {
					m.logs = append(m.logs, logErrStyle.Render("[ERR] ")+"Compaction failed: "+err.Error())
				} else if compacted {
					m.logs = append(m.logs, logPutStyle.Render("[OK]  ")+"Compaction merged old files!")
				} else {
					m.logs = append(m.logs, "ℹ️ Skipped: Not enough SSTables.")
				}

			case "CLEAR":
				m.logs = []string{}

			default:
				m.logs = append(m.logs, logErrStyle.Render("[?]   ")+"Unknown command: "+command)
			}
			return m, nil
		}
	}

	m.textInput, cmd = m.textInput.Update(msg)
	return m, cmd
}

// 4. VIEW
func (m model) View() string {
	stats := m.db.GetStats()

	// --- LEFT PANE (Stats & Progress) ---
	percent := float64(stats.MemtableSize) / float64(stats.MaxMemtableSize)
	if percent < 0 {
		percent = 0
	} else if percent > 1 {
		percent = 1
	}

	statsText := fmt.Sprintf(
		"%s\n\nActive SSTables:  %d\nMemtable Bytes:   %d / %d\n\nMemtable Capacity:\n%s",
		titleStyle.Render("🔥 LSM-DB Live Engine Metrics"),
		stats.ActiveSSTables,
		stats.MemtableSize,
		stats.MaxMemtableSize,
		m.progressBar.ViewAs(percent),
	)
	leftPane := statsBoxStyle.Render(statsText)

	// --- RIGHT PANE (Logs) ---
	logsSection := titleStyle.Render("💻 Execution Logs") + "\n\n"
	startIdx := 0
	if len(m.logs) > 8 {
		startIdx = len(m.logs) - 8
	}
	for _, log := range m.logs[startIdx:] {
		logsSection += log + "\n"
	}
	rightPane := logsBoxStyle.Render(logsSection)

	// Combine Panes Horizontally!
	topHalf := lipgloss.JoinHorizontal(lipgloss.Top, leftPane, rightPane)

	// --- BOTTOM PANE (Input) ---
	inputSection := "\n  " + m.textInput.View() + "\n\n  (Commands: PUT, GET, COMPACT, CLEAR | Up/Down for History | Esc to quit)"

	return topHalf + "\n" + inputSection
}

// StartDashboard is the public entry point for the UI package
func StartDashboard(database *lsmdb.Db) error {
	ti := textinput.New()
	ti.Placeholder = "Enter command here..."
	ti.Focus()

	prog := progress.New(progress.WithDefaultGradient())

	initialModel := model{
		db:          database,
		textInput:   ti,
		progressBar: prog,
		logs:        []string{"Database engine initialized successfully."},
	}

	p := tea.NewProgram(initialModel)
	_, err := p.Run()
	return err
}
