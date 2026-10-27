package gen_test

import (
	"testing"

	"github.com/go-kanna/i18n"
	"github.com/go-kanna/i18n/internal/gen/testdata/basic/messages"
	names "github.com/go-kanna/i18n/internal/gen/testdata/names/messages"
)

// TestGenerated localizes through the golden files, closing the gap a golden
// file leaves: that what it compiles to says what the locale files say.
func TestGenerated(t *testing.T) {
	t.Parallel()

	en := messages.Localizer("en")
	ja := messages.Localizer("ja")

	tests := []struct {
		name string
		loc  i18n.Localizer
		msg  i18n.Message
		want string
	}{
		{name: "text", loc: en, msg: messages.Greeting(), want: "Hello!"},
		{name: "string", loc: en, msg: messages.Hello("World"), want: "Hello, World!"},
		{name: "plural one", loc: en, msg: messages.ItemsCount(1), want: "You have 1 item."},
		{name: "plural other", loc: en, msg: messages.ItemsCount(5), want: "You have 5 items."},
		{name: "number", loc: en, msg: messages.TotalPrice(1234.56), want: "Total: 1,234.56"},
		{name: "two arguments", loc: en, msg: messages.PriceOf("Tea", 3.5), want: "Tea costs 3.5"},
		{name: "a predeclared name", loc: en, msg: messages.Failed("timeout"), want: "Reason: timeout"},
		{name: "literal braces", loc: en, msg: messages.Braces(), want: "Write {name} for the name."},
		{name: "empty", loc: en, msg: messages.Empty(), want: ""},
		{name: "lines", loc: en, msg: messages.Terms(), want: "By signing up, you agree\nto the terms.\n"},
		{name: "nested key", loc: en, msg: messages.UserNotFound(), want: "User not found."},
		{name: "ja", loc: ja, msg: messages.Hello("太郎"), want: "こんにちは、太郎さん！"},
		{name: "ja plural", loc: ja, msg: messages.ItemsCount(1), want: "アイテムが1個あります。"},
		{name: "ja number", loc: ja, msg: messages.TotalPrice(1234.56), want: "合計: 1,234.56"},
		{name: "ja literal braces", loc: ja, msg: messages.Braces(), want: "名前には {name} と書きます。"},
		{name: "missing from ja", loc: ja, msg: messages.UserDeleted("Alice"), want: "User Alice has been deleted."},
		{name: "Accept-Language", loc: messages.Localizer("fr, ja;q=0.8"), msg: messages.Greeting(), want: "こんにちは！"},
		{name: "names kept clear", loc: names.Localizer(), msg: names.Items(2, "you"), want: "2 items for you"},
		{name: "reserved names", loc: names.Localizer(), msg: names.Kinds("a", "b"), want: "a of b"},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()

			if got := tt.loc.Localize(tt.msg); got != tt.want {
				t.Errorf("Localize(%q) = %q, want %q", tt.msg.Key, got, tt.want)
			}
		})
	}
}
