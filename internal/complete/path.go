// Package complete provides filesystem path completion.
package complete

import (
	"fmt"
	"os"
	"path/filepath"
	"sort"
	"strings"
)

// Candidate is a completion value. Directories end in a separator so they can be
// completed further without another keystroke.
type Candidate struct {
	Value string
	IsDir bool
}

// PathType limits completion candidates by filesystem entry type.
type PathType string

const (
	TypeAny       PathType = ""
	TypeFile      PathType = "file"
	TypeDirectory PathType = "directory"
)

// ParseType accepts fd-style type names and their short forms.
func ParseType(value string) (PathType, error) {
	switch strings.ToLower(value) {
	case "f", "file":
		return TypeFile, nil
	case "d", "dir", "directory":
		return TypeDirectory, nil
	default:
		return TypeAny, fmt.Errorf("unsupported path type %q (want file or directory)", value)
	}
}

// Engine completes user-supplied filesystem path prefixes and expands leading ~.
type Engine struct {
	home     string
	pathType PathType
}

func New(home string, pathTypes ...PathType) Engine {
	if home == "" {
		home, _ = os.UserHomeDir()
	}
	pathType := TypeAny
	if len(pathTypes) > 0 {
		pathType = pathTypes[0]
	}
	return Engine{home: filepath.Clean(home), pathType: pathType}
}

// Complete returns matching paths in stable directory-first order.
func (e Engine) Complete(input string) []Candidate {
	expanded := e.expandHome(input)
	parent, prefix := filepath.Split(expanded)
	if parent == "" {
		parent = "."
	}

	entries, err := os.ReadDir(parent)
	if err != nil {
		return nil
	}
	candidates := make([]Candidate, 0, len(entries))
	for _, entry := range entries {
		name := entry.Name()
		if !e.matchesType(entry) || !strings.HasPrefix(name, prefix) || (strings.HasPrefix(name, ".") && !strings.HasPrefix(prefix, ".")) {
			continue
		}
		// Keep the parent exactly as the user supplied it, including ./ or .\.
		value := parent + name
		if !filepath.IsAbs(expanded) && parent == "." {
			value = name
		}
		value = e.contractHome(value, input)
		candidate := Candidate{Value: value, IsDir: entry.IsDir()}
		if candidate.IsDir {
			candidate.Value += string(filepath.Separator)
		}
		candidates = append(candidates, candidate)
	}
	sort.Slice(candidates, func(i, j int) bool {
		if candidates[i].IsDir != candidates[j].IsDir {
			return candidates[i].IsDir
		}
		return candidates[i].Value < candidates[j].Value
	})
	return candidates
}

func (e Engine) matchesType(entry os.DirEntry) bool {
	switch e.pathType {
	case TypeAny:
		return true
	case TypeFile:
		return entry.Type().IsRegular()
	case TypeDirectory:
		return entry.IsDir()
	default:
		return false
	}
}

func (e Engine) expandHome(input string) string {
	if input == "~" {
		return e.home
	}
	if strings.HasPrefix(input, "~/") {
		return filepath.Join(e.home, input[2:])
	}
	return input
}

func (e Engine) contractHome(value, original string) string {
	if original != "~" && !strings.HasPrefix(original, "~/") {
		return value
	}
	if value == e.home {
		return "~"
	}
	if strings.HasPrefix(value, e.home+string(filepath.Separator)) {
		return "~" + strings.TrimPrefix(value, e.home)
	}
	return value
}

// CommonPrefix returns the shared byte prefix for non-empty candidates. Paths are
// opaque byte strings on Unix, and the result is only used to extend an existing input.
func CommonPrefix(candidates []Candidate) string {
	if len(candidates) == 0 {
		return ""
	}
	prefix := candidates[0].Value
	for _, candidate := range candidates[1:] {
		for !strings.HasPrefix(candidate.Value, prefix) {
			prefix = prefix[:len(prefix)-1]
		}
	}
	return prefix
}
