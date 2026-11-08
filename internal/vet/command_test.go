package vet_test

import (
	"bytes"
	"path/filepath"
	"strings"
	"testing"

	"github.com/go-kanna/internal/cli"

	"github.com/go-kanna/i18n/internal/vet"
)

func TestCommand(t *testing.T) {
	t.Parallel()

	dir := module(t, map[string]string{
		"locales/en.yaml": "errors:\n  messages:\n    taken: \"is taken\"\n",
		"clean/clean.go":  "package clean\n",
		"usecase/user.go": `package usecase

import "github.com/go-kanna/validator"

var _ = validator.Fail("takn")
`,
		"tagged/tagged.go": `//go:build extra

package tagged

import "github.com/go-kanna/validator"

var _ = validator.Fail("tkaen")
`,
	}, nil)
	const finding = `error: "takn" has no text in any language of messages, so it renders as errors.messages.takn
  did you mean "taken"?
`
	for _, tt := range []struct {
		dir    string
		args   []string
		code   int
		stdout string
		stderr string
	}{
		{filepath.Join(dir, "clean"), nil, cli.ExitOK, "", ""},
		{dir, []string{"./usecase"}, cli.ExitFailure, "", "usecase/user.go:5:24: " + finding},
		{filepath.Join(dir, "usecase"), nil, cli.ExitFailure, "", "./user.go:5:24: " + finding},
		{dir, []string{"./clean", "./tagged"}, cli.ExitFailure, "", ""},
		{dir, []string{"-tags", "extra", "./tagged"}, cli.ExitFailure, "",
			`tagged/tagged.go:7:24: error: "tkaen" has no text in any language of messages, ` +
				"so it renders as errors.messages.tkaen\n" +
				"  did you mean \"taken\"?\n"},
		{dir, []string{"./does-not-exist"}, cli.ExitFailure, "", ""},
		{dir, []string{"-h"}, cli.ExitOK, "kanna-i18n vet: find the error codes that render as their key", ""},
	} {
		var out, errOut bytes.Buffer
		code := vet.Command().Exec(cli.Env{Out: &out, Err: &errOut, Dir: tt.dir}, tt.args)
		if code != tt.code ||
			!strings.HasPrefix(out.String(), tt.stdout) ||
			(tt.stderr != "" && !strings.HasPrefix(errOut.String(), tt.stderr)) {
			t.Errorf("vet %v in %s = %d, want %d\nstdout:\n%s\nstderr:\n%s",
				tt.args, tt.dir, code, tt.code, out.String(), errOut.String())
		}
	}
}
