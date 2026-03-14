package tui

import "github.com/charmbracelet/lipgloss"

var (
	titleStyle = lipgloss.NewStyle().
			Bold(true).
			Foreground(lipgloss.Color("15")).
			Background(lipgloss.Color("62")).
			Padding(0, 1)

	statusHealthy  = lipgloss.NewStyle().Foreground(lipgloss.Color("42")).Bold(true)
	statusDegraded = lipgloss.NewStyle().Foreground(lipgloss.Color("214")).Bold(true)
	statusDown     = lipgloss.NewStyle().Foreground(lipgloss.Color("196")).Bold(true)

	labelStyle = lipgloss.NewStyle().Bold(true).Foreground(lipgloss.Color("252"))
	valueStyle = lipgloss.NewStyle().Foreground(lipgloss.Color("250"))
	dimStyle   = lipgloss.NewStyle().Foreground(lipgloss.Color("240"))

	sparkGreen  = lipgloss.NewStyle().Foreground(lipgloss.Color("42"))
	sparkYellow = lipgloss.NewStyle().Foreground(lipgloss.Color("214"))
	sparkRed    = lipgloss.NewStyle().Foreground(lipgloss.Color("196"))
	sparkGray   = lipgloss.NewStyle().Foreground(lipgloss.Color("240"))

	outageStyle = lipgloss.NewStyle().Foreground(lipgloss.Color("245"))

	helpStyle = lipgloss.NewStyle().Foreground(lipgloss.Color("241"))

	splashLogoStyle = lipgloss.NewStyle().Foreground(lipgloss.Color("62")).Bold(true)
	splashSubStyle  = lipgloss.NewStyle().Foreground(lipgloss.Color("240"))

	borderStyle = lipgloss.NewStyle().
			Border(lipgloss.RoundedBorder()).
			BorderForeground(lipgloss.Color("62")).
			Padding(1, 2)
)
