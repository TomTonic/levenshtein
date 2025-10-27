// Package levenshtein implements the Levenshtein edit distance algorithm for
// sequences of runes (Unicode code points). The package focuses on clarity
// and small API surface: it provides functions to compute an edit distance,
// the underlying dynamic programming matrix, a normalized similarity ratio,
// and an edit script (sequence of operations) to transform one sequence into
// another.
//
// Important: callers should convert Go strings to []rune before calling the
// public functions in this package. Go strings are byte sequences and indexing
// them yields bytes; converting to []rune ensures Unicode code points are
// compared as single elements (see README for examples).
//
// The main functions are:
//   - MatrixForStrings: build the full DP matrix (useful for backtraces).
//   - DistanceForStrings: compute the edit distance using an optimized
//     two-row approach (lower memory).
//   - EditScriptForStrings: backtrace an optimal edit script given the input
//     sequences and options.
//
// The Options struct lets you customize insertion, deletion and substitution
// costs and supply a custom Match function (for case-insensitive comparisons
// or language-specific matching).
package levenshtein

import (
	"fmt"
	"io"
	"os"
)

// EditOperation describes a single edit operation in an edit script produced
// by EditScriptForStrings or EditScriptForMatrix.
//
// Possible values:
//   - Ins: insertion (element present in target but not source)
//   - Del: deletion (element present in source but not target)
//   - Sub: substitution (source element replaced by target element)
//   - Match: elements match (no cost if Matches returns true)
type EditOperation int

const (
	Ins = iota
	Del
	Sub
	Match
)

// EditScript is a sequence of EditOperation values describing an edit
// procedure to transform a source sequence into a target sequence. The
// operations are ordered from left-to-right (source/target prefixes).
type EditScript []EditOperation

// MatchFunction is used to determine whether two runes are considered a
// match. The function should be symmetric and must not have side-effects.
// It must return true if the two runes are considered equal for matching
// purposes (for example, case-insensitive matching can be implemented by
// comparing unicode.ToLower(a) == unicode.ToLower(b)).
type MatchFunction func(rune, rune) bool

// IdenticalRunes is the default MatchFunction: it checks whether two runes
// are identical (==). Use this for strict, case-sensitive Unicode-aware
// comparisons.
func IdenticalRunes(a rune, b rune) bool {
	return a == b
}

// Options controls the costs associated with edit operations and the matching
// function used to decide whether two runes count as equal.
//
// Fields:
//   - InsCost: cost of inserting a rune from the target into the source.
//   - DelCost: cost of deleting a rune from the source.
//   - SubCost: cost of substituting (replacing) one rune with another. If
//     SubCost is greater than InsCost + DelCost the algorithm will
//     prefer a delete+insert instead of a substitution.
//   - Matches: a MatchFunction that returns true when two runes are
//     considered equal (no substitution cost).
//
// Contracts / boundaries: costs are treated as ints and the algorithm assumes
// non-negative costs. Negative costs will produce undefined or nonsensical
// results and should be avoided. There is no explicit overflow checking on
// very large costs; prefer small positive integers (1, 2, ...).
type Options struct {
	InsCost int
	DelCost int
	SubCost int
	Matches MatchFunction
}

// DefaultOptions is a sensible default Options value where substitution is
// effectively disabled (SubCost == 2 while InsCost + DelCost == 2) so that
// a substitution is implemented as delete+insert. Use this when you prefer
// substitutions to be treated as two operations.
var DefaultOptions Options = Options{
	InsCost: 1,
	DelCost: 1,
	SubCost: 2,
	Matches: IdenticalRunes,
}

// DefaultOptionsWithSub is similar to DefaultOptions but gives substitution a
// cost of 1. With these options a substitution is cheaper than delete+insert
// and will often be chosen if two runes differ.
var DefaultOptionsWithSub Options = Options{
	InsCost: 1,
	DelCost: 1,
	SubCost: 1,
	Matches: IdenticalRunes,
}

// String prints a concise human-readable representation for an
// EditOperation. Useful for debugging and tests.
func (operation EditOperation) String() string {
	switch operation {
	case Match:
		return "match"
	case Ins:
		return "ins"
	case Sub:
		return "sub"
	}
	return "del"
}

