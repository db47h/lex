# state

## Overview

Package state provides state functions for lexing quoted strings,
quoted characters and numbers (integers in any base as well as floats) and
graceful handling of EOF.

According to the convention on Lexer.StateFn, all state functions expect that
the first character that is part of the lexed entity has already been read by
Lexer.Next() and will be retrieved by the state function via Lexer.Last.

All functions (with the exception of EOF) are in fact constructors that
take a at least a token type as argument and return closures. Note that
because some of these constructors pre-allocate buffers, using the returned
state functions concurently is not safe. See the examples for correct usage.

## Installation

```bash
go get -u github.com/db47h/parsekit/lexer/state
```

Then just import the package in your cusrom lexer:

```go
import "github.com/db47h/parsekit/lexer/state"
```

## License

Package state is released under the terms of the MIT license:

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