package gen

import (
	"cmp"
	"fmt"
	"go/token"
	"go/types"
	"maps"
	"path/filepath"
	"slices"
	"strings"

	"github.com/go-kanna/internal/diag"
	"github.com/go-kanna/internal/suggest"
	"golang.org/x/text/feature/plural"
	"golang.org/x/text/language"

	"github.com/go-kanna/i18n"
)

// localizerFunc is the function the generated file declares besides the
// constructors.
const localizerFunc = "Localizer"

// model is what the generated file is made of.
type model struct {
	// messages are the messages of the default language, one constructor each,
	// in the order of their keys.
	messages []message
	// catalogs are every language, the default first and the rest in the order
	// of their tags.
	catalogs []catalog
}

// message is a message of the default language, and so a constructor.
type message struct {
	entry

	// fn is the name of the constructor.
	fn string
	// params are the parameters of the constructor, the count first in a
	// plural message.
	params []param
}

// analyze builds the model from def, the catalog of the default language, and
// checks every other of catalogs against it. A constructor may not take a name
// in declared, the top-level names the destination package declares already,
// each with where.
//
// The model holds whatever passed. It is only good to generate from when no
// error is reported: a translation that does not fit the default language
// would otherwise wait for the first request to meet it.
func analyze(def catalog, catalogs []catalog, declared map[string]token.Position) (model, []diag.Diag) {
	messages, diags := constructors(def, declared)
	if diag.HasErrors(diags) {
		// The rest would be checked against messages that are not all there.
		return model{}, diags
	}

	m := model{messages: messages, catalogs: []catalog{def}}
	byKey := make(map[string]message, len(messages))
	for _, msg := range messages {
		byKey[msg.key] = msg
	}
	diags = append(diags, missingForms(def)...)
	for _, c := range catalogs {
		if c.tag == def.tag {
			continue
		}
		diags = append(diags, check(c, def.tag, byKey)...)
		m.catalogs = append(m.catalogs, c)
	}
	slices.SortFunc(m.catalogs[1:], func(a, b catalog) int { return cmp.Compare(a.tag.String(), b.tag.String()) })
	return m, diags
}

// constructors makes a constructor of each message of def.
func constructors(def catalog, declared map[string]token.Position) ([]message, []diag.Diag) {
	var (
		messages []message
		diags    []diag.Diag
		byFn     = make(map[string]string, len(def.entries))
	)
	for _, e := range def.entries {
		fn := funcName(e.key)
		switch prev, ok := byFn[fn]; {
		case ok:
			diags = append(diags, diag.Errorf(e.pos, "keys %q and %q both generate %s", prev, e.key, fn))
			continue
		case fn == localizerFunc:
			diags = append(diags, diag.Errorf(e.pos, "key %q generates %s, which the generated file declares itself", e.key, fn).
				WithHints("rename the key"))
			continue
		}
		if at, ok := declared[fn]; ok {
			d := diag.Errorf(e.pos, "key %q generates %s, which %s:%d declares already",
				e.key, fn, filepath.Base(at.Filename), at.Line)
			diags = append(diags, d.WithHints("rename the key, or what declares "+fn))
			continue
		}
		byFn[fn] = e.key

		params, ds := entryParams(e)
		if len(ds) > 0 {
			diags = append(diags, ds...)
			continue
		}
		messages = append(messages, message{entry: e, fn: fn, params: params})
	}
	slices.SortFunc(messages, func(a, b message) int { return cmp.Compare(a.key, b.key) })
	return messages, diags
}

// entryParams returns the parameters of the constructor of e: the
// placeholders of its forms, each once, in the order they first appear. A
// plural message takes its count first, as an int.
func entryParams(e entry) ([]param, []diag.Diag) {
	var params []param
	if e.plural() {
		params = []param{{name: i18n.CountParam, kind: kindInt, explicit: true}}
	}
	var diags []diag.Diag
	for _, f := range e.forms {
		for _, p := range f.params {
			var (
				prev kind
				ok   bool
			)
			if params, prev, ok = addParam(params, p); !ok {
				diags = append(diags, diag.Errorf(f.pos, "placeholder %s of key %q is written as both %s and %s",
					p.name, e.key, prev, p.kind))
			}
		}
	}

	byGo := make(map[string]string, len(params))
	for _, p := range params {
		g := goName(p.name)
		if prev, ok := byGo[g]; ok {
			diags = append(diags, diag.Errorf(e.pos, "placeholders %s and %s of key %q both become the parameter %s",
				prev, p.name, e.key, g))
		}
		byGo[g] = p.name
	}
	return params, diags
}