// DistanceForStrings returns the minimal edit distance between the provided
// source and target rune slices using the costs defined in op.
//
// Parameters:
//   - source: source sequence as []rune. Length >= 0.
//   - target: target sequence as []rune. Length >= 0.
//   - op: Options controlling costs and matching function (see Options
//     documentation). Costs should be non-negative integers.
//
// Return value:
//   - an int representing the minimal total cost to transform source into
//     target using insertions, deletions and (optionally) substitutions.
//
// Complexity: time O(len(source)*len(target)), memory O(len(target)). For
// large inputs consider using MatrixForStrings when you need the full matrix
// (EditScriptForStrings calls MatrixForStrings internally).
//
// Example:
//
//	distance := DistanceForStrings([]rune("kitten"), []rune("sitting"), DefaultOptions)
//
// Notes:
//   - The function expects rune slices rather than raw strings so that Unicode
//     code points are handled correctly.
func DistanceForStrings(source []rune, target []rune, op Options) int {
	// Note: This algorithm is a specialization of MatrixForStrings.
	// MatrixForStrings returns the full edit matrix. However, we only need a
	// single value (see DistanceForMatrix) and the main loop of the algorithm
	// only uses the current and previous row. As such we create a 2D matrix,
	// but with height 2 (enough to store current and previous row).
	height := len(source) + 1
	width := len(target) + 1
	matrix := make([][]int, 2)

	// Initialize trivial distances (from/to empty string). That is, fill
	// the left column and the top row with row/column indices multiplied
	// by deletion/insertion cost.
	for i := range 2 {
		matrix[i] = make([]int, width)
		matrix[i][0] = i * op.DelCost
	}
	for j := 1; j < width; j++ {
		matrix[0][j] = j * op.InsCost
	}

	// Fill in the remaining cells: for each prefix pair, choose the
	// (edit history, operation) pair with the lowest cost.
	for i := 1; i < height; i++ {
		cur := matrix[i%2]
		prev := matrix[(i-1)%2]
		cur[0] = i * op.DelCost
		for j := 1; j < width; j++ {
			delCost := prev[j] + op.DelCost
			matchSubCost := prev[j-1]
			if !op.Matches(source[i-1], target[j-1]) {
				matchSubCost += op.SubCost
			}
			insCost := cur[j-1] + op.InsCost
			cur[j] = min(delCost, min(matchSubCost, insCost))
		}
	}
	return matrix[(height-1)%2][width-1]
}

// DistanceForMatrix extracts the edit distance from a Levenshtein matrix
// produced by MatrixForStrings. The matrix must be a (height x width) table
// where height == len(source)+1 and width == len(target)+1 when built by
// MatrixForStrings.
//
// Return value: bottom-right cell containing the minimal edit cost.
func DistanceForMatrix(matrix [][]int) int {
	return matrix[len(matrix)-1][len(matrix[0])-1]
}

// RatioForStrings returns a normalized similarity score in the range [0,1]
// computed from the Levenshtein distance. The formula used is:
//
//	(sourceLength + targetLength - distance) / (sourceLength + targetLength)
//
// where sourceLength == len(source) and targetLength == len(target).
//
// Notes:
//   - If both source and target are empty (sum == 0) the function returns 0.
//     This mirrors the behavior in RatioForMatrix and avoids division by zero.
//   - Higher values indicate more similar sequences; 1.0 means identical and
//     0.0 means no similarity under the chosen cost model.
func RatioForStrings(source []rune, target []rune, op Options) float64 {
	matrix := MatrixForStrings(source, target, op)
	return RatioForMatrix(matrix)
}

// RatioForMatrix computes the normalized similarity ratio from a DP matrix
// produced by MatrixForStrings. The ratio is in [0,1]. If the combined length
// of source and target is zero the function returns 0 to avoid division by
// zero.
func RatioForMatrix(matrix [][]int) float64 {
	sourcelength := len(matrix) - 1
	targetlength := len(matrix[0]) - 1
	sum := sourcelength + targetlength

	if sum == 0 {
		return 0
	}

	dist := DistanceForMatrix(matrix)
	return float64(sum-dist) / float64(sum)
}

