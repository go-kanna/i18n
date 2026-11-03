package i18n

import (
	"slices"
	"strings"
)

// builtin holds the texts every Bundle knows without a locale file: how a
// validation error reads, errors.format and errors.messages.<code>, for the
// codes kanna-validate and kanna-value report. English follows Rails, Japanese
// rails-i18n. A catalog of the same language overrides any of them.
var builtin = []Catalog{
	{Lang: "en", Entries: map[string]Entry{
		"errors.format":         single("{attribute} {message}"),
		"errors.messages.blank": single("can't be blank"),
		"errors.messages.too_short": counted(
			"is too short (minimum is 1 character)",
			"is too short (minimum is {count} characters)",
		),
		"errors.messages.too_long": counted(
			"is too long (maximum is 1 character)",
			"is too long (maximum is {count} characters)",
		),
		"errors.messages.wrong_length": counted(
			"is the wrong length (should be 1 character)",
			"is the wrong length (should be {count} characters)",
		),
		"errors.messages.greater_than":             single("must be greater than {count}"),
		"errors.messages.greater_than_or_equal_to": single("must be greater than or equal to {count}"),
		"errors.messages.less_than":                single("must be less than {count}"),
		"errors.messages.less_than_or_equal_to":    single("must be less than or equal to {count}"),
		"errors.messages.equal_to":                 single("must be equal to {count}"),
		"errors.messages.other_than":               single("must be other than {count}"),
		"errors.messages.inclusion":                single("is not included in the list"),
		"errors.messages.confirmation":             single("doesn't match {attribute}"),
		"errors.messages.invalid":                  single("is invalid"),
		"errors.messages.email":                    single("is not an email address"),
		"errors.messages.url":                      single("is not a URL"),
		"errors.messages.uuid":                     single("is not a UUID"),
	}},
	{Lang: "ja", Entries: map[string]Entry{
		"errors.format":                            single("{attribute}{message}"),
		"errors.messages.blank":                    single("を入力してください"),
		"errors.messages.too_short":                single("は{count}文字以上で入力してください"),
		"errors.messages.too_long":                 single("は{count}文字以内で入力してください"),
		"errors.messages.wrong_length":             single("は{count}文字で入力してください"),
		"errors.messages.greater_than":             single("は{count}より大きい値にしてください"),
		"errors.messages.greater_than_or_equal_to": single("は{count}以上の値にしてください"),
		"errors.messages.less_than":                single("は{count}より小さい値にしてください"),
		"errors.messages.less_than_or_equal_to":    single("は{count}以下の値にしてください"),
		"errors.messages.equal_to":                 single("は{count}にしてください"),
		"errors.messages.other_than":               single("は{count}以外の値にしてください"),
		"errors.messages.inclusion":                single("は一覧にありません"),
		"errors.messages.confirmation":             single("と{attribute}の入力が一致しません"),
		"errors.messages.invalid":                  single("は不正な値です"),
		"errors.messages.email":                    single("はメールアドレスの形式で入力してください"),
		"errors.messages.url":                      single("は URL の形式で入力してください"),
		"errors.messages.uuid":                     single("は UUID の形式で入力してください"),
	}},
}

// IsBuiltin reports whether key is a built-in text, which the catalog of any
// language may override, the default language's included or not. It is
// exported for kanna-i18n, which lets a translation hold such a key alone.
func IsBuiltin(key string) bool {
	for _, c := range builtin {
		if _, ok := c.Entries[key]; ok {
			return true
		}
	}
	return false
}

// BuiltinParams returns, sorted, the names of the placeholders the built-in text
// key fills in any language, and false when key is no built-in text. It is
// exported for kanna-i18n, which holds a text overriding it to them.
func BuiltinParams(key string) ([]string, bool) {
	var (
		names []string
		found bool
	)
	for _, c := range builtin {
		e, ok := c.Entries[key]
		if !ok {
			continue
		}
		found = true
		forms := []Template{e.Single}
		if p := e.Plural; p != nil {
			forms = append(forms, p.Zero, p.One, p.Two, p.Few, p.Many, p.Other)
		}
		for _, t := range forms {
			for _, seg := range t {
				if seg.Param != "" {
					names = append(names, seg.Param)
				}
			}
		}
	}
	slices.Sort(names)
	return slices.Compact(names), found
}

// single is an Entry of one form, written as a locale file writes it.
func single(text string) Entry {
	return Entry{Single: parseTemplate(text)}
}

// counted is an Entry whose form depends on its count: one for 1, other for
// any other.
func counted(one, other string) Entry {
	return Entry{Plural: &Plural{One: parseTemplate(one), Other: parseTemplate(other)}}
}

// parseTemplate splits text at its placeholders, {name}, which the texts above
// write without a kind.
func parseTemplate(text string) Template {
	var t Template
	for text != "" {
		before, rest, found := strings.Cut(text, "{")
		if before != "" {
			t = append(t, Segment{Text: before})
		}
		if !found {
			break
		}
		param, after, _ := strings.Cut(rest, "}")
		t = append(t, Segment{Param: param})
		text = after
	}
	return t
}
