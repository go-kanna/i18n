package gen_test

import (
	"path/filepath"
	"testing"

	"github.com/go-kanna/internal/pkgtest"

	"github.com/go-kanna/i18n/internal/gen"
)

// Each golden file is i18n_gen.go in a package of testdata, which
// TestGenerated imports: a golden file that stops compiling fails the build of
// the test, and what it compiles to is localized there.
func TestEmit(t *testing.T) {
	t.Parallel()

	for _, dir := range []string{"testdata/basic", "testdata/names"} {
		t.Run(dir, func(t *testing.T) {
			t.Parallel()

			got, err := gen.Emit(dir)
			if err != nil {
				t.Fatal(err)
			}
			pkgtest.Golden(t, filepath.Join(dir, "messages", "i18n_gen.go"), got)
		})
	}
}
