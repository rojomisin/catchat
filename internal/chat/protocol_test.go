package chat

import (
	"bufio"
	"strings"
	"testing"
)

func TestReadFrame(t *testing.T) {
	frame, err := ReadFrame(bufio.NewReader(strings.NewReader(`{"type":"message","text":"hello"}` + "\n")))
	if err != nil {
		t.Fatal(err)
	}
	if frame.Type != TypeMessage || frame.Text != "hello" {
		t.Fatalf("ReadFrame = %#v", frame)
	}
}

func TestReadFrameRejectsOversizedLine(t *testing.T) {
	_, err := ReadFrame(bufio.NewReader(strings.NewReader(strings.Repeat("x", maxFrameBytes+1) + "\n")))
	if err == nil {
		t.Fatal("ReadFrame accepted an oversized line")
	}
}

func TestCleanText(t *testing.T) {
	got := CleanText("  hello\x1b[2J\nworld\t ")
	if got != "hello[2J world" {
		t.Fatalf("CleanText = %q", got)
	}
}

func TestValidNick(t *testing.T) {
	if _, err := validNick("alice_42"); err != nil {
		t.Fatal(err)
	}
	if _, err := validNick("not okay!"); err == nil {
		t.Fatal("validNick accepted punctuation")
	}
}
