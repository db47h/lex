package lexer_test

import (
	"fmt"
	"strings"
	"testing"

	"github.com/db47h/asm/lexer"
	"github.com/db47h/asm/token"
)

func tokenString(f *token.File, i *lexer.Item) string {
	line, col := f.Position(i.Pos)
	return fmt.Sprintf("%d:%d: %s", line, col, i.String())
}

func TestLexer_Lex(t *testing.T) {
	tests := []struct {
		name  string
		input string
		want  []string
	}{
		{"identifier", "foo", []string{"1:1: Identifier foo", "1:4: EOF"}},
		{"bad_identifier", "$r\nok_id", []string{
			"1:1: Error invalid character U+0024 '$'",
			"1:3: EOL",
		}},
		{"comment", "ok;hello", []string{
			"1:1: Identifier ok",
			"1:3: Comment ;hello",
			"1:9: EOF",
		}},
		{"immediate8", "01234567\n012345678\n0a\n09", []string{
			"1:1: Immediate 342391", "1:9: EOL",
			"2:9: Error invalid character U+0038 '8' in base 8 immediate value", "2:10: EOL",
			"3:2: Error invalid character U+0061 'a' in immediate value", "3:3: EOL",
			"4:2: Error invalid character U+0039 '9' in base 8 immediate value", "4:3: EOF",
		}},
		{"immediate10", "0\n12\n1a", []string{
			"1:1: Immediate 0", "1:2: EOL",
			"2:1: Immediate 12", "2:3: EOL",
			"3:2: Error invalid character U+0061 'a' in base 10 immediate value", "3:3: EOF",
		}},
		{"immediate16", "0x\n0x24(r0)", []string{
			"1:2: Error malformed immediate value \"0x\"", "1:3: EOL",
			"2:1: Immediate 36", "2:5: LeftParen",
		}},
		{"immediate2_LocalLabel", "0:\nlui zero 0b11\nj 0b\nj 0f", []string{
			"1:1: Immediate 0", "1:2: Colon", "1:3: EOL",
			"2:1: Identifier lui", "2:4: Space", "2:5: Identifier zero", "2:9: Space", "2:10: Immediate 3", "2:14: EOL",
			"3:1: Identifier j", "3:2: Space", "3:3: LocalLabel 0b", "3:5: EOL",
			"4:1: Identifier j", "4:2: Space", "4:3: LocalLabel 0f", "4:5: EOF",
		}},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			var i int
			f := token.NewFile("", strings.NewReader(tt.input))
			l := lexer.New(f)
			for i = 0; i < len(tt.want); i++ {
				lx := l.Lex()
				if got := tokenString(l.File(), lx); got != tt.want[i] {
					t.Errorf("Got:\n\t%s\nWant:\n\t%s", got, tt.want[i])
				}
				if lx.Token == token.EOF {
					i++
					break
				}
			}
			if i < len(tt.want) {
				t.Errorf("Missing token:\n\t%s", tt.want[i])
			}
			l.Close()
		})
	}
}

func ExampleLexer_Lex() {
	input := "DéjàVu:\n"
	f := token.NewFile("test_eof", strings.NewReader(input))
	l := lexer.New(f)
	for n := 0; n < 5; n++ {
		i := l.Lex()
		line, col := l.File().Position(i.Pos)
		fmt.Println(line, col, i.String())
	}
	// Output:
	// 1 1 Identifier DéjàVu
	// 1 7 Colon
	// 1 8 EOL
	// 2 1 EOF
	// 2 1 EOF
}
