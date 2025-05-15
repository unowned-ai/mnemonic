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
	entryCreating         bool
	entryCreatingStep     int // 0 = editing entry title, 1 = editing entry content
	entryCreatingError    string
	entryTitleInput       textinput.Model
	entryContentInput     textinput.Model
	entryDeleting         bool
	entryDeleteConfirmIdx int // 0 = "Yes" selected, 1 = "No"

	// Animation state
	marqueeOffset int
	marqueeTimer  int

	// Paddings, offsets, element dimensions
	pointerLen             int
	bordersAndPaddingWidth int
	panelHeightPadding     int
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

	// Initialize text input fields for the new entry form
	ettitle := textinput.New()
	ettitle.Placeholder = "Entry Title"
	ettitle.Focus() // focus name field initially
	ettitle.CharLimit = 256

	etcont := textinput.New()
	etcont.Placeholder = "Entry content"
	etcont.CharLimit = 10240

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

		entryCursor:       0,
		entryTitleInput:   ettitle,
		entryContentInput: etcont,

		marqueeOffset: 0,
		marqueeTimer:  0,

		pointerLen:             2,
		bordersAndPaddingWidth: 4,
		panelHeightPadding:     3,
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
						return m, nil
					}

					// Exit create mode and reset form inputs
					m.journalCreating = false
					m.journalCreatingStep = 0
					m.journalNameInput.Reset()
					m.journalDescInput.Reset()

					// Prepend new journal to the list and focus it
					m.journals = append([]memories.Journal{journal}, m.journals...)
					m.journalCursor = 0 // highlight the newly created journal
					m.columnFocus = 0

					// Empty entry list and current entry
					m.entries = []memories.Entry{}
					m.currentEntry = entryDetailsMsg{}
					return m, nil
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

		if m.entryCreating {
			switch msg.Type {
			case tea.KeyEnter:
				if m.entryCreatingStep == 0 {
					// Validate that the entry title is not empty
					if m.entryTitleInput.Value() == "" {
						m.entryCreatingError = "Entry title cannot be empty"
						return m, nil
					}

					// Press Enter on name field -> move to content field
					m.entryCreatingError = ""
					m.entryCreatingStep = 1
					m.entryTitleInput.Blur()
					m.entryContentInput.Focus()
				} else {
					// Press Enter on content field -> submit the form (create entry)
					entry, err := memories.CreateEntry(context.Background(), m.db, m.journals[m.journalCursor].ID,
						m.entryTitleInput.Value(), m.entryContentInput.Value(), "text/plain")
					if err != nil {
						m.err = err
						return m, nil
					}

					// Exit create mode and reset form inputs
					m.entryCreating = false
					m.entryCreatingStep = 0
					m.entryTitleInput.Reset()
					m.entryContentInput.Reset()

					// Prepend new entry to the list and focus it
					m.entries = append([]memories.Entry{entry}, m.entries...)
					m.entryCursor = 0 // highlight the newly created entry
					m.columnFocus = 1 // focus the entries column

					// Empty old current entry and fetch details of newly created
					m.currentEntry = entryDetailsMsg{}
					return m, getEntryDetails(m.db, m.entries[m.entryCursor].ID)
				}

			case tea.KeyEsc:
				// Cancel entry creation and reset form inputs
				m.entryCreating = false
				m.entryCreatingStep = 0
				m.entryTitleInput.Reset()
				m.entryContentInput.Reset()
			}

			// If still in creating mode, route character input to the appropriate text field
			var cmd tea.Cmd
			if m.entryCreatingStep == 0 {
				m.entryTitleInput, cmd = m.entryTitleInput.Update(msg)
			} else {
				m.entryContentInput, cmd = m.entryContentInput.Update(msg)
			}
			return m, cmd
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
			if m.columnFocus < 1 {
				m.entryCursor = 0
				m.columnFocus++
				if len(m.entries) > 0 {
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
			if m.columnFocus == 0 {
				m.journalCreatingStep = 0
				m.journalNameInput.Reset()
				m.journalDescInput.Reset()
				m.journalDescInput.Blur()  // Ensure description input is not focused
				m.journalNameInput.Focus() // Make sure to focus the name input
				m.journalCreating = true
			} else if m.columnFocus == 1 {
				m.entryCreatingStep = 0
				m.entryTitleInput.Reset()
				m.entryContentInput.Reset()
				m.entryContentInput.Blur()
				m.entryTitleInput.Focus()
				m.entryCreating = true
			}

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

	// Title Bar
	titleText := "Recall - datastore for memories"
	titleBar := titleStyle.Width(m.width).Render(titleText)

	// Calculate column widths (left ~25%, middle ~25%, right ~50%)
	halfWidth := m.width / 2
	leftWidth := halfWidth / 2
	middleWidth := halfWidth - leftWidth
	rightWidth := m.width - (leftWidth + middleWidth)

	// Update input widths to match right pane
	m.journalNameInput.Width = rightWidth - m.bordersAndPaddingWidth
	m.journalDescInput.Width = rightWidth - m.bordersAndPaddingWidth
	m.entryTitleInput.Width = rightWidth - m.bordersAndPaddingWidth
	m.entryContentInput.Width = rightWidth - m.bordersAndPaddingWidth

	// Left Column: Journals list and Info panel
	var journalsBuilder, infoBuilder strings.Builder

	// Calculate heights for the split panels (subtract 4 for borders and padding)
	quarterHeight := (m.height - m.bordersAndPaddingWidth) / 4

	// Build journals section
	journalsBuilder.WriteString(subtitleStyle.Width(leftWidth - m.bordersAndPaddingWidth).Render("  Journals"))
	journalsBuilder.WriteString("\n\n")

	if len(m.journals) == 0 {
		journalsBuilder.WriteString("No journals yet. Press 'n' to create new.\n")
	} else {
		// List each journal in the left column
		for i, journal := range m.journals {
			// Calculate available width for journal name (panel width - pointer - padding - border)
			availableWidth := leftWidth - m.pointerLen - 4 - 1

			if i == m.journalCursor {
				// Selected journal is highlighted
				m.ViewListElemMarquee(journal.Name, &journalsBuilder, availableWidth)
			} else {
				m.ViewListElemNormal(journal.Name, &journalsBuilder, availableWidth)
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

	// Middle Column - Entries list
	var middleBuilder strings.Builder
	middleBuilder.WriteString(subtitleStyle.Width(middleWidth - m.bordersAndPaddingWidth).Render("  Entries"))
	middleBuilder.WriteString("\n\n")

	if len(m.entries) == 0 {
		middleBuilder.WriteString("  No entries yet.\n")
	} else {
		for i, entry := range m.entries {
			// Calculate available width for entry title (panel width - pointer - padding - border)
			availableWidth := middleWidth - m.pointerLen - 4 - 1

			if i == m.entryCursor && m.columnFocus == 1 {
				// Selected entry is highlighted
				m.ViewListElemMarquee(entry.Title, &middleBuilder, availableWidth)
			} else {
				m.ViewListElemNormal(entry.Title, &middleBuilder, availableWidth)
			}
		}
	}

	// Right Column: Entry preview or New elements form
	var rightBuilder strings.Builder

	rightBuilderSubtitleText := "Entry"
	if m.journalCreating {
		rightBuilderSubtitleText = "Create New Journal"
	}
	if m.journalDeleting {
		rightBuilderSubtitleText = "Delete Journal"
	}
	if m.entryCreating {
		rightBuilderSubtitleText = "Create New Entry"
	}
	if m.entryDeleting {
		rightBuilderSubtitleText = "Delete Entry"
	}
	rightBuilder.WriteString(subtitleStyle.Width(rightWidth - m.bordersAndPaddingWidth).Render(rightBuilderSubtitleText))
	rightBuilder.WriteString("\n\n")

	if m.journalCreating {
		// Show the form for creating a new journal
		rightBuilder.WriteString(elemTitleHeaderStyle.Render("Name: ") + m.journalNameInput.View() + "\n")
		rightBuilder.WriteString(elemTitleHeaderStyle.Render("Description: ") + m.journalDescInput.View() + "\n\n")
		rightBuilder.WriteString("(enter to submit, esc to cancel)")

		if m.journalCreatingError != "" {
			rightBuilder.WriteString("\n\n" +
				textRedStyle.
					Render(m.journalCreatingError) + "\n")
		}
	} else if m.journalDeleting {
		// Show delete confirmation prompt for journal
		rightBuilder.WriteString(elemTitleHeaderStyle.Render("Name: ") + textStyle.
			Render(m.journals[m.journalCursor].Name) + "\n\n")
		yesOpt, noOpt := "Yes", "No"
		if m.journalDeleteConfirmIdx == 0 {
			yesOpt = dangerSelectedStyle.Render(generateLinePointer(true, m.pointerLen) + yesOpt)
			noOpt = textStyle.Render(generateLinePointer(false, m.pointerLen) + noOpt)
		} else {
			yesOpt = textStyle.Render(generateLinePointer(false, m.pointerLen) + yesOpt)
			noOpt = selectedStyle.Render(generateLinePointer(true, m.pointerLen) + noOpt)
		}
		rightBuilder.WriteString(fmt.Sprintf("%s\n%s\n\n", yesOpt, noOpt))
		rightBuilder.WriteString("(enter to confirm, esc to cancel, up/down to switch)")
	} else if m.entryCreating {
		// Show the form for creating a new entry
		rightBuilder.WriteString(elemTitleHeaderStyle.Render("Title: ") + m.entryTitleInput.View() + "\n")
		rightBuilder.WriteString(elemTitleHeaderStyle.Render("Content: ") + m.entryContentInput.View() + "\n\n")
		rightBuilder.WriteString("(enter to submit, esc to cancel)")

		if m.entryCreatingError != "" {
			rightBuilder.WriteString("\n\n" +
				textRedStyle.
					Render(m.entryCreatingError) + "\n")
		}
	} else if m.entryDeleting {
		// Show delete confirmation prompt for entry
		rightBuilder.WriteString(elemTitleHeaderStyle.Render("Title: ") + textStyle.
			Render(m.entries[m.entryCursor].Title) + "\n\n")
		yesOpt, noOpt := "Yes", "No"
		if m.entryDeleteConfirmIdx == 0 {
			yesOpt = dangerSelectedStyle.Render(generateLinePointer(true, m.pointerLen) + yesOpt)
			noOpt = textStyle.Render(generateLinePointer(false, m.pointerLen) + noOpt)
		} else {
			yesOpt = textStyle.Render(generateLinePointer(false, m.pointerLen) + yesOpt)
			noOpt = selectedStyle.Render(generateLinePointer(true, m.pointerLen) + noOpt)
		}
		rightBuilder.WriteString(fmt.Sprintf("%s\n%s\n\n", yesOpt, noOpt))
		rightBuilder.WriteString("(enter to confirm, esc to cancel, up/down to switch)")
	} else if len(m.journals) > 0 && m.journalCursor <= len(m.journals) {
		if m.currentEntry.entry.ID != uuid.Nil {
			rightBuilder.WriteString(elemTitleHeaderStyle.Render("Title: ") + textStyle.Render(m.currentEntry.entry.Title) + "\n\n")

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
			rightBuilder.WriteString(elemTitleHeaderStyle.Render("Tags: ") + multiElemsTitleStyle.Render(tagsLine) + "\n\n")

			// Entry content
			rightBuilder.WriteString(textStyle.Render(m.currentEntry.entry.Content))
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
		BorderForeground(lipgloss.Color(colorGray)).
		Padding(0, 2)
	leftPanelStyle.Width(leftWidth).Height(m.height - m.panelHeightPadding).
		Render(leftPanel)

	// Middle panel: border on the right side only
	middlePanelStyle := lipgloss.NewStyle().
		Border(lipgloss.NormalBorder(), false, true, false, false).
		BorderForeground(lipgloss.Color(colorGray)).
		Padding(0, 2)
	middlePanel := middlePanelStyle.Width(middleWidth).Height(m.height - m.panelHeightPadding).
		Render(middleBuilder.String())

	// Right panel: no border (open content area)
	rightPanelStyle := lipgloss.NewStyle().Padding(0, 2)
	rightPanel := rightPanelStyle.Width(rightWidth).Height(m.height - m.panelHeightPadding).
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

// View of normal truncation for non-selected list element
func (m model) ViewListElemNormal(elemName string, builder *strings.Builder, availableWidth int) {
	if len(elemName) > availableWidth {
		if availableWidth > 3 {
			elemName = fmt.Sprintf("%s..", elemName[:availableWidth-2]) // Minus 2 dots
		}
	}
	elemName = lipgloss.NewStyle().
		MaxWidth(availableWidth).
		Render(elemName)
	builder.WriteString(generateLinePointer(false, m.pointerLen) + textStyle.Render(elemName) + "\n")
}

// View of marquee truncation for selected list element
func (m model) ViewListElemMarquee(elemName string, builder *strings.Builder, availableWidth int) {
	if len(elemName) > availableWidth {
		elemName = marqueeText(elemName, m.marqueeOffset, m.bordersAndPaddingWidth, availableWidth)
	}
	elemName = lipgloss.NewStyle().
		MaxWidth(availableWidth).
		Render(elemName)
	builder.WriteString(generateLinePointer(true, m.pointerLen) + selectedStyle.Render(elemName) + "\n")
}

// Create and start the Bubble Tea TUI
func ShowTUI(db *sql.DB) error {
	p := tea.NewProgram(initModel(db), tea.WithAltScreen())
	_, err := p.Run()
	return err
}
