package tui

import (
	tea "github.com/charmbracelet/bubbletea"
	"github.com/v4run/hangar/internal/config"
)

func Run(cfg *config.HangarConfig, globalCfg *config.GlobalConfig, configDir string, sshChanged bool) error {
	m := NewModel(cfg, globalCfg, configDir, sshChanged)
	p := tea.NewProgram(m, tea.WithAltScreen())
	_, err := p.Run()
	return err
}
