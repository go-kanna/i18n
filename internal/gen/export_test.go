package gen

import (
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
