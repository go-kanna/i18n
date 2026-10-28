package i18n_test

import (
	"math"
	"strconv"
	"testing"

	"github.com/go-kanna/i18n"
)

func text(s string) i18n.Segment  { return i18n.Segment{Text: s} }
func param(s string) i18n.Segment { return i18n.Segment{Param: s} }

func arg(name string, value any) i18n.Arg { return i18n.Arg{Name: name, Value: value} }

func TestRender(t *testing.T) {
	t.Parallel()

	hello := i18n.Entry{Single: i18n.Template{text("Hello, "), param("name"), text("!")}}
	total := i18n.Entry{Single: i18n.Template{text("Total: "), param("price")}}
	items := i18n.Entry{Plural: &i18n.Plural{
		One:   i18n.Template{text("You have "), param("count"), text(" item.")},
		Other: i18n.Template{text("You have "), param("count"), text(" items.")},
	}}
	itemsJa := i18n.Entry{Plural: &i18n.Plural{
		Other: i18n.Template{text("アイテムが"), param("count"), text("個あります。")},
	}}
	// Every form, so that the category a count falls in shows.
	forms := i18n.Entry{Plural: &i18n.Plural{
		Zero:  i18n.Template{text("zero:"), param("count")},
		One:   i18n.Template{text("one:"), param("count")},
		Two:   i18n.Template{text("two:"), param("count")},
		Few:   i18n.Template{text("few:"), param("count")},
		Many:  i18n.Template{text("many:"), param("count")},
		Other: i18n.Template{text("other:"), param("count")},
	}}

	tests := []struct {
		name  string
		lang  string
		entry i18n.Entry
		args  []i18n.Arg
		want  string
	}{
		{name: "text only", lang: "en", entry: i18n.Entry{Single: i18n.Template{text("Hello!")}}, want: "Hello!"},
		{name: "empty", lang: "en", entry: i18n.Entry{Single: i18n.Template{}}, want: ""},
		{name: "string", lang: "en", entry: hello, args: []i18n.Arg{arg("name", "World")}, want: "Hello, World!"},
		{name: "missing argument", lang: "en", entry: hello, want: "Hello, {name}!"},
		{
			name:  "arguments found by name, whatever their order",
			lang:  "en",
			entry: i18n.Entry{Single: i18n.Template{param("b"), text(" "), param("a")}},
			args:  []i18n.Arg{arg("a", "1"), arg("b", "2")},
			want:  "2 1",
		},
		// An int is a count or an ID, written without grouping.
		{name: "int", lang: "en", entry: total, args: []i18n.Arg{arg("price", 1234567)}, want: "Total: 1234567"},
		{name: "float64", lang: "en", entry: total, args: []i18n.Arg{arg("price", 1234.56)}, want: "Total: 1,234.56"},
		{name: "float64 in de", lang: "de", entry: total, args: []i18n.Arg{arg("price", 1234.56)}, want: "Total: 1.234,56"},
		{name: "float64 in ja", lang: "ja", entry: total, args: []i18n.Arg{arg("price", 1234.56)}, want: "Total: 1,234.56"},
		// CLDR's default pattern keeps three fraction digits at most.
		{
			name:  "float64 rounded",
			lang:  "en",
			entry: total,
			args:  []i18n.Arg{arg("price", 1234.5678)},
			want:  "Total: 1,234.568",
		},
		{name: "float64 rounded to none", lang: "en", entry: total, args: []i18n.Arg{arg("price", 0.0001)}, want: "Total: 0"},
		// What no generated constructor passes is left to fmt.
		{
			name:  "other types",
			lang:  "en",
			entry: total,
			args:  []i18n.Arg{arg("price", float32(1234.5))},
			want:  "Total: 1234.5",
		},
		{name: "nil", lang: "en", entry: total, args: []i18n.Arg{arg("price", nil)}, want: "Total: <nil>"},

		{name: "plural one", lang: "en", entry: items, args: []i18n.Arg{arg("count", 1)}, want: "You have 1 item."},
		{name: "plural other", lang: "en", entry: items, args: []i18n.Arg{arg("count", 5)}, want: "You have 5 items."},
		{name: "plural zero in en", lang: "en", entry: items, args: []i18n.Arg{arg("count", 0)}, want: "You have 0 items."},
		{name: "plural negative", lang: "en", entry: items, args: []i18n.Arg{arg("count", -1)}, want: "You have -1 item."},
		{name: "plural without a count", lang: "en", entry: items, want: "You have {count} items."},
		{
			name:  "plural with a count that is not an int",
			lang:  "en",
			entry: items,
			args:  []i18n.Arg{arg("count", int64(1))},
			want:  "You have 1 items.",
		},
		// Japanese has no one; 1 takes other, the form the catalog has.
		{name: "plural in ja", lang: "ja", entry: itemsJa, args: []i18n.Arg{arg("count", 1)}, want: "アイテムが1個あります。"},
		{
			name:  "plural form the catalog lacks",
			lang:  "en",
			entry: itemsJa,
			args:  []i18n.Arg{arg("count", 1)},
			want:  "アイテムが1個あります。",
		},
		// The language's rules pick the form, not the number itself, except that
		// a count of 0 takes zero when there is one.
		{name: "plural 0 in en", lang: "en", entry: forms, args: []i18n.Arg{arg("count", 0)}, want: "zero:0"},
		{name: "plural 0 in fr", lang: "fr", entry: forms, args: []i18n.Arg{arg("count", 0)}, want: "zero:0"},
		{name: "plural 1 in fr", lang: "fr", entry: forms, args: []i18n.Arg{arg("count", 1)}, want: "one:1"},
		{name: "plural 0 in ar", lang: "ar", entry: forms, args: []i18n.Arg{arg("count", 0)}, want: "zero:0"},
		{name: "plural 10 in lv", lang: "lv", entry: forms, args: []i18n.Arg{arg("count", 10)}, want: "zero:10"},
		{name: "plural 2 in en", lang: "en", entry: forms, args: []i18n.Arg{arg("count", 2)}, want: "other:2"},
		{name: "plural 2 in ar", lang: "ar", entry: forms, args: []i18n.Arg{arg("count", 2)}, want: "two:2"},
		{name: "plural 100 in ar", lang: "ar", entry: forms, args: []i18n.Arg{arg("count", 100)}, want: "other:100"},
		{name: "plural 2 in ru", lang: "ru", entry: forms, args: []i18n.Arg{arg("count", 2)}, want: "few:2"},
		{name: "plural 5 in ru", lang: "ru", entry: forms, args: []i18n.Arg{arg("count", 5)}, want: "many:5"},
		{name: "plural 21 in ru", lang: "ru", entry: forms, args: []i18n.Arg{arg("count", 21)}, want: "one:21"},
		// Russian takes remainders, which x/text cannot take of a negative count.
		{
			name:  "plural MinInt in ru",
			lang:  "ru",
			entry: forms,
			args:  []i18n.Arg{arg("count", math.MinInt)},
			want:  "many:" + strconv.Itoa(math.MinInt),
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()

			got, err := i18n.Render(tt.lang, tt.entry, tt.args...)
			if err != nil {
				t.Fatal(err)
			}
			if got != tt.want {
				t.Errorf("Render() = %q, want %q", got, tt.want)
			}
		})
	}
}

func TestRender_invalidLanguage(t *testing.T) {
	t.Parallel()

	if _, err := i18n.Render("not a tag", i18n.Entry{}); err == nil {
		t.Error("Render() in an invalid language succeeded")
	}
}
