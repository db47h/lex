package lexer_test

import (
	"bytes"
	"io/ioutil"

	"github.com/db47h/parsekit/lexer"
	"github.com/db47h/parsekit/token"
)

// This example demonstrates how to implement a lexer for https://golang.org/pkg/text/template/
// In this implementation we wrap a lexer.Interface in a custom struct that will
// hold some state data.
//
// Note that because we cannot backup in the input stream by more than
// one character, we cannot easily allow arbitrary delimiters. For example, if
// the user chose ".Comment-start" as a left comment delimiter, lexing properly
// "{{.Component}}" would be somewhat tedious.
//
func Example_text_template() {
	data, err := ioutil.ReadFile("testdata/test_template.tpl")
	if err != nil {
		panic(err)
	}
	f := token.NewFile("testdata", bytes.NewBuffer(data))
	l := NewLexer(f)
	_ = l
}

const (
	itemError        token.Type = iota // error occurred; value is text of error
	itemBool                           // boolean constant
	itemChar                           // printable ASCII character; grab bag for comma etc.
	itemCharConstant                   // character constant
	itemComplex                        // complex constant (1+2i); imaginary is just a number
	itemColonEquals                    // colon-equals (':=') introducing a declaration
	itemEOF
	itemField      // alphanumeric identifier starting with '.'
	itemIdentifier // alphanumeric identifier not starting with '.'
	itemLeftDelim  // left action delimiter
	itemLeftParen  // '(' inside action
	itemNumber     // simple number, including imaginary
	itemPipe       // pipe symbol
	itemRawString  // raw quoted string (includes quotes)
	itemRightDelim // right action delimiter
	itemRightParen // ')' inside action
	itemSpace      // run of spaces separating arguments
	itemString     // quoted string (includes quotes)
	itemText       // plain text
	itemVariable   // variable starting with '$', such as '$' or  '$1' or '$hello'
	// Keywords appear after all the rest.
	itemKeyword  // used only to delimit the keywords
	itemBlock    // block keyword
	itemDot      // the cursor, spelled '.'
	itemDefine   // define keyword
	itemElse     // else keyword
	itemEnd      // end keyword
	itemIf       // if keyword
	itemNil      // the untyped nil constant, easiest to treat as a keyword
	itemRange    // range keyword
	itemTemplate // template keyword
	itemWith     // with keyword
)

// NewLexer creates a TemplateLexer and sets up a new lexer with
// TemplateLexer.text as its initial state.
//
func NewLexer(f *token.File) lexer.Interface {
	l := &TemplateLexer{}
	l.Interface = lexer.New(f, l.text)
	return l
}

// isSpace returns true if r is a space character.
//
func isSpace(r rune) bool {
	switch r {
	case ' ', '\t', '\r', '\n':
		return true
	}
	return false
}

// TemplateLexer is a thin wrapper around lexer.Interface.
//
type TemplateLexer struct {
	lexer.Interface        // embedded field so that we can skip redefining Lex
	buf             []rune // temp buffer for input
}

// text is the initial state function for the lexer. It reads text until it
// finds a left action delimiter "{{". At this point, it will optionally trim
// trailing white space from the text, emit it, then switch the lexer to action
// lexing.
//
func (tl *TemplateLexer) text(l *lexer.Lexer) lexer.StateFn {
	r := l.Next()

	if r == lexer.EOF {
		return tl.eof
	}

	// left action delimiter ?
	if r == '{' && l.Accept('{') {
		l.Emit(itemLeftDelim, "{{")
		// transition state
		return tl.actionStart
	}

	// something else, buffer it and continue
	tl.buf = append(tl.buf, r)
	return nil
}

// eof is an eof state. It stop reading input and keeps emitting eof.
// Note that we don't actually need to make this function a member of TemplateLexer.
//
func (tl *TemplateLexer) eof(l *lexer.Lexer) lexer.StateFn {
	l.Emit(token.EOF, nil)
	return tl.eof
}

// actionStart checks the special cases for "{{- " and "{{/*".
//
func (tl *TemplateLexer) actionStart(l *lexer.Lexer) lexer.StateFn {
	r := l.Next()

	if r == '-' {
		if l.Accept(' ') {
			// trim end of text buffer
			var i = len(tl.buf)
			for i > 0 && isSpace(tl.buf[i-1]) {
				i--
			}
			tl.buf = tl.buf[:i]
		}
	}

	if len(tl.buf) > 0 {
		l.Emit(itemText, string(tl.buf))
		tl.buf = tl.buf[:0]
	}

	if r == '/' && l.Accept('*') {
		// this action is a comment.
		return tl.comment
	}

	// Now that the special cases are covered, switch to action lexing by
	// making tl.action the new initial state function for the lexer.
	return l.Init(tl.action)
}

// comment skips over a comment.
//
func (tl *TemplateLexer) comment(l *lexer.Lexer) lexer.StateFn {
	// skip input to end of comment
	for {
		r := l.Next()
		if r == lexer.EOF {
			l.Errorf(l.Pos(), "unterminated comment")
			return tl.eof
		}
		if r == '*' && l.Accept('/') {
			break
		}
	}

	// end of comment, expect "}}"
	if r := l.Next(); r != '}' || !l.Accept('}') {
		// just report the error, keep lexing as if properly closed
		l.Errorf(l.Pos(), "comment must be terminated by */}}")
	}
	return nil
}

// action is the initial state for actions.
//
func (tl *TemplateLexer) action(l *lexer.Lexer) lexer.StateFn {
	r := l.Current()

	if isSpace(r) {
		n := l.AcceptWhile(isSpace)
		// check for " -{{"
		if l.Current() == ' ' && l.Peek() == '-' {
			if n > 1 {
				// any amount of space before
			}
		}
	}

	switch r {
	case lexer.EOF:
		return tl.eof
	case '}':
		if l.Accept('}') {
			l.Emit(itemRightDelim, "}}")
			// end of action
			return l.Init(tl.text)
		}
	case ' ':
		if l.Accept('-') {
			// a very special case
			l.Next()
			return tl.negNumberOrRightDelim
		}
		l.AcceptWhile(isSpace)
		l.Emit(itemSpace, " ")
		return nil
	}

	// anything else
	l.Emit(itemChar, string(r))
	return nil
}

// negNumberOrRightDelim handles input starting with " -"
//
func (tl *TemplateLexer) negNumberOrRightDelim(l *lexer.Lexer) lexer.StateFn {
	r := l.Current()

	switch r {
	case '}':
		if l.Accept('}') {
			l.Emit(itemRightDelim, "}}")
			// "}}" preceded by " -", left-trim the following plain text.
			l.AcceptWhile(isSpace)
			// switch back to text lexing.
			return l.Init(tl.text)
		}
	case '.', '0', '1', '2', '3', '4', '5', '6', '7', '8', '9':
		// a number. add the minus sign to the buffer, and switch to number state
		tl.buf = append(tl.buf, '-')
		return tl.number
	}

	// If input following " -" is not a number or a right delimiter, this is
	// likely an error but not as far as the lexer is concerned.
	// Our current token is " -" + something else. Simply emit it as separate
	// tokens and let the parser deal with it.
	l.Emit(itemSpace, " ")
	l.S++
	l.Emit(itemChar, "-")
	// we cannot unread r. do not return nil, update token start manually.
	l.S = l.Pos()
	return tl.action
}

// number expects a number.
//
func (tl *TemplateLexer) number(l *lexer.Lexer) lexer.StateFn {
	panic("TODO")
	return nil
}
