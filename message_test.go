package i18n_test

import (
	"reflect"
	"testing"

	"github.com/go-kanna/i18n"
)

func TestCoded(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name   string
		prefix string
		code   string
		params map[string]any
		want   i18n.Message
	}{
		{
			name:   "key and args",
			prefix: "errors",
			code:   "too_long",
			params: map[string]any{"count": 20},
			want:   msg("errors.too_long", arg("count", 20)),
		},
		// A map has no order, so it is the names that give the arguments one.
		{
			name:   "args sorted by name",
			prefix: "errors",
			code:   "range",
			params: map[string]any{"min": 18, "max": 130, "unit": "years"},
			want:   msg("errors.range", arg("max", 130), arg("min", 18), arg("unit", "years")),
		},
		{name: "no params", prefix: "errors", code: "blank", want: msg("errors.blank")},
		{name: "empty params", prefix: "errors", code: "blank", params: map[string]any{}, want: msg("errors.blank")},
		{name: "no prefix", code: "blank", want: msg("blank")},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()

			if got := i18n.Coded(tt.prefix, tt.code, tt.params); !reflect.DeepEqual(got, tt.want) {
				t.Errorf("Coded(%q, %q, %v) = %v, want %v", tt.prefix, tt.code, tt.params, got, tt.want)
			}
		})
	}
}

// The texts of the codes are messages like any other:
//
//	# en.yaml
//	errors:
//	  blank: "can't be blank"
//	  too_long: "is too long (maximum is {count:int} characters)"
//	  greater_than_or_equal_to: "must be greater than or equal to {count}"
//
//	# ja.yaml, lacking greater_than_or_equal_to
//	errors:
//	  blank: "を入力してください"
//	  too_long: "は{count}文字以内で入力してください"
func TestCoded_localize(t *testing.T) {
	t.Parallel()

	b := i18n.NewBundle(
		i18n.Catalog{Lang: "en", Entries: map[string]i18n.Entry{
			"errors.blank":    single(text("can't be blank")),
			"errors.too_long": single(text("is too long (maximum is "), param("count"), text(" characters)")),
			"errors.greater_than_or_equal_to": single(
				text("must be greater than or equal to "), param("count"),
			),
		}},
		i18n.Catalog{Lang: "ja", Entries: map[string]i18n.Entry{
			"errors.blank":    single(text("を入力してください")),
			"errors.too_long": single(text("は"), param("count"), text("文字以内で入力してください")),
		}},
	)

	tests := []struct {
		name   string
		lang   string
		code   string
		params map[string]any
		want   string
	}{
		{
			name:   "en",
			lang:   "en",
			code:   "too_long",
			params: map[string]any{"count": 20},
			want:   "is too long (maximum is 20 characters)",
		},
		{name: "ja", lang: "ja", code: "too_long", params: map[string]any{"count": 20}, want: "は20文字以内で入力してください"},
		{name: "no params", lang: "ja", code: "blank", want: "を入力してください"},
		{
			name:   "missing from ja, found in the default",
			lang:   "ja",
			code:   "greater_than_or_equal_to",
			params: map[string]any{"count": 18},
			want:   "must be greater than or equal to 18",
		},
		// A code no language has a text for shows as its key, not as nothing.
		{name: "in no language", lang: "en", code: "taken", want: "errors.taken"},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()

			got := b.Localizer(tt.lang).Localize(i18n.Coded("errors", tt.code, tt.params))
			if got != tt.want {
				t.Errorf("Localizer(%q).Localize(Coded(errors, %q, %v)) = %q, want %q", tt.lang, tt.code, tt.params, got, tt.want)
			}
		})
	}
}
