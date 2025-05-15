package tui

import (
	"context"
	"database/sql"
	"fmt"
	"path/filepath"
	"strconv"
	"strings"
	"time"

	"github.com/unowned-ai/recall/pkg/memories"

	textinput "github.com/charmbracelet/bubbles/textinput"
	tea "github.com/charmbracelet/bubbletea"
	"github.com/charmbracelet/lipgloss"
	"github.com/google/uuid"
)

type model struct {
	journals []memories.Journal
	entries  []memories.Entry

	currentEntry entryDetailsMsg // Currently loaded entry details

	columnFocus int // 0 = journals, 1 = entries, 2 = entry details and manipulations
	width       int // Current terminal width (for layout)
	height      int // Current terminal height
	err         error

	mcpUsage bool

	db         *sql.DB
	dbFilename string

	quitting bool

	journalCursor           int // Index of selected journal
	journalCreating         bool
	journalCreatingStep     int // 0 = editing journal name, 1 = editing journal description
	journalCreatingError    string
	journalNameInput        textinput.Model
	journalDescInput        textinput.Model
	journalDeleting         bool
	journalDeleteConfirmIdx int // 0 = "Yes" selected, 1 = "No"

	entryCursor           int // Index of selected entry
	entryDeleting         bool
	entryDeleteConfirmIdx int // 0 = "Yes" selected, 1 = "No"

	// Animation state
	marqueeOffset int
	marqueeTimer  int
}

// Initialize TUI model
func initModel(db *sql.DB) model {
	// Fetch database file path with name
	_, file := getDbPragmaList(db)

	// Initialize text input fields for the new journal form
	jtname := textinput.New()
	jtname.Placeholder = "Journal Name"
	jtname.Focus() // focus name field initially
	jtname.CharLimit = 256

	jtdesc := textinput.New()
	jtdesc.Placeholder = "Description of the journal (optional)"
	jtdesc.CharLimit = 512

	return model{
		journals: []memories.Journal{},
		entries:  []memories.Entry{},

		currentEntry: entryDetailsMsg{},

		columnFocus: 0,
		width:       0,
		height:      0,

		mcpUsage: false,

		db:         db,
		dbFilename: filepath.Base(file),

		journalCursor:    0,
		journalNameInput: jtname,
		journalDescInput: jtdesc,

		entryCursor: 0,

		marqueeOffset: 0,
		marqueeTimer:  0,
	}
}

