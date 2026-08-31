// Package ui provides Catchat's small BitchX-inspired terminal interface.
package ui

import (
	"bufio"
	"fmt"
	"io"
	"net"
	"os"
	"strings"
	"time"
	"unicode/utf8"

	"github.com/rogermicone/catchat/internal/chat"
	"golang.org/x/term"
)

const (
	reset  = "\x1b[0m"
	bold   = "\x1b[1m"
	dim    = "\x1b[2m"
	cyan   = "\x1b[38;5;51m"
	green  = "\x1b[38;5;114m"
	yellow = "\x1b[38;5;221m"
	gray   = "\x1b[38;5;245m"
)

type inputEvent struct {
	line   string
	submit bool
	quit   bool
}

// Run drives the terminal UI until the peer disconnects or the user presses
// Ctrl-C. It requires a real terminal because it enters raw input mode.
func Run(conn net.Conn, nick string) error {
	if !term.IsTerminal(int(os.Stdin.Fd())) || !term.IsTerminal(int(os.Stdout.Fd())) {
		return fmt.Errorf("catchat join needs an interactive terminal")
	}

	oldState, err := term.MakeRaw(int(os.Stdin.Fd()))
	if err != nil {
		return fmt.Errorf("enable raw terminal mode: %w", err)
	}
	defer term.Restore(int(os.Stdin.Fd()), oldState)
	defer fmt.Fprint(os.Stdout, reset+"\x1b[?25h\n")

	if err := chat.WriteFrame(conn, chat.Frame{Type: chat.TypeHello, Nick: nick}); err != nil {
		return err
	}

	frames := make(chan chat.Frame, 16)
	readErr := make(chan error, 1)
	go readFrames(conn, frames, readErr)
	inputs := make(chan inputEvent, 8)
	go readInput(os.Stdin, inputs)

	m := model{nick: nick, users: []string{nick}}
	m.addSystem("connected; waiting for the room")
	m.render()

	for {
		select {
		case frame, ok := <-frames:
			if !ok {
				frames = nil
				continue
			}
			m.apply(frame)
			m.render()
		case err := <-readErr:
			if err != nil && err != io.EOF {
				m.addSystem("connection lost: " + err.Error())
				m.render()
			}
			return nil
		case event := <-inputs:
			if event.quit {
				return nil
			}
			if !event.submit {
				m.input = event.line
				m.render()
				continue
			}
			m.input = ""
			if event.line == "" {
				m.render()
				continue
			}
			if strings.HasPrefix(event.line, "/") {
				if m.command(event.line) {
					return nil
				}
				m.render()
				continue
			}
			if err := chat.WriteFrame(conn, chat.Frame{Type: chat.TypeMessage, Text: event.line}); err != nil {
				return err
			}
		case <-time.After(250 * time.Millisecond):
			// Raw input is handled in its own goroutine. Redraw to show typed text.
			m.render()
		}
	}
}

func readFrames(conn net.Conn, frames chan<- chat.Frame, errs chan<- error) {
	defer close(frames)
	reader := bufio.NewReader(conn)
	for {
		frame, err := chat.ReadFrame(reader)
		if err != nil {
			errs <- err
			return
		}
		frames <- frame
	}
}

func readInput(r io.Reader, events chan<- inputEvent) {
	reader := bufio.NewReader(r)
	var line []rune
	for {
		b, err := reader.ReadByte()
		if err != nil {
			events <- inputEvent{quit: true}
			return
		}
		switch b {
		case 3: // Ctrl-C
			events <- inputEvent{quit: true}
			return
		case '\r', '\n':
			events <- inputEvent{line: chat.CleanText(string(line)), submit: true}
			line = nil
		case 8, 127:
			if len(line) > 0 {
				line = line[:len(line)-1]
			}
			events <- inputEvent{line: string(line)}
		case 27: // Ignore ANSI escape sequences, including arrow keys.
			_, _ = reader.ReadByte()
			_, _ = reader.ReadByte()
		default:
			if b >= 0x20 && b < 0x80 && len(line) < 1000 {
				line = append(line, rune(b))
				events <- inputEvent{line: string(line)}
			}
		}
	}
}

type model struct {
	nick  string
	input string
	users []string
	lines []string
}

