// Command rig-midimon is a standalone MIDI monitor.
package main

import (
	"context"
	"flag"
	"fmt"
	"os"
	"time"

	tea "github.com/charmbracelet/bubbletea"

	"github.com/isfopo/rig-core/midi"
	"github.com/isfopo/rig-midimon/internal/ui"
)

func main() {
	engineName := flag.String("engine", "rtmidi", "midi driver: rtmidi | test")
	watch := flag.Duration("watch", 0, "headless mode: print events for the given duration")
	flag.Parse()

	eng, err := newEngine(*engineName)
	if err != nil {
		fmt.Fprintln(os.Stderr, "rig-midimon:", err)
		os.Exit(1)
	}

	if *watch > 0 {
		code := runWatch(eng, *watch)
		_ = eng.Close()
		os.Exit(code)
	}

	model := ui.New(eng)
	program := tea.NewProgram(model, tea.WithAltScreen())
	final, err := program.Run()
	if err != nil {
		fmt.Fprintln(os.Stderr, "rig-midimon:", err)
		os.Exit(1)
	}
	if m, ok := final.(ui.Model); ok {
		e := m.Shutdown()
		_ = e.Close()
	} else {
		_ = eng.Close()
	}
}

func newEngine(name string) (*midi.Engine, error) {
	switch name {
	case "rtmidi":
		return midi.NewRtMidiEngine()
	case "test":
		return midi.NewTestEngine()
	default:
		return nil, fmt.Errorf("unknown engine %q (want rtmidi | test)", name)
	}
}

func runWatch(eng *midi.Engine, d time.Duration) int {
	mon := midi.NewMonitor(eng)
	defer mon.Stop()
	if err := mon.Open(0); err != nil {
		fmt.Fprintln(os.Stderr, "rig-midimon: open port 0:", err)
		return 1
	}
	fmt.Printf("watching %s port 0 for %s...\n", eng.Name(), d)
	ctx, cancel := context.WithTimeout(context.Background(), d)
	defer cancel()
	for {
		select {
		case ev, ok := <-mon.Events():
			if !ok {
				return 0
			}
			fmt.Println(ev.String())
		case <-ctx.Done():
			fmt.Println("watch ended")
			return 0
		}
	}
}
