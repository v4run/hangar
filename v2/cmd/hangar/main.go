// Command hangar — v2 TUI for SSH / database connection management.
//
// Install:
//
//	go install github.com/v4run/hangar/v2/cmd/hangar@latest
//
// Config lives at ~/.hangar/v2/config.yaml. On first run with a v1
// config present at ~/.hangar/config.yaml, that file is read, migrated
// in memory, and the result written to the v2 path. The v1 file is
// never modified.
package main

import (
	"fmt"
	"os"

	tea "github.com/charmbracelet/bubbletea"

	"github.com/v4run/hangar/v2/internal/config"
	"github.com/v4run/hangar/v2/internal/tui"
)

func main() {
	if len(os.Args) > 1 {
		switch os.Args[1] {
		case "--help", "-h", "help":
			printUsage()
			return
		case "--version", "-v", "version":
			fmt.Println("hangar v2.0.0-alpha.1")
			return
		}
	}

	dir, err := config.DefaultDir()
	if err != nil {
		die("resolve config dir: %v", err)
	}
	cfg, err := config.Load(dir)
	if err != nil {
		die("load config: %v", err)
	}

	m := tui.New(cfg, dir)
	p := tea.NewProgram(m, tea.WithAltScreen(), tea.WithMouseCellMotion())
	if _, err := p.Run(); err != nil {
		die("run: %v", err)
	}
}

func printUsage() {
	fmt.Println(`hangar v2 — connection manager

  hangar              launch the TUI
  hangar version      print the version
  hangar help         print this help

Config:
  ~/.hangar/v2/config.yaml   (schema 2; migrated from v1 on first run)`)
}

func die(format string, args ...any) {
	fmt.Fprintf(os.Stderr, "hangar: "+format+"\n", args...)
	os.Exit(1)
}
