# state

[![GoDoc](https://godoc.org/github.com/db47h/lex/state?status.svg)](https://godoc.org/github.com/db47h/lex/state)

## Overview

Package state provides state functions for lexing numbers, quoted strings and
quoted characters.

State functions in this package expect that the first character that is
part of the lexed entity has already been read by lex.Next. For example:


	r := s.Next()
	switch r {
	case '"':
		// do not call s.Backup() here
		return state.QuotedString(tokString)
	}

All functions (with the exception of EOF) are in fact constructors that
take a at least a token type as argument and return closures. Note that
because some of these constructors pre-allocate buffers, using the returned
state functions concurrently is not safe. See the examples for correct usage.

## Installation

```bash
go get -u github.com/db47h/lex/state
```

Then just import the package in your custom lexer:

```go
import "github.com/db47h/lex/state"
```

## License

Package state is released under the terms of the MIT license:

> Copyright 2017-2018 Denis Bernard <db047h@gmail.com>
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