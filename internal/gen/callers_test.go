package gen_test

import (
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/go-kanna/internal/cli"
)

// TestMain loads every module the tests write outside any workspace. A GOWORK
// pinned by the environment would otherwise put them outside the modules it
// lists, which the go command refuses to load.
func TestMain(m *testing.M) {
	// Before any test runs: t.Setenv would rule out t.Parallel.
	if err := os.Setenv("GOWORK", "off"); err != nil {
		panic(err)
	}
	os.Exit(m.Run())
}

// app writes a module calling the messages of locales into a new directory and
// returns it: main from another package, and Welcome from the destination.
func app(t *testing.T) string {
	t.Helper()

	root, err := filepath.Abs(filepath.Join("..", ".."))
	if err != nil {
		t.Fatal(err)
	}
	sum, err := os.ReadFile(filepath.Join(root, "go.sum")) //nolint:gosec // this checkout's own go.sum
	if err != nil {
		t.Fatal(err)
	}

	dir := t.TempDir()
	writeFiles(t, dir, locales)
	writeFiles(t, dir, map[string]string{
		"go.mod": "module example.com/app\n\ngo 1.26.0\n\nrequire github.com/go-kanna/i18n v0.0.0\n\n" +
			"require golang.org/x/text v0.43.0 // indirect\n\nreplace github.com/go-kanna/i18n => " + root + "\n",
		"go.sum": string(sum),
		"main.go": "package main\n\nimport (\n\t\"fmt\"\n\n\t\"example.com/app/messages\"\n)\n\n" +
			"func main() { fmt.Println(messages.Localizer(\"ja\").Localize(messages.Hello(\"Alice\"))) }\n",
		"messages/welcome.go": "package messages\n\nimport \"github.com/go-kanna/i18n\"\n\n" +
			"func Welcome(name string) i18n.Message { return Hello(name) }\n",
	})
	return dir
}

func TestCommand_generatesWhatCodeAlreadyCalls(t *testing.T) {
	t.Parallel()

	dir := app(t)
	if got := execute(t, dir); got != (result{}) {
		t.Fatalf("Exec() = %+v, want success without a word", got)
	}
}

func TestCommand_keepsWhatCodeStillCalls(t *testing.T) {
	t.Parallel()

	// Each change follows a first run, which main and Welcome compile with.
	tests := []struct {
		name string
		edit func(t *testing.T, dir string)
		want string
	}{
		{
			// main, which imports messages, is not compiled either.
			name: "a key removed",
			edit: func(t *testing.T, dir string) { editLocales(t, dir, "hello: ", "gone: ") },
			want: "messages/welcome.go:5:49: error: undefined: Hello\n",
		},
		{
			name: "a placeholder added",
			edit: func(t *testing.T, dir string) { editLocales(t, dir, "{name}", "{name} {age:int}") },
			want: "messages/welcome.go:5:59: error: not enough arguments in call to Hello\n\thave (string)\n" +
				"\twant (string, int)\n",
		},
		{
			name: "a key called from a test alone removed",
			edit: func(t *testing.T, dir string) {
				writeFiles(t, dir, map[string]string{
					"messages/welcome.go":      "package messages\n",
					"main.go":                  "package main\n\nfunc main() {}\n",
					"messages/welcome_test.go": "package messages\n\nvar _ = Hello(\"x\")\n",
				})
				editLocales(t, dir, "hello: ", "gone: ")
			},
			want: "messages/welcome_test.go:3:9: error: undefined: Hello\n",
		},
		{
			// The run writes Bye, which goes on to call what it has not.
			name: "a mistake behind a call to a message added",
			edit: func(t *testing.T, dir string) {
				editLocales(t, dir, "greeting: ", "bye: \"Bye!\"\ngreeting: ")
				writeFiles(t, dir, map[string]string{
					"other.go": "package main\n\nimport \"example.com/app/messages\"\n\nvar _ = messages.Bye().Render()\n",
				})
			},
			want: "./other.go:5:24: error: messages.Bye().Render undefined (type i18n.Message has no field or method Render)\n",
		},
		{
			// Whatever is wrong in the destination keeps its callers from
			// being checked, so it counts though it was there before.
			name: "an error in the destination",
			edit: func(t *testing.T, dir string) {
				writeFiles(t, dir, map[string]string{"messages/wip.go": "package messages\n\nvar _ int = \"wip\"\n"})
				editLocales(t, dir, "hello: ", "gone: ")
			},
			want: "messages/welcome.go:5:49: error: undefined: Hello\n" +
				"messages/wip.go:3:13: error: cannot use \"wip\" (untyped string constant) as int value in variable declaration\n",
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()

			dir := app(t)
			if got := execute(t, dir); got != (result{}) {
				t.Fatalf("first Exec() = %+v", got)
			}
			output := filepath.Join(dir, "messages", "i18n_gen.go")
			generated := readFile(t, output)

			tt.edit(t, dir)
			if got := execute(t, dir); got.code != cli.ExitFailure || got.err != tt.want {
				t.Errorf("Exec() = %d, stderr:\n%s\nwant %d and\n%s", got.code, got.err, cli.ExitFailure, tt.want)
			}
			if got := readFile(t, output); got != generated {
				t.Errorf("i18n_gen.go was rewritten:\n%s", got)
			}
		})
	}
}

