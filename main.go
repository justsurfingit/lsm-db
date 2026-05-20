package main

import (
	"fmt"
	"os"
	"strings"

	"github.com/charmbracelet/bubbles/textinput"
	tea "github.com/charmbracelet/bubbletea"
	lsmdb "github.com/justsurfingit/lsm-db/db"
)

// 1. THE MODEL
type model struct {
	db        *lsmdb.Db
	textInput textinput.Model
	logs      []string
}

// 2. INIT (Runs once when the app starts)
func (m model) Init() tea.Cmd {
	// We want the text input cursor to start blinking immediately
	return textinput.Blink
}

// 3. UPDATE (Handles all keystrokes)
func (m model) Update(msg tea.Msg) (tea.Model, tea.Cmd) {
	var cmd tea.Cmd

	switch msg := msg.(type) {
	case tea.KeyMsg:
		switch msg.Type {
		case tea.KeyCtrlC, tea.KeyEsc:
			// If they press Ctrl+C or Escape, quit the app!
			return m, tea.Quit
		case tea.KeyEnter:
			//grab next from the user
			rawInput := m.textInput.Value()
			m.textInput.Reset()
			//append it into the logs
			m.logs = append(m.logs, "> "+rawInput)
			// 2. Tokenize the input!
			parts := strings.Fields(rawInput)
			if len(parts) == 0 {
				return m, nil // They just pressed enter on an empty line, do nothing
			}
			// 3. The Router (Always uppercase the command so 'put' and 'PUT' both work)
			command := strings.ToUpper(parts[0])
			switch command {
			case "PUT":
				if len(parts) < 3 {
					m.logs = append(m.logs, "❌ Error: PUT requires a key and a value")
					break
				}
				// Parts: [0]="PUT", [1]=Key, [2]=Value
				err := m.db.Put(parts[1], []byte(parts[2]))
				if err != nil {
					m.logs = append(m.logs, "❌ Error writing to DB: "+err.Error())
				} else {
					m.logs = append(m.logs, "✅ Successfully inserted "+parts[1])
				}
			case "GET":
				if len(parts) < 2 {
					m.logs = append(m.logs, "❌ Error: GET requires a key")
					break
				}
				val, found, err := m.db.Get(parts[1])
				if err != nil {
					m.logs = append(m.logs, "❌ Error reading DB: "+err.Error())
				} else if !found {
					m.logs = append(m.logs, "⚠️ Key not found")
				} else {
					m.logs = append(m.logs, fmt.Sprintf("🔍 Value: %s", string(val)))
				}
			case "COMPACT":
				m.logs = append(m.logs, "⚙️ Forcing manual compaction...")
				err := m.db.CompactionManual()
				if err != nil {
					m.logs = append(m.logs, "❌ Compaction failed: "+err.Error())
				} else {
					m.logs = append(m.logs, "✅ Compaction successful (if there were enough files)")
				}
			case "CLEAR":
				// A nice utility command for the UI!
				m.logs = []string{}
			default:
				m.logs = append(m.logs, "❓ Unknown command. Try PUT, GET, COMPACT, or CLEAR.")
			}
			return m, nil
		}
	}

	// If it wasn't a special key, pass the keystroke to the text input component
	// so it can type letters normally.
	m.textInput, cmd = m.textInput.Update(msg)
	return m, cmd
}

// 4. VIEW (Draws the screen)
func (m model) View() string {
	// Let's build the UI string!
	s := " LSM-DB Dashboard\n\n"

	// Draw the logs
	s += "--- Logs ---\n"
	for _, log := range m.logs {
		s += log + "\n"
	}

	// Draw the input box at the bottom
	s += "\n--- Enter Command (PUT, GET, DEL, COMPACT) ---\n"
	s += m.textInput.View() + "\n\n"
	s += "(Press Esc to quit)\n"

	return s
}

// 5. MAIN (Bootstrapping)
func main() {
	// Start your actual database!
	database, err := lsmdb.NewDb("data")
	if err != nil {
		fmt.Println("Error starting DB:", err)
		os.Exit(1)
	}

	// Configure the text input component
	ti := textinput.New()
	ti.Placeholder = "PUT user123 alice..."
	ti.Focus()

	// Create the initial Model
	initialModel := model{
		db:        database,
		textInput: ti,
		logs:      []string{"Database started successfully."},
	}

	// Start the Bubble Tea program!
	p := tea.NewProgram(initialModel)
	if _, err := p.Run(); err != nil {
		fmt.Printf("Alas, there's been an error: %v", err)
		os.Exit(1)
	}
}
