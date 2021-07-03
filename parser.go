package main

import (
	"fmt"
	"io"
)

// state function
type stateFn func(*fsm, []byte) stateFn

// state machine
type fsm struct {
	state  stateFn
	buffer []byte
	out    io.Writer
	quit   chan struct{}
	data   <-chan []byte

	pending []byte
}

func NewParser(data <-chan []byte, writer io.Writer, quit chan struct{}) *fsm {
	return &fsm{
		out:  writer,
		data: data,
		quit: quit,
	}
}

func (m *fsm) Parse() {
	var line []byte
	for m.state = normal; m.state != nil; {
		select {
		case <-m.quit:
			m.flush()
			m.state = nil
		case line = <-m.data:
			m.state = m.state(m, line)
		}
	}
}

func (m *fsm) flush() {
	if len(m.pending) > 0 {
		fmt.Fprintf(m.out, string(m.pending)+"\n")
		m.pending = m.pending[:0]
	}
}

func isBlank(data []byte) bool {
	return len(data) == 0
}

func isHeader(data []byte) bool {
	return len(data) > 0 && data[0] == '#'
}

func triggerBreak(data []byte) bool {
	return len(data) == 0 || data[len(data)-1] == '.'
}

func isFence(data []byte) bool {
	return len(data) >= 3 && string(data[0:3]) == "```"
}

func needsFence(data []byte) bool {
	return len(data) >= 4 && string(data[0:4]) == "    "
}

func normal(m *fsm, data []byte) stateFn {
	m.flush()
	// blank line
	if isBlank(data) {
		fmt.Fprintf(m.out, "\n")
		return normal
	}
	// header
	if isHeader(data) {
		fmt.Fprintf(m.out, string(data)+"\n")
		return normal
	}
	if isFence(data) {
		fmt.Fprintf(m.out, string(data)+"\n")
		return fence
	}
	if needsFence(data) {
		fmt.Fprintf(m.out, string("```")+"\n")
		fmt.Fprintf(m.out, string(data)+"\n")
		m.pending = []byte("```")
		return toFence
	}
	if data[len(data)-1] != '.' {
		m.buffer = append(m.buffer, data...)
		return paragraph
	}
	// TODO
	// find links
	// collapse lists
	fmt.Fprintf(m.out, string(data)+"\n")

	return normal
}

func fence(m *fsm, data []byte) stateFn {
	fmt.Fprintf(m.out, string(data)+"\n")
	if isFence(data) {
		return normal
	}
	return fence
}

func toFence(m *fsm, data []byte) stateFn {
	if needsFence(data) {
		fmt.Fprintf(m.out, string(data)+"\n")
		return toFence
	}
	fmt.Fprintf(m.out, string(data)+"\n")
	return normal
}

func paragraph(m *fsm, data []byte) stateFn {
	if triggerBreak(data) {
		m.buffer = append(m.buffer, data...)
		fmt.Fprintf(m.out, string(m.buffer)+"\n")
		m.buffer = m.buffer[:0]
		return normal
	}
	m.buffer = append(m.buffer, data...)
	return paragraph
}
