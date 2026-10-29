package i18n

import (
	"maps"
	"regexp"
	"strings"
	"unicode"
	"unicode/utf8"
)

// Problem is a broken rule as a validation reports it: the path of the field
// it broke on, such as "address.zip" or "tags[1]", or "base" for the value as
// a whole, and the code and params naming the rule. validate.FieldError of
// go-kanna/validate is one, and so is anything with these methods: this
// package imports no validation package.
type Problem interface {
	Path() string
	Code() string
	Params() map[string]any
}

// basePath is where a problem with the value as a whole is reported, as
// kanna-validate and Rails report it.
const basePath = "base"

// ErrorMessages renders problems as Rails renders errors.messages: by path,
// each the message keyed errors.messages.<code>, in the order they come. It
// suits an API response, which shows each message beside its field.
func ErrorMessages[P Problem](l Localizer, problems []P) map[string][]string {
	out := make(map[string][]string)
	for _, p := range problems {
		out[p.Path()] = append(out[p.Path()], l.problemMessage(p))
	}
	return out
}

// FullMessages renders problems as Rails renders errors.full_messages: each a
// sentence naming its field, put together by errors.format, such as "Title
// can't be blank" or "タイトルを入力してください". A problem at base reads as its
// message alone.
//
// The field is named by errors.attributes: the path with any index dropped,
// address.zip or tags, then its last part, zip, and failing both, the last part
// written out, Completed at for completed_at. A confirmation's attribute, the
// other field it names, is named the same way.
func FullMessages[P Problem](l Localizer, problems []P) []string {
	out := make([]string, len(problems))
	for i, p := range problems {
		msg := l.problemMessage(p)
		if p.Path() == basePath {
			out[i] = msg
			continue
		}
		out[i] = l.Localize(Message{Key: "errors.format", Args: []Arg{
			{Name: "attribute", Value: l.attributeName(p.Path())},
			{Name: "message", Value: msg},
		}})
	}
	return out
}

// problemMessage renders the message of p, keyed errors.messages.<code>.
func (l Localizer) problemMessage(p Problem) string {
	params := maps.Clone(p.Params())
	if other, ok := params["attribute"].(string); ok {
		params["attribute"] = l.attributeName(other)
	}
	return l.Localize(Coded("errors.messages", p.Code(), params))
}

// indices matches the indices and map keys a path holds, such as [1].
var indices = regexp.MustCompile(`\[[^\]]*\]`)

// attributeName names the field at path as FullMessages says.
func (l Localizer) attributeName(path string) string {
	path = indices.ReplaceAllString(path, "")
	last := path[strings.LastIndex(path, ".")+1:]
	for _, key := range []string{"errors.attributes." + path, "errors.attributes." + last} {
		if l.has(key) {
			return l.Localize(Message{Key: key})
		}
	}
	if last == "" {
		return path
	}
	r, size := utf8.DecodeRuneInString(last)
	return string(unicode.ToUpper(r)) + strings.ReplaceAll(last[size:], "_", " ")
}

// has reports whether a language of l holds the message key.
func (l Localizer) has(key string) bool {
	for _, loc := range l.chain {
		if _, ok := loc.entries[key]; ok {
			return true
		}
	}
	return false
}