// check checks c, the catalog of a translation, against the messages of the
// default language def, by key.
func check(c catalog, def language.Tag, byKey map[string]message) []diag.Diag {
	var diags []diag.Diag
	seen := make(map[string]bool, len(c.entries))
	for _, e := range c.entries {
		seen[e.key] = true
		m, ok := byKey[e.key]
		if !ok {
			d := diag.Errorf(e.pos, "key %q is not in %s", e.key, def)
			if s, ok := suggest.Closest(e.key, slices.Sorted(maps.Keys(byKey))); ok {
				d = d.WithHints(fmt.Sprintf("did you mean %q?", s))
			}
			diags = append(diags, d)
			continue
		}
		switch {
		case e.plural() && !m.plural():
			diags = append(diags, diag.Errorf(e.pos, "key %q is plural here, but not in %s", e.key, def))
			continue
		case !e.plural() && m.plural():
			diags = append(diags, diag.Errorf(e.pos, "key %q is not plural here, but plural in %s", e.key, def))
			continue
		}
		for _, f := range e.forms {
			for _, p := range f.params {
				diags = append(diags, checkParam(p, f.pos, e.key, def, m.params)...)
			}
		}
	}
	diags = append(diags, missingForms(c)...)

	var missing []string
	for _, key := range slices.Sorted(maps.Keys(byKey)) {
		if !seen[key] {
			missing = append(missing, key)
		}
	}
	if len(missing) > 0 {
		at := token.Position{Filename: c.path}
		diags = append(diags, diag.Warningf(at, "missing keys: %s", strings.Join(missing, ", ")))
	}
	return diags
}

// checkParam checks p, a placeholder written at pos in a translation of key,
// against params, those of the constructor.
func checkParam(p param, pos token.Position, key string, def language.Tag, params []param) []diag.Diag {
	i := slices.IndexFunc(params, func(q param) bool { return q.name == p.name })
	if i < 0 {
		d := diag.Errorf(pos, "parameter %q of key %q is not in %s", p.name, key, def)
		names := make([]string, len(params))
		for j, q := range params {
			names[j] = q.name
		}
		if s, ok := suggest.Closest(p.name, names); ok {
			d = d.WithHints(fmt.Sprintf("did you mean %q?", s))
		}
		return []diag.Diag{d}
	}
	if p.explicit && p.kind != params[i].kind {
		return []diag.Diag{diag.Errorf(pos, "parameter %q of key %q is %s, but %s in %s",
			p.name, key, p.kind, params[i].kind, def)}
	}
	return nil
}

// missingForms warns of each plural message of c that lacks a form its
// language tells counts apart by: the counts needing it render with other,
// which is grammatical in some languages and wrong in the rest.
func missingForms(c catalog) []diag.Diag {
	used := usedCategories(c.tag)
	var diags []diag.Diag
	for _, e := range c.entries {
		if !e.plural() {
			continue
		}
		var missing []string
		for _, cat := range used {
			if !slices.ContainsFunc(e.forms, func(f form) bool { return f.category == cat }) {
				missing = append(missing, cat)
			}
		}
		if len(missing) > 0 {
			noun := "forms"
			if len(missing) == 1 {
				noun = "form"
			}
			diags = append(diags, diag.Warningf(e.pos, "key %q lacks the plural %s %s, so those counts render with %q",
				e.key, noun, andList(missing), "other"))
		}
	}
	return diags
}

// formNames names each plural.Form as categories does.
var formNames = map[plural.Form]string{
	plural.Zero:  "zero",
	plural.One:   "one",
	plural.Two:   "two",
	plural.Few:   "few",
	plural.Many:  "many",
	plural.Other: "other",
}

// usedCategories returns the categories that the plural rules of tag tell
// integers apart by, in the order of categories.
//
// x/text does not list them, so they are found by asking about enough integers
// to meet every rule: the rules look at small numbers, and at the remainders by
// 10, 100, and, in a few languages, 1,000,000.
func usedCategories(tag language.Tag) []string {
	found := make(map[string]bool, len(categories))
	sample := func(n int) {
		found[formNames[plural.Cardinal.MatchPlural(tag, n, 0, 0, 0, 0)]] = true
	}
	for n := range 1001 {
		sample(n)
	}
	for _, n := range []int{10_000, 100_000, 1_000_000, 2_000_000} {
		sample(n)
	}
	return slices.DeleteFunc(slices.Clone(categories), func(c string) bool { return !found[c] })
}

// funcName returns the name of the constructor of key: user.not_found becomes
// UserNotFound.
func funcName(key string) string {
	var b strings.Builder
	for part := range strings.FieldsFuncSeq(key, func(r rune) bool { return r == '.' || r == '_' }) {
		b.WriteString(strings.ToUpper(part[:1]) + part[1:])
	}
	return b.String()
}

// goName returns the name of the parameter of placeholder p: user_name becomes
// userName. A name Go reserves or predeclares takes Arg after it, as type
// becomes typeArg and error errorArg: the one would not compile, and the other
// would hide what it names.
func goName(p string) string {
	name := funcName(p)
	name = strings.ToLower(name[:1]) + name[1:]
	if token.IsKeyword(name) || types.Universe.Lookup(name) != nil {
		name += "Arg"
	}
	return name
}

// andList spells s as "a and b", or "a, b, and c".
func andList(s []string) string {
	switch len(s) {
	case 1:
		return s[0]
	case 2:
		return s[0] + " and " + s[1]
	default:
		return strings.Join(s[:len(s)-1], ", ") + ", and " + s[len(s)-1]
	}
}
