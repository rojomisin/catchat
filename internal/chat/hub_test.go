package chat

import (
	"bufio"
	"net"
	"testing"
	"time"
)

func TestHubWelcomesAndBroadcastsMessage(t *testing.T) {
	hub := NewHub()
	server, client := net.Pipe()
	defer client.Close()
	go hub.ServeConn(server)

	if err := WriteFrame(client, Frame{Type: TypeHello, Nick: "alice"}); err != nil {
		t.Fatal(err)
	}
	reader := bufio.NewReader(client)
	for i, want := range []string{TypeWelcome, TypeSystem, TypeUsers} {
		frame := readFrameBefore(t, reader)
		if frame.Type != want {
			t.Fatalf("initial frame %d type = %q, want %q", i, frame.Type, want)
		}
	}

	if err := WriteFrame(client, Frame{Type: TypeMessage, Text: "hello\nworld"}); err != nil {
		t.Fatal(err)
	}
	frame := readFrameBefore(t, reader)
	if frame.Type != TypeMessage || frame.Nick != "alice" || frame.Text != "hello world" {
		t.Fatalf("message frame = %#v", frame)
	}
}

func readFrameBefore(t *testing.T, reader *bufio.Reader) Frame {
	t.Helper()
	result := make(chan struct {
		frame Frame
		err   error
	}, 1)
	go func() {
		frame, err := ReadFrame(reader)
		result <- struct {
			frame Frame
			err   error
		}{frame, err}
	}()
	select {
	case result := <-result:
		if result.err != nil {
			t.Fatal(result.err)
		}
		return result.frame
	case <-time.After(time.Second):
		t.Fatal("timed out reading hub frame")
		return Frame{}
	}
}
