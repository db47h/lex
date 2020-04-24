// Copyright 2017-2020 Denis Bernard <db047h@gmail.com>
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
Package lex provides the core of a lexer built as a Deterministic Finite State
Automaton whose states and associated actions are implemented as functions.

Clients of the package only need to provide state functions specialized in
lexing the target language. The package provides facilities to stream input
from a io.Reader with up to 15 bytes look-ahead, as well as utility functions
commonly used in lexers.

State functions

The implementation is similar to https://golang.org/src/text/template/parse/lex.go.
See also Rob Pike's talk about combining states and actions into state
functions: https://talks.golang.org/2011/lex.slide.

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

	type StateFn func(*State) StateFn

A StateFn is both state and action. It takes a lexer argument (to allow it to
read from the input stream and emit tokens) and returns another state function.

Then the state transition loop can be rewritten like this:

	// One iteration:
	state = state()

This package can also be used as a toolbox for a more traditional switch-based
lexer (see implementation below).

Implementation details

Unlike the Go text template package which uses Go channels as a means of
asynchronous token emission, this package uses a FIFO queue instead.
Benchmarks with an earlier implementation that used a channel showed that
using a FIFO is about 5 times faster. There is also no need for the parser
to take care of cancellation or draining the input in case of error.

The drawback of using a FIFO is that once a StateFn has called Emit, it should
return as soon as possible so that the caller of Lex (usually a parser) can
dequeue the lexed item. Since the item queue grows dynamically, it is also
possible to write a state function that never returns until EOF is reached.

The initial state of the DFA is the state where we expect to read a new token.
From that initial state, the lexer transitions to other states until a token is
successfully matched or an error occurs. The state function that finds a match
or error emits the corresponding token and finally returns nil to transition
back to the initial state.

The "nil-return means initial state" convention enables building a library of
state functions for common things like quoted strings or numbers where returning
to the initial state is as simple as a nil return.

Implementing a lexer

A lexer for any given language is simply a set of state functions referencing
each other. The initial state of the DFA is the state where we expect to read a
new token. Depending on the input, that initial StateFn returns the appropriate
StateFn as the next state. The returned StateFn repeats this process until a
match is eventually found for a token, at this point the StateFn calls Emit and
returns nil so that the DFA transitions back to the initial state.

EOF conditions must be handled manually. This means that at the very least, the
initial state function should always check for EOF and emit an EOF token.
Other states should not have to deal with it explicitly. i.e. EOF is not a valid
rune, as such unicode.IsDigit() will return false, so will IsSpace.
A common exception is tokens that need a terminator (like quoted strings) where
EOF should be checked explicitly in order to emit errors in the absence of a
terminator.

Error handling

The lex package provides a single built-in Error token. This token is
automatically emitted whenever an I/O error occurs or on invalid UTF-8 input.

I/O errors are non-recoverable, that is any subsequent call to Lexer.Lex will
return EOF.

State.Next and State.ReadRune always return valid runes. Any invalid UTF-8 input
will be skipped and a matching Error token will be automatiocally generated.

Caveat: This may lead to Error tokens appearing alongside seemingly valid
tokens in the token stream. For example, lexing numbers in "1\02" will yield
an automatically generated Error token followed by some other token with literal
value "12". This should be taken into account when implementing error handling
strategies.

State sub-package

The state sub-package provides state functions for lexing numbers, quoted
strings and quoted characters. By convention, all functions in the state package
expect that the first character of the lexed entity has already been read by
Lexer.Next. Custom StateFn should also follow this convention.

*/
package lex
