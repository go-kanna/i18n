package gen_test

import (
	"slices"
	"testing"

	"github.com/go-kanna/i18n/internal/gen"
)

// The en.yaml of the README.
const enYAML = `greeting: "Hello!"
hello: "Hello, {name}!"
items_count:
  plural:
    one: "You have {count} item."
    other: "You have {count} items."
total_price: "Total: {price:number}"
user:
  not_found: "User not found."
`

func TestAnalyze(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name             string
		def              string
		locales          map[string]string
		declared         []string
		wantConstructors []string
		wantCatalogs     []string
	}{
		{
			name: "the README",
			def:  "en",
			locales: map[string]string{
				"en": enYAML,
				"ja": "greeting: \"こんにちは！\"\nhello: \"こんにちは、{name}さん！\"\n" +
					"items_count:\n  plural:\n    other: \"アイテムが{count}個あります。\"\n" +
					"total_price: \"合計: {price}\"\nuser:\n  not_found: \"ユーザーが見つかりません。\"\n",
			},
			wantConstructors: []string{
				"Greeting()", "Hello(name string)", "ItemsCount(count int)", "TotalPrice(price number)", "UserNotFound()",
			},
			wantCatalogs: []string{"en", "ja"},
		},
		{
			name:         "the default first, then the rest by tag",
			def:          "ja",
			locales:      map[string]string{"en": "", "ja": "", "de": ""},
			wantCatalogs: []string{"ja", "de", "en"},
		},
		{
			name: "the count first, then the placeholders of every form",
			def:  "en",
			locales: map[string]string{
				"en": "items:\n  plural:\n" +
					"    other: \"{count} items, {price:number}, for {name}\"\n" +
					"    one: \"{name} has {count} item\"\n",
			},
			wantConstructors: []string{"Items(count int, name string, price number)"},
			wantCatalogs:     []string{"en"},
		},
		{
			name: "a kind written in one form",
			def:  "en",
			locales: map[string]string{
				"en": "total:\n  plural:\n    one: \"{price} for one\"\n    other: \"{price:number} for {count}\"\n",
			},
			wantConstructors: []string{"Total(count int, price number)"},
			wantCatalogs:     []string{"en"},
		},
		{
			name:             "Go names",
			def:              "en",
			locales:          map[string]string{"en": "user:\n  name_of: \"{type} {error} {user_name} {i18n}\"\n"},
			wantConstructors: []string{"UserNameOf(typeArg string, errorArg string, userName string, i18n string)"},
			wantCatalogs:     []string{"en"},
		},
		{
			name:             "what the destination declares besides",
			def:              "en",
			locales:          map[string]string{"en": "greeting: \"Hi\"\n"},
			declared:         []string{"greeting@extra.go:3", "Hello@extra.go:5"},
			wantConstructors: []string{"Greeting()"},
			wantCatalogs:     []string{"en"},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()

			constructors, catalogs, diags := gen.Analyze(tt.def, tt.locales, tt.declared...)
			if len(diags) > 0 {
				t.Fatalf("Analyze() diagnostics: %q", diags)
			}
			if !slices.Equal(constructors, tt.wantConstructors) {
				t.Errorf("constructors = %q, want %q", constructors, tt.wantConstructors)
			}
			if !slices.Equal(catalogs, tt.wantCatalogs) {
				t.Errorf("catalogs = %q, want %q", catalogs, tt.wantCatalogs)
			}
		})
	}
}

