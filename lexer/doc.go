// Copyright 2017 Denis Bernard <db047h@gmail.com>
//
// Permission is hereby granted, free of charge, to any person obtaining a copy of
// this software and associated documentation files (the "Software"), to deal in
// the Software without restriction, including without limitation the rights to
// use, copy, modify, merge, publish, distribute, sublicense, and/or sell copies of
// the Software, and to permit persons to whom the Software is furnished to do so,
// subject to the following conditions:
//
// The above copyright notice and this permission notice shall be included in all
// copies or substantial portions of the Software.
//
// THE SOFTWARE IS PROVIDED "AS IS", WITHOUT WARRANTY OF ANY KIND, EXPRESS OR
// IMPLIED, INCLUDING BUT NOT LIMITED TO THE WARRANTIES OF MERCHANTABILITY, FITNESS
// FOR A PARTICULAR PURPOSE AND NONINFRINGEMENT. IN NO EVENT SHALL THE AUTHORS OR
// COPYRIGHT HOLDERS BE LIABLE FOR ANY CLAIM, DAMAGES OR OTHER LIABILITY, WHETHER
// IN AN ACTION OF CONTRACT, TORT OR OTHERWISE, ARISING FROM, OUT OF OR IN
// CONNECTION WITH THE SOFTWARE OR THE USE OR OTHER DEALINGS IN THE SOFTWARE.

/*
Package lexer provides the core of a lexer built as a Deterministic Finite
Automaton whose states and associated actions are implemented as functions.

Clients of the package only need to provide state functions specialized in
lexing the target language. The package provides facilities to stream input
from a io.Reader with up to 7 characters look-ahead, as well as utility
functions commonly used in lexers.


State functions

The implementation is similar to https://golang.org/src/text/template/parse/lex.go.
Se also Rob Pike's talk about combining states and actions into state functions:
https://talks.golang.org/2011/lex.slide.

TL;DR: State machines are often implemented with a switch statement:

	// One iteration:
	switch state {
	case sate1:
		state = action1()
	case state2:
		state = action2()
	// etc.
	}

States represent where we are and what to expect while actions represent what we
do and result in a new state. By taking advantage of the fact that functions are
values, we can aggregate state and action. First we define a state function
type:

	type StateFn func(*Lexer) StateFn

A StateFn is both state and action. It takes a lexer argument (to allow it to
read from the input stream and emit tokens) and returns another state function.

Then the state transition loop can be rewritten like this:

	// One iteration:
	state = state()

This package can also be used as a toolbox for a more traditional switch-based
lexer (see implementation below).

Implementation details

Unlike the Go text template package which uses Go channels as a mean of
asynchronous token emission, this package uses a FIFO queue instead.
Benchmarks with an earlier implementation that used a channel showed that
using a FIFO is about 5 times faster. There is also no need for the parser
to take care of cancellation or draining the input in case of error.

The drawback of using a FIFO is that once a StateFn has called Emit, it should
return as soon as possible so that the caller of Lex (usually a parser) can
dequeue the lexed item. Since item queue grows dynamically, it is also possible
to never return from the initial state function until EOF is reached.

The initial state of the DFA is the state where we expect to read a new token.
From that initial state, the lexer transitions to other states until a token is
successfully matched or an error occurs. The state function that found the match
or error emits the corresponding token and finally returns nil to transition
back to the initial state.

The "nil return means initial state" convention enables building a library of
state functions for common things like quoted strings or numbers where returning
to the initial state is as simple as a nil return.

Implementing a custom lexer

A lexer for any given language is simply a set of state functions referencing
each other. The initial state of the DFA is the state where we expect to read a
new token. Depending on the input, that initial StateFn returns the appropriate
StateFn as the next state. The returned StateFn repeats this process until a
match is eventually found for a token, at this point the StateFn calls Emit and
returns nil so that the DFA transitions back to the initial state.

Upon returning nil from a StateFn, the lexer will do the following:

	l.nextState = l.init(l)	// call the initial state function

Transitions to the initial state should be done by returning nil from a StateFn.

EOF conditions must be handled manually. This means that at the very least, the
initial state function should always check for EOF and return lexer.StateEOF.
Other states should not have to deal with it explicitly. i.e. EOF is not a valid
rune, as such unicode.IsDigit() will return false, so will IsSpace.
A common exception is tokens that need a terminator (like quoted strings) where
EOF should be checked explicitly in order to emit errors in the absence of a
terminator.

The state sub-package provides state functions for lexing quoted strings and
quoted characters. By convention, all functions in the state package expect that
the first character that is part of the lexed entity has already been read by
Lexer.Next. While this is a useful convention in the context of that package (it
allows client code to look ahead further in the input stream), it does not apply
to custom state functions in client code.

*/
package lexer

//go:generate bash -c "godoc2md -ex -template ../template/README.md.tpl github.com/db47h/parsekit/lexer >README.md"
