# formbind

A standalone library for binding nested form data to Go structs. This library provides advanced form binding capabilities with support for nested structures, arrays, and pointer fields.

## Features

- **Nested Struct Binding**: Bind deeply nested structures from form data
- **Array Support**: Handle array notation with indices (e.g., `items[0].name`)
- **Pointer Field Support**: Automatically handle pointer fields and nil checks
- **Dot Notation**: Support for nested field access using dot notation
- **Sparse Arrays**: Handle non-sequential array indices gracefully
- **Zero Dependencies**: No external dependencies beyond Go standard library

## Installation

```bash
go get github.com/labstack/echo-contrib/formbind
```

## Usage

```go
package main

import (
    "fmt"
    "net/url"
    
    "github.com/labstack/echo-contrib/formbind"
)

type Person struct {
    Name  string `form:"name"`
    Email string `form:"email"`
}

type Team struct {
    Name    string   `form:"name"`
    Members []Person `form:"members"`
}

func main() {
    // Parse form data
    formData := url.Values{
        "name":               {"Engineering Team"},
        "members[0].name":    {"Alice"},
        "members[0].email":   {"alice@example.com"},
        "members[1].name":    {"Bob"},
        "members[1].email":   {"bob@example.com"},
    }
    
    var team Team
    err := formbind.Bind(&team, formData)
    if err != nil {
        panic(err)
    }
    
    fmt.Printf("%+v\n", team)
    // Output: {Name:Engineering Team Members:[{Name:Alice Email:alice@example.com} {Name:Bob Email:bob@example.com}]}
}
```

## API Reference

### formbind.Bind

```go
func Bind(dst interface{}, data url.Values) error
```

Binds form data to the destination struct. The destination must be a pointer to a struct.

### Supported Field Types

- `string`
- `int`, `int8`, `int16`, `int32`, `int64`
- `uint`, `uint8`, `uint16`, `uint32`, `uint64`
- `float32`, `float64`
- `bool`
- `time.Time` (with custom time format support)
- Slices of above types
- Nested structs
- Pointers to any of the above types

### Form Tags

The library uses the `form` tag to map form fields to struct fields:

```go
type User struct {
    Name  string `form:"name"`
    Email string `form:"email_address"`
}
```

If no `form` tag is provided, the field name is used (case-insensitive matching).

## Error Handling

The library returns descriptive errors for common issues:

```go
var data FormData
err := formbind.Bind(&data, formValues)
if err != nil {
    // Handle specific error types
    switch err.(type) {
    case *formbind.BindError:
        // Field-specific binding error
    case *formbind.ParseError:
        // Value parsing error
    default:
        // Other errors
    }
}
```

## Origin

This library is based on the nested form binding implementation proposed in [Echo PR #2834](https://github.com/labstack/echo/pull/2834), extracted as a standalone library for broader use.