package token

import "testing"

func TestIsIdentifier(t *testing.T) {
	tests := []struct {
		name string
		in   string
		want bool
	}{
		{"Empty", "", false},
		{"Space", " ", false},
		{"SpaceSuffix", "foo ", false},
		{"Number", "123", false},
		{"Keyword", "func", false},

		{"LettersASCII", "foo", true},
		{"MixedASCII", "_bar123", true},
		{"UppercaseKeyword", "Func", true},
		{"LettersUnicode", "fóö", true},
	}
	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			if got := IsIdentifier(test.in); got != test.want {
				t.Fatalf("IsIdentifier(%q) = %t, want %v", test.in, got, test.want)
			}
		})
	}
}

func TestGonTokens(t *testing.T) {
	// Existing public values must not move when additive tokens are introduced.
	if TILDE != 88 || IDENT != 4 || ADD != 12 || VAR != 85 {
		t.Fatal("legacy token values changed")
	}
	for _, tok := range []Token{FATARROW, SAFE_PERIOD, SAFE_LPAREN, COALESCE, COALESCE_ASSIGN, QUESTION} {
		if !tok.IsOperator() || tok.IsKeyword() || tok.IsLiteral() {
			t.Errorf("token classification: %v", tok)
		}
	}
	if COALESCE.Precedence() != LOR.Precedence() || FATARROW.Precedence() != LowestPrec {
		t.Fatal("invalid Gon precedence")
	}
}
