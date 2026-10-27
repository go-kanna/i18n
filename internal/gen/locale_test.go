package gen_test

import (
	"slices"
	"testing"

	"github.com/go-kanna/i18n/internal/gen"
)

func TestParseLocale(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name string
		src  string
		want []string
	}{
		{name: "empty", src: ""},
		{name: "comments only", src: "# nothing yet\n"},
		{name: "empty document", src: "---\n"},
		{
			name: "messages",
			src:  "greeting: \"Hello!\"\nhello: Hello, {name}!\n",
			want: []string{`greeting@1:1 "Hello!"@1:11`, `hello@2:1 "Hello, {name}!"@2:8`},
		},
		{
			name: "nesting",
			src:  "user:\n  not_found: \"User not found.\"\n  admin:\n    deleted: \"Gone.\"\n",
			want: []string{`user.not_found@2:3 "User not found."@2:14`, `user.admin.deleted@4:5 "Gone."@4:14`},
		},
		{
			name: "plural",
			src:  "items:\n  plural:\n    one: \"One\"\n    other: \"Many\"\n",
			want: []string{`items@1:1 one "One"@3:10 other "Many"@4:12`},
		},
		{
			name: "plural forms in the order of categories",
			src:  "items:\n  plural:\n    other: \"Many\"\n    zero: \"None\"\n    one: \"One\"\n",
			want: []string{`items@1:1 zero "None"@4:11 one "One"@5:10 other "Many"@3:12`},
		},
		{
			name: "category names elsewhere are ordinary keys",
			src:  "count:\n  one: \"One\"\n  other: \"Many\"\n",
			want: []string{`count.one@2:3 "One"@2:8`, `count.other@3:3 "Many"@3:10`},
		},
		{
			name: "a string under plural is an ordinary message",
			src:  "items:\n  plural: \"Items\"\n",
			want: []string{`items.plural@2:3 "Items"@2:11`},
		},
		{
			name: "block scalar",
			src:  "terms: |\n  Line one.\n  Line two.\n",
			want: []string{`terms@1:1 "Line one.\nLine two.\n"@1:8`},
		},
		{name: "quoted key", src: "\"greeting\": \"Hi\"\n", want: []string{`greeting@1:1 "Hi"@1:13`}},
		{name: "empty message", src: "blank: \"\"\n", want: []string{`blank@1:1 ""@1:8`}},
		{
			// An alias is where its anchor is written.
			name: "alias",
			src:  "hi: &hi \"Hi\"\nhello: *hi\n",
			want: []string{`hi@1:1 "Hi"@1:5`, `hello@2:1 "Hi"@1:5`},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()

			entries, diags := gen.ParseLocale(tt.src)
			if len(diags) > 0 {
				t.Fatalf("ParseLocale() diagnostics: %q", diags)
			}
			if !slices.Equal(entries, tt.want) {
				t.Errorf("ParseLocale() =\n%q\nwant\n%q", entries, tt.want)
			}
		})
	}
}

