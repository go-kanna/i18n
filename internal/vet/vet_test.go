package vet_test

import (
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/go-kanna/internal/cli"
	"github.com/go-kanna/internal/diag"

	"github.com/go-kanna/i18n/internal/gen"
	"github.com/go-kanna/i18n/internal/vet"
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

func TestRun(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name  string
		files map[string]string
		// gens are the arguments of each run of kanna-i18n.
		gens [][]string
		want []string
	}{
		{
			name: "two packages",
			files: map[string]string{
				"locales/en.yaml": `errors:
  messages:
    too_young: "is too young"
    corp_domain: "must not be at {domain}"
    taken: "is taken by {owner}"
    items_left:
      plural:
        one: "has {count} item left for {name}"
        other: "has {count} items left"
    seats:
      plural:
        one: "has one seat"
        other: "has {count} seats"
    slots:
      plural:
        one: "has {count} slot"
        other: "has slots"
notices:
  welcome: "Welcome, {name}!"
`,
				"locales/ja.yaml": `errors:
  messages:
    too_young: "は若すぎます"
    corp_domain: "は使えないドメインです"
    taken: "は{owner}が使っています"
notices:
  welcome: "ようこそ、{name}さん！"
`,
				"admin/locales/en.yaml": "errors:\n  messages:\n    admin_only: \"is for admins\"\n",
				"usecase/user.go": `package usecase

import (
	"github.com/go-kanna/i18n"
	"github.com/go-kanna/validate"
	"github.com/go-kanna/value/rule"
)

const tooYoung = "too_young"

func Check(code, name string, n int, params map[string]any) []error {
	return []error{
		validate.Fail(tooYoung),
		validate.Fail("too_yuong"),
		validate.Fail("blank"),
		validate.Fail("blamk"),
		validate.Fail("admin_only"),
		validate.Fail(code),
		validate.FailWith("corp_domain", nil),
		validate.FailWith("taken", nil),
		validate.FailWith("taken", map[string]any{"owner": name}),
		validate.FailWith("taken", params),
		rule.FailWith("items_left", map[string]any{"count": n}),
		rule.FailWith("seats", nil),
		rule.Fail("slots"),
		Fail("nothing"),
	}
}

func Fail(code string) error { return nil }

var (
	_ = i18n.Coded("notices", "welcom", nil)
	_ = i18n.Coded("notices", "welcome", nil)
	_ = i18n.Coded("", "notices.welcome", map[string]any{"name": "Alice"})
)
`,
				"usecase/user_test.go": "package usecase\n\nimport \"github.com/go-kanna/validate\"\n\n" +
					"var _ = validate.Fail(\"nope\")\n",
			},
			gens: [][]string{nil, {"-locales", "admin/locales", "-destination", "admin/messages"}},
			want: []string{
				`user.go:14:17: error: "too_yuong" has no text in any language of admin/messages, messages, ` +
					"so it renders as errors.messages.too_yuong\n" +
					`  did you mean "too_young"?`,
				`user.go:16:17: error: "blamk" has no text in any language of admin/messages, messages, ` +
					"so it renders as errors.messages.blamk\n" +
					`  did you mean "blank"?`,
				`user.go:20:30: error: errors.messages.taken renders "{owner}" in every language of admin/messages, ` +
					"messages that has its text: the call gives no owner",
				`user.go:24:26: error: errors.messages.seats renders "{count}" in every language of admin/messages, ` +
					"messages that has its text: the call gives no count",
				`user.go:33:28: error: "welcom" has no text in any language of admin/messages, messages, ` +
					"so it renders as notices.welcom\n" +
					`  did you mean "welcome"?`,
				`user.go:34:39: error: notices.welcome renders "{name}" in every language of admin/messages, ` +
					"messages that has its text: the call gives no name",
			},
		},
		{
			name: "built-in texts no language reaches",
			files: map[string]string{
				"locales/fr.yaml": "greeting: \"Bonjour\"\n",
				"check/check.go":  checkBuiltin,
			},
			gens: [][]string{{"-default", "fr"}},
			want: []string{
				`check.go:6:16: error: "blank" has no text in any language of messages, so it renders as errors.messages.blank`,
				`check.go:7:20: error: "too_long" has no text in any language of messages, ` +
					"so it renders as errors.messages.too_long",
				`check.go:8:20: error: "too_long" has no text in any language of messages, ` +
					"so it renders as errors.messages.too_long",
			},
		},
		{
			name: "built-in texts a language reaches",
			files: map[string]string{
				"locales/fr.yaml": "greeting: \"Bonjour\"\n",
				"locales/ja.yaml": "greeting: \"こんにちは\"\n",
				"check/check.go":  checkBuiltin,
			},
			gens: [][]string{{"-default", "fr"}},
			want: []string{
				`check.go:7:32: error: errors.messages.too_long renders "{count}" in every language of messages ` +
					"that has its text: the call gives no count",
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()

			dir := module(t, tt.files, tt.gens...)
			got := report(vet.Run(dir, "", []string{"./..."}))
			if want := strings.Join(tt.want, "\n"); got != want {
				t.Errorf("Run():\n%s\nwant:\n%s", got, want)
			}
		})
	}
}

