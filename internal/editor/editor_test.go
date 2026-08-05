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

func TestReadAltBackspaceClearsInput(t *testing.T) {
	var out bytes.Buffer
	e := editor.New(strings.NewReader("path\x1b\x7f\r"), &out, editor.Config{})

	value, err := e.Read()
	if err != nil {
		t.Fatal(err)
	}
	if value != "" {
		t.Fatalf("read value = %q, want empty string", value)
	}
}

func TestReadAltDeleteClearsInput(t *testing.T) {
	var out bytes.Buffer
	e := editor.New(strings.NewReader("path\x1b[3;3~\r"), &out, editor.Config{})

	value, err := e.Read()
	if err != nil {
		t.Fatal(err)
	}
	if value != "" {
		t.Fatalf("read value = %q, want empty string", value)
	}
}

func TestReadCtrlBackspaceDeletesToPathBoundary(t *testing.T) {
	var out bytes.Buffer
	e := editor.New(strings.NewReader("\x08\x08\r"), &out, editor.Config{
		Initial: "foo/bar.baz-qux",
	})

	value, err := e.Read()
	if err != nil {
		t.Fatal(err)
	}
	if value != "foo/bar." {
		t.Fatalf("read value = %q, want %q", value, "foo/bar.")
	}
}

func TestReadCtrlDeleteDeletesToPathBoundary(t *testing.T) {
	var out bytes.Buffer
	e := editor.New(strings.NewReader("\x01\x1b[3;5~\r"), &out, editor.Config{
		Initial: "foo/bar.baz",
	})

	value, err := e.Read()
	if err != nil {
		t.Fatal(err)
	}
	if value != "/bar.baz" {
		t.Fatalf("read value = %q, want %q", value, "/bar.baz")
	}
}
