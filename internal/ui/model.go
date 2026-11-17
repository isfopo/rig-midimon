// Package ui holds the standalone MIDI monitor TUI for rig-midimon.
package ui

import (
	"fmt"
	"strings"

	tea "github.com/charmbracelet/bubbletea"

	lamps "github.com/isfopo/rig-components/lamps"
	list "github.com/isfopo/rig-components/list"

	"github.com/isfopo/rig-core/midi"
	gmid "gitlab.com/gomidi/midi/v2"
)

// Model is a MIDI monitor: list input ports, open one, watch traffic.
type Model struct {
	width, height int

	eng      *midi.Engine
	ports    []string
	cursor   int
	portList list.Model

	monitor *midi.Monitor
	openNum int

	lamps lamps.Model

	events  uint64
	clocks  uint64
	dropped uint64
	lastRaw string
	status  string
	isErr   bool

	heightSet bool
}

func New(eng *midi.Engine) Model {
	return Model{
		eng:      eng,
		monitor:  midi.NewMonitor(eng),
		portList: list.New(nil),
		lamps:    lamps.New([]lamps.Spec{{Name: "in", Color: "42"}, {Name: "out", Color: "205"}}, 0),
		openNum:  -1,
		status:   "ready",
		height:   24,
		width:    80,
	}
}

func (m Model) Shutdown() *midi.Engine {
	if m.monitor != nil {
		m.monitor.Stop()
	}
	return m.eng
}

// ---- messages ----
type portsMsg struct {
	ports []string
	err   error
}
type portOpenedMsg struct{ num int }
type portOpenErrMsg struct {
	num int
	err error
}
type portClosedMsg struct{ num int }
type eventMsg struct{ Event midi.Event }
type eventsStoppedMsg struct{}

// ---- commands ----
func (m Model) portsCmd() tea.Cmd {
	return func() tea.Msg {
		ports, err := m.eng.InputPorts()
		return portsMsg{ports: ports, err: err}
	}
}
func openPortCmd(mon *midi.Monitor, num int) tea.Cmd {
	return func() tea.Msg {
		if err := mon.Open(num); err != nil {
			return portOpenErrMsg{num: num, err: err}
		}
		return portOpenedMsg{num: num}
	}
}
func closePortCmd(mon *midi.Monitor, num int) tea.Cmd {
	return func() tea.Msg {
		mon.Stop()
		return portClosedMsg{num: num}
	}
}
func waitForEventCmd(ch <-chan midi.Event) tea.Cmd {
	return func() tea.Msg {
		ev, ok := <-ch
		if !ok {
			return eventsStoppedMsg{}
		}
		return eventMsg{Event: ev}
	}
}

func (m Model) Init() tea.Cmd {
	return m.portsCmd()
}

func (m Model) Update(msg tea.Msg) (tea.Model, tea.Cmd) {
	switch msg := msg.(type) {
	case tea.WindowSizeMsg:
		m.width, m.height = msg.Width, msg.Height
		m.heightSet = true
		return m, nil

	case portsMsg:
		if msg.err != nil {
			m.status = fmt.Sprintf("port error: %v", msg.err)
			m.isErr = true
			return m, nil
		}
		m.ports = msg.ports
		m.portList = m.portList.SetItems(msg.ports)
		if len(msg.ports) == 0 {
			m.portList.Cursor = 0
		}
		m.cursor = m.portList.Cursor
		m.status = fmt.Sprintf("%d input ports", len(msg.ports))
		return m, nil

	case portOpenedMsg:
		m.openNum = msg.num
		m.portList = m.portList.SetActive(msg.num)
		if msg.num >= 0 && msg.num < len(m.ports) {
			m.status = "listening: " + m.ports[msg.num]
		}
		m.isErr = false
		return m, waitForEventCmd(m.monitor.Events())

	case portOpenErrMsg:
		m.status = fmt.Sprintf("open %d: %v", msg.num, msg.err)
		m.isErr = true
		return m, nil

	case portClosedMsg:
		m.openNum = -1
		m.portList = m.portList.SetActive(-1)
		m.status = fmt.Sprintf("closed %d", msg.num)
		return m, nil

	case eventMsg:
		ev := msg.Event
		m.events++
		if ev.Msg.Type() == gmid.TimingClockMsg {
			m.clocks++
		}
		m.dropped = m.monitor.Dropped()
		m.lamps.Light("in")
		m.lastRaw = describeRaw(ev)
		// schedule lamp-off repaint
		cmd := waitForEventCmd(m.monitor.Events())
		return m, m.lamps.WithLampOff(cmd)

	case eventsStoppedMsg:
		return m, nil

	case tea.KeyMsg:
		switch msg.String() {
		case "up", "k":
			m.portList = m.portList.Move(-1)
			m.cursor = m.portList.Cursor
			return m, nil
		case "down", "j":
			m.portList = m.portList.Move(1)
			m.cursor = m.portList.Cursor
			return m, nil
		case "enter", " ":
			if len(m.ports) == 0 {
				return m, nil
			}
			idx := m.portList.Cursor
			if m.openNum == idx {
				// close
				return m, closePortCmd(m.monitor, idx)
			}
			if m.openNum >= 0 {
				// close current first
				m.monitor.Stop()
			}
			return m, openPortCmd(m.monitor, idx)
		case "c":
			if m.openNum >= 0 {
				return m, closePortCmd(m.monitor, m.openNum)
			}
			return m, nil
		case "r":
			return m, m.portsCmd()
		case "q", "ctrl+c":
			return m, tea.Quit
		}
	}

	// update lamps model for LampOffMsg
	nm, cmd := m.lamps.Update(msg)
	if lm, ok := nm.(lamps.Model); ok {
		m.lamps = lm
	}
	return m, cmd
}

func (m Model) View() string {
	var b strings.Builder
	b.WriteString("rig-midimon")
	if m.lamps.Names() != nil {
		b.WriteString("  ")
		b.WriteString(m.lamps.View())
	}
	b.WriteString("\n\n")

	if len(m.ports) == 0 {
		b.WriteString("no input ports\n\n")
	} else {
		b.WriteString("midi in ports:\n")
		b.WriteString(m.portList.View())
		b.WriteString("\n")
	}

	b.WriteString(fmt.Sprintf("events=%d clocks=%d dropped=%d\n", m.events, m.clocks, m.dropped))
	if m.lastRaw != "" {
		b.WriteString("last: " + m.lastRaw + "\n")
	}
	b.WriteString("\n")
	b.WriteString(m.status + "\n\n")
	if m.isErr {
		b.WriteString("(error)\n")
	}
	b.WriteString("up/down select  enter open/close  c close  r refresh  q quit\n")
	return b.String()
}

// describeRaw renders the channel/type/number/value of an event for mapping discovery.
func describeRaw(ev midi.Event) string {
	msg := ev.Msg
	switch msg.Type() {
	case gmid.NoteOnMsg:
		var ch, note, vel uint8
		if msg.GetNoteStart(&ch, &note, &vel) {
			return fmt.Sprintf("NoteOn  ch=%d note=%d vel=%d", ch+1, note, vel)
		}
	case gmid.NoteOffMsg:
		var ch, note, vel uint8
		if msg.GetNoteOff(&ch, &note, &vel) {
			return fmt.Sprintf("NoteOff ch=%d note=%d vel=%d", ch+1, note, vel)
		}
	case gmid.ControlChangeMsg:
		var ch, cc, val uint8
		if msg.GetControlChange(&ch, &cc, &val) {
			return fmt.Sprintf("CC      ch=%d cc=%d val=%d", ch+1, cc, val)
		}
	}
	return msg.Type().String()
}
