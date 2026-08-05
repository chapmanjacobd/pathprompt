package complete_test

import (
	"os"
	"path/filepath"
	"reflect"
	"testing"

	"github.com/chapmanjacobd/pathprompt/internal/complete"
)

func TestCompleteOrdersDirectoriesAndHidesDotfiles(t *testing.T) {
	root := t.TempDir()
	for _, name := range []string{"alpha", "atom", ".archive"} {
		if err := os.Mkdir(filepath.Join(root, name), 0o755); err != nil {
			t.Fatal(err)
		}
	}
	if err := os.WriteFile(filepath.Join(root, "apple"), nil, 0o644); err != nil {
		t.Fatal(err)
	}

	engine := complete.New(t.TempDir())
	candidates := engine.Complete(filepath.Join(root, "a"))
	got := make([]string, len(candidates))
	for i, candidate := range candidates {
		got[i] = candidate.Value
	}
	want := []string{
		filepath.Join(root, "alpha") + string(filepath.Separator),
		filepath.Join(root, "atom") + string(filepath.Separator),
		filepath.Join(root, "apple"),
	}
	if !reflect.DeepEqual(got, want) {
		t.Fatalf("Complete() = %q, want %q", got, want)
	}
}

func TestCompletePreservesTildeNotation(t *testing.T) {
	home := t.TempDir()
	if err := os.Mkdir(filepath.Join(home, "Documents"), 0o755); err != nil {
		t.Fatal(err)
	}
	got := complete.New(home).Complete("~/Do")
	if len(got) != 1 || got[0].Value != "~/Documents/" {
		t.Fatalf("Complete() = %#v, want ~/Documents/", got)
	}
}

func TestCompletePreservesExplicitRelativePrefix(t *testing.T) {
	root := t.TempDir()
	if err := os.Mkdir(filepath.Join(root, "alpha"), 0o755); err != nil {
		t.Fatal(err)
	}
	t.Chdir(root)

	input := "." + string(filepath.Separator) + "a"
	got := complete.New(t.TempDir()).Complete(input)
	want := "." + string(filepath.Separator) + "alpha" + string(filepath.Separator)
	if len(got) != 1 || got[0].Value != want {
		t.Fatalf("Complete(%q) = %#v, want %q", input, got, want)
	}
}

func TestCompleteFiltersByType(t *testing.T) {
	root := t.TempDir()
	if err := os.Mkdir(filepath.Join(root, "directory"), 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(root, "file"), nil, 0o644); err != nil {
		t.Fatal(err)
	}

	for _, test := range []struct {
		name string
		kind complete.PathType
		want string
	}{
		{name: "file", kind: complete.TypeFile, want: filepath.Join(root, "file")},
		{name: "directory", kind: complete.TypeDirectory, want: filepath.Join(root, "directory") + string(filepath.Separator)},
	} {
		t.Run(test.name, func(t *testing.T) {
			input := root + string(filepath.Separator)
			got := complete.New(t.TempDir(), test.kind).Complete(input)
			if len(got) != 1 || got[0].Value != test.want {
				t.Fatalf("Complete(%q) = %#v, want %q", input, got, test.want)
			}
		})
	}
}

func TestCommonPrefix(t *testing.T) {
	candidates := []complete.Candidate{{Value: "archive/"}, {Value: "artist/"}, {Value: "art.txt"}}
	if got := complete.CommonPrefix(candidates); got != "ar" {
		t.Fatalf("CommonPrefix() = %q, want ar", got)
	}
}
