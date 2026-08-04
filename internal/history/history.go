// Package history stores durable, de-duplicated prompt entries.
package history

import (
	"bufio"
	"encoding/json"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"time"
)

const DefaultLimit = 10_000

// Entry records an accepted path and when it was last selected.
type Entry struct {
	Value string    `json:"value"`
	Used  time.Time `json:"used"`
}

// Store is a bounded, most-recent-last history backed by a JSON Lines file.
type Store struct {
	path    string
	limit   int
	entries []Entry
}

// FilePath returns the namespace-specific history path under the XDG state directory.
func FilePath(namespace string) string {
	stateHome := os.Getenv("XDG_STATE_HOME")
	if stateHome == "" {
		home, err := os.UserHomeDir()
		if err != nil {
			home = "."
		}
		stateHome = filepath.Join(home, ".local", "state")
	}
	return filepath.Join(stateHome, "pathprompt", safeNamespace(namespace)+".history")
}

func safeNamespace(namespace string) string {
	namespace = strings.TrimSpace(namespace)
	if namespace == "" {
		return "default"
	}
	namespace = filepath.Base(namespace)
	if namespace == "." || namespace == string(filepath.Separator) {
		return "default"
	}
	return namespace
}

// Open loads a store. A missing history file creates an empty store.
func Open(path string, limit int) (*Store, error) {
	if limit < 1 {
		return nil, fmt.Errorf("history limit must be positive")
	}

	store := &Store{path: path, limit: limit}
	file, err := os.Open(path)
	if errors.Is(err, os.ErrNotExist) {
		return store, nil
	}
	if err != nil {
		return nil, err
	}
	defer file.Close()

	scanner := bufio.NewScanner(file)
	scanner.Buffer(make([]byte, 4*1024), 1024*1024)
	line := 0
	for scanner.Scan() {
		line++
		var entry Entry
		if err := json.Unmarshal(scanner.Bytes(), &entry); err != nil {
			return nil, fmt.Errorf("decode history line %d: %w", line, err)
		}
		if strings.TrimSpace(entry.Value) == "" {
			continue
		}
		store.add(entry)
	}
	if err := scanner.Err(); err != nil {
		return nil, err
	}
	return store, nil
}

// Add records value and persists the new state. Existing copies are moved to the end.
func (s *Store) Add(value string) error {
	value = strings.TrimSpace(value)
	if value == "" {
		return nil
	}
	s.add(Entry{Value: value, Used: time.Now().UTC()})
	return s.save()
}

func (s *Store) add(entry Entry) {
	for i := len(s.entries) - 1; i >= 0; i-- {
		if s.entries[i].Value == entry.Value {
			s.entries = append(s.entries[:i], s.entries[i+1:]...)
		}
	}
	s.entries = append(s.entries, entry)
	if excess := len(s.entries) - s.limit; excess > 0 {
		s.entries = append([]Entry(nil), s.entries[excess:]...)
	}
}

func (s *Store) save() error {
	if err := os.MkdirAll(filepath.Dir(s.path), 0o700); err != nil {
		return err
	}

	file, err := os.CreateTemp(filepath.Dir(s.path), ".history-*")
	if err != nil {
		return err
	}
	tempName := file.Name()
	defer os.Remove(tempName)

	if err := file.Chmod(0o600); err != nil {
		file.Close()
		return err
	}
	writer := bufio.NewWriter(file)
	encoder := json.NewEncoder(writer)
	for _, entry := range s.entries {
		if err := encoder.Encode(entry); err != nil {
			file.Close()
			return err
		}
	}
	if err := writer.Flush(); err != nil {
		file.Close()
		return err
	}
	if err := file.Sync(); err != nil {
		file.Close()
		return err
	}
	if err := file.Close(); err != nil {
		return err
	}
	return os.Rename(tempName, s.path)
}

// ValuesMatchingPrefix returns newest-first entries beginning with prefix.
func (s *Store) ValuesMatchingPrefix(prefix string) []string {
	return s.valuesMatching(func(value string) bool {
		return strings.HasPrefix(value, prefix)
	})
}

// ValuesContaining returns newest-first entries containing query.
func (s *Store) ValuesContaining(query string) []string {
	return s.valuesMatching(func(value string) bool {
		return strings.Contains(value, query)
	})
}

func (s *Store) valuesMatching(match func(string) bool) []string {
	values := make([]string, 0)
	for i := len(s.entries) - 1; i >= 0; i-- {
		if match(s.entries[i].Value) {
			values = append(values, s.entries[i].Value)
		}
	}
	return values
}

// Suggest returns the newest longer history entry that starts with prefix.
func (s *Store) Suggest(prefix string) string {
	for _, value := range s.ValuesMatchingPrefix(prefix) {
		if value != prefix {
			return value
		}
	}
	return ""
}
