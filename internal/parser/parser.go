// Package parser extracts function / method / type signatures from source code
// in any programming language, using a universal set of patterns.
package parser

// Signature holds a function/method/type name and its full signature string.
type Signature struct {
	Name      string
	Signature string
}

// ExtractSignatures detects any structural declaration in the source and
// returns all found signatures. It works for every programming language
// (Python, Go, Rust, C, Java, JavaScript, Ruby, PHP, Lua, Julia, SQL, bash,
//
//	and many more) as well as markup / data languages (JSON, YAML, XML, etc.).
func ExtractSignatures(filePath string, source string) []Signature {
	return extractSignatures(filePath, source)
}
