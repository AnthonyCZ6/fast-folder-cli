package tui

import "charm.land/lipgloss/v2"

// Estilos. Se usan los 16 colores básicos para respetar el tema de la terminal
// y funcionar también en la consola clásica de Windows.
var (
	accent      = lipgloss.Color("6")
	styleTitle  = lipgloss.NewStyle().Bold(true).Foreground(lipgloss.Color("0")).Background(accent).Padding(0, 1)
	styleDim    = lipgloss.NewStyle().Foreground(lipgloss.Color("8"))
	styleFocus  = lipgloss.NewStyle().Bold(true).Foreground(accent)
	styleName   = lipgloss.NewStyle().Bold(true)
	styleCursor = lipgloss.NewStyle().Bold(true).Foreground(lipgloss.Color("0")).Background(accent)
	styleOK     = lipgloss.NewStyle().Foreground(lipgloss.Color("2"))
	styleWarn   = lipgloss.NewStyle().Foreground(lipgloss.Color("3"))
	styleErr    = lipgloss.NewStyle().Foreground(lipgloss.Color("1"))
	styleKey    = lipgloss.NewStyle().Bold(true)
)
