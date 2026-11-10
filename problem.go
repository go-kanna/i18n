package i18n

import (
	"maps"
	"strings"
	"unicode"
	"unicode/utf8"

	"github.com/go-kanna/validator"
)

// Problem is a broken rule as a validation reports it: the path of the field
// it broke on, such as "address.zip" or "tags[1]", or "base" for the value as
// a whole, and the code and params naming the rule. validator.FieldError of
// go-kanna/validator is one, and so is anything with these methods.
type Problem interface {
	Path() string
	Code() string
	Params() map[string]any
}

// basePath is where a problem with the value as a whole is reported, as
// kanna-validator and Rails report it.
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
// The field is named by errors.attributes: the names of the fields on its path,
// address.zip, then the last of them, zip, and failing both, the last written
// out, Completed at for completed_at. A map key or an index on the path is no
// field and is left out: tags[1] is named as tags, and notes.first_note, under
// the key first_note of the map notes, as notes. Only a validator.FieldError
// tells a field from a key: the path of any other problem, or of a FieldError
// without Segments, is named as one field. A confirmation's attribute, the
// other field it names, is named as the field beside it.
func FullMessages[P Problem](l Localizer, problems []P) []string {
	out := make([]string, len(problems))
	for i, p := range problems {
		msg := l.problemMessage(p)
		if p.Path() == basePath {
			out[i] = msg
			continue
		}
		out[i] = l.Localize(Message{Key: "errors.format", Args: []Arg{
			{Name: "attribute", Value: l.attributeName(fieldNames(p))},
			{Name: "message", Value: msg},
		}})
	}
	return out
}

// problemMessage renders the message of p, keyed errors.messages.<code>.
func (l Localizer) problemMessage(p Problem) string {
	params := maps.Clone(p.Params())
	if other, ok := params["attribute"].(string); ok {
		names := fieldNames(p)
		params["attribute"] = l.attributeName(append(names[:len(names)-1:len(names)-1], other))
	}
	return l.Localize(Coded("errors.messages", p.Code(), params))
}

// fieldNames returns the names of the fields on the path of p, as FullMessages
// says.
func fieldNames(p Problem) []string {
	fe, ok := any(p).(validator.FieldError)
	if !ok {
		return []string{p.Path()}
	}
	var names []string
	for _, s := range fe.Segments {
		if s.Kind == validator.FieldSegment {
			names = append(names, s.Name)
		}
	}
	if len(names) == 0 {
		return []string{p.Path()}
	}
	return names
}

// attributeName names the field whose path goes through the fields names.
func (l Localizer) attributeName(names []string) string {
	path, last := strings.Join(names, "."), names[len(names)-1]
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
	_, _, ok := find(l.chain, key)
	return ok
}
