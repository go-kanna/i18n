package i18n_test

import (
	"testing"

	"github.com/go-kanna/i18n"
)

func single(segs ...i18n.Segment) i18n.Entry { return i18n.Entry{Single: segs} }

func msg(key string, args ...i18n.Arg) i18n.Message { return i18n.Message{Key: key, Args: args} }

// newBundle is what kanna-i18n compiles these files into, plus a German catalog
// holding greeting alone:
//
//	# en.yaml
//	greeting: "Hello!"
//	hello: "Hello, {name}!"
//	items_count:
//	  plural:
//	    one: "You have {count} item."
//	    other: "You have {count} items."
//	total_price: "Total: {price:number}"
//	user:
//	  deleted: "User {name} has been deleted."
//
//	# ja.yaml, lacking user.deleted
//	greeting: "こんにちは！"
//	hello: "こんにちは、{name}さん！"
//	items_count:
//	  plural:
//	    other: "アイテムが{count}個あります。"
//	total_price: "合計: {price}"
func newBundle() i18n.Bundle {
	return i18n.NewBundle(
		i18n.Catalog{Lang: "en", Entries: map[string]i18n.Entry{
			"greeting": single(text("Hello!")),
			"hello":    single(text("Hello, "), param("name"), text("!")),
			"items_count": {Plural: &i18n.Plural{
				One:   i18n.Template{text("You have "), param("count"), text(" item.")},
				Other: i18n.Template{text("You have "), param("count"), text(" items.")},
			}},
			"total_price":  single(text("Total: "), param("price")),
			"user.deleted": single(text("User "), param("name"), text(" has been deleted.")),
		}},
		i18n.Catalog{Lang: "de", Entries: map[string]i18n.Entry{
			"greeting": single(text("Hallo!")),
		}},
		i18n.Catalog{Lang: "ja", Entries: map[string]i18n.Entry{
			"greeting": single(text("こんにちは！")),
			"hello":    single(text("こんにちは、"), param("name"), text("さん！")),
			"items_count": {Plural: &i18n.Plural{
				Other: i18n.Template{text("アイテムが"), param("count"), text("個あります。")},
			}},
			"total_price": single(text("合計: "), param("price")),
		}},
	)
}

func TestLocalizer_Localize(t *testing.T) {
	t.Parallel()

	b := newBundle()

	tests := []struct {
		name string
		lang string
		msg  i18n.Message
		want string
	}{
		{name: "en", lang: "en", msg: msg("hello", arg("name", "World")), want: "Hello, World!"},
		{name: "ja", lang: "ja", msg: msg("hello", arg("name", "太郎")), want: "こんにちは、太郎さん！"},
		{name: "en plural", lang: "en", msg: msg("items_count", arg("count", 1)), want: "You have 1 item."},
		{name: "ja plural", lang: "ja", msg: msg("items_count", arg("count", 1)), want: "アイテムが1個あります。"},
		{
			name: "missing from ja, found in the default",
			lang: "ja",
			msg:  msg("user.deleted", arg("name", "Alice")),
			want: "User Alice has been deleted.",
		},
		// German would write 1.234,56; the message is English, and so is its number.
		{
			name: "rendered as the language it is found in",
			lang: "de",
			msg:  msg("total_price", arg("price", 1234.56)),
			want: "Total: 1,234.56",
		},
		{name: "in no language", lang: "en", msg: msg("nope"), want: "nope"},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()

			if got := b.Localizer(tt.lang).Localize(tt.msg); got != tt.want {
				t.Errorf("Localizer(%q).Localize(%q) = %q, want %q", tt.lang, tt.msg.Key, got, tt.want)
			}
		})
	}
}

func TestBundle_Localizer(t *testing.T) {
	t.Parallel()

	b := newBundle()

	tests := []struct {
		name  string
		langs []string
		want  string
	}{
		{name: "tag", langs: []string{"ja"}, want: "こんにちは！"},
		{name: "region of a language", langs: []string{"ja-JP"}, want: "こんにちは！"},
		{name: "en-US", langs: []string{"en-US"}, want: "Hello!"},
		{name: "no match", langs: []string{"fr"}, want: "Hello!"},
		{name: "nothing asked", want: "Hello!"},
		{name: "Accept-Language", langs: []string{"fr, ja;q=0.8"}, want: "こんにちは！"},
		{name: "Accept-Language by weight", langs: []string{"ja;q=0.5, de"}, want: "Hallo!"},
		{name: "first that matches", langs: []string{"fr", "ja", "de"}, want: "こんにちは！"},
		{name: "first wins over later", langs: []string{"de", "ja"}, want: "Hallo!"},
		{name: "unparsable skipped", langs: []string{"not a tag", "ja"}, want: "こんにちは！"},
		{name: "a bad entry drops its whole value", langs: []string{"ja, not a tag", "de"}, want: "Hallo!"},
		{name: "empty skipped", langs: []string{"", "de"}, want: "Hallo!"},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()

			if got := b.Localizer(tt.langs...).Localize(msg("greeting")); got != tt.want {
				t.Errorf("Localizer(%q).Localize(greeting) = %q, want %q", tt.langs, got, tt.want)
			}
		})
	}
}

