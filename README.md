# Parsekit

Parsekit is a lexer and parser toolbox for Go.

## Lexer

[![godoc][godoc badge]][godoc]

The lexer package provides the core of a lexer built as a Deterministic Finite
Automaton whose states and associated actions are implemented as functions.

The implementation is similar to the [text template lexer][golex] in the Go
standard library (but without using channels). Se also Rob Pike's talk about
[combining states and actions into state functions][pike].

See the README in the lexer package for more details.

The [state] package also contains an example lexer for a minimal Go-like language.

## Parser

The parser is a work in progress and there is no code available yet. I initially
intended to implement it as a general purpose [Pratt parser]. This kind of
parser is especially good at parsing mathematical expressions and can
theoretically parse anything you throw at it. I still need to get my head around
that "parsing anything" bit and see if I can come up with a package that can
really provide the building blocks for a parser, in a useful way.

[godoc]: https://godoc.org/github.com/db47h/parsekit/lexer
[godoc badge]: https://godoc.org/github.com/db47h/parsekit/lexer?status.svg
[golex]: https://golang.org/src/text/template/parse/lex.go
[pike]: https://talks.golang.org/2011/lex.slide
[state]: https://godoc.org/github.com/db47h/parsekit/lexer/state
[Pratt parser]: http://journal.stuffwithstuff.com/2011/03/19/pratt-parsers-expression-parsing-made-easy/
