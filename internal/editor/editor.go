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

	"github.com/chapmanjacobd/pathprompt/internal/complete"
	"github.com/chapmanjacobd/pathprompt/internal/history"
	"golang.org/x/term"
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
func Read(in *os.File, out io.Writer, config Config) (string, error) {
	if !term.IsTerminal(int(in.Fd())) {
		return "", errors.New("standard input is not a terminal")
	}
	state, err := term.MakeRaw(int(in.Fd()))
	if err != nil {
		return "", err
	}
	defer term.Restore(int(in.Fd()), state)

	e := &editor{
		reader:    bufio.NewReader(in),
		out:       out,
		prompt:    config.Prompt,
		history:   config.History,
		completer: config.Completer,
		buffer:    []rune(config.Initial),
		cursor:    len([]rune(config.Initial)),
	}
	e.render()
	return e.read()
}

type editor struct {
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

func (e *editor) read() (string, error) {
	for {
		key, err := e.reader.ReadByte()
		if err != nil {
			if errors.Is(err, io.EOF) {
				fmt.Fprintln(e.out)
				return "", ErrCancelled
			}
			return "", err
		}
		switch key {
		case '\r', '\n':
			value := strings.TrimSpace(string(e.buffer))
			fmt.Fprintln(e.out)
			return value, nil
		case 3, 4: // Ctrl-C or Ctrl-D
			fmt.Fprintln(e.out)
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
		case 8, 127:
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

func (e *editor) escape() {
	next, err := e.reader.ReadByte()
	if err != nil || next != '[' {
		return
	}
	key, err := e.reader.ReadByte()
	if err != nil {
		return
	}
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
	case '3':
		if terminator, _ := e.reader.ReadByte(); terminator == '~' && e.cursor < len(e.buffer) {
			e.buffer = append(e.buffer[:e.cursor], e.buffer[e.cursor+1:]...)
			e.resetNavigation()
		}
	case '1', '4':
		if terminator, _ := e.reader.ReadByte(); terminator == '~' {
			if key == '1' {
				e.cursor = 0
			} else {
				e.cursor = len(e.buffer)
			}
		}
	}
}

func (e *editor) insert(value rune) {
	e.buffer = append(e.buffer, 0)
	copy(e.buffer[e.cursor+1:], e.buffer[e.cursor:])
	e.buffer[e.cursor] = value
	e.cursor++
	e.resetNavigation()
}

func (e *editor) deleteWordBackward() {
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

func (e *editor) previousHistory() {
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

func (e *editor) nextHistory() {
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

func (e *editor) reverseSearch() {
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

func (e *editor) complete() {
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

func (e *editor) setBuffer(value string) {
	e.buffer = []rune(value)
	e.cursor = len(e.buffer)
}

func (e *editor) resetNavigation() {
	e.historyItems = nil
	e.historyIndex = 0
	e.searchItems = nil
	e.searchIndex = 0
}

func (e *editor) suggestion() string {
	if e.history == nil || e.cursor != len(e.buffer) {
		return ""
	}
	return e.history.Suggest(string(e.buffer))
}

func (e *editor) render() {
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
