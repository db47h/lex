# lexer

## Overview

Package lexer provides the core of a lexer built as a DFA whose states are
implemented as functions.

Clients of the package only need to provide state functions specialized in
lexing the target language. The package provides facilities to stream input
from a RuneReader (this will be changed in the future to a simple io.Reader)
with one char look-ahead, as well as utility functions commonly used in lexers.

Implementation details:

The implementation is similar to <a href="https://golang.org/src/text/template/parse/lex.go">https://golang.org/src/text/template/parse/lex.go</a>.
Se also Rob Pike's talk about using state functions for lexing: <a href="https://talks.golang.org/2011/lex.slide">https://talks.golang.org/2011/lex.slide</a>.

Unlike the Go text template package which uses Go channels as a mean of
asynchronous token emission, this package uses a FIFO queue instead.
Benchmarks with an earlier implementation that used a channel showed that
using a FIFO is about 5 times faster. There is also no need for the parser
to take care of cancellation or draining the input in case of error.

The drawback of using a FIFO is that once a state function has called Emit,
it must return as soon as possible so that the caller of Lex (usually a
parser) can receive the lexed item.

The state sub-package provides state functions for lexing quoted strings,
quoted characters and numbers (integers in any base as well as floats) and
graceful handling of EOF.

## Installation

```bash
go get -u github.com/db47h/parsekit/lexer
```

Then just import the package in your cusrom lexer:

```go
import "github.com/db47h/parsekit/lexer"
```

## License

Package lexer is released under the terms of the MIT license:

> Copyright 2017 Denis Bernard <db047h@gmail.com>
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