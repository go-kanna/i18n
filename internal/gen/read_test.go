package gen_test

import (
	"go/ast"
	"go/token"
	"go/types"
	"path/filepath"
	"reflect"
	"testing"

	"github.com/go-kanna/internal/packages"

	"github.com/go-kanna/i18n"
	"github.com/go-kanna/i18n/internal/gen"
	"github.com/go-kanna/i18n/internal/runtime"
)

// TestReadBundle reads back the golden files, which TestEmit keeps what emit
// writes.
func TestReadBundle(t *testing.T) {
	t.Parallel()

	for _, dir := range []string{"testdata/basic", "testdata/names"} {
		t.Run(dir, func(t *testing.T) {
			t.Parallel()

			want, err := gen.Catalogs(dir)
			if err != nil {
				t.Fatal(err)
			}
			_, got, ds := gen.ReadBundle(loadGenerated(t, dir, ""))
			if len(ds) > 0 {
				t.Fatalf("ReadBundle() = %v, want no diagnostics", ds)
			}
			if !reflect.DeepEqual(got, want) {
				t.Errorf("ReadBundle() = %+v, want %+v", got, want)
			}
		})
	}
}

func TestReadBundle_layout(t *testing.T) {
	t.Parallel()

	b, got, ds := gen.ReadBundle(loadGenerated(t, "testdata/basic", `const en = "e" + "n"

var bundle = i18n.NewBundle(
	i18n.Catalog{Lang: en, Entries: map[string]i18n.Entry{
		"hello": {Single: i18n.Template{{"Hello, ", ""}, {Param: "name"}}},
		"items": {Plural: &i18n.Plural{Other: i18n.Template{{Param: "count"}}}},
	}},
	i18n.Catalog{Entries: map[string]i18n.Entry{}, Lang: "ja"},
	i18n.Catalog{Lang: "fr"},
)
`))
	if len(ds) > 0 {
		t.Fatalf("ReadBundle() = %v, want no diagnostics", ds)
	}
	want := []i18n.Catalog{
		{Lang: "en", Entries: map[string]i18n.Entry{
			"hello": {Single: i18n.Template{{Text: "Hello, "}, {Param: "name"}}},
			"items": {Plural: &i18n.Plural{Other: i18n.Template{{Param: "count"}}}},
		}},
		{Lang: "ja", Entries: map[string]i18n.Entry{}},
		{Lang: "fr"},
	}
	if !reflect.DeepEqual(got, want) {
		t.Errorf("ReadBundle() = %+v, want %+v", got, want)
	}
	// The Bundle is built of them: fr falls back to en, the first.
	if got := runtime.Lookup[i18n.Entry](b, "hello"); len(got) != 3 {
		t.Errorf("Lookup(hello) = %+v, want en's in each of the 3 languages", got)
	}
}

func TestReadBundle_unreadable(t *testing.T) {
	t.Parallel()

	// The body starts on line 7, after the header, the package clause and the
	// import.
	tests := []struct {
		name string
		src  string
		at   string
		want string
	}{
		{
			name: "no call",
			src:  "var bundle i18n.Bundle\n",
			at:   "3:9",
			want: "the file does not call i18n.NewBundle",
		},
		{
			name: "a function of the same name",
			src: `func NewBundle(c i18n.Catalog) i18n.Bundle { return i18n.Bundle{} }

var bundle = NewBundle(i18n.Catalog{Lang: "en"})
`,
			at:   "3:9",
			want: "the file does not call i18n.NewBundle",
		},
		{
			name: "two calls",
			src: `var bundle = i18n.NewBundle(i18n.Catalog{Lang: "en"})

var other = i18n.NewBundle(i18n.Catalog{Lang: "en"})
`,
			at:   "9:13",
			want: "the file calls i18n.NewBundle a second time",
		},
		{
			name: "a slice",
			src: `var catalogs = []i18n.Catalog{{Lang: "en"}}

var bundle = i18n.NewBundle(catalogs[0], catalogs[1:]...)
`,
			at:   "9:14",
			want: "the catalogs are passed as a slice",
		},
		{
			name: "a catalog",
			src:  "var en = i18n.Catalog{Lang: \"en\"}\n\nvar bundle = i18n.NewBundle(en)\n",
			at:   "9:29",
			want: "en is not a literal",
		},
		{
			name: "a language",
			src:  "var en = \"en\"\n\nvar bundle = i18n.NewBundle(i18n.Catalog{Lang: en})\n",
			at:   "9:48",
			want: "en is not a constant string",
		},
		{
			name: "entries",
			src: `var bundle = i18n.NewBundle(i18n.Catalog{Lang: "en", Entries: entries()})

func entries() map[string]i18n.Entry { return nil }
`,
			at:   "7:63",
			want: "entries() is not a literal",
		},
		{
			name: "a template",
			src: `var hello = i18n.Template{{Text: "Hello"}}

var bundle = i18n.NewBundle(i18n.Catalog{Lang: "en", Entries: map[string]i18n.Entry{
	"hello": {Single: hello},
}})
`,
			at:   "10:20",
			want: "hello is not a literal",
		},
		{
			name: "a plural",
			src: `var items = &i18n.Plural{Other: i18n.Template{}}

var bundle = i18n.NewBundle(i18n.Catalog{Lang: "en", Entries: map[string]i18n.Entry{
	"items": {Plural: items},
}})
`,
			at:   "10:20",
			want: "items is not a literal",
		},
		{
			name: "two catalogs of a language",
			src:  "var bundle = i18n.NewBundle(i18n.Catalog{Lang: \"en\"}, i18n.Catalog{Lang: \"en\"})\n",
			at:   "7:14",
			want: "i18n.NewBundle panics with i18n: two catalogs of en",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()

			_, _, ds := gen.ReadBundle(loadGenerated(t, "testdata/basic", tt.src))
			if len(ds) != 1 {
				t.Fatalf("ReadBundle() = %v, want one diagnostic", ds)
			}
			ds[0].Pos.Filename = filepath.Base(ds[0].Pos.Filename)
			want := "i18n_gen.go:" + tt.at + ": error: cannot read the catalogs kanna-i18n generated: " + tt.want +
				"\n  regenerate the file with kanna-i18n"
			if got := ds[0].String(); got != want {
				t.Errorf("ReadBundle():\n%s\nwant:\n%s", got, want)
			}
		})
	}
}

// loadGenerated loads the package dir/messages with its types, its i18n_gen.go
// holding body after the header and import emit writes, unless body is empty,
// and returns that file.
func loadGenerated(t *testing.T, dir, body string) (*token.FileSet, *ast.File, *types.Info) {
	t.Helper()

	path, err := filepath.Abs(filepath.Join(dir, "messages", "i18n_gen.go"))
	if err != nil {
		t.Fatal(err)
	}
	cfg := packages.Config{Dir: ".", TypesInfo: true}
	if body != "" {
		const head = "// Code generated by kanna-i18n. DO NOT EDIT.\n\npackage messages\n\n"
		cfg.Overlay = map[string][]byte{path: []byte(head + "import \"github.com/go-kanna/i18n\"\n\n" + body)}
	}
	set, err := packages.Load([]string{"./" + dir + "/messages"}, cfg)
	if err != nil {
		t.Fatal(err)
	}
	if len(set.Diags) > 0 {
		t.Fatalf("loading %s: %v", dir, set.Diags)
	}
	p := set.Packages[0]
	for i, name := range p.CompiledGoFiles {
		if name == path {
			return set.Fset, p.Syntax[i], p.TypesInfo
		}
	}
	t.Fatalf("%s has no %s", p.PkgPath, path)
	return nil, nil, nil
}
