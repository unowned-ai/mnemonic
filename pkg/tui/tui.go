package tui

import (
	"context"
	"database/sql"
	"fmt"
	"strings"

	textinput "github.com/charmbracelet/bubbles/textinput"
	tea "github.com/charmbracelet/bubbletea"
	"github.com/charmbracelet/lipgloss"
	"github.com/unowned-ai/recall/pkg/memories"
)

// UI styles and layout settings
// Color palette "Blue Moon" from https://gogh-co.github.io/Gogh/
const (
	borderColor = "#353b52"
)

var (
	titleStyle = lipgloss.NewStyle().Bold(true).
			Foreground(lipgloss.Color("#89ddff")).
			Background(lipgloss.Color("#353b52")).
			Padding(0, 2).Align(lipgloss.Center)
	selectedStyle = lipgloss.NewStyle().
			Foreground(lipgloss.Color("#ffffff")).
			Background(lipgloss.Color("#8796b0"))
	inactiveStyle = lipgloss.NewStyle().Foreground(lipgloss.Color("#8796b0"))
	// Specific border styles will be defined for panels in the View function
	hRule = lipgloss.NewStyle().Foreground(lipgloss.Color(borderColor)).
		Render(strings.Repeat("─", 30))
)

type model struct {
	journals []memories.Journal
	entries  []memories.Entry
	tags     []memories.EntryTag

	cursor int // Current selection 0-based index into journals slice
	width  int // Current terminal width (for layout)
	height int // Current terminal height
	err    error

	db *sql.DB

	quitting bool

	journalCreating     bool
	journalCreatingStep int // 0 = editing journal name, 1 = editing journal description
	journalNameInput    textinput.Model
	journalDescInput    textinput.Model
}

func initialModel(db *sql.DB) model {
	// Initialize text input fields for the new journal form
	jtin := textinput.New()
	jtin.Placeholder = "Journal Name"
	jtin.Focus() // focus name field initially
	jtin.CharLimit = 64
	jtin.Width = 30

	jtdesc := textinput.New()
	jtdesc.Placeholder = "Description of the journal (optional)"
	jtdesc.CharLimit = 128
	jtdesc.Width = 30

	return model{
		journals: []memories.Journal{},

		cursor: 0,
		width:  0,
		height: 0,

		db: db,

		journalNameInput: jtin,
		journalDescInput: jtdesc,
	}
}

func (m model) Init() tea.Cmd {
	// Load journals from database on initialization
	return loadJournals(m.db)
}

// Processes events like window resize, errors, loaded data, and key presses
func (m model) Update(msg tea.Msg) (tea.Model, tea.Cmd) {
	switch msg := msg.(type) {
	case tea.WindowSizeMsg:
		// Save the new window size in the model for responsive layout
		m.width = msg.Width
		m.height = msg.Height
		return m, nil

	case error:
		m.err = msg
		return m, nil

	case []memories.Journal:
		// When journals are loaded from DB, store them in model
		// TODO: Re-load if journal list updated outside
		m.journals = msg
		return m, nil

	// Handle key presses for navigation and input
	case tea.KeyMsg:
		if m.journalCreating {
			// Creating New Journal Mode
			switch msg.Type {
			case tea.KeyEnter:
				if m.journalCreatingStep == 0 {
					// Press Enter on name field -> move to description field
					// TODO: Verify if name is not empty
					m.journalCreatingStep = 1
					m.journalNameInput.Blur()
					m.journalDescInput.Focus()
				} else {
					// Press Enter on description field -> submit the form (create journal)
					journal, err := memories.CreateJournal(context.Background(), m.db,
						m.journalNameInput.Value(), m.journalDescInput.Value())
					if err != nil {
						m.err = err
					} else {
						// Prepend new journal to the list and focus it
						m.journals = append([]memories.Journal{journal}, m.journals...)
						m.cursor = 0 // highlight the newly created journal
					}
					// Exit create mode and reset form inputs
					m.journalCreating = false
					m.journalCreatingStep = 0
					m.journalNameInput.Reset()
					m.journalDescInput.Reset()
				}
			case tea.KeyEsc:
				// Cancel journal creation and reset form inputs
				m.journalCreating = false
				m.journalCreatingStep = 0
				m.journalNameInput.Reset()
				m.journalDescInput.Reset()
			}
			// If still in creating mode, route character input to the appropriate text field
			var cmd tea.Cmd
			if m.journalCreatingStep == 0 {
				m.journalNameInput, cmd = m.journalNameInput.Update(msg)
			} else {
				m.journalDescInput, cmd = m.journalDescInput.Update(msg)
			}
			return m, cmd
		}

		// Normal Navigation Mode
		switch msg.String() {
		case "q", "ctrl+c":
			m.quitting = true
			return m, tea.Quit

		case "up", "k":
			// Move selection up (stop at top)
			if m.cursor > 0 {
				m.cursor--
			}

		case "down", "j":
			// Move selection down (stop at last item)
			if m.cursor < len(m.journals)-1 {
				m.cursor++
			}

		case "n":
			m.journalCreating = true
			m.journalCreatingStep = 0
			m.journalNameInput.Reset()
			m.journalDescInput.Reset()

		case "enter":
			// "Enter" on selection (future functionality):
			// Currently, highlighting a journal auto-displays its entries, so no action needed.
		}
	}

	return m, nil
}

