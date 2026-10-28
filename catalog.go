package i18n

// Catalog holds the messages of one language. It and what it is made of, Entry,
// Plural, Template, and Segment, are what kanna-i18n compiles a locale file
// into, written into the generated code as literals; the generator has checked
// them against every other language already, so nothing here checks them again.
type Catalog struct {
	// Lang is the language as a BCP 47 tag, such as "en" or "pt-BR".
	Lang    string
	Entries map[string]Entry
}

// Entry is one message of one language: Single holds a message with one form,
// and Plural, when set, one whose form depends on its count.
type Entry struct {
	Single Template
	Plural *Plural
}

// Plural holds the forms of a plural message, one for each CLDR plural category
// the language tells apart. Other is always there, and a count whose category
// has no form, left nil, renders with it. Zero, when set, is the form of a count
// of 0 in every language, English and others whose rules have no zero included.
type Plural struct {
	Zero, One, Two, Few, Many, Other Template
}

// Template is a message split into segments: "Hello, {name}!" is the text
// "Hello, ", the placeholder name, and the text "!".
type Template []Segment

// Segment is a run of text or, when Param is set, the placeholder of the
// argument named Param.
type Segment struct {
	Text  string
	Param string
}
