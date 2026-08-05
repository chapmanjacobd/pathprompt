package editor_test

import (
	"bytes"
	"errors"
	"strings"
	"testing"

	"github.com/chapmanjacobd/pathprompt/internal/editor"
)

func TestReadEndsAcceptedInputAtStartOfLine(t *testing.T) {
	var out bytes.Buffer
	e := editor.New(strings.NewReader("\r"), &out, editor.Config{})

	value, err := e.Read()
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
	e := editor.New(strings.NewReader(string(rune(3))), &out, editor.Config{})

	_, err := e.Read()
	if !errors.Is(err, editor.ErrCancelled) {
		t.Fatalf("read error = %v, want ErrCancelled", err)
	}
	if got := out.String(); got != "\r\n" {
		t.Fatalf("terminal output = %q, want CRLF", got)
	}
}
