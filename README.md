# Echo Community Contribution middlewares

 [![GoDoc](http://img.shields.io/badge/go-documentation-blue.svg?style=flat-square)](http://godoc.org/github.com/labstack/echo-contrib)
 [![Codecov](https://img.shields.io/codecov/c/github/labstack/echo-contrib.svg?style=flat-square)](https://codecov.io/gh/labstack/echo-contrib)
 [![Twitter](https://img.shields.io/badge/twitter-@labstack-55acee.svg?style=flat-square)](https://twitter.com/labstack)

* [Official website](https://echo.labstack.com)
* [All middleware docs](https://echo.labstack.com/docs/category/middleware)

## Usage

For Echo `v5` support:
```bash
go get github.com/labstack/echo-contrib/v5
```

## Versioning

This repository does not use semantic versioning. MAJOR version tracks which Echo version should be used. MINOR version
tracks API changes (possibly backwards incompatible, which is a very rare occasion), and a PATCH version is incremented for fixes.

> **Always add at least one integration test in your project.**

Minimal needed Echo versions:

* `v5.x.y` needs Echo `v5.0.0+`, use `go get github.com/labstack/echo-contrib/v5@latest`
* `v0.18.0` needs Echo `v4.15.0+`, use `go get github.com/labstack/echo-contrib@v0`

For `v0.x.y` releases the code is located in `v4` branch.

# Supported Go version

Each major Go release is supported until there are two newer major releases. https://golang.org/doc/devel/release.html#policy


[Echo CORE](https://github.com/labstack/echo) tests with last FOUR major releases (unless there are pressing vulnerabilities)
As this library depends on MANY DIFFERENT libraries which of SOME support only last 2 Go releases we could have situations when
we derive from last four major releases promise.

p.s. you really should use latest versions of Go as there are many vulnerebilites fixed only in supported versions. Please see https://pkg.go.dev/vuln/
