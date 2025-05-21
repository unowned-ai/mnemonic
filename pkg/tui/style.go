package tui

import (
	"strings"
	"time"

	"github.com/charmbracelet/lipgloss"
)

// UI styles and layout settings
// Color palette "Blue Moon" from https://gogh-co.github.io/Gogh/
const (
	colorGray     = "#353b52"
	colorWhite    = "#ffffff"
	colorGreen    = "#acfab4"
	colorGreenDim = "#b4c4b4"
	colorRed      = "#e61f44"
	colorRedDim   = "#d06178"
	colorPurple   = "#b9a3eb"
	colorBlue     = "#89ddff"

	marqueeTickDuration = time.Duration(time.Second / 20)
)

var (
	titleStyle = lipgloss.NewStyle().Bold(true).
			Foreground(lipgloss.Color(colorBlue)).
			Background(lipgloss.Color(colorGray)).
			Padding(0, 2).Align(lipgloss.Center)
	subtitleStyle = lipgloss.NewStyle().Bold(true).
			Foreground(lipgloss.Color(colorBlue))
	selectedStyle = lipgloss.NewStyle().
			Foreground(lipgloss.Color(colorGray)).
			Background(lipgloss.Color(colorGreen))
	dangerSelectedStyle = lipgloss.NewStyle().
				Foreground(lipgloss.Color(colorGray)).
				Background(lipgloss.Color(colorRed))
	textStyle    = lipgloss.NewStyle().Foreground(lipgloss.Color(colorWhite))
	textRedStyle = lipgloss.NewStyle().Foreground(lipgloss.Color(colorRed))

	elemTitleHeaderStyle = lipgloss.NewStyle().Foreground(lipgloss.
				Color(colorBlue))
	multiElemsTitleStyle = lipgloss.NewStyle().
				Foreground(lipgloss.Color(colorPurple))

	// Specific border styles will be defined for panels in the View function
	footerStyle = lipgloss.NewStyle().
			Foreground(lipgloss.Color(colorGray))
)

// Function to colorize text based on its status
// 0 (default) - unknown, 1 - green, 2 - red
func TextStatusColorize(text string, status int) string {
	switch status {
	case 1:
		return lipgloss.NewStyle().Foreground(lipgloss.Color(colorGreenDim)).Render(text)
	case 2:
		return lipgloss.NewStyle().Foreground(lipgloss.Color(colorRedDim)).Render(text)
	default:
		return lipgloss.NewStyle().Foreground(lipgloss.Color(colorGray)).Render(text)
	}
}

// Generates pointer symbol when line in focus
func generateLinePointer(isPoint bool, length int) string {
	if isPoint {
		return ">" + strings.Repeat(" ", length-1)
	}
	return strings.Repeat(" ", length)
}

// Create a padded version marquee text for scrolling
func (m model) marqueeText(text string, availableWidth int) string {
	paddedText := text + "    " + text
	offset := m.marqueeOffset % (len(text) + m.bordersAndPaddingWidth)
	if offset+availableWidth <= len(paddedText) {
		text = paddedText[offset : offset+availableWidth]
	}
	return text
}

func (m model) dynamicColumnWidth() (int, int, int) {
	var leftWidth, middleWidth, rightWidth int
	if m.dynamicWidth {
		// Dynamic widths based on focus
		switch m.columnFocus {
		case 0: // Journals column focused
			leftWidth = (m.width * 30) / 100   // 30%
			middleWidth = (m.width * 40) / 100 // 40%
			rightWidth = (m.width * 30) / 100  // 30%
		case 1: // Entries column focused
			leftWidth = (m.width * 20) / 100   // 20%
			middleWidth = (m.width * 40) / 100 // 40%
			rightWidth = (m.width * 40) / 100  // 40%
		case 2: // Entry details focused
			leftWidth = (m.width * 20) / 100   // 20%
			middleWidth = (m.width * 20) / 100 // 20%
			rightWidth = (m.width * 60) / 100  // 60%
		}
	} else {
		// Fixed widths (25%, 25%, 50%)
		halfWidth := m.width / 2
		leftWidth = halfWidth / 2                        // 25%
		middleWidth = halfWidth - leftWidth              // 25%
		rightWidth = m.width - (leftWidth + middleWidth) // 50%
	}
	return leftWidth, middleWidth, rightWidth
}
