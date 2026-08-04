package editor

import (
	"bufio"
	"bytes"
	"errors"
	"strings"
	"testing"
)

func TestReadEndsAcceptedInputAtStartOfLine(t *testing.T) {
	var out bytes.Buffer
	e := editor{
		reader: bufio.NewReader(strings.NewReader("\r")),
		out:    &out,
	}

	value, err := e.read()
	if err != nil {
		t.Fatal(err)
	}
	if value != "" {
		t.Fatalf("read value = %q, want empty string", value)
	}
	if got := out.String(); got != "\r\n" {
		t.Fatalf("terminal output = %q, want CRLF", got)
	}
}

func TestReadEndsCancelledInputAtStartOfLine(t *testing.T) {
	var out bytes.Buffer
	e := editor{
		reader: bufio.NewReader(strings.NewReader(string(rune(3)))),
		out:    &out,
	}

	_, err := e.read()
	if !errors.Is(err, ErrCancelled) {
		t.Fatalf("read error = %v, want ErrCancelled", err)
	}
	if got := out.String(); got != "\r\n" {
		t.Fatalf("terminal output = %q, want CRLF", got)
	}
}
