package hook

import "testing"

// Doc chunking picks the sections that mention a changed symbol, so the name
// must be recovered from every language's signature shape. The old
// implementation only handled a leading `func`/`def`/`fn`, so anything with a
// modifier returned "" and chunking silently degraded to the whole document.
func TestExtractNameFromSignature(t *testing.T) {
	cases := []struct {
		sig  string
		want string
	}{
		{"func Hello(name string) string", "Hello"},
		{"func (a *A) Close() error", "Close"},
		{"func (s *Stack[T]) Push(v T)", "Push"},
		{"func Map[T any, U any](xs []T) []U", "Map"},
		{"public int add(int a, int b)", "add"},
		{"void log(String msg)", "log"},
		{"pub fn build(&self) -> Self", "build"},
		{"pub async fn fetch(url: &str) -> Data", "fetch"},
		{"export function makeUser(name: string): User", "makeUser"},
		{"async def fetch(url: str) -> bytes", "fetch"},
		{"fun transform(input: List<Int>): List<String>", "transform"},
		{"func onDone(handler: (Int) -> Void)", "onDone"},
		{"class Calculator", "Calculator"},
		{"interface Shape", "Shape"},
		{"type Stack[T any] struct", "Stack"},
		{"export type ID = string", "ID"},
		// Rejections: private-by-convention and control flow.
		{"def _private(x)", ""},
		{"if (x)", ""},
		{"", ""},
		{"class", ""},
	}
	for _, tc := range cases {
		if got := extractNameFromSignature(tc.sig); got != tc.want {
			t.Errorf("extractNameFromSignature(%q) = %q, want %q", tc.sig, got, tc.want)
		}
	}
}
