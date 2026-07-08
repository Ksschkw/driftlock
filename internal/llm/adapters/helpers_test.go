package adapters

import "testing"

func TestParseCheckResponse(t *testing.T) {
	cases := []struct {
		name    string
		in      string
		wantOK  bool
		wantErr bool
	}{
		{"plain true", "TRUE. Docs are accurate.", true, false},
		{"plain false", "FALSE. Missing the new param.", false, false},
		{"lowercase", "true, everything matches", true, false},
		{"markdown bold", "**FALSE** - the signature changed", false, false},
		{"prefixed", "Answer: FALSE. The return type is wrong.", false, false},
		{"leading verdict wins over prose", "TRUE. It could be false in edge cases but here it is fine.", true, false},
		{"code fence", "```\nFALSE\n```", false, false},
		{"no verdict", "The documentation looks reasonable to me.", false, true},
		{"empty", "", false, true},
	}
	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			ok, _, err := parseCheckResponse(c.in)
			if (err != nil) != c.wantErr {
				t.Fatalf("err = %v, wantErr = %v", err, c.wantErr)
			}
			if err == nil && ok != c.wantOK {
				t.Fatalf("ok = %v, want %v", ok, c.wantOK)
			}
		})
	}
}

func TestStripPreambleMarkdown(t *testing.T) {
	in := "Here is your updated doc:\n\n# Title\nbody"
	out := stripPreambleMarkdown(in)
	if out != "# Title\nbody" {
		t.Errorf("unexpected: %q", out)
	}
}
