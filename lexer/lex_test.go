package lexer_test

// func tokenString(f *token.File, i *lexer.Item) string {
// 	return fmt.Sprintf("%v: %s", f.Position(i.Pos), i.String())
// }

// func isSeparator(t token.Token, r rune) bool {
// 	return strings.IndexRune("!\"$%^&*()[]{}'#@~,./<>?", r) >= 0
// }

// var lang = token.Lang{
// 	IsIdentifier: func(r rune, first bool) bool {
// 		return first == true && (r == '_' || unicode.IsLetter(r)) || !isSeparator(token.Identifier, r) && unicode.IsPrint(r)
// 	},
// 	IsSeparator: isSeparator,
// }

// func TestLexer_Lex(t *testing.T) {
// 	tests := []struct {
// 		name  string
// 		input string
// 		want  []string
// 	}{
// 		{"identifier", "foo", []string{"1:1: Identifier foo", "1:4: EOF"}},
// 		{"bad_identifier", "$r\nok_id", []string{
// 			"1:1: Error invalid character U+0024 '$'",
// 			"1:2: Identifier r",
// 			"1:3: EOL",
// 		}},
// 		{"comment", "ok;hello", []string{
// 			"1:1: Identifier ok",
// 			"1:3: Comment ;hello",
// 			"1:9: EOF",
// 		}},
// 		{"immediate8", "01234567\n012345678\n0a\n094", []string{
// 			"1:1: Immediate 342391", "1:9: EOL",
// 			"2:9: Error invalid character U+0038 '8' in base 8 immediate value", "2:10: EOL",
// 			"3:1: Immediate 0", "3:2: Identifier a", "3:3: EOL",
// 			"4:2: Error invalid character U+0039 '9' in base 8 immediate value", "4:4: EOF",
// 		}},
// 		{"immediate10", "0\n12\n1a", []string{
// 			"1:1: Immediate 0", "1:2: EOL",
// 			"2:1: Immediate 12", "2:3: EOL",
// 			"3:1: Immediate 1", "3:2: Identifier a", "3:3: EOF",
// 		}},
// 		{"immediate16", "0x\n0x24(r0)", []string{
// 			"1:1: Immediate 0", "1:2: Identifier x", "1:3: EOL",
// 			"2:1: Immediate 36", "2:5: LeftParen",
// 		}},
// 		{"immediate2", "0b11\nj 0b\nj 0f", []string{
// 			"1:1: Immediate 3", "1:5: EOL",
// 			"2:1: Identifier j", "2:2: Space", "2:3: Immediate 0", "2:4: Identifier b", "2:5: EOL",
// 			"3:1: Identifier j", "3:2: Space", "3:3: Immediate 0", "3:4: Identifier f", "3:5: EOF",
// 		}},
// 	}
// 	for _, tt := range tests {
// 		t.Run(tt.name, func(t *testing.T) {
// 			var i int
// 			f := token.NewFile("", strings.NewReader(tt.input))
// 			l := lexer.New(f)
// 			for i = 0; i < len(tt.want); i++ {
// 				lx := l.Lex()
// 				if got := tokenString(l.File(), lx); got != tt.want[i] {
// 					t.Errorf("Got:\n\t%s\nWant:\n\t%s", got, tt.want[i])
// 				}
// 				if lx.Token == token.EOF {
// 					i++
// 					break
// 				}
// 			}
// 			if i < len(tt.want) {
// 				t.Errorf("Missing token:\n\t%s", tt.want[i])
// 			}
// 			l.Close()
// 		})
// 	}
// }

// func ExampleLexer_Lex() {
// 	input := "DéjàVu:\n"
// 	f := token.NewFile("test_eof", strings.NewReader(input))
// 	l := lexer.New(f)
// 	for n := 0; n < 5; n++ {
// 		i := l.Lex()
// 		line, col := l.File().Position(i.Pos)
// 		fmt.Println(line, col, i.String())
// 	}
// 	// Output:
// 	// 1 1 Identifier DéjàVu
// 	// 1 7 Colon
// 	// 1 8 EOL
// 	// 2 1 EOF
// 	// 2 1 EOF
// }