// checkBuiltin reports codes of built-in texts.
const checkBuiltin = `package check

import "github.com/go-kanna/validate"

var _ = []error{
	validate.Fail("blank"),
	validate.FailWith("too_long", nil),
	validate.FailWith("too_long", map[string]any{"count": 10}),
}
`

func TestRun_messages(t *testing.T) {
	t.Parallel()

	const use = "package use\n\nimport \"github.com/go-kanna/validate\"\n\nvar _ = validate.Fail(\"blank\")\n"
	tests := []struct {
		name  string
		files map[string]string
		want  string
	}{
		{
			name:  "none",
			files: map[string]string{"use/use.go": use},
			want:  "error: no package of module example.com/app holds what kanna-i18n generates",
		},
		{
			name: "hand-written",
			files: map[string]string{
				"use/use.go":               use,
				"messages/i18n_gen.go":     "package messages\n\nvar Lang = \"en\"\n",
				"messages/notices/i18n.go": "// Code generated by kanna-i18n. DO NOT EDIT.\n\npackage notices\n",
			},
			want: "error: no package of module example.com/app holds what kanna-i18n generates",
		},
		{
			name: "unreadable",
			files: map[string]string{
				"use/use.go": use,
				"messages/i18n_gen.go": `// Code generated by kanna-i18n v0.0.19. DO NOT EDIT.

package messages

import "github.com/go-kanna/i18n"

var lang = "en"

var bundle = i18n.NewBundle(i18n.Catalog{Lang: lang})
`,
			},
			want: "i18n_gen.go:9:48: error: cannot read the catalogs kanna-i18n generated: lang is not a constant string\n" +
				"  regenerate the file with kanna-i18n",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()

			dir := module(t, tt.files)
			if got := report(vet.Run(dir, "", []string{"./..."})); got != tt.want {
				t.Errorf("Run():\n%s\nwant:\n%s", got, tt.want)
			}
		})
	}
}

// fakes stand in for kanna-validate and kanna-value, declaring what vet checks
// with the signatures of the real ones.
var fakes = map[string]string{
	"fake/validate/go.mod": "module github.com/go-kanna/validate\n\ngo 1.26.0\n",
	"fake/validate/validate.go": `package validate

type Error struct{}

func (*Error) Error() string { return "" }

func Fail(code string) *Error { return nil }

func FailWith(code string, params map[string]any) *Error { return nil }
`,
	"fake/value/go.mod": "module github.com/go-kanna/value\n\ngo 1.26.0\n",
	"fake/value/rule/rule.go": `package rule

type Error struct{}

func (*Error) Error() string { return "" }

func Fail(code string) *Error { return nil }

func FailWith(code string, params map[string]any) *Error { return nil }
`,
}

// module writes files into a new directory, as the module example.com/app
// requiring kanna-i18n of this checkout and the fakes, runs kanna-i18n there
// with each of gens, and returns the directory.
func module(t *testing.T, files map[string]string, gens ...[]string) string {
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
	write := func(files map[string]string) {
		for name, src := range files {
			path := filepath.Join(dir, filepath.FromSlash(name))
			if err := os.MkdirAll(filepath.Dir(path), 0o750); err != nil {
				t.Fatal(err)
			}
			if err := os.WriteFile(path, []byte(src), 0o600); err != nil {
				t.Fatal(err)
			}
		}
	}
	write(fakes)
	write(files)
	write(map[string]string{
		"go.mod": `module example.com/app

go 1.26.0

require (
	github.com/go-kanna/i18n v0.0.0
	github.com/go-kanna/validate v0.0.0
	github.com/go-kanna/value v0.0.0
)

require golang.org/x/text v0.43.0 // indirect

replace (
	github.com/go-kanna/i18n => ` + root + `
	github.com/go-kanna/validate => ./fake/validate
	github.com/go-kanna/value => ./fake/value
)
`,
		"go.sum": string(sum),
	})

	for _, args := range gens {
		var out, errOut strings.Builder
		if code := gen.Command().Exec(cli.Env{Out: &out, Err: &errOut, Dir: dir}, args); code != cli.ExitOK {
			t.Fatalf("kanna-i18n %v = %d:\n%s", args, code, errOut.String())
		}
	}
	return dir
}

// report renders ds as diag does, naming each file by its base name.
func report(ds []diag.Diag) string {
	lines := make([]string, 0, len(ds))
	for _, d := range ds {
		if d.Pos.Filename != "" {
			d.Pos.Filename = filepath.Base(d.Pos.Filename)
		}
		lines = append(lines, d.String())
	}
	return strings.Join(lines, "\n")
}
