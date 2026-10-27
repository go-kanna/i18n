package i18n

// Message is a message to render, with the arguments its placeholders take.
// The constructors kanna-i18n generates return one, so that code that does not
// know the reader's language can hand it to code that does.
type Message struct {
	Key  string
	Args []Arg
}

// Arg is the value of the placeholder Name.
//
// A string or an int is rendered as it is, and a float64 in the number format
// of the language rendering it: that covers everything a generated constructor
// passes. Any other value is rendered by fmt.
type Arg struct {
	Name  string
	Value any
}

// CountParam is the argument whose value, an int, picks the form of a plural
// message.
const CountParam = "count"
