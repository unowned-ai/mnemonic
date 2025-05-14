package tui

import (
	"context"
	"database/sql"
	"fmt"
	"path/filepath"
	"strconv"
	"strings"

	textinput "github.com/charmbracelet/bubbles/textinput"
	tea "github.com/charmbracelet/bubbletea"
	"github.com/charmbracelet/lipgloss"
	"github.com/google/uuid"
	"github.com/unowned-ai/recall/pkg/memories"
)

// UI styles and layout settings
// Color palette "Blue Moon" from https://gogh-co.github.io/Gogh/
const (
	colorBorder = "#353b52"
	colorError  = "#e61f44"

	colorGreen    = "#acfab4"
	colorGreenDim = "#b4c4b4"
	colorRedDim   = "#d06178"
)

var (
	titleStyle = lipgloss.NewStyle().Bold(true).
			Foreground(lipgloss.Color("#89ddff")).
			Background(lipgloss.Color("#353b52")).
			Padding(0, 2).Align(lipgloss.Center)
	selectedStyle = lipgloss.NewStyle().
			Foreground(lipgloss.Color("#353b52")).
			Background(lipgloss.Color(colorGreen))
	inactiveStyle = lipgloss.NewStyle().Foreground(lipgloss.Color("#ffffff"))
	// Specific border styles will be defined for panels in the View function
	footerStyle = lipgloss.NewStyle().
			Foreground(lipgloss.Color(colorBorder))
)