func (m *model) apply(frame chat.Frame) {
	switch frame.Type {
	case chat.TypeWelcome:
		if len(frame.Users) > 0 {
			m.users = frame.Users
		}
		m.addSystem(frame.Text)
	case chat.TypeUsers:
		m.users = frame.Users
	case chat.TypeSystem:
		m.addSystem(frame.Text)
	case chat.TypeMessage:
		stamp := frame.Time.Local().Format("15:04")
		m.lines = append(m.lines, fmt.Sprintf("%s[%s]%s %s<%s>%s %s", gray, stamp, reset, nickColor(frame.Nick), frame.Nick, reset, frame.Text))
	}
	if len(m.lines) > 500 {
		m.lines = m.lines[len(m.lines)-500:]
	}
}

func (m *model) addSystem(text string) {
	m.lines = append(m.lines, yellow+"*** "+reset+text)
}

// command returns true when the UI should quit.
func (m *model) command(line string) bool {
	switch strings.ToLower(strings.TrimSpace(line)) {
	case "/quit", "/exit":
		return true
	case "/help":
		m.addSystem("commands: /help, /quit")
	default:
		m.addSystem("unknown command; try /help")
	}
	return false
}

func (m *model) render() {
	width, height, err := term.GetSize(int(os.Stdout.Fd()))
	if err != nil || width < 30 {
		width, height = 80, 24
	}
	if height < 8 {
		height = 8
	}
	available := height - 5
	lines := wrapLines(m.lines, width-2)
	if len(lines) > available {
		lines = lines[len(lines)-available:]
	}

	fmt.Fprint(os.Stdout, "\x1b[?25l\x1b[H\x1b[2J")
	header := fmt.Sprintf(" CaTChaT  #lobby  |  %s  |  Tailcat/WireGuard ", m.nick)
	fmt.Fprint(os.Stdout, cyan+bold+fitPlain(header, width)+reset+"\n")
	fmt.Fprint(os.Stdout, gray+strings.Repeat("─", width)+reset+"\n")
	for _, line := range lines {
		fmt.Fprint(os.Stdout, line+"\n")
	}
	for i := len(lines); i < available; i++ {
		fmt.Fprint(os.Stdout, "\n")
	}
	fmt.Fprint(os.Stdout, gray+strings.Repeat("─", width)+reset+"\n")
	users := strings.Join(m.users, " ")
	fmt.Fprintf(os.Stdout, "%s[%d users]%s %s\n", cyan, len(m.users), reset, fitPlain(users, width-12))
	prompt := fitPlain(m.input, width-2)
	if prompt == "" {
		prompt = dim + "type a message  |  /help  |  ^C quit" + reset
	}
	fmt.Fprint(os.Stdout, bold+"> "+reset+prompt)
	fmt.Fprint(os.Stdout, "\x1b[?25h")
}

func wrapLines(lines []string, width int) []string {
	if width < 10 {
		return lines
	}
	var out []string
	for _, line := range lines {
		// ANSI escape codes are only our own fixed prefixes. Keep wrapping simple
		// and avoid splitting a rune, while reserving room for their bytes.
		if visibleLen(line) <= width {
			out = append(out, line)
			continue
		}
		plain := stripANSI(line)
		for visibleLen(plain) > width {
			cut := cutRunes(plain, width)
			out = append(out, cut)
			plain = strings.TrimLeft(plain[len(cut):], " ")
		}
		out = append(out, plain)
	}
	return out
}

func fitPlain(s string, width int) string {
	if utf8.RuneCountInString(s) <= width {
		return s
	}
	return string([]rune(s)[:width-1]) + "…"
}

func cutRunes(s string, width int) string {
	r := []rune(s)
	if len(r) <= width {
		return s
	}
	return string(r[:width])
}

func visibleLen(s string) int { return utf8.RuneCountInString(stripANSI(s)) }

func stripANSI(s string) string {
	for {
		start := strings.Index(s, "\x1b[")
		if start < 0 {
			return s
		}
		end := start + 2
		for end < len(s) && (s[end] < '@' || s[end] > '~') {
			end++
		}
		if end == len(s) {
			return s[:start]
		}
		s = s[:start] + s[end+1:]
	}
}

func nickColor(nick string) string {
	colors := []string{"\x1b[38;5;81m", "\x1b[38;5;213m", "\x1b[38;5;120m", "\x1b[38;5;214m"}
	var sum int
	for _, r := range nick {
		sum += int(r)
	}
	return colors[sum%len(colors)]
}
