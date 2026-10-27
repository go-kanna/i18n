package gen

import (
	"cmp"
	"fmt"
	"slices"
	"strconv"
	"strings"

	"github.com/go-kanna/internal/gofile"
)

const (
	// generator names the command in the header of the generated file.
	generator = "kanna-i18n"

	// runtimePath is the import path of the runtime the generated code calls.
	runtimePath = "github.com/go-kanna/i18n"
)

// goTypes are the types a constructor takes the argument of each kind as.
var goTypes = []string{kindString: "string", kindInt: "int", kindNumber: "float64"}

// emit renders m as a file of the package pkgName, which declares declared
// besides: a constructor of each message, the Localizer, and the catalogs of
// every language, from which nothing is read at run time.
func emit(m model, pkgName string, declared []string) *gofile.File {
	f := gofile.New(generator, "", pkgName)
	f.Reserve(declared...)
	bundle := unique("bundle", declared)
	f.Reserve(bundle, localizerFunc)
	for _, msg := range m.messages {
		f.Reserve(msg.fn)
	}

	e := emitter{f: f, rt: f.Import(runtimePath, "i18n"), bundle: bundle}
	for _, msg := range m.messages {
		e.constructor(msg)
	}
	e.localizer(m.catalogs[0])
	e.catalogs(m.catalogs)
	return f
}

type emitter struct {
	f *gofile.File
	// rt is the name the file imports the runtime by.
	rt string
	// bundle is the name of the variable holding the catalogs.
	bundle string
}

func (e emitter) constructor(msg message) {
	s := e.f.Scope()
	names := make([]string, len(msg.params))
	params := make([]string, len(msg.params))
	for i, p := range msg.params {
		names[i] = s.Name(goName(p.name))
		params[i] = names[i] + " " + goTypes[p.kind]
	}

	e.doc(msg, names)
	fmt.Fprintf(e.f, "func %s(%s) %s.Message {\n", msg.fn, strings.Join(params, ", "), e.rt)
	if len(msg.params) == 0 {
		fmt.Fprintf(e.f, "\treturn %s.Message{Key: %q}\n}\n\n", e.rt, msg.key)
		return
	}
	fmt.Fprintf(e.f, "\treturn %s.Message{Key: %q, Args: []%s.Arg{\n", e.rt, msg.key, e.rt)
	for i, p := range msg.params {
		fmt.Fprintf(e.f, "\t\t{Name: %q, Value: %s},\n", p.name, names[i])
	}
	fmt.Fprint(e.f, "\t}}\n}\n\n")
}

// doc writes the doc comment of the constructor of msg, whose parameters are
// named names. It quotes the default language, which is what an editor shows
// where the constructor is called.
func (e emitter) doc(msg message, names []string) {
	if !msg.plural() {
		src := msg.forms[0].src
		if src == "" {
			fmt.Fprintf(e.f, "// %s returns the message %q, which is empty.\n", msg.fn, msg.key)
			return
		}
		fmt.Fprintf(e.f, "// %s returns the message %q:\n//\n", msg.fn, msg.key)
		e.quote("", "", src)
		return
	}

	fmt.Fprintf(e.f, "// %s returns the message %q, in the form %s selects:\n//\n", msg.fn, msg.key, names[0])
	width := 0
	for _, f := range msg.forms {
		width = max(width, len(f.category)+len(": "))
	}
	for _, f := range msg.forms {
		label := f.category + ":"
		e.quote(label+strings.Repeat(" ", width-len(label)), strings.Repeat(" ", width), f.src)
	}
}

// quote writes src as lines of a code block in a doc comment, the first after
// first and the others after indent.
func (e emitter) quote(first, indent, src string) {
	for i, line := range strings.Split(strings.TrimSuffix(src, "\n"), "\n") {
		if i == 0 {
			fmt.Fprintf(e.f, "//\t%s%s\n", first, line)
			continue
		}
		fmt.Fprintf(e.f, "//\t%s%s\n", indent, line)
	}
}

func (e emitter) localizer(def catalog) {
	langs := e.f.Scope().Name("langs")
	fmt.Fprintf(e.f, `// %[1]s renders this package's messages in the language that best matches
// %[2]s, each a language tag or an Accept-Language value, tried in order.
// A message missing from that language falls back to its parent languages and
// then to %[3]s.
func %[1]s(%[2]s ...string) %[4]s.Localizer {
	return %[5]s.Localizer(%[2]s...)
}

`, localizerFunc, langs, def.tag, e.rt, e.bundle)
}

func (e emitter) catalogs(catalogs []catalog) {
	fmt.Fprintf(e.f, "// %s holds every language this package was generated from, %s, the\n// default, first.\n",
		e.bundle, catalogs[0].tag)
	fmt.Fprintf(e.f, "var %s = %s.NewBundle(\n", e.bundle, e.rt)
	for _, c := range catalogs {
		fmt.Fprintf(e.f, "\t%s.Catalog{Lang: %q, Entries: map[string]%s.Entry{\n", e.rt, c.tag, e.rt)
		byKey := slices.SortedFunc(slices.Values(c.entries), func(a, b entry) int { return cmp.Compare(a.key, b.key) })
		for _, en := range byKey {
			if !en.plural() {
				fmt.Fprintf(e.f, "\t\t%q: {Single: %s},\n", en.key, e.template(en.forms[0].template))
				continue
			}
			fmt.Fprintf(e.f, "\t\t%q: {Plural: &%s.Plural{\n", en.key, e.rt)
			for _, f := range en.forms {
				fmt.Fprintf(e.f, "\t\t\t%s: %s,\n", strings.ToUpper(f.category[:1])+f.category[1:], e.template(f.template))
			}
			fmt.Fprint(e.f, "\t\t}},\n")
		}
		fmt.Fprint(e.f, "\t}},\n")
	}
	fmt.Fprint(e.f, ")\n")
}

// template spells t as an i18n.Template literal.
func (e emitter) template(t template) string {
	segs := make([]string, len(t.segments))
	for i, s := range t.segments {
		if s.param != "" {
			segs[i] = fmt.Sprintf("{Param: %q}", s.param)
		} else {
			segs[i] = fmt.Sprintf("{Text: %q}", s.text)
		}
	}
	return e.rt + ".Template{" + strings.Join(segs, ", ") + "}"
}

// unique returns base, or base2, base3, and so on, whichever taken lacks.
func unique(base string, taken []string) string {
	name := base
	for i := 2; slices.Contains(taken, name); i++ {
		name = base + strconv.Itoa(i)
	}
	return name
}
