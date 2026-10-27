package gen_test

import (
	"slices"
	"testing"

	"github.com/go-kanna/i18n/internal/gen"
)

func TestParseTemplate(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name         string
		src          string
		wantSegments []string
		wantParams   []string
	}{
		{name: "empty", src: ""},
		{name: "text", src: "Hello!", wantSegments: []string{`"Hello!"`}},
		{
			name:         "placeholder",
			src:          "Hello, {name}!",
			wantSegments: []string{`"Hello, "`, "{name}", `"!"`},
			wantParams:   []string{"name:string"},
		},
		{
			name:         "kinds",
			src:          "{a:string}{b:int}{c:number}",
			wantSegments: []string{"{a}", "{b}", "{c}"},
			wantParams:   []string{"a:string!", "b:int!", "c:number!"},
		},
		{
			name:         "a placeholder twice is one parameter",
			src:          "{a} and {a}",
			wantSegments: []string{"{a}", `" and "`, "{a}"},
			wantParams:   []string{"a:string"},
		},
		{
			name:         "the kind written out later",
			src:          "{n} of {n:int}",
			wantSegments: []string{"{n}", `" of "`, "{n}"},
			wantParams:   []string{"n:int!"},
		},
		{
			name:         "the kind written out first",
			src:          "{n:int} of {n}",
			wantSegments: []string{"{n}", `" of "`, "{n}"},
			wantParams:   []string{"n:int!"},
		},
		{name: "literal braces", src: "{{name}} and }}", wantSegments: []string{`"{name} and }"`}},
		{
			name:         "literal braces around a placeholder",
			src:          "{{{name}}}",
			wantSegments: []string{`"{"`, "{name}", `"}"`},
			wantParams:   []string{"name:string"},
		},
		{
			name:         "multibyte text",
			src:          "こんにちは、{name}さん！",
			wantSegments: []string{`"こんにちは、"`, "{name}", `"さん！"`},
			wantParams:   []string{"name:string"},
		},
		{
			name:         "digits and underscores in a name",
			src:          "{user_name2}",
			wantSegments: []string{"{user_name2}"},
			wantParams:   []string{"user_name2:string"},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()

			segments, params, diags := gen.ParseTemplate(tt.src)
			if len(diags) > 0 {
				t.Fatalf("ParseTemplate(%q) diagnostics: %q", tt.src, diags)
			}
			if !slices.Equal(segments, tt.wantSegments) {
				t.Errorf("segments = %q, want %q", segments, tt.wantSegments)
			}
			if !slices.Equal(params, tt.wantParams) {
				t.Errorf("params = %q, want %q", params, tt.wantParams)
			}
		})
	}
}

func TestParseTemplate_errors(t *testing.T) {
	t.Parallel()

	const badName = "a name matches [a-z][a-z0-9_]*"

	tests := []struct {
		name string
		src  string
		want []string
	}{
		{
			name: "unclosed",
			src:  "Hello, {name",
			want: []string{"error: \"{\" opens a placeholder that is not closed\n  write \"{{\" for a literal \"{\""},
		},
		{
			name: "closing nothing",
			src:  "Hello }",
			want: []string{"error: \"}\" closes no placeholder\n  write \"}}\" for a literal \"}\""},
		},
		{name: "no name", src: "{}", want: []string{"error: invalid placeholder {}: " + badName}},
		{name: "starting with a digit", src: "{9lives}", want: []string{"error: invalid placeholder {9lives}: " + badName}},
		{name: "starting with an underscore", src: "{_x}", want: []string{"error: invalid placeholder {_x}: " + badName}},
		{name: "upper case", src: "{Name}", want: []string{"error: invalid placeholder {Name}: " + badName}},
		{name: "a space", src: "{first name}", want: []string{"error: invalid placeholder {first name}: " + badName}},
		{
			name: "misspelled kind",
			src:  "{price:numbr}",
			want: []string{"error: unknown kind \"numbr\" in {price:numbr}\n  did you mean \"number\"?"},
		},
		{
			name: "unknown kind",
			src:  "{price:float}",
			want: []string{"error: unknown kind \"float\" in {price:float}\n  a kind is string, int, or number"},
		},
		{
			name: "no kind after the colon",
			src:  "{price:}",
			want: []string{"error: unknown kind \"\" in {price:}\n  a kind is string, int, or number"},
		},
		{
			name: "two kinds",
			src:  "{x:int} and {x:number}",
			want: []string{"error: placeholder x is written as both int and number"},
		},
		{
			name: "every mistake that can be told apart",
			src:  "} {Name} {price:numbr} {a",
			want: []string{
				"error: \"}\" closes no placeholder\n  write \"}}\" for a literal \"}\"",
				"error: invalid placeholder {Name}: " + badName,
				"error: unknown kind \"numbr\" in {price:numbr}\n  did you mean \"number\"?",
				"error: \"{\" opens a placeholder that is not closed\n  write \"{{\" for a literal \"{\"",
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()

			if _, _, diags := gen.ParseTemplate(tt.src); !slices.Equal(diags, tt.want) {
				t.Errorf("ParseTemplate(%q) diagnostics =\n%q\nwant\n%q", tt.src, diags, tt.want)
			}
		})
	}
}
