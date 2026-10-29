package i18n_test

import (
	"maps"
	"slices"
	"testing"

	"github.com/go-kanna/i18n"
)

// problem is a broken rule as validate.FieldError reports one.
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
//	    address:
//	      zip: 郵便番号
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
			"errors.attributes.address.zip":       single(text("郵便番号")),
			"errors.messages.blank":               single(text("は必須です")),
			"errors.messages.too_young_for_admin": single(text("管理者は21歳以上にしてください")),
		}},
	)
}

var problems = []problem{
	{path: "title", code: "blank"},
	{path: "tags[1]", code: "too_long", params: map[string]any{"count": 20}},
	{path: "tags[2]", code: "too_long", params: map[string]any{"count": 1}},
	{path: "address.zip", code: "invalid"},
	{path: "completed_at", code: "greater_than_or_equal_to", params: map[string]any{"count": 2000}},
	{path: "confirm", code: "confirmation", params: map[string]any{"attribute": "password"}},
	{path: "email", code: "corp_domain"},
	{path: "base", code: "too_young_for_admin"},
	{path: "name", code: "taken"},
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

			got := i18n.FullMessages(problemBundle().Localizer(tt.lang), problems)
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
		if got := i18n.IsBuiltin(key); got != want {
			t.Errorf("IsBuiltin(%q) = %t, want %t", key, got, want)
		}
	}
}
