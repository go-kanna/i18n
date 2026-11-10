package i18n

import (
	"fmt"
	"math"
	"reflect"
	"slices"
	"strings"

	"golang.org/x/text/feature/plural"
	"golang.org/x/text/language"
	"golang.org/x/text/message"
	"golang.org/x/text/number"

	"github.com/go-kanna/i18n/internal/runtime"
)

// locale is a Catalog made ready to render: its language parsed, and a printer
// for writing numbers the way the language does.
type locale struct {
	tag     language.Tag
	entries map[string]Entry
	printer *message.Printer
}

func newLocale(c Catalog) (locale, error) {
	tag, err := language.Parse(c.Lang)
	if err != nil {
		return locale{}, fmt.Errorf("invalid language %q: %w", c.Lang, err)
	}
	return locale{tag: tag, entries: cloneEntries(c.Entries), printer: message.NewPrinter(tag)}, nil
}

// cloneEntries copies entries down to their segments, so that a Bundle built
// from a catalog stays as it was built when the catalog is changed afterwards.
func cloneEntries(entries map[string]Entry) map[string]Entry {
	out := make(map[string]Entry, len(entries))
	for key, e := range entries {
		out[key] = cloneEntry(e)
	}
	return out
}

// cloneEntry copies e down to its segments.
func cloneEntry(e Entry) Entry {
	e.Single = slices.Clone(e.Single)
	if e.Plural != nil {
		p := *e.Plural
		for _, t := range []*Template{&p.Zero, &p.One, &p.Two, &p.Few, &p.Many, &p.Other} {
			*t = slices.Clone(*t)
		}
		e.Plural = &p
	}
	return e
}

// render renders e with args. A placeholder whose argument is missing stays as
// it was written, {name}, so that the gap shows instead of vanishing.
func (l locale) render(e Entry, args []Arg) string {
	t := e.Single
	if e.Plural != nil {
		t = l.pick(*e.Plural, args)
	}

	var b strings.Builder
	for _, s := range t {
		if s.Param == "" {
			b.WriteString(s.Text)
			continue
		}
		v, ok := lookup(args, s.Param)
		if !ok {
			b.WriteString("{" + s.Param + "}")
			continue
		}
		b.WriteString(l.format(v))
	}
	return b.String()
}

// pick returns the form of p for the count in args. A count of 0 takes Zero
// when p has it, whatever the language, as in Rails and i18next; any other
// count takes the form of the category it falls in under the rules of this
// language. Without an int count, it is Other.
func (l locale) pick(p Plural, args []Arg) Template {
	v, _ := lookup(args, runtime.CountParam)
	n, ok := v.(int)
	switch {
	case !ok:
		return p.Other
	case n == 0 && p.Zero != nil:
		return p.Zero
	}
	// The rules are written for the absolute value, and those taking remainders
	// panic on a negative one. MinInt has no absolute value as an int, so it
	// counts as the next one up.
	if n == math.MinInt {
		n++
	}
	if n < 0 {
		n = -n
	}
	return p.form(plural.Cardinal.MatchPlural(l.tag, n, 0, 0, 0, 0))
}

// format writes a number, whatever its type is named, in the number format of
// the language: an integer in its digits but ungrouped, and a float under
// CLDR's default pattern. A value that says how it is written, as a String
// method does, is left to fmt.
func (l locale) format(v any) string {
	switch v := v.(type) {
	case string:
		return v
	case fmt.Stringer, error:
		return fmt.Sprint(v)
	}
	switch r := reflect.ValueOf(v); {
	case r.CanInt():
		return l.printer.Sprint(number.Decimal(r.Int(), number.NoSeparator()))
	case r.CanUint():
		return l.printer.Sprint(number.Decimal(r.Uint(), number.NoSeparator()))
	case r.Kind() == reflect.Float32:
		return l.printer.Sprint(number.Decimal(float32(r.Float())))
	case r.CanFloat():
		return l.printer.Sprint(number.Decimal(r.Float()))
	default:
		return fmt.Sprint(v)
	}
}

// form returns the form for category f, or Other when there is none.
func (p Plural) form(f plural.Form) Template {
	var t Template
	switch f {
	case plural.Zero:
		t = p.Zero
	case plural.One:
		t = p.One
	case plural.Two:
		t = p.Two
	case plural.Few:
		t = p.Few
	case plural.Many:
		t = p.Many
	case plural.Other:
		return p.Other
	}
	if t == nil {
		return p.Other
	}
	return t
}

func lookup(args []Arg, name string) (any, bool) {
	i := slices.IndexFunc(args, func(a Arg) bool { return a.Name == name })
	if i < 0 {
		return nil, false
	}
	return args[i].Value, true
}
