package output

import "fmt"

var (
	Reset  = "\033[0m"
	Red    = "\033[31m"
	Green  = "\033[32m"
	Yellow = "\033[33m"
	Bold   = "\033[1m"
)

// Plain color wrappers – just wrap a string with colour codes.
func Redf(format string, a ...interface{}) string {
	return Red + fmt.Sprintf(format, a...) + Reset
}

func Greenf(format string, a ...interface{}) string {
	return Green + fmt.Sprintf(format, a...) + Reset
}

func Yellowf(format string, a ...interface{}) string {
	return Yellow + fmt.Sprintf(format, a...) + Reset
}

func Boldf(format string, a ...interface{}) string {
	return Bold + fmt.Sprintf(format, a...) + Reset
}

// Simple colour wrappers that just wrap a pre‑composed string.
func RedStr(s string) string {
	return Red + s + Reset
}

func GreenStr(s string) string {
	return Green + s + Reset
}

func YellowStr(s string) string {
	return Yellow + s + Reset
}

// I am adding this line to test ts

func BoldStr(s string) string {
	return Bold + s + Reset
}
