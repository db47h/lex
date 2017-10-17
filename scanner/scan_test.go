package scanner_test

import (
	"testing"

	"github.com/db47h/asm/scanner"
	"github.com/db47h/asm/token"
)

func TestScanner_Scan(t *testing.T) {
	tests := []struct {
		name  string
		input string
		want  []string
	}{
		{"identifier", "foo", []string{"1:1: Identifier \"foo\"", "1:4: EOF \"\""}},
		{"bad_identifier", "$r\nok_id", []string{
			"1:1: Error \"illegal symbol '$'\"",
			"1:3: EOL \"\\n\"",
		}},
		{"comment", "ok;hello", []string{
			"1:1: Identifier \"ok\"",
			"1:3: Comment \";hello\"",
			"1:9: EOF \"\"",
		}},
		{"immediate8", "01234567\n012345678\n0a\n01a", []string{
			"1:1: Immediate \"01234567\"", "1:9: EOL \"\\n\"",
			"2:9: Error \"illegal symbol '8' in base 8 immediate value\"", "2:10: EOL \"\\n\"",
			"3:2: Error \"illegal symbol 'a' in immediate value\"", "3:3: EOL \"\\n\"",
			"4:3: Error \"illegal symbol 'a' in base 8 immediate value\"", "4:4: EOF \"\"",
		}},
		{"immediate10", "0\n12\n1a", []string{
			"1:1: Immediate \"0\"", "1:2: EOL \"\\n\"",
			"2:1: Immediate \"12\"", "2:3: EOL \"\\n\"",
			"3:2: Error \"illegal symbol 'a' in base 10 immediate value\"", "3:3: EOF \"\"",
		}},
		{"immediate16", "0x24(r0)", []string{
			//			"1:1: Error \"malformed immediate value \"0x\"", "1:3: EOL \"\\n\"",
			"1:1: Immediate \"0x24\"", "1:5: LeftParen \"(\"",
		}},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			var s scanner.Scanner
			var i int
			s.Init([]byte(tt.input))
			for i = 0; i < len(tt.want); i++ {
				tok := s.Scan()
				if tok.String() != tt.want[i] {
					t.Errorf("Got:\n\t%s\nWant:\n\t%s", tok.String(), tt.want[i])
				}
				if tok.Token == token.EOF {
					i++
					break
				}
			}
			if i < len(tt.want) {
				t.Errorf("Missing token:\n\t%s", tt.want[i])
			}
			s.Close()
		})
	}
}
