package complete

import (
	"os"
	"path/filepath"
	"reflect"
	"testing"
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

	engine := New(t.TempDir())
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
	got := New(home).Complete("~/Do")
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
	got := New(t.TempDir()).Complete(input)
	want := "." + string(filepath.Separator) + "alpha" + string(filepath.Separator)
	if len(got) != 1 || got[0].Value != want {
		t.Fatalf("Complete(%q) = %#v, want %q", input, got, want)
	}
}

func TestCommonPrefix(t *testing.T) {
	candidates := []Candidate{{Value: "archive/"}, {Value: "artist/"}, {Value: "art.txt"}}
	if got := CommonPrefix(candidates); got != "ar" {
		t.Fatalf("CommonPrefix() = %q, want ar", got)
	}
}
