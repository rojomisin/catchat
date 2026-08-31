package chat

import (
	"bufio"
	"fmt"
	"net"
	"sort"
	"strings"
	"sync"
	"time"
	"unicode"
)

// Hub is a small IRC-style room. It intentionally has no persistence,
// accounts, rooms, or history: the Tailcat listener is the invite boundary.
type Hub struct {
	mu       sync.Mutex
	sessions map[*session]struct{}
}

type session struct {
	nick string
	out  chan Frame
}

func NewHub() *Hub {
	return &Hub{sessions: make(map[*session]struct{})}
}

// ServeConn serves one already-authenticated transport connection.
func (h *Hub) ServeConn(conn net.Conn) {
	defer conn.Close()
	reader := bufio.NewReader(conn)

	first, err := ReadFrame(reader)
	if err != nil || first.Type != TypeHello {
		return
	}
	nick, err := validNick(first.Nick)
	if err != nil {
		_ = WriteFrame(conn, Frame{Type: TypeSystem, Text: err.Error()})
		return
	}

	s := &session{nick: nick, out: make(chan Frame, 32)}
	if !h.add(s) {
		_ = WriteFrame(conn, Frame{Type: TypeSystem, Text: "nickname is already in use"})
		return
	}
	defer h.remove(s)

	go func() {
		for frame := range s.out {
			if err := WriteFrame(conn, frame); err != nil {
				return
			}
		}
	}()

	h.send(s, Frame{Type: TypeWelcome, Text: "welcome to Catchat #lobby", Users: h.users()})
	h.broadcast(Frame{Type: TypeSystem, Text: fmt.Sprintf("*** %s joined #lobby", nick), Time: time.Now().UTC()})
	h.broadcastUsers()

	for {
		frame, err := ReadFrame(reader)
		if err != nil {
			return
		}
		if frame.Type != TypeMessage {
			continue
		}
		text := CleanText(frame.Text)
		if text == "" {
			continue
		}
		if len([]rune(text)) > 1000 {
			h.send(s, Frame{Type: TypeSystem, Text: "message is limited to 1000 characters"})
			continue
		}
		h.broadcast(Frame{Type: TypeMessage, Nick: nick, Text: text, Time: time.Now().UTC()})
	}
}

func (h *Hub) add(s *session) bool {
	h.mu.Lock()
	defer h.mu.Unlock()
	for peer := range h.sessions {
		if strings.EqualFold(peer.nick, s.nick) {
			return false
		}
	}
	h.sessions[s] = struct{}{}
	return true
}

func (h *Hub) remove(s *session) {
	h.mu.Lock()
	if _, ok := h.sessions[s]; !ok {
		h.mu.Unlock()
		return
	}
	delete(h.sessions, s)
	close(s.out)
	h.mu.Unlock()
	h.broadcast(Frame{Type: TypeSystem, Text: fmt.Sprintf("*** %s left #lobby", s.nick), Time: time.Now().UTC()})
	h.broadcastUsers()
}

func (h *Hub) broadcast(frame Frame) {
	h.mu.Lock()
	defer h.mu.Unlock()
	for peer := range h.sessions {
		select {
		case peer.out <- frame:
		default: // A slow peer may miss a frame rather than stall the room.
		}
	}
}

func (h *Hub) send(s *session, frame Frame) {
	h.mu.Lock()
	defer h.mu.Unlock()
	if _, ok := h.sessions[s]; !ok {
		return
	}
	select {
	case s.out <- frame:
	default:
	}
}

func (h *Hub) users() []string {
	h.mu.Lock()
	defer h.mu.Unlock()
	return h.usersLocked()
}

func (h *Hub) usersLocked() []string {
	users := make([]string, 0, len(h.sessions))
	for peer := range h.sessions {
		users = append(users, peer.nick)
	}
	sort.Slice(users, func(i, j int) bool { return strings.ToLower(users[i]) < strings.ToLower(users[j]) })
	return users
}

func (h *Hub) broadcastUsers() {
	h.mu.Lock()
	defer h.mu.Unlock()
	frame := Frame{Type: TypeUsers, Users: h.usersLocked()}
	for peer := range h.sessions {
		select {
		case peer.out <- frame:
		default:
		}
	}
}

func validNick(nick string) (string, error) {
	nick = CleanText(nick)
	if n := len([]rune(nick)); n < 1 || n > 24 {
		return "", fmt.Errorf("nickname must be 1-24 characters")
	}
	for _, r := range nick {
		if !(unicode.IsLetter(r) || unicode.IsDigit(r) || strings.ContainsRune("_-[]\\`^{}", r)) {
			return "", fmt.Errorf("nickname contains an unsupported character")
		}
	}
	return nick, nil
}