// Execute commands concurrently with no ordering guarantees during initialization
func (m model) Init() tea.Cmd {
	return tea.Batch(
		listJournals(m.db),
		tea.Tick(marqueeTickDuration, func(t time.Time) tea.Msg {
			return t
		}),
	)
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
			return m, listEntries(m.db, m.journals[0].ID, false)
		}
		return m, nil

	case []memories.Entry:
		// Store loaded entries for the currently selected journal
		m.entries = msg
		// Reset entry selection and clear any previously loaded entry detail
		m.entryCursor = 0
		m.currentEntry = entryDetailsMsg{}
		return m, nil

	case entryDetailsMsg:
		// Store the full entry and tags in the model for the detail view
		m.currentEntry = msg
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

		if m.journalDeleting {
			// Deleting Journal Mode
			switch msg.String() {
			case "up", "k":
				m.journalDeleteConfirmIdx = 0

			case "down", "j":
				m.journalDeleteConfirmIdx = 1

			case "enter":
				if m.journalDeleteConfirmIdx == 0 {
					// Confirmed deletion of selected journal
					journalID := m.journals[m.journalCursor].ID
					err := memories.DeleteJournal(context.Background(), m.db, journalID)
					if err != nil {
						m.err = err
						m.journalDeleting = false
						return m, nil
					}
					// Remove journal from list and adjust selection
					oldIndex := m.journalCursor
					m.journals = append(m.journals[:oldIndex], m.journals[oldIndex+1:]...)

					m.journalDeleting = false

					// Adjust cursor
					if len(m.journals) > 0 {
						if oldIndex > 0 {
							m.journalCursor--
						}
						m.currentEntry = entryDetailsMsg{}
						return m, listEntries(m.db, m.journals[m.journalCursor].ID, false)
					} else {
						// No journals remaining; clear entries
						m.entries = []memories.Entry{}
						m.currentEntry = entryDetailsMsg{}
					}
				} else {
					// Chosen No, cancel deletion
					m.journalDeleting = false
				}
				return m, nil

			case "esc":
				// Cancel deletion on Escape
				m.journalDeleting = false
				return m, nil
			}
			return m, nil
		}

		if m.entryDeleting {
			// Deleting Entry Mode
			switch msg.String() {
			case "up", "k":
				m.entryDeleteConfirmIdx = 0

			case "down", "j":
				m.entryDeleteConfirmIdx = 1

			case "enter":
				if m.entryDeleteConfirmIdx == 0 {
					// Confirm deletion of selected entry
					entryID := m.entries[m.entryCursor].ID
					err := memories.DeleteEntry(context.Background(), m.db, entryID)
					if err != nil {
						m.err = err
						m.entryDeleting = false
						return m, nil
					}
					// Remove entry from list and adjust selection
					oldIndex := m.entryCursor
					m.entries = append(m.entries[:oldIndex], m.entries[oldIndex+1:]...)

					m.entryDeleting = false

					// Adjust cursor
					if len(m.entries) > 0 {
						if oldIndex > 0 {
							m.entryCursor--
						}
						m.currentEntry = entryDetailsMsg{}
						return m, getEntryDetails(m.db, m.entries[m.entryCursor].ID)
					} else {
						// No entry remaining; clear current entry, entry list, move focus to journals
						m.currentEntry = entryDetailsMsg{}
						m.entries = []memories.Entry{}
						m.columnFocus = 0
					}
					return m, nil
				} else {
					// Chosen No, cancel deletion
					m.entryDeleting = false
				}
				return m, nil

			case "esc":
				// Cancel deletion on Escape
				m.entryDeleting = false
				return m, nil
			}
			return m, nil
		}

		// Root Navigation Mode
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
				return m, listEntries(m.db, m.journals[m.journalCursor].ID, false)
			}
			if m.columnFocus == 1 && m.entryCursor > 0 {
				// Iterating over entries column
				m.entryCursor--
				return m, getEntryDetails(m.db, m.entries[m.entryCursor].ID)
			}

		case "down", "j":
			// Move selection down (stop at last item)
			if m.columnFocus == 0 && m.journalCursor < len(m.journals)-1 {
				// Iterating over journals column
				m.journalCursor++
				return m, listEntries(m.db, m.journals[m.journalCursor].ID, false)
			}

			if m.columnFocus == 1 && m.entryCursor < len(m.entries)-1 {
				// Iterating over entries column
				m.entryCursor++
				return m, getEntryDetails(m.db, m.entries[m.entryCursor].ID)
			}

		case "right", "l":
			// Move selection right to other column
			if m.columnFocus < 1 {
				if len(m.entries) > 0 {
					// Moved focus to entries - auto-select the first entry and load it
					m.columnFocus++
					m.entryCursor = 0
					return m, getEntryDetails(m.db, m.entries[0].ID)
				}

			}
			return m, nil

		case "left", "h":
			// Move selection left to other column
			if m.columnFocus > 0 {
				m.columnFocus--
			}
			return m, nil

		case "n":
			m.journalCreatingStep = 0
			m.journalNameInput.Reset()
			m.journalDescInput.Reset()
			m.journalDescInput.Blur()  // Ensure description input is not focused
			m.journalNameInput.Focus() // Make sure to focus the name input

			m.journalCreating = true

		case "d":
			if m.columnFocus == 0 && len(m.journals) > 0 {
				m.journalDeleteConfirmIdx = 1
				m.journalDeleting = true
			} else if m.columnFocus == 1 && len(m.entries) > 0 {
				m.entryDeleteConfirmIdx = 1
				m.entryDeleting = true
			}
			return m, nil
		}

	case time.Time:
		// Update marquee animation every x ticks (adjust for speed)
		m.marqueeTimer++
		if m.marqueeTimer >= 10 {
			m.marqueeTimer = 0
			m.marqueeOffset++
		}
		return m, tea.Tick(marqueeTickDuration, func(t time.Time) tea.Msg {
			return t
		})
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
	// Render the title bar (full width)
	titleBar := titleStyle.Width(m.width).Render(titleText)

	// Calculate column widths (left ~25%, middle ~25%, right ~50%)
	halfWidth := m.width / 2
	leftWidth := halfWidth / 2
	middleWidth := halfWidth - leftWidth
	rightWidth := m.width - (leftWidth + middleWidth)

	bordersAndPaddingWidth := 4

	// Update input widths to match right pane
	m.journalNameInput.Width = rightWidth - bordersAndPaddingWidth
	m.journalDescInput.Width = rightWidth - bordersAndPaddingWidth

	// Left column: Journals list and Info
	var journalsBuilder, infoBuilder strings.Builder

	// Calculate heights for the split panels (subtract 4 for borders and padding)
	quarterHeight := (m.height - bordersAndPaddingWidth) / 4

	// Build journals section
	journalsBuilder.WriteString(subtitleStyle.Width(leftWidth - bordersAndPaddingWidth).Render("  Journals"))
	journalsBuilder.WriteString("\n\n")

	if len(m.journals) == 0 {
		journalsBuilder.WriteString("No journals yet. Press 'n' to create new.\n")
	} else {
		// List each journal in the left column
		for i, journal := range m.journals {
			pointer := "  "
			itemStyle := inactiveStyle
			// Calculate available width for journal name (panel width - pointer - padding - border)
			availableWidth := leftWidth - len(pointer) - 4 - 1

			if m.journalCursor == i {
				pointer = "> "
				itemStyle = selectedStyle

				// Handle marquee animation for selected journal
				journalName := journal.Name
				if len(journalName) > availableWidth {
					// Create a padded version for scrolling
					paddedName := journalName + "    " + journalName
					offset := m.marqueeOffset % (len(journalName) + 4) // 4 is padding space
					if offset+availableWidth <= len(paddedName) {
						journalName = paddedName[offset : offset+availableWidth]
					}
				}
				journalName = lipgloss.NewStyle().
					MaxWidth(availableWidth).
					Render(journalName)
				journalsBuilder.WriteString(pointer + itemStyle.Render(journalName) + "\n")
			} else {
				// Normal truncation for non-selected journals
				journalName := journal.Name
				if len(journalName) > availableWidth {
					if availableWidth > 3 {
						journalName = fmt.Sprintf("%s..", journalName[:availableWidth-2])
					}
				}
				journalName = lipgloss.NewStyle().
					MaxWidth(availableWidth).
					Render(journalName)
				journalsBuilder.WriteString(pointer + itemStyle.Render(journalName) + "\n")
			}
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
		BorderForeground(lipgloss.Color(colorGray)).
		Padding(0, 2)
	journalsPanel := journalsPanelStyle.Width(leftWidth).Height(quarterHeight * 3).
		Render(journalsBuilder.String())

	// Style and render the info panel (bottom)
	infoPanelStyle := lipgloss.NewStyle().
		Border(lipgloss.NormalBorder(), false, true, false, false).
		BorderForeground(lipgloss.Color(colorGray)).
		Padding(1, 2)
	infoPanel := infoPanelStyle.Width(leftWidth).Height(quarterHeight).
		Render(infoBuilder.String())

	// Combine the panels vertically
	leftPanel := lipgloss.JoinVertical(lipgloss.Left, journalsPanel, infoPanel)

	// Middle column: Entries list
	var middleBuilder strings.Builder
	middleBuilder.WriteString(subtitleStyle.Width(middleWidth - bordersAndPaddingWidth).Render("  Entries"))
	middleBuilder.WriteString("\n\n")

	if m.journalCursor <= len(m.journals) {
		// If a journal is selected
		if len(m.entries) == 0 {
			middleBuilder.WriteString("  No entries yet.\n")
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

				// Calculate available width for entry title (panel width - pointer - padding - border)
				availableWidth := middleWidth - len(pointer) - 4 - 1
				entryTitle := entry.Title
				if len(entryTitle) > availableWidth {
					if availableWidth > 3 {
						entryTitle = fmt.Sprintf("%s..", entryTitle[:availableWidth-2]) // Minus 2 dots
					}
				}

				entryTitle = lipgloss.NewStyle().
					MaxWidth(availableWidth).
					Render(entryTitle)

				middleBuilder.WriteString(pointer + itemStyle.Render(entryTitle) + "\n")
			}
		}
	} else {
		// No journal selected (e.g., currently on "+ New Journal")
		middleBuilder.WriteString("  No journal selected.\n")
	}

	// Right column: Entry preview or New Journal form
	var rightBuilder strings.Builder

	rightBuilderSubtitleText := "Entry"
	if m.journalCreating {
		rightBuilderSubtitleText = "Create New Journal"
	}
	if m.journalDeleting {
		rightBuilderSubtitleText = "Delete Journal"
	}
	if m.entryDeleting {
		rightBuilderSubtitleText = "Delete Entry"
	}
	rightBuilder.WriteString(subtitleStyle.Width(rightWidth - bordersAndPaddingWidth).Render(rightBuilderSubtitleText))
	rightBuilder.WriteString("\n\n")

	if m.journalCreating {
		// Show the form for creating a new journal
		rightBuilder.WriteString("Name: " + m.journalNameInput.View() + "\n")
		rightBuilder.WriteString("Description: " + m.journalDescInput.View() + "\n\n")
		rightBuilder.WriteString("(enter to submit, esc to cancel)")

		if m.journalCreatingError != "" {
			rightBuilder.WriteString("\n\n" +
				lipgloss.NewStyle().Foreground(lipgloss.Color(colorRed)).
					Render(m.journalCreatingError) + "\n")
		}
	} else if m.journalDeleting {
		// Show delete confirmation prompt
		rightBuilder.WriteString("Name: " + lipgloss.NewStyle().Foreground(lipgloss.Color(colorRed)).
			Render(m.journals[m.journalCursor].Name) + "\n\n")
		yesOpt, noOpt := "Yes", "No"
		if m.journalDeleteConfirmIdx == 0 {
			yesOpt = dangerSelectedStyle.Render(" >" + yesOpt)
			noOpt = inactiveStyle.Render("  " + noOpt)
		} else {
			yesOpt = inactiveStyle.Render("  " + yesOpt)
			noOpt = selectedStyle.Render(" >" + noOpt)
		}
		rightBuilder.WriteString(fmt.Sprintf("%s\n%s\n\n", yesOpt, noOpt))
		rightBuilder.WriteString("(enter to confirm, esc to cancel, up/down to switch)")
	} else if m.entryDeleting {
		// Show delete confirmation prompt
		rightBuilder.WriteString("Title: " + lipgloss.NewStyle().Foreground(lipgloss.Color(colorRed)).
			Render(m.entries[m.entryCursor].Title) + "\n\n")
		yesOpt, noOpt := "Yes", "No"
		if m.entryDeleteConfirmIdx == 0 {
			yesOpt = dangerSelectedStyle.Render(" >" + yesOpt)
			noOpt = inactiveStyle.Render("  " + noOpt)
		} else {
			yesOpt = inactiveStyle.Render("  " + yesOpt)
			noOpt = selectedStyle.Render(" >" + noOpt)
		}
		rightBuilder.WriteString(fmt.Sprintf("%s\n%s\n\n", yesOpt, noOpt))
		rightBuilder.WriteString("(enter to confirm, esc to cancel, up/down to switch)")
	} else if len(m.journals) > 0 && m.journalCursor <= len(m.journals) {
		if m.currentEntry.entry.ID != uuid.Nil {
			rightBuilder.WriteString(
				lipgloss.NewStyle().Bold(true).
					Render(lipgloss.NewStyle().Foreground(lipgloss.Color(colorBlue)).Render("Title: ")+lipgloss.NewStyle().Foreground(lipgloss.Color(colorWhite)).Render(m.currentEntry.entry.Title)) + "\n\n")

			// Tags for the entry
			var tagsLine string
			if len(m.currentEntry.tags) > 0 {
				tags := []string{}
				for _, tag := range m.currentEntry.tags {
					tags = append(tags, tag.Tag)
				}
				tagsLine += strings.Join(tags, " ")
			} else {
				tagsLine += "-"
			}
			rightBuilder.WriteString(lipgloss.NewStyle().Foreground(lipgloss.Color(colorBlue)).Render("Tags: ") + lipgloss.NewStyle().Foreground(lipgloss.Color(colorPurple)).Render(tagsLine) + "\n\n")

			// Entry content
			rightBuilder.WriteString(lipgloss.NewStyle().Foreground(lipgloss.Color(colorWhite)).Render(m.currentEntry.entry.Content))
		} else {
			rightBuilder.WriteString("Select an entry to view details.")
		}
	} else {
		// Nothing to preview (no journal selected)
		rightBuilder.WriteString("Select a journal to view details.")
	}

	panelHeightPadding := 3

	// Left panel: border on the right side and horizontal split to journal list and info section
	leftPanelStyle := lipgloss.NewStyle().
		Border(lipgloss.NormalBorder(), false, true, false, false).
		BorderForeground(lipgloss.Color(colorGray)).
		Padding(0, 2)
	leftPanelStyle.Width(leftWidth).Height(m.height - panelHeightPadding).
		Render(leftPanel)

	// Middle panel: border on the right side only
	middlePanelStyle := lipgloss.NewStyle().
		Border(lipgloss.NormalBorder(), false, true, false, false).
		BorderForeground(lipgloss.Color(colorGray)).
		Padding(0, 2)
	middlePanel := middlePanelStyle.Width(middleWidth).Height(m.height - panelHeightPadding).
		Render(middleBuilder.String())

	// Right panel: no border (open content area)
	rightPanelStyle := lipgloss.NewStyle().Padding(0, 2)
	rightPanel := rightPanelStyle.Width(rightWidth).Height(m.height - panelHeightPadding).
		Render(rightBuilder.String())

	// Join the three panels horizontally (top aligned)
	columns := lipgloss.JoinHorizontal(lipgloss.Top, leftPanel, middlePanel, rightPanel)

	// Footer with usage instructions
	footerText := "\n↑/↓ to navigate • Enter to select • n to create • d to delete • q to quit"
	// Render the footer bar (full width)
	footerBar := footerStyle.Width(m.width).Render(footerText)

	// Assemble final UI string
	return titleBar + "\n\n" + columns + footerBar
}

// Create and start the Bubble Tea TUI
func ShowTUI(db *sql.DB) error {
	p := tea.NewProgram(initModel(db), tea.WithAltScreen())
	_, err := p.Run()
	return err
}
