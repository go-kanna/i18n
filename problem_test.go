package i18n_test

import (
	"maps"
	"slices"
	"testing"

	"github.com/go-kanna/validator"
	"golang.org/x/text/language"

	"github.com/go-kanna/i18n"
	"github.com/go-kanna/i18n/internal/runtime"
)

// problem is a broken rule with a path alone, as a Problem of a package other
// than go-kanna/validator reports one.
type problem struct {
	path, code string
	params     map[string]any
}

func (p problem) Path() string           { return p.path }
func (p problem) Code() string           { return p.code }
func (p problem) Params() map[string]any { return maps.Clone(p.params) }

// problemBundle is what kanna-i18n compiles these files into:
//
//	# en.yaml
//	errors:
//	  messages:
//	    corp_domain: "must be a company address"
//	    too_young_for_admin: "An admin must be 21 or older"
//
//	# ja.yaml
//	errors:
//	  attributes:
//	    title: タイトル
//	    password: パスワード
//	    confirm: 確認用パスワード
//	    address: 住所
//	    address.zip: 郵便番号
//	  messages:
//	    blank: は必須です
//	    too_young_for_admin: 管理者は21歳以上にしてください
func problemBundle() i18n.Bundle {
	return i18n.NewBundle(
		i18n.Catalog{Lang: "en", Entries: map[string]i18n.Entry{
			"errors.messages.corp_domain":         single(text("must be a company address")),
			"errors.messages.too_young_for_admin": single(text("An admin must be 21 or older")),
		}},
		i18n.Catalog{Lang: "ja", Entries: map[string]i18n.Entry{
			"errors.attributes.title":             single(text("タイトル")),
			"errors.attributes.password":          single(text("パスワード")),
			"errors.attributes.confirm":           single(text("確認用パスワード")),
			"errors.attributes.address":           single(text("住所")),
			"errors.attributes.address.zip":       single(text("郵便番号")),
			"errors.messages.blank":               single(text("は必須です")),
			"errors.messages.too_young_for_admin": single(text("管理者は21歳以上にしてください")),
		}},
	)
}

func problems() validator.Errors {
	var errs validator.Errors
	errs.Add(validator.Field("title"), validator.Blank())
	errs.Add(validator.Index(validator.Field("tags"), 1), validator.TooLong(20))
	errs.Add(validator.Index(validator.Field("tags"), 2), validator.TooLong(1))
	errs.Add(validator.Field("tags"), validator.TooMany(3))
	errs.Add(validator.Field("tags"), validator.TooFew(1))
	errs.Add(validator.Field("tags"), validator.WrongCount(2))
	errs.Add(validator.Field("address", "zip"), validator.Invalid())
	errs.Add(validator.Field("completed_at"), validator.GreaterThanOrEqualTo(2000))
	errs.Add(validator.Field("confirm"), validator.Confirmation("password"))
	errs.Add(validator.Field("email"), validator.Fail("corp_domain"))
	errs.Add(validator.Base, validator.Fail("too_young_for_admin"))
	errs.Add(validator.Field("name"), validator.Fail("taken"))
	return errs
}