// MatrixForStrings builds and returns the full Levenshtein dynamic
// programming matrix for the given source and target rune slices and
// Options. The returned matrix has dimensions (len(source)+1) x
// (len(target)+1). Rows correspond to prefixes of source and columns to
// prefixes of target; the bottom-right cell contains the minimal edit cost.
//
// Use this function when you need the full matrix for backtracing an edit
// script, visualizing the computation, or computing additional properties.
// If you only need the scalar distance, prefer DistanceForStrings which uses
// O(len(target)) memory.
func MatrixForStrings(source []rune, target []rune, op Options) [][]int {
	// Make a 2-D matrix. Rows correspond to prefixes of source, columns to
	// prefixes of target. Cells will contain edit distances.
	// Cf. http://www.let.rug.nl/~kleiweg/lev/levenshtein.html
	height := len(source) + 1
	width := len(target) + 1
	matrix := make([][]int, height)

	// Initialize trivial distances (from/to empty string). That is, fill
	// the left column and the top row with row/column indices multiplied
	// by deletion/insertion cost.
	for i := range height {
		matrix[i] = make([]int, width)
		matrix[i][0] = i * op.DelCost
	}
	for j := 1; j < width; j++ {
		matrix[0][j] = j * op.InsCost
	}

	// Fill in the remaining cells: for each prefix pair, choose the
	// (edit history, operation) pair with the lowest cost.
	for i := 1; i < height; i++ {
		for j := 1; j < width; j++ {
			delCost := matrix[i-1][j] + op.DelCost
			matchSubCost := matrix[i-1][j-1]
			if !op.Matches(source[i-1], target[j-1]) {
				matchSubCost += op.SubCost
			}
			insCost := matrix[i][j-1] + op.InsCost
			matrix[i][j] = min(delCost, min(matchSubCost,
				insCost))
		}
	}
	//LogMatrix(source, target, matrix)
	return matrix
}

// EditScriptForStrings returns an optimal EditScript (sequence of
// EditOperation values) that transforms source into target using the costs
// defined in op. The function computes the full matrix and then backtraces
// an optimal path.
//
// Example:
//
//	script := EditScriptForStrings([]rune("a"), []rune("aa"), DefaultOptions)
//	// script is something like {Match, Ins}
func EditScriptForStrings(source []rune, target []rune, op Options) EditScript {
	return backtrace(len(source), len(target),
		MatrixForStrings(source, target, op), op)
}

// EditScriptForMatrix computes an optimal edit script using an already
// computed Levenshtein matrix. The matrix must be produced by
// MatrixForStrings and have dimensions (len(source)+1) x (len(target)+1).
func EditScriptForMatrix(matrix [][]int, op Options) EditScript {
	return backtrace(len(matrix)-1, len(matrix[0])-1, matrix, op)
}

// WriteMatrix prints a human-readable representation of the DP matrix to the
// supplied writer. This can be used for debugging and demonstration. The
// matrix must correspond to the provided source and target rune slices (i.e.
// have dimensions (len(source)+1) x (len(target)+1)).
func WriteMatrix(source []rune, target []rune, matrix [][]int, writer io.Writer) {
	_, _ = fmt.Fprintf(writer, "    ")
	for _, targetRune := range target {
		_, _ = fmt.Fprintf(writer, "  %c", targetRune)
	}
	_, _ = fmt.Fprintf(writer, "\n")
	_, _ = fmt.Fprintf(writer, "  %2d", matrix[0][0])
	for j := range target {
		_, _ = fmt.Fprintf(writer, " %2d", matrix[0][j+1])
	}
	_, _ = fmt.Fprintf(writer, "\n")
	for i, sourceRune := range source {
		_, _ = fmt.Fprintf(writer, "%c %2d", sourceRune, matrix[i+1][0])
		for j := range target {
			_, _ = fmt.Fprintf(writer, " %2d", matrix[i+1][j+1])
		}
		_, _ = fmt.Fprintf(writer, "\n")
	}
}

// LogMatrix writes a visual representation of the given matrix to
// os.Stderr. It is a convenience wrapper around WriteMatrix and is
// deprecated in favor of using WriteMatrix with an explicit writer.
func LogMatrix(source []rune, target []rune, matrix [][]int) {
	WriteMatrix(source, target, matrix, os.Stderr)
}

func backtrace(i int, j int, matrix [][]int, op Options) EditScript {
	if i > 0 && matrix[i-1][j]+op.DelCost == matrix[i][j] {
		return append(backtrace(i-1, j, matrix, op), Del)
	}
	if j > 0 && matrix[i][j-1]+op.InsCost == matrix[i][j] {
		return append(backtrace(i, j-1, matrix, op), Ins)
	}
	if i > 0 && j > 0 && matrix[i-1][j-1]+op.SubCost == matrix[i][j] {
		return append(backtrace(i-1, j-1, matrix, op), Sub)
	}
	if i > 0 && j > 0 && matrix[i-1][j-1] == matrix[i][j] {
		return append(backtrace(i-1, j-1, matrix, op), Match)
	}
	return []EditOperation{}
}

func min(a int, b int) int {
	if b < a {
		return b
	}
	return a
}
