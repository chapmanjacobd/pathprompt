// Package editor implements a small single-line terminal editor.
package editor

import (
	"bufio"
	"errors"
	"fmt"
	"io"
	"os"
	"strings"
	"unicode"
	"unicode/utf8"

	"golang.org/x/term"

	"github.com/chapmanjacobd/pathprompt/internal/complete"
	"github.com/chapmanjacobd/pathprompt/internal/history"
)

var ErrCancelled = errors.New("prompt cancelled")

// Config supplies editor dependencies and initial state.
type Config struct {
	Prompt    string
	Initial   string
	History   *history.Store
	Completer complete.Engine
}

// Read presents a path prompt and returns the accepted text.
func Read(in *os.File, out io.Writer, config Config) (value string, readErr error) {
	if !term.IsTerminal(int(in.Fd())) {
		return "", errors.New("standard input is not a terminal")
	}
	state, err := term.MakeRaw(int(in.Fd()))
	if err != nil {
		return "", err
	}
	defer func() {
		if restoreErr := term.Restore(int(in.Fd()), state); restoreErr != nil && readErr == nil {
			value = ""
			readErr = fmt.Errorf("restore terminal: %w", restoreErr)
		}
	}()

	e := New(in, out, config)
	e.render()
	return e.Read()
}

// Editor reads and edits a single line from a byte stream.
type Editor struct {
	reader    *bufio.Reader
	out       io.Writer
	prompt    string
	history   *history.Store
	completer complete.Engine
	buffer    []rune
	cursor    int

	historyDraft string
	historyItems []string
	historyIndex int
	searchQuery  string
	searchItems  []string
	searchIndex  int
}

// New creates an editor for reader and out.
func New(reader io.Reader, out io.Writer, config Config) *Editor {
	initial := []rune(config.Initial)
	return &Editor{
		reader:    bufio.NewReader(reader),
		out:       out,
		prompt:    config.Prompt,
		history:   config.History,
		completer: config.Completer,
		buffer:    initial,
		cursor:    len(initial),
	}
}

// Read reads until the line is accepted or cancelled.
func (e *Editor) Read() (string, error) {
	for {
		key, err := e.reader.ReadByte()
		if err != nil {
			if errors.Is(err, io.EOF) {
				fmt.Fprint(e.out, "\r\n")
				return "", ErrCancelled
			}
			return "", err
		}
		switch key {
		case '\r', '\n':
			value := strings.TrimSpace(string(e.buffer))
			fmt.Fprint(e.out, "\r\n")
			return value, nil
		case 3, 4: // Ctrl-C or Ctrl-D
			fmt.Fprint(e.out, "\r\n")
			return "", ErrCancelled
		case 1: // Ctrl-A
			e.cursor = 0
		case 5: // Ctrl-E
			e.cursor = len(e.buffer)
		case 11: // Ctrl-K
			e.buffer = e.buffer[:e.cursor]
			e.resetNavigation()
		case 12: // Ctrl-L
			fmt.Fprint(e.out, "\033[H\033[2J")
		case 18: // Ctrl-R
			e.reverseSearch()
		case 21: // Ctrl-U
			e.buffer = append([]rune(nil), e.buffer[e.cursor:]...)
			e.cursor = 0
			e.resetNavigation()
		case 23: // Ctrl-W
			e.deleteWordBackward()
			e.resetNavigation()
		case 8: // Ctrl-Backspace
			e.deleteBoundaryBackward()
			e.resetNavigation()
		case 127:
			if e.cursor > 0 {
				e.buffer = append(e.buffer[:e.cursor-1], e.buffer[e.cursor:]...)
				e.cursor--
				e.resetNavigation()
			}
		case '\t':
			e.complete()
		case 27:
			e.escape()
		default:
			if key >= 32 {
				if key < utf8.RuneSelf {
					e.insert(rune(key))
				} else {
					if err := e.reader.UnreadByte(); err != nil {
						return "", err
					}
					value, _, err := e.reader.ReadRune()
					if err != nil {
						return "", err
					}
					e.insert(value)
				}
			}
		}
		e.render()
	}
}

func (e *Editor) escape() {
	next, err := e.reader.ReadByte()
	if err != nil {
		return
	}
	if next == 8 || next == 127 {
		e.clearLine()
		return
	}
	if next != '[' {
		return
	}
	params := make([]byte, 0, 8)
	for {
		key, readErr := e.reader.ReadByte()
		if readErr != nil {
			return
		}
		if key >= 0x40 && key <= 0x7e {
			e.handleCSI(string(params), key)
			return
		}
		params = append(params, key)
		if len(params) == 16 {
			return
		}
	}
}

func (e *Editor) handleCSI(params string, key byte) {
	if params == "" {
		switch key {
		case 'A':
			e.previousHistory()
		case 'B':
			e.nextHistory()
		case 'C':
			if suggestion := e.suggestion(); e.cursor == len(e.buffer) && suggestion != "" {
				e.buffer = []rune(suggestion)
				e.cursor = len(e.buffer)
			} else if e.cursor < len(e.buffer) {
				e.cursor++
			}
		case 'D':
			if e.cursor > 0 {
				e.cursor--
			}
		case 'H':
			e.cursor = 0
		case 'F':
			e.cursor = len(e.buffer)
		}
		return
	}
	if key != '~' {
		return
	}
	switch params {
	case "1":
		e.cursor = 0
	case "4":
		e.cursor = len(e.buffer)
	case "3":
		e.deleteForward()
	case "3;3", "8;3", "127;3": // Alt-Delete and Alt-Backspace
		e.clearLine()
	case "3;5": // Ctrl-Delete
		e.deleteBoundaryForward()
		e.resetNavigation()
	case "8;5", "127;5": // Ctrl-Backspace
		e.deleteBoundaryBackward()
		e.resetNavigation()
	}
}