func TestFullMessages(t *testing.T) {
	t.Parallel()

	tests := []struct {
		lang string
		want []string
	}{
		{
			// Every text but the application's own is built in, in Rails' words.
			lang: "en",
			want: []string{
				"Title can't be blank",
				"Tags is too long (maximum is 20 characters)",
				"Tags is too long (maximum is 1 character)",
				"Tags is too long (maximum is 3 items)",
				"Tags is too short (minimum is 1 item)",
				"Tags is the wrong length (should be 2 items)",
				"Zip is invalid",
				"Completed at must be greater than or equal to 2000",
				"Confirm doesn't match Password",
				"Email must be a company address",
				"An admin must be 21 or older",
				"Name errors.messages.taken",
			},
		},
		{
			// The application's Japanese overrides the built-in, and the
			// built-in Japanese comes before the application's English.
			lang: "ja",
			want: []string{
				"タイトルは必須です",
				"Tagsは20文字以内で入力してください",
				"Tagsは1文字以内で入力してください",
				"Tagsは3件以内で入力してください",
				"Tagsは1件以上で入力してください",
				"Tagsは2件で入力してください",
				"郵便番号は不正な値です",
				"Completed atは2000以上の値にしてください",
				"確認用パスワードとパスワードの入力が一致しません",
				"Emailmust be a company address",
				"管理者は21歳以上にしてください",
				"Nameerrors.messages.taken",
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.lang, func(t *testing.T) {
			t.Parallel()

			got := i18n.FullMessages(problemBundle().Localizer(tt.lang), problems())
			if !slices.Equal(got, tt.want) {
				t.Errorf("FullMessages =\n%q\nwant\n%q", got, tt.want)
			}
		})
	}
}

func TestErrorMessages(t *testing.T) {
	t.Parallel()

	got := i18n.ErrorMessages(problemBundle().Localizer("ja"), []problem{
		{path: "title", code: "blank"},
		{path: "title", code: "too_short", params: map[string]any{"count": 3}},
		{path: "base", code: "too_young_for_admin"},
	})
	want := map[string][]string{
		"title": {"は必須です", "は3文字以上で入力してください"},
		"base":  {"管理者は21歳以上にしてください"},
	}
	if !maps.EqualFunc(got, want, slices.Equal) {
		t.Errorf("ErrorMessages = %q, want %q", got, want)
	}
}

// The params a problem hands out are its own: naming the field a confirmation
// refers to must not change them.
func TestFullMessages_leavesParams(t *testing.T) {
	t.Parallel()

	params := map[string]any{"attribute": "password"}
	shared := sharedProblem{params: params}
	i18n.FullMessages(problemBundle().Localizer("ja"), []sharedProblem{shared})
	if params["attribute"] != "password" {
		t.Errorf("params[attribute] = %v after rendering, want it left as password", params["attribute"])
	}
}

// A field and a field of it are each named by their paths.
func TestFullMessages_namesAFieldAndItsFields(t *testing.T) {
	t.Parallel()

	var errs validator.Errors
	errs.Add(validator.Field("address"), validator.Blank())
	errs.Add(validator.Field("address", "zip"), validator.Blank())
	got := i18n.FullMessages(problemBundle().Localizer("ja"), errs)
	if want := []string{"住所は必須です", "郵便番号は必須です"}; !slices.Equal(got, want) {
		t.Errorf("FullMessages = %q, want %q", got, want)
	}
}

// A map key and an index on a path are no fields: a field is named by the
// fields alone, even when a key spells the name of one.
func TestFullMessages_skipsKeysAndIndexes(t *testing.T) {
	t.Parallel()

	var errs validator.Errors
	errs.Add(validator.Key(validator.Field("notes"), "first_note"), validator.Blank())
	errs.Add(validator.Key(validator.Field("labels"), "zip"), validator.Blank())
	var zip validator.Errors
	zip.Add(validator.Field("zip"), validator.Blank())
	errs.Nest(validator.Index(validator.Field("address"), 0), zip)
	var account validator.Errors
	account.Add(validator.Field("confirm"), validator.Confirmation("password"))
	errs.Nest(validator.Key(validator.Field("accounts"), "main"), account)
	got := i18n.FullMessages(problemBundle().Localizer("ja"), errs)
	want := []string{
		"Notesは必須です",
		"Labelsは必須です",
		"郵便番号は必須です",
		"確認用パスワードとパスワードの入力が一致しません",
	}
	if !slices.Equal(got, want) {
		t.Errorf("FullMessages = %q, want %q", got, want)
	}
}

// Without Segments a path names one field as it is spelled: what the dots and
// brackets of a path step into, a field or a key, only Segments say.
func TestFullMessages_withoutSegments(t *testing.T) {
	t.Parallel()

	got := i18n.FullMessages(problemBundle().Localizer("ja"), []problem{
		{path: "title", code: "blank"},
		{path: "address.zip", code: "blank"},
		{path: "notes.first_note", code: "blank"},
		{path: "tags[1]", code: "blank"},
		{path: "confirm", code: "confirmation", params: map[string]any{"attribute": "password"}},
	})
	want := []string{
		"タイトルは必須です",
		"郵便番号は必須です",
		"Notes.first noteは必須です",
		"Tags[1]は必須です",
		"確認用パスワードとパスワードの入力が一致しません",
	}
	if !slices.Equal(got, want) {
		t.Errorf("FullMessages = %q, want %q", got, want)
	}

	// A FieldError built by hand is one too.
	got = i18n.FullMessages(problemBundle().Localizer("ja"),
		validator.Errors{{Field: "address.zip", Err: validator.Blank()}, {Field: "tags[1]", Err: validator.Blank()}})
	if want := []string{"郵便番号は必須です", "Tags[1]は必須です"}; !slices.Equal(got, want) {
		t.Errorf("FullMessages = %q, want %q", got, want)
	}
}

// sharedProblem hands out its params without copying them.
type sharedProblem struct{ params map[string]any }

func (sharedProblem) Path() string             { return "confirm" }
func (sharedProblem) Code() string             { return "confirmation" }
func (p sharedProblem) Params() map[string]any { return p.params }

// A Localizer that renders every message as its key names a field after its
// path all the same.
func TestFullMessages_zeroLocalizer(t *testing.T) {
	t.Parallel()

	got := i18n.FullMessages(i18n.Localizer{}, []problem{{path: "title", code: "blank"}, {path: "", code: "blank"}})
	if want := []string{"errors.format", "errors.format"}; !slices.Equal(got, want) {
		t.Errorf("FullMessages = %q, want %q", got, want)
	}
}

func TestIsBuiltin(t *testing.T) {
	t.Parallel()

	for key, want := range map[string]bool{
		"errors.format":         true,
		"errors.messages.blank": true,
		"errors.messages.taken": false,
		"greeting":              false,
	} {
		if got := runtime.IsBuiltin(key); got != want {
			t.Errorf("IsBuiltin(%q) = %t, want %t", key, got, want)
		}
	}
}

func TestBuiltinIn(t *testing.T) {
	t.Parallel()

	for _, tt := range []struct {
		lang, key string
		want      bool
	}{
		{"ja", "errors.messages.blank", true},
		{"en-GB", "errors.messages.blank", true},
		{"de", "errors.messages.blank", false},
		{"ja", "errors.messages.taken", false},
		{"ja", "greeting", false},
	} {
		if got := runtime.BuiltinIn(language.MustParse(tt.lang), tt.key); got != tt.want {
			t.Errorf("BuiltinIn(%s, %q) = %t, want %t", tt.lang, tt.key, got, tt.want)
		}
	}
}

func TestBuiltinParams(t *testing.T) {
	t.Parallel()

	for key, want := range map[string][]string{
		"errors.format":                {"attribute", "message"},
		"errors.messages.too_long":     {"count"},
		"errors.messages.confirmation": {"attribute"},
		"errors.messages.blank":        nil,
	} {
		if got, ok := runtime.BuiltinParams(key); !ok || !slices.Equal(got, want) {
			t.Errorf("BuiltinParams(%q) = %q, %t; want %q, true", key, got, ok, want)
		}
	}
	if got, ok := runtime.BuiltinParams("greeting"); ok || got != nil {
		t.Errorf("BuiltinParams(greeting) = %q, %t; want nil, false", got, ok)
	}
}
