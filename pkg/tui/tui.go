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
	viewport "github.com/charmbracelet/bubbles/viewport"
	tea "github.com/charmbracelet/bubbletea"
	"github.com/charmbracelet/lipgloss"
	"github.com/google/uuid"
)

type model struct {
	journals []memories.Journal
	entries  []memories.Entry

	currentEntry entryDetailsMsg // Currently loaded entry details

	contentViewport   viewport.Model
	contentEditing    bool // true if entry content is in edit mode
	editCursorPos     int  // cursor position in content (rune index)
	editCursorVisible bool // whether cursor is shown

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
	entryCreatingStep     int // 0 = editing entry title, 1 = editing entry tags, 2 = editing entry content
	entryCreatingError    string
	entryTitleInput       textinput.Model
	entryContentInput     textinput.Model
	entryTagsInput        textinput.Model
	entryDeleting         bool
	entryDeleteConfirmIdx int // 0 = "Yes" selected, 1 = "No"

	dynamicWidth bool // Toggle for dynamic column widths

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

	ettags := textinput.New()
	ettags.Placeholder = "Tags (comma or space separated)"
	ettags.CharLimit = 1024

	vp := viewport.New(0, 0)
	vp.YPosition = 0
	vp.SetContent("")

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
		entryTagsInput:    ettags,

		contentViewport: vp,

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
	var cmds []tea.Cmd

	switch msg := msg.(type) {
	case tea.WindowSizeMsg:
		// Save the new window size in the model for responsive layout
		m.width = msg.Width
		m.height = msg.Height

		// Calculate content viewport height (subtract space used by title and tags)
		contentHeight := m.height - m.panelHeightPadding - 6 // 6 lines for title and tags sections
		// TODO: make title and tags section height dynamic
		if contentHeight < 0 {
			contentHeight = 0
		}

		// Calculate viewport width based on dynamic width setting
		_, _, viewportWidth := m.dynamicColumnWidth()

		// Update viewport dimensions
		m.contentViewport.Width = viewportWidth - m.bordersAndPaddingWidth
		m.contentViewport.Height = contentHeight
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
		// Initialize viewport with the content when entry is loaded
		wrappedContent := lipgloss.NewStyle().
			Width(m.contentViewport.Width). // Set width to force wrapping
			Render(textStyle.Render(m.currentEntry.entry.Content))
		m.contentViewport.SetContent(wrappedContent)
		m.contentViewport.GotoTop()

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

					// Press Enter on title field -> move to tags field
					m.entryCreatingError = ""
					m.entryCreatingStep = 1
					m.entryTitleInput.Blur()
					m.entryTagsInput.Focus()
				} else if m.entryCreatingStep == 1 {
					// Press Enter on tags field -> move to content field
					m.entryCreatingError = ""
					m.entryCreatingStep = 2
					m.entryTagsInput.Blur()
					m.entryContentInput.Focus()
				} else {
					// Press Enter on content field -> submit the form (create entry)
					entry, err := memories.CreateEntry(context.Background(), m.db, m.journals[m.journalCursor].ID,
						m.entryTitleInput.Value(), m.entryContentInput.Value(), "text/plain")
					if err != nil {
						m.err = err
						return m, nil
					}

					// Process tags if any were entered
					if m.entryTagsInput.Value() != "" {
						// Split tags by comma or space
						tags := strings.FieldsFunc(m.entryTagsInput.Value(), func(r rune) bool {
							return r == ',' || r == ' '
						})

						// Create each tag
						for _, tag := range tags {
							tag = strings.TrimSpace(tag)
							if tag != "" {
								err := memories.TagEntry(context.Background(), m.db, entry.ID, tag)
								if err != nil {
									m.err = fmt.Errorf("error creating tag '%s': %v", tag, err)
									return m, nil
								}
							}
						}
					}

					// Exit create mode and reset form inputs
					m.entryCreating = false
					m.entryCreatingStep = 0
					m.entryTitleInput.Reset()
					m.entryContentInput.Reset()
					m.entryTagsInput.Reset()

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
				m.entryTagsInput.Reset()
			}

			// If still in creating mode, route character input to the appropriate text field
			var cmd tea.Cmd
			if m.entryCreatingStep == 0 {
				m.entryTitleInput, cmd = m.entryTitleInput.Update(msg)
			} else if m.entryCreatingStep == 1 {
				m.entryTagsInput, cmd = m.entryTagsInput.Update(msg)
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

		// If we're in the content view and an entry is loaded, handle viewport scrolling
		if m.columnFocus == 2 && m.currentEntry.entry.ID != uuid.Nil {
			var cmd tea.Cmd

			if m.contentEditing {
				// In edit mode, don't pass key events to viewport
				switch msg.Type {
				case tea.KeyRunes:
					// Insert typed characters at cursor position
					runes := []rune(m.currentEntry.entry.Content)
					if m.editCursorPos >= len(runes) {
						// At or past the end, just append
						runes = append(runes, msg.Runes...)
					} else {
						// Insert at cursor position without duplication
						newRunes := make([]rune, 0, len(runes)+len(msg.Runes))
						newRunes = append(newRunes, runes[:m.editCursorPos]...)
						newRunes = append(newRunes, msg.Runes...)
						newRunes = append(newRunes, runes[m.editCursorPos:]...)
						runes = newRunes
					}
					m.currentEntry.entry.Content = string(runes)
					m.editCursorPos += len(msg.Runes)
					m.editCursorVisible = true
				case tea.KeyBackspace:
					if m.editCursorPos > 0 {
						runes := []rune(m.currentEntry.entry.Content)
						before := runes[:m.editCursorPos-1]
						after := runes[m.editCursorPos:]
						m.currentEntry.entry.Content = string(append(before, after...))
						m.editCursorPos--
						m.editCursorVisible = true
					}
				case tea.KeyDelete:
					runes := []rune(m.currentEntry.entry.Content)
					if m.editCursorPos < len(runes) {
						before := runes[:m.editCursorPos]
						after := runes[m.editCursorPos+1:]
						m.currentEntry.entry.Content = string(append(before, after...))
						m.editCursorVisible = true
					}
				case tea.KeyLeft:
					if m.editCursorPos > 0 {
						m.editCursorPos--
						m.editCursorVisible = true
					}
				case tea.KeyRight:
					runes := []rune(m.currentEntry.entry.Content)
					if m.editCursorPos < len(runes) {
						m.editCursorPos++
						m.editCursorVisible = true
					}
				case tea.KeyUp:
					// Find the previous line's equivalent position
					content := m.currentEntry.entry.Content
					runes := []rune(content)
					currentLine := getLineNumber(content, m.editCursorPos)
					if currentLine > 0 {
						// Find start of current line
						lineStart := m.editCursorPos
						for lineStart > 0 && runes[lineStart-1] != '\n' {
							lineStart--
						}
						// Find start of previous line
						prevLineStart := lineStart - 1
						for prevLineStart > 0 && runes[prevLineStart-1] != '\n' {
							prevLineStart--
						}

						// First move cursor
						offset := m.editCursorPos - lineStart
						if prevLineStart+offset < lineStart {
							m.editCursorPos = prevLineStart + offset
						} else {
							m.editCursorPos = lineStart - 1
						}
						m.editCursorVisible = true

						// Then check if we need to scroll
						cursorLine := getLineNumber(content, m.editCursorPos)
						if cursorLine < m.contentViewport.YOffset {
							m.contentViewport.ScrollUp(1)
						}
					}
				case tea.KeyDown:
					// Find the next line's equivalent position
					content := m.currentEntry.entry.Content
					runes := []rune(content)
					// Find start of current line
					lineStart := m.editCursorPos
					for lineStart > 0 && runes[lineStart-1] != '\n' {
						lineStart--
					}
					// Find start of next line
					nextLineStart := m.editCursorPos
					for nextLineStart < len(runes) && runes[nextLineStart] != '\n' {
						nextLineStart++
					}
					if nextLineStart < len(runes) {
						nextLineStart++ // Move past the newline
						// Calculate position in next line
						offset := m.editCursorPos - lineStart
						nextLineEnd := nextLineStart
						for nextLineEnd < len(runes) && runes[nextLineEnd] != '\n' {
							nextLineEnd++
						}
						if nextLineStart+offset < nextLineEnd {
							m.editCursorPos = nextLineStart + offset
						} else {
							m.editCursorPos = nextLineEnd
						}
						m.editCursorVisible = true

						// Check if we need to scroll the viewport
						cursorLine := getLineNumber(content, m.editCursorPos)
						visibleLines := m.contentViewport.Height
						if cursorLine >= m.contentViewport.YOffset+visibleLines {
							m.contentViewport.ScrollDown(1)
						}
					}
				case tea.KeyEnter:
					// Insert newline at cursor position
					runes := []rune(m.currentEntry.entry.Content)
					if m.editCursorPos == len(runes) {
						// At the end, just append newline
						runes = append(runes, '\n')
					} else {
						// Insert newline before current character
						before := runes[:m.editCursorPos]
						after := runes[m.editCursorPos:]
						runes = append(before, append([]rune{'\n'}, after...)...)
					}
					m.currentEntry.entry.Content = string(runes)
					m.editCursorPos++
					m.editCursorVisible = true

					// Update viewport content with cursor
					updateContentWithCursor(&m)
				case tea.KeyEsc:
					// Exit edit mode and update entry in database
					updatedEntry, err := memories.UpdateEntry(context.Background(), m.db,
						m.currentEntry.entry.ID,
						m.currentEntry.entry.Title,
						m.currentEntry.entry.Content,
						m.currentEntry.entry.ContentType)
					if err != nil {
						m.err = fmt.Errorf("failed to update entry: %v", err)
						return m, nil
					}
					m.currentEntry.entry = updatedEntry
					m.contentEditing = false
					// Update the entry in the entries list as well
					for i := range m.entries {
						if m.entries[i].ID == updatedEntry.ID {
							m.entries[i] = updatedEntry
							break
						}
					}
				}
				// Update viewport content with cursor
				updateContentWithCursor(&m)
				return m, tea.Batch(cmds...)
			} else {
				// In normal mode, only pass navigation keys to viewport
				switch msg.Type {
				case tea.KeyUp, tea.KeyDown, tea.KeyHome, tea.KeyEnd:
					m.contentViewport, cmd = m.contentViewport.Update(msg)
					cmds = append(cmds, cmd)
				}
			}

			// Handle mode switching and other commands
			switch msg.String() {
			case "enter", "i":
				if !m.contentEditing {
					m.contentEditing = true
					m.editCursorPos = 0
					m.editCursorVisible = true
					m.contentViewport.GotoTop()
					updateContentWithCursor(&m)
				}
			case "left", "h":
				if !m.contentEditing {
					m.columnFocus--
				}
			case "q", "ctrl+c":
				m.quitting = true
				// Exit alt screen before quitting so the goodbye message displays
				return m, tea.Sequence(tea.ExitAltScreen, tea.Quit)
			}
			return m, tea.Batch(cmds...)
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
			// Don't move right if we're already at the rightmost column
			if m.columnFocus >= 2 {
				return m, nil
			}

			// Moving from journals to entries
			if m.columnFocus == 0 {
				// Only allow moving to entries if we have a journal selected
				if len(m.journals) == 0 {
					return m, nil
				}
				m.entryCursor = 0
				m.columnFocus++
				// If there are entries, load the first entry's details
				if len(m.entries) > 0 {
					return m, getEntryDetails(m.db, m.entries[0].ID)
				}
				return m, nil
			}

			// Moving from entries to details
			if m.columnFocus == 1 {
				// Only allow moving to details if we have an entry selected
				if len(m.entries) == 0 {
					return m, nil
				}
				m.columnFocus++
				return m, nil
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

		case "z":
			if m.journalCreating || m.entryCreating {
				return m, nil
			}
			// Toggle dynamic width mode
			m.dynamicWidth = !m.dynamicWidth
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

	return m, tea.Batch(cmds...)
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

	// Calculate column widths based on focus
	leftWidth, middleWidth, rightWidth := m.dynamicColumnWidth()

	// Account for rounding errors to ensure total width matches screen width
	remainder := m.width - (leftWidth + middleWidth + rightWidth)
	rightWidth += remainder

	// Update input widths to match right pane
	m.journalNameInput.Width = rightWidth - m.bordersAndPaddingWidth
	m.journalDescInput.Width = rightWidth - m.bordersAndPaddingWidth
	m.entryTitleInput.Width = rightWidth - m.bordersAndPaddingWidth
	m.entryContentInput.Width = rightWidth - m.bordersAndPaddingWidth
	m.entryTagsInput.Width = rightWidth - m.bordersAndPaddingWidth

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

			if i == m.journalCursor && m.columnFocus >= 0 {
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

			if i == m.entryCursor && m.columnFocus >= 1 {
				// Selected entry is highlighted
				m.ViewListElemMarquee(entry.Title, &middleBuilder, availableWidth)
			} else {
				m.ViewListElemNormal(entry.Title, &middleBuilder, availableWidth)
			}
		}
	}

	// Right Column: Entry preview or New elements form
	var rightBuilder strings.Builder
	var entryTitleBuilder, entryTagsBuilder strings.Builder

	rightBuilderSubtitleText := "Entry"
	if m.columnFocus == 2 && m.currentEntry.entry.ID != uuid.Nil {
		rightBuilderSubtitleText = "Entry (view mode)"
		if m.contentEditing {
			rightBuilderSubtitleText = "Entry (edit mode)"
		}
	}
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
		rightBuilder.WriteString(elemTitleHeaderStyle.Render("Tags: ") + m.entryTagsInput.View() + "\n")
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
			// Title section
			entryTitleBuilder.WriteString(elemTitleHeaderStyle.Render("Title: ") + textStyle.Render(m.currentEntry.entry.Title) + "\n\n")

			// Tags section
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
			entryTagsBuilder.WriteString(elemTitleHeaderStyle.Render("Tags: ") + multiElemsTitleStyle.Render(tagsLine) + "\n\n")

			// Combine all sections into rightBuilder
			rightBuilder.WriteString(entryTitleBuilder.String())
			rightBuilder.WriteString(entryTagsBuilder.String())
			rightBuilder.WriteString(m.contentViewport.View())
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
	footerText := "\n↑/↓ to navigate • n to create • d to delete • i to edit • z to toggle layout • esc to apply and exit edit mode • q to quit"
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
		elemName = m.marqueeText(elemName, availableWidth)
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

// Prepare content string with cursor and update viewport
func updateContentWithCursor(m *model) {
	content := m.currentEntry.entry.Content
	runes := []rune(content)

	if m.editCursorPos > len(runes) {
		m.editCursorPos = len(runes)
	}

	var display string
	if m.contentEditing && m.editCursorVisible {
		if len(runes) == 0 {
			// If content is empty, show cursor
			display = lipgloss.NewStyle().
				Background(lipgloss.Color(colorWhite)).
				Foreground(lipgloss.Color(colorGray)).
				Render(" ")
		} else if m.editCursorPos == len(runes) {
			// If cursor is at the end, append it
			display = content + lipgloss.NewStyle().
				Background(lipgloss.Color(colorWhite)).
				Foreground(lipgloss.Color(colorGray)).
				Render(" ")
		} else {
			// Highlight the current character by inverting its colors
			before := string(runes[:m.editCursorPos])
			cursorChar := string(runes[m.editCursorPos])
			after := string(runes[m.editCursorPos+1:])

			// Special handling for newline and carriage return
			var invertedCursor string
			if cursorChar == "\n" || cursorChar == "\r" {
				// Show a visible character for newline/carriage return
				invertedCursor = lipgloss.NewStyle().
					Background(lipgloss.Color(colorWhite)).
					Render(" ") // Use pilcrow sign to represent newline
				// Keep the actual newline after the cursor
				display = before + invertedCursor + cursorChar + after
			} else {
				// Normal character handling
				invertedCursor = lipgloss.NewStyle().
					Background(lipgloss.Color(colorWhite)).
					Foreground(lipgloss.Color(colorGray)).
					Render(cursorChar)
				display = before + invertedCursor + after
			}
		}
	} else {
		display = content
	}

	// Update viewport content with word wrap
	wrappedContent := lipgloss.NewStyle().
		Width(m.contentViewport.Width). // Set width to force wrapping
		Render(textStyle.Render(display))
	m.contentViewport.SetContent(wrappedContent)

	// Check if cursor is beyond viewport and scroll if needed
	cursorLine := getLineNumber(content, m.editCursorPos)
	lastVisibleLine := m.contentViewport.YOffset + m.contentViewport.Height - 1

	if cursorLine > lastVisibleLine {
		m.contentViewport.ScrollDown(1)
	}
}

// Count lines before cursor position
func getLineNumber(content string, pos int) int {
	runes := []rune(content)
	if pos > len(runes) {
		pos = len(runes)
	}

	lineCount := 0
	for i := 0; i < pos; i++ {
		if runes[i] == '\n' {
			lineCount++
		}
	}
	return lineCount
}
