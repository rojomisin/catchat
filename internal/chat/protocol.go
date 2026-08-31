// Package chat implements Catchat's deliberately small, line-delimited JSON
// protocol. It is an application protocol carried inside Tailcat's encrypted
// TCP connection.
package chat

import (
	"bufio"
	"encoding/json"
	"errors"
	"io"
	"strings"
	"time"
)

const maxFrameBytes = 8 << 10

const (
	TypeHello   = "hello"
	TypeWelcome = "welcome"
	TypeMessage = "message"
	TypeSystem  = "system"
	TypeUsers   = "users"
)

// Frame is one protocol message. Frames are JSON values, one per line.
type Frame struct {
	Type  string    `json:"type"`
	Nick  string    `json:"nick,omitempty"`
	Text  string    `json:"text,omitempty"`
	Users []string  `json:"users,omitempty"`
	Time  time.Time `json:"time,omitempty"`
}

// ReadFrame reads one bounded frame. The bound protects a hub from a peer that
// sends an unbounded line before JSON decoding begins.
func ReadFrame(r *bufio.Reader) (Frame, error) {
	line, err := r.ReadString('\n')
	if err != nil {
		if errors.Is(err, io.EOF) && len(line) > 0 {
			return Frame{}, errors.New("unterminated frame")
		}
		return Frame{}, err
	}
	if len(line) > maxFrameBytes {
		return Frame{}, errors.New("frame exceeds 8 KiB")
	}
	var frame Frame
	if err := json.Unmarshal([]byte(line), &frame); err != nil {
		return Frame{}, err
	}
	return frame, nil
}

// WriteFrame writes a single line-delimited JSON frame.
func WriteFrame(w io.Writer, frame Frame) error {
	return json.NewEncoder(w).Encode(frame)
}

// CleanText removes controls that could manipulate a terminal and normalizes
// surrounding whitespace. Newlines are rendered as spaces in this MVP.
func CleanText(text string) string {
	text = strings.Map(func(r rune) rune {
		switch {
		case r == '\n' || r == '\r' || r == '\t':
			return ' '
		case r < 0x20 || r == 0x7f:
			return -1
		default:
			return r
		}
	}, text)
	return strings.TrimSpace(text)
}