func TestParseLocale_errors(t *testing.T) {
	t.Parallel()

	const badKey = "each part of a key matches [a-z][a-z0-9_]*"

	tests := []struct {
		name string
		src  string
		want []string
	}{
		{
			name: "syntax",
			src:  "greeting: [\n",
			want: []string{"en.yaml:1: error: did not find expected node content"},
		},
		{
			name: "syntax without a line",
			src:  "\tgreeting: hi\n",
			want: []string{"en.yaml: error: found character that cannot start any token"},
		},
		{
			name: "two documents",
			src:  "a: x\n---\nb: y\n",
			want: []string{"en.yaml:2:1: error: a locale file holds one YAML document, and this is a second"},
		},
		{
			name: "syntax in a second document",
			src:  "a: x\n---\nb: [\n",
			want: []string{"en.yaml:3: error: did not find expected node content"},
		},
		{name: "list", src: "- a\n", want: []string{"en.yaml:1:1: error: a locale file maps keys to messages"}},
		{name: "string", src: "hello\n", want: []string{"en.yaml:1:1: error: a locale file maps keys to messages"}},
		{
			name: "upper case key",
			src:  "Greeting: hi\n",
			want: []string{`en.yaml:1:1: error: invalid key "Greeting": ` + badKey},
		},
		{name: "key with a dot", src: "\"a.b\": hi\n", want: []string{`en.yaml:1:1: error: invalid key "a.b": ` + badKey}},
		{name: "number key", src: "123: hi\n", want: []string{`en.yaml:1:1: error: invalid key "123": ` + badKey}},
		{
			name: "merge key",
			src:  "a: &a {b: x}\nc:\n  <<: *a\n",
			want: []string{`en.yaml:3:3: error: invalid key "<<": ` + badKey},
		},
		{
			name: "duplicate key",
			src:  "a: x\nb: y\na: z\n",
			want: []string{`en.yaml:3:1: error: key "a" is written twice`},
		},
		{
			name: "duplicate nested key",
			src:  "user:\n  name: x\n  name: y\n",
			want: []string{`en.yaml:3:3: error: key "user.name" is written twice`},
		},
		{
			name: "number",
			src:  "greeting: 123\n",
			want: []string{
				"en.yaml:1:11: error: message of key \"greeting\" is not a string\n" +
					"  quote it to have it read as text: \"123\"",
			},
		},
		{
			name: "bool",
			src:  "greeting: true\n",
			want: []string{
				"en.yaml:1:11: error: message of key \"greeting\" is not a string\n" +
					"  quote it to have it read as text: \"true\"",
			},
		},
		{
			name: "null",
			src:  "greeting:\n",
			want: []string{"en.yaml:1:10: error: message of key \"greeting\" is missing\n  write \"\" for an empty message"},
		},
		{
			name: "list value",
			src:  "greeting:\n  - hi\n",
			want: []string{`en.yaml:2:3: error: message of key "greeting" is not a string`},
		},
		{name: "empty mapping", src: "user: {}\n", want: []string{`en.yaml:1:7: error: key "user" holds an empty mapping`}},
		{
			name: "template",
			src:  "greeting: \"Hello, {name\"\n",
			want: []string{
				"en.yaml:1:11: error: \"{\" opens a placeholder that is not closed\n" +
					"  write \"{{\" for a literal \"{\"",
			},
		},
		{
			name: "plural without other",
			src:  "items:\n  plural:\n    one: \"One\"\n",
			want: []string{"en.yaml:3:5: error: plural forms of key \"items\" lack \"other\"\n" +
				"  other is the form a count without a form of its own renders with"},
		},
		{
			name: "plural beside other keys",
			src:  "items:\n  plural:\n    other: \"x\"\n  title: \"y\"\n",
			want: []string{"en.yaml:1:1: error: key \"items\" holds plural forms and other keys beside them\n" +
				"  \"plural\" is the key reserved for the forms of a plural message"},
		},
		{
			name: "misspelled category",
			src:  "items:\n  plural:\n    others: \"x\"\n",
			want: []string{
				"en.yaml:3:5: error: \"others\" is not a plural category\n  did you mean \"other\"?",
				"en.yaml:3:5: error: plural forms of key \"items\" lack \"other\"\n" +
					"  other is the form a count without a form of its own renders with",
			},
		},
		{
			name: "unknown category",
			src:  "items:\n  plural:\n    other: \"x\"\n    custom: \"y\"\n",
			want: []string{
				"en.yaml:4:5: error: \"custom\" is not a plural category\n" +
					"  a category is zero, one, two, few, many, or other",
			},
		},
		{
			name: "category twice",
			src:  "items:\n  plural:\n    other: \"x\"\n    other: \"y\"\n",
			want: []string{`en.yaml:4:5: error: plural form "other" of key "items" is written twice`},
		},
		{
			name: "form not a string",
			src:  "items:\n  plural:\n    other:\n      nested: \"x\"\n",
			want: []string{`en.yaml:4:7: error: plural form "other" of key "items" is not a string`},
		},
		{
			name: "count as a number",
			src:  "items:\n  plural:\n    other: \"{count:number} items\"\n",
			want: []string{"en.yaml:3:12: error: plural form \"other\" of key \"items\" writes the count as number, " +
				"but the count of a plural message is an int"},
		},
		{
			name: "every message with a mistake",
			src:  "a: 1\nb: \"fine\"\nc: \"{x\"\n",
			want: []string{
				"en.yaml:1:4: error: message of key \"a\" is not a string\n  quote it to have it read as text: \"1\"",
				"en.yaml:3:4: error: \"{\" opens a placeholder that is not closed\n  write \"{{\" for a literal \"{\"",
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()

			if _, diags := gen.ParseLocale(tt.src); !slices.Equal(diags, tt.want) {
				t.Errorf("ParseLocale() diagnostics =\n%q\nwant\n%q", diags, tt.want)
			}
		})
	}
}

// A message with a mistake is left out, and the others are kept.
func TestParseLocale_keepsTheRest(t *testing.T) {
	t.Parallel()

	entries, diags := gen.ParseLocale("a: 1\nb: \"fine\"\n")
	if len(diags) != 1 {
		t.Errorf("diagnostics = %q, want one", diags)
	}
	if want := []string{`b@2:1 "fine"@2:4`}; !slices.Equal(entries, want) {
		t.Errorf("entries = %q, want %q", entries, want)
	}
}