type model struct {
	journals     []memories.Journal
	entries      []memories.Entry
	tags         []memories.Tag
	currentEntry memories.Entry // Currently loaded entry detail

	columnFocus int // 0 = journals, 1 = entries, 2 = entry
	width       int // Current terminal width (for layout)
	height      int // Current terminal height
	err         error

	mcpUsage bool

	db         *sql.DB
	dbFilename string

	quitting bool

	journalCursor        int // Index of selected journal
	journalCreating      bool
	journalCreatingStep  int // 0 = editing journal name, 1 = editing journal description
	journalCreatingError string
	journalNameInput     textinput.Model
	journalDescInput     textinput.Model

	entryCursor int // Index of selected entry
	// entryCreating ...
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

	// Fetch database file path with name
	var name, file string
	err := db.QueryRow(`PRAGMA database_list`).Scan(new(int), &name, &file)
	if err != nil {
		return model{}
	}

	fileName := filepath.Base(file)

	return model{
		journals:     []memories.Journal{},
		entries:      []memories.Entry{},
		tags:         []memories.Tag{},
		currentEntry: memories.Entry{},

		columnFocus: 0,
		width:       0,
		height:      0,

		mcpUsage: false,

		db:         db,
		dbFilename: fileName,

		journalCursor:    0,
		journalNameInput: jtin,
		journalDescInput: jtdesc,

		entryCursor: 0,
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
		if len(m.journals) > 0 {
			// Load entries for the first journal
			return m, loadEntries(m.db, m.journals[0].ID, false)
		}
		return m, nil

	case []memories.Entry:
		// Store loaded entries for the currently selected journal
		m.entries = msg
		// Reset entry selection and clear any previously loaded entry detail
		m.entryCursor = 0
		m.currentEntry = memories.Entry{}
		m.tags = []memories.Tag{}
		return m, nil

	case entryDetailMsg:
		// Store the full entry and tags in the model for the detail view
		m.currentEntry = msg.entry
		m.tags = msg.tags
		return m, nil

	// Handle key presses for navigation and input
	case tea.KeyMsg:
		if m.journalCreating {
			// Creating New Journal Mode
			switch msg.Type {
			case tea.KeyEnter:
				if m.journalCreatingStep == 0 {
					// Validate that the journal name is not empty
					if m.journalNameInput.Value() == "" {
						m.journalCreatingError = "Journal name cannot be empty"
						return m, nil
					}

					// Press Enter on name field -> move to description field
					m.journalCreatingError = ""
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
						m.journalCursor = 0 // highlight the newly created journal
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
			// Exit alt screen before quitting so the goodbye message displays
			return m, tea.Sequence(tea.ExitAltScreen, tea.Quit)

		case "up", "k":
			// Move selection up (stop at top)
			if m.columnFocus == 0 && m.journalCursor > 0 {
				// Iterating over journals column
				m.journalCursor--
				return m, loadEntries(m.db, m.journals[m.journalCursor].ID, false)
			}
			if m.columnFocus == 1 && m.entryCursor > 0 {
				// Iterating over entries column
				m.entryCursor--
				return m, loadEntryDetail(m.db, m.entries[m.entryCursor].ID)
			}

		case "down", "j":
			// Move selection down (stop at last item)
			if m.columnFocus == 0 && m.journalCursor < len(m.journals)-1 {
				// Iterating over journals column
				m.journalCursor++
				return m, loadEntries(m.db, m.journals[m.journalCursor].ID, false)
			}

			if m.columnFocus == 1 && m.entryCursor < len(m.entries)-1 {
				// Iterating over entries column
				m.entryCursor++
				return m, loadEntryDetail(m.db, m.entries[m.entryCursor].ID)
			}

		case "right", "l":
			// Move selection right to other column
			if m.columnFocus < 2 {
				m.columnFocus++
			}
			if m.columnFocus == 1 && len(m.entries) > 0 {
				// Moved focus to entries - auto-select the first entry and load it
				m.entryCursor = 0
			}
			return m, loadEntryDetail(m.db, m.entries[0].ID)

		case "left", "h":
			// Move selection left to other column
			if m.columnFocus > 0 {
				m.columnFocus--
			}

			return m, nil

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

	// Left column: Journals list and Info
	var journalsBuilder, infoBuilder strings.Builder

	// Calculate heights for the split panels (subtract 4 for borders and padding)
	quarterHeight := (m.height - 4) / 4

	// Build journals section
	if len(m.journals) == 0 {
		journalsBuilder.WriteString("No journals yet. Press 'n' to create new.\n")
	} else {
		// List each journal in the left column
		for i, journal := range m.journals {
			// Default: no pointer, inactive (grey) text
			pointer := "  "
			itemStyle := inactiveStyle
			if m.journalCursor == i {
				// Highlighted journal (cursor position)
				pointer = "> "
				itemStyle = selectedStyle
			}
			journalsBuilder.WriteString(pointer + itemStyle.Render(journal.Name) + "\n")
		}
	}

	// Build info section
	var mcpServerStatus, databaseStatus int
	if m.mcpUsage {
		mcpServerStatus = 1
	}
	if m.dbFilename != "" {
		databaseStatus = 1
	}

	infoBuilder.WriteString(fmt.Sprintf("MCP server status: %v\nDatabase file: %v\n",
		TextStatusColorize(strconv.FormatBool(m.mcpUsage), mcpServerStatus),
		TextStatusColorize(m.dbFilename, databaseStatus)))

	// Style and render the journals panel (top)
	journalsPanelStyle := lipgloss.NewStyle().
		Border(lipgloss.NormalBorder(), false, true, true, false).
		BorderForeground(lipgloss.Color(colorBorder)).
		Padding(1, 2)
	journalsPanel := journalsPanelStyle.Width(leftWidth).Height(quarterHeight * 3).
		Render(journalsBuilder.String())

	// Style and render the info panel (bottom)
	infoPanelStyle := lipgloss.NewStyle().
		Border(lipgloss.NormalBorder(), false, true, false, false).
		BorderForeground(lipgloss.Color(colorBorder)).
		Padding(1, 2)
	infoPanel := infoPanelStyle.Width(leftWidth).Height(quarterHeight).
		Render(infoBuilder.String())

	// Combine the panels vertically
	leftPanel := lipgloss.JoinVertical(lipgloss.Left, journalsPanel, infoPanel)

	// Middle column: Entries list
	var middleBuilder strings.Builder
	if m.journalCursor <= len(m.journals) {
		// If a journal is selected
		if len(m.entries) == 0 {
			middleBuilder.WriteString("No entries yet.\n")
		} else {
			for i, entry := range m.entries {
				pointer := "  "
				itemStyle := inactiveStyle
				if i == m.entryCursor && m.columnFocus != 0 {
					// Selected entry is highlighted; pointer if entries column is focused
					if m.columnFocus == 1 {
						pointer = "> "
					}
					// TODO: Handle if we focus 3rd column entry editor, not reduce selected entry highlight
					itemStyle = selectedStyle
				}
				title := entry.Title
				middleBuilder.WriteString(pointer + itemStyle.Render(title) + "\n")
			}
		}
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

		if m.journalCreatingError != "" {
			rightBuilder.WriteString("\n" +
				lipgloss.NewStyle().Foreground(lipgloss.Color(colorError)).
					Render(m.journalCreatingError) + "\n")
		}
	} else if len(m.journals) > 0 && m.journalCursor <= len(m.journals) {
		if m.currentEntry.ID != uuid.Nil {
			rightBuilder.WriteString(
				lipgloss.NewStyle().Bold(true).
					Render("Title: "+m.currentEntry.Title) + "\n\n")

			// Tags for the entry
			tagsLine := "Tags: "
			if len(m.tags) > 0 {
				tags := []string{}
				for _, tag := range m.tags {
					tags = append(tags, tag.Tag)
				}
				tagsLine += strings.Join(tags, " ")
			} else {
				tagsLine += "-"
			}
			rightBuilder.WriteString(tagsLine + "\n\n")

			// Entry content
			rightBuilder.WriteString(m.currentEntry.Content)
		} else {
			rightBuilder.WriteString("Select an entry to view details.")
		}
	} else {
		// Nothing to preview (no journal selected)
		rightBuilder.WriteString("Select a journal to view details.")
	}

	// Left panel: border on the right side and horizontal split to journal list and info section
	leftPanelStyle := lipgloss.NewStyle().
		Border(lipgloss.NormalBorder(), false, true, false, false).
		BorderForeground(lipgloss.Color(colorBorder)).
		Padding(1, 2)
	leftPanelStyle.Width(leftWidth).Height(m.height - 3).
		Render(leftPanel)

	// Middle panel: border on the right side only
	middlePanelStyle := lipgloss.NewStyle().
		Border(lipgloss.NormalBorder(), false, true, false, false).
		BorderForeground(lipgloss.Color(colorBorder)).
		Padding(1, 2)
	middlePanel := middlePanelStyle.Width(middleWidth).Height(m.height - 3).
		Render(middleBuilder.String())

	// Right panel: no border (open content area)
	rightPanelStyle := lipgloss.NewStyle().Padding(1, 2)
	rightPanel := rightPanelStyle.Width(rightWidth).Height(m.height - 3).
		Render(rightBuilder.String())

	// Join the three panels horizontally (top aligned)
	columns := lipgloss.JoinHorizontal(lipgloss.Top, leftPanel, middlePanel, rightPanel)

	// Footer with usage instructions
	footerText := "\n↑/↓ to navigate • Enter to select • n to create • q to quit"
	// Render the footer bar (full width)
	footerBar := footerStyle.Width(m.width).Render(footerText)

	// Assemble final UI string
	return titleBar + "\n\n" + columns + footerBar
}

// Load journals from the database
func loadJournals(db *sql.DB) tea.Cmd {
	return func() tea.Msg {
		journals, err := memories.ListJournals(context.Background(), db, false)
		if err != nil {
			return err
		}
		return journals
	}
}

// Load entries for journal from the database
func loadEntries(db *sql.DB, journalID uuid.UUID, includeDeleted bool) tea.Cmd {
	return func() tea.Msg {
		entries, err := memories.ListEntries(context.Background(), db, journalID, includeDeleted)
		if err != nil {
			return err
		}
		return entries
	}
}

type entryDetailMsg struct {
	entry memories.Entry
	tags  []memories.Tag
}

func loadEntryDetail(db *sql.DB, entryID uuid.UUID) tea.Cmd {
	return func() tea.Msg {
		entry, err := memories.GetEntry(context.Background(), db, entryID)
		if err != nil {
			return err
		}
		tags, err := memories.ListTagsForEntry(context.Background(), db, entry.ID)
		if err != nil {
			return err
		}
		// Return a combined message with the entry and its tags
		return entryDetailMsg{entry: entry, tags: tags}
	}
}

// Function to colorize text based on its status
// 0 (default) - unknown, 1 - green, 2 - red
func TextStatusColorize(text string, status int) string {
	switch status {
	case 1:
		return lipgloss.NewStyle().Foreground(lipgloss.Color(colorGreenDim)).Render(text)
	case 2:
		return lipgloss.NewStyle().Foreground(lipgloss.Color(colorRedDim)).Render(text)
	default:
		return lipgloss.NewStyle().Foreground(lipgloss.Color(colorBorder)).Render(text)
	}
}

// Create and start the Bubble Tea TUI
func ShowTUI(db *sql.DB) error {
	p := tea.NewProgram(initialModel(db), tea.WithAltScreen())
	_, err := p.Run()
	return err
}