// Assembles the UI string for each frame
func (m model) View() string {
	if m.quitting {
		return "Unplugging mnemonic unit... Session saved.\n"
	}
	if m.err != nil {
		return fmt.Sprintf("Error: %v\n", m.err)
	}

	// Determine title bar text based on mode
	titleText := "Recall - datastore for memories"
	if m.journalCreating {
		titleText = "Create New Journal"
	}
	// Render the title bar (full width)
	titleBar := titleStyle.Width(m.width).Render(titleText)

	// Calculate column widths (left ~25%, middle ~25%, right ~50%)
	halfWidth := m.width / 2
	leftWidth := halfWidth / 2
	middleWidth := halfWidth - leftWidth
	rightWidth := m.width - (leftWidth + middleWidth)

	// Left column: Journals list + Tags
	var leftBuilder strings.Builder

	if len(m.journals) == 0 {
		leftBuilder.WriteString("No journals yet. Press 'n' to create new.\n")
	} else {
		// List each journal in the left column
		for i, journal := range m.journals {
			// Default: no pointer, inactive (grey) text
			pointer := "  "
			itemStyle := inactiveStyle
			if m.cursor == i {
				// Highlighted journal (cursor position)
				pointer = "> "
				itemStyle = selectedStyle
			}
			leftBuilder.WriteString(pointer + itemStyle.Render(journal.Name) + "\n")
		}
	}

	// Tags section (placeholder tags list)
	leftBuilder.WriteString("\n" + hRule + "\n")
	leftBuilder.WriteString("Tags:\n#tag1\n#tag2\n#tag3\n")

	// Middle column: Entries list (placeholder)
	var middleBuilder strings.Builder
	if m.cursor <= len(m.journals) {
		// If a journal is selected, show dummy entries for that journal
		middleBuilder.WriteString(" Entry 1\n")
		middleBuilder.WriteString(" Entry 2\n")
		middleBuilder.WriteString(" Entry 3\n")
	} else {
		// No journal selected (e.g., currently on "+ New Journal")
		middleBuilder.WriteString("No journal selected.\n")
	}

	// Right column: Entry preview or New Journal form
	var rightBuilder strings.Builder
	if m.journalCreating {
		// Show the form for creating a new journal
		rightBuilder.WriteString("Name: " + m.journalNameInput.View() + "\n")
		rightBuilder.WriteString("Description: " + m.journalDescInput.View() + "\n\n")
		rightBuilder.WriteString("(enter to submit, esc to cancel)")
	} else if len(m.journals) > 0 && m.cursor <= len(m.journals) {
		// Show placeholder details for the selected journal's content
		selectedJournal := m.journals[m.cursor]
		rightBuilder.WriteString(lipgloss.NewStyle().Bold(true).
			Render(selectedJournal.Name) + "\n")
		// Example metadata (date/URL placeholders)
		rightBuilder.WriteString(fmt.Sprintf("created_at: %s\n\n",
			"2025-05-13"))
		// Example content preview (summary and key points placeholders)
		rightBuilder.WriteString("Title: Entry 1\n\n")
		rightBuilder.WriteString("Tags: #tag1 #tag3\n\n")
		rightBuilder.WriteString("Lorem ipsum dolor sit amet, consectetur adipiscing elit.")
	} else {
		// Nothing to preview (no journal selected)
		rightBuilder.WriteString("[ Select an entry to view details ]")
	}

	// Apply styles and borders to each column
	// Left panel: border on the right side only (vertical separator)
	leftPanelStyle := lipgloss.NewStyle().
		Border(lipgloss.NormalBorder(), false, true, false, false).
		BorderForeground(lipgloss.Color(borderColor)).
		Padding(1, 2)
	leftPanel := leftPanelStyle.Width(leftWidth).Height(m.height - 2).
		Render(leftBuilder.String())

	// Middle panel: border on the right side only
	middlePanelStyle := lipgloss.NewStyle().
		Border(lipgloss.NormalBorder(), false, true, false, false).
		BorderForeground(lipgloss.Color(borderColor)).
		Padding(1, 2)
	middlePanel := middlePanelStyle.Width(middleWidth).Height(m.height - 2).
		Render(middleBuilder.String())

	// Right panel: no border (open content area)
	rightPanelStyle := lipgloss.NewStyle().Padding(1, 2)
	rightPanel := rightPanelStyle.Width(rightWidth).Height(m.height - 2).
		Render(rightBuilder.String())

	// Join the three panels horizontally (top aligned)
	columns := lipgloss.JoinHorizontal(lipgloss.Top, leftPanel, middlePanel, rightPanel)

	// Footer with usage instructions
	footer := "\n↑/↓ to navigate • Enter to select • n to create • q to quit"

	// Assemble final UI string
	return titleBar + "\n" + columns + footer
}

func loadJournals(db *sql.DB) tea.Cmd {
	// Asynchronously load journals from the database
	return func() tea.Msg {
		journals, err := memories.ListJournals(context.Background(), db, false)
		if err != nil {
			return err
		}
		return journals
	}
}

// Create and start the Bubble Tea TUI
func ShowTUI(db *sql.DB) error {
	p := tea.NewProgram(initialModel(db), tea.WithAltScreen())
	_, err := p.Run()
	return err
}
