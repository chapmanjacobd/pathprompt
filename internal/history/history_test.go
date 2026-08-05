package history_test

import (
	"path/filepath"
	"reflect"
	"testing"

	"github.com/chapmanjacobd/pathprompt/internal/history"
)

func TestStorePersistsDeduplicatedEntries(t *testing.T) {
	path := filepath.Join(t.TempDir(), "nested", "history")
	store, err := history.Open(path, 3)
	if err != nil {
		t.Fatal(err)
	}
	for _, value := range []string{"/one", "/two", "/one", "/three", "/four"} {
		if addErr := store.Add(value); addErr != nil {
			t.Fatal(addErr)
		}
	}

	loaded, err := history.Open(path, 3)
	if err != nil {
		t.Fatal(err)
	}
	want := []string{"/four", "/three", "/one"}
	if got := loaded.ValuesContaining(""); !reflect.DeepEqual(got, want) {
		t.Fatalf("ValuesContaining() = %q, want %q", got, want)
	}
	if got := loaded.Suggest("/o"); got != "/one" {
		t.Fatalf("Suggest() = %q, want /one", got)
	}
}

func TestFilePathUsesSafeNamespace(t *testing.T) {
	t.Setenv("XDG_STATE_HOME", "/state")
	got := history.FilePath("../work")
	want := filepath.Join("/state", "pathprompt", "work.history")
	if got != want {
		t.Fatalf("FilePath() = %q, want %q", got, want)
	}
}
