package gen_test

import (
	"encoding/json"
	"path/filepath"
	"slices"
	"strings"
	"testing"

	"github.com/go-kanna/internal/deps"
)

func TestDeps(t *testing.T) {
	t.Parallel()

	got := execute(t, app(t), "-deps")
	if got.code != 0 {
		t.Fatalf("exit %d: %s", got.code, got.err)
	}
	var r deps.Report
	if err := json.Unmarshal([]byte(got.stdout), &r); err != nil {
		t.Fatal(err)
	}

	if !slices.Equal(r.Packages, []string{"example.com/app/messages"}) {
		t.Errorf("Packages = %q", r.Packages)
	}
	if len(r.Inputs) != 1 || filepath.Base(r.Inputs[0]) != "locales" {
		t.Errorf("Inputs = %q", r.Inputs)
	}
	if len(r.Writes) != 1 || !strings.HasSuffix(r.Writes[0], filepath.Join("messages", "i18n_gen.go")) {
		t.Errorf("Writes = %q", r.Writes)
	}
	for _, name := range []string{"example.com/app/messages.Hello", "example.com/app/messages.Localizer"} {
		if !slices.Contains(r.Declares, name) {
			t.Errorf("Declares = %q, want %s among them", r.Declares, name)
		}
	}
}

// A destination that holds no code yet is no package to read, and locales that
// are not there are for the kanna command to report.
func TestDeps_NothingThereYet(t *testing.T) {
	t.Parallel()

	dir := t.TempDir()
	writeFiles(t, dir, map[string]string{"go.mod": "module example.com/app\n\ngo 1.26.0\n"})

	got := execute(t, dir, "-deps")
	if got.code != 0 {
		t.Fatalf("exit %d: %s", got.code, got.err)
	}
	var r deps.Report
	if err := json.Unmarshal([]byte(got.stdout), &r); err != nil {
		t.Fatal(err)
	}

	if len(r.Packages) != 0 {
		t.Errorf("Packages = %q, want none", r.Packages)
	}
	if len(r.Writes) != 1 {
		t.Errorf("Writes = %q", r.Writes)
	}
}
