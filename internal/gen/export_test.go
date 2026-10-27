package gen

import (
	"fmt"
	"go/token"
	"strconv"
)

// ParseTemplate parses src, spelling each segment as its text quoted or its
// placeholder in braces, each parameter as name:kind with a ! after a kind
// written out, and each diagnostic as diag.Diag.String does.
func ParseTemplate(src string) (segments, params, diags []string) {
	t, ds := parseTemplate(src, token.Position{})
	for _, s := range t.segments {
		if s.param != "" {
			segments = append(segments, "{"+s.param+"}")
			continue
		}
		segments = append(segments, strconv.Quote(s.text))
	}
	for _, p := range t.params {
		spelled := p.name + ":" + p.kind.String()
		if p.explicit {
			spelled += "!"
		}
		params = append(params, spelled)
	}
	for _, d := range ds {
		diags = append(diags, d.String())
	}
	return segments, params, diags
}

// ParseLocale reads src as the locale file en.yaml, spelling each message as
// its key and where it is written, then each form: its category, if any, its
// text quoted, and where it is written, such as
// `items@1:1 one "One"@3:10 other "Many"@4:12`; and each diagnostic as
// diag.Diag.String does.
func ParseLocale(src string) (entries, diags []string) {
	es, ds := parseLocale("en.yaml", []byte(src))
	for _, e := range es {
		spelled := fmt.Appendf(nil, "%s@%d:%d", e.key, e.pos.Line, e.pos.Column)
		for _, f := range e.forms {
			if f.category != "" {
				spelled = fmt.Appendf(spelled, " %s", f.category)
			}
			spelled = fmt.Appendf(spelled, " %q@%d:%d", f.src, f.pos.Line, f.pos.Column)
		}
		entries = append(entries, string(spelled))
	}
	for _, d := range ds {
		diags = append(diags, d.String())
	}
	return entries, diags
}