func TestBundle_Localizer_parents(t *testing.T) {
	t.Parallel()

	b := i18n.NewBundle(
		i18n.Catalog{Lang: "ja", Entries: map[string]i18n.Entry{
			"greeting": single(text("こんにちは！")),
			"only_ja":  single(text("ja")),
		}},
		i18n.Catalog{Lang: "en", Entries: map[string]i18n.Entry{
			"greeting": single(text("Hello!")),
			"only_en":  single(text("en")),
		}},
		i18n.Catalog{Lang: "en-GB", Entries: map[string]i18n.Entry{
			"greeting": single(text("Hello, mate!")),
		}},
	)
	enGB := b.Localizer("en-GB")

	tests := []struct {
		name string
		key  string
		want string
	}{
		{name: "its own catalog first", key: "greeting", want: "Hello, mate!"},
		{name: "then the parent", key: "only_en", want: "en"},
		{name: "then the default", key: "only_ja", want: "ja"},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()

			if got := enGB.Localize(msg(tt.key)); got != tt.want {
				t.Errorf("Localize(%q) = %q, want %q", tt.key, got, tt.want)
			}
		})
	}

	// The default need not be English: what matches nothing gets Japanese.
	if got := b.Localizer("fr").Localize(msg("greeting")); got != "こんにちは！" {
		t.Errorf("Localizer(fr).Localize(greeting) = %q, want こんにちは！", got)
	}
}

// The language matched is the closest one, as CLDR judges closeness, not only
// one named; and the values are tried one at a time, so the first with a close
// language wins over a later one with an exact match.
func TestBundle_Localizer_closest(t *testing.T) {
	t.Parallel()

	b := i18n.NewBundle(
		i18n.Catalog{Lang: "en", Entries: map[string]i18n.Entry{"greeting": single(text("Hello!"))}},
		i18n.Catalog{Lang: "da", Entries: map[string]i18n.Entry{"greeting": single(text("Hej!"))}},
	)

	tests := []struct {
		name  string
		langs []string
		want  string
	}{
		{name: "Norwegian by Danish", langs: []string{"nb"}, want: "Hej!"},
		{name: "value by value", langs: []string{"nb", "en-GB"}, want: "Hej!"},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()

			if got := b.Localizer(tt.langs...).Localize(msg("greeting")); got != tt.want {
				t.Errorf("Localizer(%q).Localize(greeting) = %q, want %q", tt.langs, got, tt.want)
			}
		})
	}
}

func TestZeroValues(t *testing.T) {
	t.Parallel()

	var l i18n.Localizer
	if got := l.Localize(msg("greeting")); got != "greeting" {
		t.Errorf("zero Localizer: Localize(greeting) = %q, want %q", got, "greeting")
	}

	var b i18n.Bundle
	if got := b.Localizer("en").Localize(msg("greeting")); got != "greeting" {
		t.Errorf("zero Bundle: Localize(greeting) = %q, want %q", got, "greeting")
	}
}

func TestNewBundle_panics(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name string
		def  i18n.Catalog
		rest []i18n.Catalog
	}{
		{name: "invalid default language", def: i18n.Catalog{Lang: "not a tag"}},
		{name: "invalid language", def: i18n.Catalog{Lang: "en"}, rest: []i18n.Catalog{{Lang: "not a tag"}}},
		{name: "default twice", def: i18n.Catalog{Lang: "en"}, rest: []i18n.Catalog{{Lang: "en"}}},
		// Tags are compared once parsed, so spelling does not tell two apart.
		{name: "one language twice", def: i18n.Catalog{Lang: "en"}, rest: []i18n.Catalog{{Lang: "pt-BR"}, {Lang: "pt-br"}}},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()

			defer func() {
				if recover() == nil {
					t.Error("NewBundle() did not panic")
				}
			}()
			i18n.NewBundle(tt.def, tt.rest...)
		})
	}
}
