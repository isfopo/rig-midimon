# rig-midimon

Standalone MIDI monitor for the rig platform.

Lists system MIDI input ports, opens one, and reports traffic as counters and
an activity lamp. Prints raw channel/type/number/value for each event so the
identifiers needed for config-driven mappings (see rig-core/midi) can be
discovered.

## Build

Requires Go >= 1.24 and a C compiler for CGO (RtMidi):

    go test ./...
    go vet ./...
    make

## Usage

    rig-midimon                  # real hardware ports
    rig-midimon -engine test     # fake loopback ports
    rig-midimon -watch 5s        # headless event print

Keys: up/down select, enter open/close, c close, r refresh, q quit.