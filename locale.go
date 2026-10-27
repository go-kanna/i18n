package i18n

import (
	"fmt"
	"math"
	"slices"
	"strconv"
	"strings"

	"golang.org/x/text/feature/plural"
	"golang.org/x/text/language"
	"golang.org/x/text/message"
	"golang.org/x/text/number"
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
	return locale{tag: tag, entries: c.Entries, printer: message.NewPrinter(tag)}, nil
}

// render renders e with args. A placeholder whose argument is missing stays as
// it was written, {name}, so that the gap shows instead of vanishing.
func (l locale) render(e Entry, args []Arg) string {
	t := e.Single
	if e.Plural != nil {
		t = e.Plural.form(l.category(args))
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

// category returns the plural category the count in args falls in under the
// rules of this language, or Other when there is no int count.
func (l locale) category(args []Arg) plural.Form {
	v, _ := lookup(args, CountParam)
	n, ok := v.(int)
	if !ok {
		return plural.Other
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
	return plural.Cardinal.MatchPlural(l.tag, n, 0, 0, 0, 0)
}

func (l locale) format(v any) string {
	switch v := v.(type) {
	case string:
		return v
	case int:
		return strconv.Itoa(v)
	case float64:
		return l.printer.Sprint(number.Decimal(v))
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
