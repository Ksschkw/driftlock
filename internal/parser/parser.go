package parser

// Signature represents a function/method signature.
type Signature struct {
	Name      string
	Signature string // full signature as string
}

// LanguageParser defines the interface for extracting signatures from source code.
type LanguageParser interface {
	ExtractSignatures(source string) []Signature
}