# levenshtein

[![Go Report Card](https://goreportcard.com/badge/github.com/TomTonic/levenshtein)](https://goreportcard.com/report/github.com/TomTonic/levenshtein)
[![Go Reference](https://pkg.go.dev/badge/github.com/TomTonic/levenshtein.svg)](https://pkg.go.dev/github.com/TomTonic/levenshtein)
[![Linter](https://github.com/TomTonic/levenshtein/actions/workflows/linter.yml/badge.svg)](https://github.com/TomTonic/levenshtein/actions/workflows/linter.yml)
[![Tests](https://github.com/TomTonic/levenshtein/actions/workflows/coverage.yml/badge.svg?branch=main)](https://github.com/TomTonic/levenshtein/actions/workflows/coverage.yml)
![coverage](https://raw.githubusercontent.com/TomTonic/levenshtein/badges/.badges/main/coverage.svg)

Small, idiomatic Go implementation of the Levenshtein edit distance algorithm.
This package provides utilities to compute distances, ratios, edit matrices and
edit scripts for sequences of runes (Unicode code points).

It is based on the original texttheater/golang-levenshtein implementation and
provides a compact API that is convenient for Go projects that need fuzzy
string comparisons, similarity scoring, or to produce edit scripts.

## Installation

Use Go modules (recommended):

        go get github.com/TomTonic/levenshtein

Then import:

        import "github.com/TomTonic/levenshtein"

## Quick API overview

Key functions and types you'll use:

- `DistanceForStrings(source []rune, target []rune, op Options) int` —
    computes the Levenshtein distance between two rune slices.
- `MatrixForStrings(source []rune, target []rune, op Options) [][]int` —
    returns the full DP matrix (useful for backtraces or visualization).
- `EditScriptForStrings(source []rune, target []rune, op Options) EditScript` —
    returns an optimal edit script (sequence of ops: Ins, Del, Sub, Match).
- `RatioForStrings(source []rune, target []rune, op Options) float64` —
    returns a normalized similarity ratio.
- `DefaultOptions` and `DefaultOptionsWithSub` — convenience `Options` values.

## Why []rune? (the important hint)

Go strings are byte sequences and indexing a string yields a byte. For
Unicode-aware string comparison you should convert to `[]rune` (which are
Unicode code points) before calling this package. That ensures multi-byte
characters (e.g. emoji, accented letters) are treated as single elements.

Example: `source := "café"` and `target := "cafe"`, if you call the library
with `[]rune(source)` and `[]rune(target)` you will get correct behavior
for Unicode text.

## Usage examples

### Simple distance:

```go
package main

import (
        "fmt"
        "github.com/TomTonic/levenshtein"
)

func main() {
        a := "kitten"
        b := "sitting"
        d := levenshtein.DistanceForStrings([]rune(a), []rune(b), levenshtein.DefaultOptions)
        fmt.Println(d) // prints 5 with DefaultOptions (no substitution)
}
```

### Using substitution (counts substitution as 1 instead of 2):

```go
d := levenshtein.DistanceForStrings([]rune("kitten"), []rune("sitting"), levenshtein.DefaultOptionsWithSub)
// d == 3
```

### Get the edit script:

```go
script := levenshtein.EditScriptForStrings([]rune("a"), []rune("aa"), levenshtein.DefaultOptions)
// script will contain EditOperation values such as Match, Ins, Del, Sub
```

### Visualize the matrix:

```go
matrix := levenshtein.MatrixForStrings([]rune("neighbor"), []rune("Neighbour"), levenshtein.DefaultOptions)
levenshtein.WriteMatrix([]rune("neighbor"), []rune("Neighbour"), matrix, os.Stdout)
```

## Options and customization

The `Options` struct lets you tweak insertion, deletion and substitution
costs and also provide a custom `Matches` function. This is handy for
case-insensitive comparisons or language-specific matching.

Example (case-insensitive matching):

```go
import "unicode"

caseInsensitive := levenshtein.Options{
        InsCost: 1,
        DelCost: 1,
        SubCost: 1,
        Matches: func(a, b rune) bool {
                return unicode.ToLower(a) == unicode.ToLower(b)
        },
}

dist := levenshtein.DistanceForStrings([]rune("Apple"), []rune("apple"), caseInsensitive)
// dist == 0
```

## Quality Management

### Automatic Quality Checks

This repository aims to be small, well-tested and CI-driven. Practices for maintaining quality:

- Tests: keep the existing unit tests in `levenshtein_test.go`. Run them with
    `go test ./...` and check coverage with `go test ./... -cover`.
- Linting: run `go vet` and a linter such as `golangci-lint` to catch
    suspicious code patterns. Example:

        golangci-lint run --enable=govet --enable=staticcheck

- Formatting: enforce `gofmt` (or `go fmt`) before commits.

### Release management

For small Go libraries the standard approach is Semantic Versioning using git
tags and GitHub Releases.

### Problem Reports

Contributors are encouraged to include a one-line summary in the PRs header and steps-to-reproduce
(test cases) in the body.

### Contributing

Contributions are welcome. Please use GitHub functionality to propose changes.

## License

This project is released under the terms found in the `LICENSE` file.

## Acknowledgements

This implementation is adapted from the original texttheater/golang-levenshtein
project.

## Contact / Support

If you find issues, please open an issue or a PR on GitHub. For general help
or usage questions, include the inputs you used and expected output.