func TestCommand_ignoresErrorsItDidNotCause(t *testing.T) {
	t.Parallel()

	// report imports messages, and had its mistake before the run.
	dir := app(t)
	writeFiles(t, dir, map[string]string{
		"report/report.go": "package report\n\nimport \"example.com/app/messages\"\n\nvar _ = messages.Greeting\n\n" +
			"var broken int = \"s\"\n",
	})
	if got := execute(t, dir); got != (result{}) {
		t.Errorf("Exec() = %+v, want success without a word", got)
	}
}

func TestCommand_unparsableDestination(t *testing.T) {
	t.Parallel()

	// extra.go declares Hello, which the run would declare again, past where
	// it no longer parses.
	dir := app(t)
	writeFiles(t, dir, map[string]string{
		"messages/welcome.go": "package messages\n",
		"messages/extra.go": "package messages\n\nimport \"github.com/go-kanna/i18n\"\n\n" +
			"func Hello(name string) i18n.Message { return i18n.Message{} }\n\nfunc broken( {\n",
	})
	want := "messages/extra.go:7:14: error: expected ')', found '{'\n"
	if got := execute(t, dir); got.code != cli.ExitFailure || got.err != want {
		t.Errorf("Exec() = %+v, want stderr %q", got, want)
	}
	if _, err := os.Stat(filepath.Join(dir, "messages", "i18n_gen.go")); err == nil {
		t.Error("i18n_gen.go was written")
	}
}

func TestCommand_regeneratesBrokenOutput(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name string
		edit func(src string) string
	}{
		{
			name: "a declaration that no longer compiles",
			edit: func(src string) string {
				return strings.Replace(src, "func Hello(name string)", "func Hello(name Missing)", 1)
			},
		},
		{
			// Two branches generated it with different releases.
			name: "a header in conflict",
			edit: func(src string) string {
				return strings.Replace(src, "// Code generated by kanna-i18n. DO NOT EDIT.\n",
					"<<<<<<< HEAD\n// Code generated by kanna-i18n v0.0.4. DO NOT EDIT.\n=======\n"+
						"// Code generated by kanna-i18n v0.0.5. DO NOT EDIT.\n>>>>>>> feature\n", 1)
			},
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()

			dir := app(t)
			if got := execute(t, dir); got != (result{}) {
				t.Fatalf("first Exec() = %+v", got)
			}
			output := filepath.Join(dir, "messages", "i18n_gen.go")
			generated := readFile(t, output)
			writeFiles(t, dir, map[string]string{"messages/i18n_gen.go": tt.edit(generated)})

			if got := execute(t, dir); got != (result{}) {
				t.Fatalf("Exec() = %+v, want success without a word", got)
			}
			if got := readFile(t, output); got != generated {
				t.Errorf("i18n_gen.go holds:\n%s\nwant what the first run wrote", got)
			}
		})
	}
}

// editLocales replaces old with replacement in every locale file of dir.
func editLocales(t *testing.T, dir, old, replacement string) {
	t.Helper()

	for name := range locales {
		path := filepath.Join(dir, name)
		writeFiles(t, dir, map[string]string{name: strings.ReplaceAll(readFile(t, path), old, replacement)})
	}
}