func TestAnalyze_diagnostics(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name     string
		locales  map[string]string
		declared []string
		want     []string
	}{
		{
			name: "the README",
			locales: map[string]string{
				"en": enYAML,
				"ja": "greeting: \"こんにちは！\"\nhello: \"こんにちは、{namae}さん！\"\n" +
					"items_count:\n  plural:\n    other: \"アイテムが{count}個あります。\"\n" +
					"total_price: \"合計: {price:int}\"\nuser:\n  not_fund: \"ユーザーが見つかりません。\"\n",
			},
			want: []string{
				"locales/ja.yaml:2:8: error: parameter \"namae\" of key \"hello\" is not in en\n  did you mean \"name\"?",
				`locales/ja.yaml:6:14: error: parameter "price" of key "total_price" is int, but number in en`,
				"locales/ja.yaml:8:3: error: key \"user.not_fund\" is not in en\n  did you mean \"user.not_found\"?",
				"locales/ja.yaml: warning: missing keys: user.not_found",
			},
		},
		{
			// A built-in text needs no key in the default language to be
			// overridden in another; a key next to it still does.
			name: "a translation overriding a built-in text",
			locales: map[string]string{
				"en": "greeting: \"Hello!\"\n",
				"ja": "greeting: \"こんにちは！\"\nerrors:\n  messages:\n    blank: \"は必須です\"\n    blnk: \"は必須です\"\n",
			},
			want: []string{"locales/ja.yaml:5:5: error: key \"errors.messages.blnk\" is not in en"},
		},
		{
			name: "two kinds in two forms",
			locales: map[string]string{
				"en": "items:\n  plural:\n    one: \"{x:int}\"\n    other: \"{x:number}\"\n",
			},
			want: []string{`locales/en.yaml:4:12: error: placeholder x of key "items" is written as both int and number`},
		},
		{
			name:    "two placeholders, one Go name",
			locales: map[string]string{"en": "t: \"{user_name} {user__name}\"\n"},
			want: []string{
				`locales/en.yaml:1:1: error: placeholders user_name and user__name of key "t" both become the parameter userName`,
			},
		},
		{
			name:    "two keys, one constructor",
			locales: map[string]string{"en": "user_name: \"a\"\nuser:\n  name: \"b\"\n"},
			want:    []string{`locales/en.yaml:3:3: error: keys "user_name" and "user.name" both generate UserName`},
		},
		{
			name:    "the Localizer",
			locales: map[string]string{"en": "localizer: \"a\"\n"},
			want: []string{
				"locales/en.yaml:1:1: error: key \"localizer\" generates Localizer, which the generated file declares itself\n" +
					"  rename the key",
			},
		},
		{
			name:     "a name the destination declares",
			locales:  map[string]string{"en": "greeting: \"Hi\"\n"},
			declared: []string{"Greeting@extra.go:3"},
			want: []string{
				"locales/en.yaml:1:1: error: key \"greeting\" generates Greeting, which extra.go:3 declares already\n" +
					"  rename the key, or what declares Greeting",
			},
		},
		{
			name:     "a Localizer the destination declares",
			locales:  map[string]string{"en": "greeting: \"Hi\"\n"},
			declared: []string{"Localizer@extra.go:5"},
			want: []string{
				"messages/extra.go:5: error: Localizer is declared here, and the generated file declares it too\n" +
					"  rename it",
			},
		},
		{
			// The translations would be blamed for what the default lacks.
			name: "translations left alone while the default is broken",
			locales: map[string]string{
				"en": "localizer: \"a\"\n",
				"ja": "localizer: \"あ\"\nnope: \"x\"\n",
			},
			want: []string{
				"locales/en.yaml:1:1: error: key \"localizer\" generates Localizer, which the generated file declares itself\n" +
					"  rename the key",
			},
		},
		{
			name: "plural in a translation alone",
			locales: map[string]string{
				"en": "items: \"Items\"\n",
				"ja": "items:\n  plural:\n    other: \"アイテム\"\n",
			},
			want: []string{`locales/ja.yaml:1:1: error: key "items" is plural here, but not in en`},
		},
		{
			name: "plural in the default alone",
			locales: map[string]string{
				"en": "items:\n  plural:\n    one: \"Item\"\n    other: \"Items\"\n",
				"ja": "items: \"アイテム\"\n",
			},
			want: []string{`locales/ja.yaml:1:1: error: key "items" is not plural here, but plural in en`},
		},
		{
			name: "a kind written out as string",
			locales: map[string]string{
				"en": "total: \"{price:number}\"\n",
				"ja": "total: \"{price:string}\"\n",
			},
			want: []string{`locales/ja.yaml:1:8: error: parameter "price" of key "total" is string, but number in en`},
		},
		{
			name: "a placeholder in a plural form of a translation",
			locales: map[string]string{
				"en": "items:\n  plural:\n    one: \"{count} item\"\n    other: \"{count} items\"\n",
				"ja": "items:\n  plural:\n    other: \"{count}個の{thing}\"\n",
			},
			want: []string{`locales/ja.yaml:3:12: error: parameter "thing" of key "items" is not in en`},
		},
		{
			name: "plural forms a language tells apart",
			locales: map[string]string{
				"en": "items:\n  plural:\n    other: \"{count} items\"\n",
				"ja": "items:\n  plural:\n    other: \"{count}個\"\n",
				"pl": "items:\n  plural:\n    other: \"{count} przedmiotu\"\n",
				"ru": "items:\n  plural:\n    one: \"{count} предмет\"\n    other: \"{count} предмета\"\n",
			},
			want: []string{
				`locales/en.yaml:1:1: warning: key "items" lacks the plural form one, so those counts render with "other"`,
				"locales/pl.yaml:1:1: warning: key \"items\" lacks the plural forms one, few, and many, " +
					"so those counts render with \"other\"",
				"locales/ru.yaml:1:1: warning: key \"items\" lacks the plural forms few and many, " +
					"so those counts render with \"other\"",
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()

			if _, _, diags := gen.Analyze("en", tt.locales, tt.declared...); !slices.Equal(diags, tt.want) {
				t.Errorf("Analyze() diagnostics =\n%q\nwant\n%q", diags, tt.want)
			}
		})
	}
}