func (e *Editor) clearLine() {
	e.buffer = nil
	e.cursor = 0
	e.resetNavigation()
}

func (e *Editor) deleteForward() {
	if e.cursor < len(e.buffer) {
		e.buffer = append(e.buffer[:e.cursor], e.buffer[e.cursor+1:]...)
		e.resetNavigation()
	}
}

func (e *Editor) insert(value rune) {
	e.buffer = append(e.buffer, 0)
	copy(e.buffer[e.cursor+1:], e.buffer[e.cursor:])
	e.buffer[e.cursor] = value
	e.cursor++
	e.resetNavigation()
}

func (e *Editor) deleteWordBackward() {
	start := e.cursor
	for start > 0 && unicode.IsSpace(e.buffer[start-1]) {
		start--
	}
	for start > 0 && !unicode.IsSpace(e.buffer[start-1]) {
		start--
	}
	e.buffer = append(e.buffer[:start], e.buffer[e.cursor:]...)
	e.cursor = start
}

func (e *Editor) deleteBoundaryBackward() {
	start := e.cursor
	for start > 0 && isWordBoundary(e.buffer[start-1]) {
		start--
	}
	for start > 0 && !isWordBoundary(e.buffer[start-1]) {
		start--
	}
	e.buffer = append(e.buffer[:start], e.buffer[e.cursor:]...)
	e.cursor = start
}

func (e *Editor) deleteBoundaryForward() {
	end := e.cursor
	for end < len(e.buffer) && isWordBoundary(e.buffer[end]) {
		end++
	}
	for end < len(e.buffer) && !isWordBoundary(e.buffer[end]) {
		end++
	}
	e.buffer = append(e.buffer[:e.cursor], e.buffer[end:]...)
}

func isWordBoundary(value rune) bool {
	return unicode.IsSpace(value) || strings.ContainsRune("/.-", value)
}

func (e *Editor) previousHistory() {
	if e.history == nil {
		return
	}
	if e.historyIndex == 0 && e.historyItems == nil {
		e.historyDraft = string(e.buffer)
		e.historyItems = e.history.ValuesMatchingPrefix(e.historyDraft)
	}
	if len(e.historyItems) == 0 || e.historyIndex >= len(e.historyItems) {
		return
	}
	e.setBuffer(e.historyItems[e.historyIndex])
	e.historyIndex++
}

func (e *Editor) nextHistory() {
	if len(e.historyItems) == 0 || e.historyIndex == 0 {
		return
	}
	e.historyIndex--
	if e.historyIndex == 0 {
		e.setBuffer(e.historyDraft)
		return
	}
	e.setBuffer(e.historyItems[e.historyIndex-1])
}

func (e *Editor) reverseSearch() {
	if e.history == nil {
		return
	}
	if e.searchItems == nil {
		e.searchQuery = string(e.buffer)
		e.searchItems = e.history.ValuesContaining(e.searchQuery)
		e.searchIndex = 0
	}
	if len(e.searchItems) == 0 {
		return
	}
	e.setBuffer(e.searchItems[e.searchIndex])
	e.searchIndex = (e.searchIndex + 1) % len(e.searchItems)
}

func (e *Editor) complete() {
	input := string(e.buffer)
	candidates := e.completer.Complete(input)
	if len(candidates) == 0 {
		fmt.Fprint(e.out, "\a")
		return
	}
	common := complete.CommonPrefix(candidates)
	if len(candidates) == 1 {
		e.setBuffer(candidates[0].Value)
		e.resetNavigation()
		return
	}
	if len(common) > len(input) {
		e.setBuffer(common)
		e.resetNavigation()
		return
	}

	fmt.Fprint(e.out, "\r\n")
	for i, candidate := range candidates {
		if i > 0 {
			fmt.Fprint(e.out, "  ")
		}
		fmt.Fprint(e.out, display(candidate.Value))
	}
	fmt.Fprint(e.out, "\r\n")
}

func (e *Editor) setBuffer(value string) {
	e.buffer = []rune(value)
	e.cursor = len(e.buffer)
}

func (e *Editor) resetNavigation() {
	e.historyItems = nil
	e.historyIndex = 0
	e.searchItems = nil
	e.searchIndex = 0
}

func (e *Editor) suggestion() string {
	if e.history == nil || e.cursor != len(e.buffer) {
		return ""
	}
	return e.history.Suggest(string(e.buffer))
}

func (e *Editor) render() {
	value := string(e.buffer)
	suggestion := e.suggestion()
	fmt.Fprint(e.out, "\r\033[2K", display(e.prompt), display(value))
	if suggestion != "" {
		remainder := strings.TrimPrefix(suggestion, value)
		fmt.Fprint(e.out, "\033[2m", display(remainder), "\033[0m")
	}
	fmt.Fprint(e.out, "\r", display(e.prompt), display(string(e.buffer[:e.cursor])))
}

// display prevents prompt and filename bytes from being interpreted as terminal controls.
func display(value string) string {
	return strings.Map(func(r rune) rune {
		if r == 0x1b || unicode.IsControl(r) {
			return -1
		}
		return r
	}, value)
}
