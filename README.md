# lex

[![godocb]][godoc]

## Overview

Package lex provides the core of a lexer built as a Deterministic Finite State
Automaton whose states and associated actions are implemented as functions.

Clients of the package only need to provide state functions specialized in
lexing the target language. The package provides facilities to stream input
from a io.Reader with up to 15 bytes look-ahead, as well as utility functions
commonly used in lexers.

The implementation is similar to https://golang.org/src/text/template/parse/lex.go.
See also Rob Pike's talk about combining states and actions into state
functions: https://talks.golang.org/2011/lex.slide.

Read the [full package ducumentation on gpkg.go.dev][godoc].

## Release notes

### v1.2.1

Improvements to error handling:

`Lexer.Errorf` now generates `error` values instead of `string`.
`lexer.Emit` enforces the use of `error` values for `Error` tokens.

`Scanner` now implements the `io.RuneScanner` interface.

This is a minor API breakage that does not impact any known client code.

### v1.0.0

Initial release.

## License

Package lex is released under the terms of the MIT license:

> Copyright 2017-2020 Denis Bernard <db047h@gmail.com>
>
> Permission is hereby granted, free of charge, to any person obtaining a copy of
> this software and associated documentation files (the "Software"), to deal in
> the Software without restriction, including without limitation the rights to
> use, copy, modify, merge, publish, distribute, sublicense, and/or sell copies of
> the Software, and to permit persons to whom the Software is furnished to do so,
> subject to the following conditions:
>
> The above copyright notice and this permission notice shall be included in all
> copies or substantial portions of the Software.
>
> THE SOFTWARE IS PROVIDED "AS IS", WITHOUT WARRANTY OF ANY KIND, EXPRESS OR
> IMPLIED, INCLUDING BUT NOT LIMITED TO THE WARRANTIES OF MERCHANTABILITY, FITNESS
> FOR A PARTICULAR PURPOSE AND NONINFRINGEMENT. IN NO EVENT SHALL THE AUTHORS OR
> COPYRIGHT HOLDERS BE LIABLE FOR ANY CLAIM, DAMAGES OR OTHER LIABILITY, WHETHER
> IN AN ACTION OF CONTRACT, TORT OR OTHERWISE, ARISING FROM, OUT OF OR IN
> CONNECTION WITH THE SOFTWARE OR THE USE OR OTHER DEALINGS IN THE SOFTWARE.

[godoc]: https://pkg.go.dev/github.com/db47h/lex?tab=doc
[godocb]: https://img.shields.io/badge/go.dev-reference-blue
