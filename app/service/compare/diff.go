package compare

import (
	"encoding/json"
	"fmt"
	"math"
	"slices"
	"strconv"
)

// defaultTolerance is the relative tolerance of two JSON numbers: the
// aggregator computes some of them in float64.
const defaultTolerance = 1e-12

// maxValueLen caps a value printed in a difference line.
const maxValueLen = 120

const missing = "<missing>"

// Difference is one JSON path whose values differ.
type Difference struct {
	Path       string
	Aggregator string
	Scan       string
}

// Diff compares two decoded JSON values (decoded with UseNumber) structurally.
// Strings are compared exactly, numbers with a relative tolerance: the one in
// tolerances for the object key holding them, else 1e-12. Lists of different
// lengths report their lengths and their first differing index only.
func Diff(agg, scan any, tolerances map[string]float64) []Difference {
	var out []Difference
	walk("$", "", agg, scan, tolerances, &out)

	return out
}

func walk(path, field string, agg, scan any, tolerances map[string]float64, out *[]Difference) {
	switch a := agg.(type) {
	case map[string]any:
		s, ok := scan.(map[string]any)
		if !ok {
			*out = append(*out, newDifference(path, agg, scan))
			return
		}

		for _, key := range unionKeys(a, s) {
			av, aok := a[key]
			sv, sok := s[key]
			keyPath := path + "." + key
			switch {
			case !aok:
				*out = append(*out, Difference{Path: keyPath, Aggregator: missing, Scan: show(sv)})
			case !sok:
				*out = append(*out, Difference{Path: keyPath, Aggregator: show(av), Scan: missing})
			default:
				walk(keyPath, key, av, sv, tolerances, out)
			}
		}
	case []any:
		s, ok := scan.([]any)
		if !ok {
			*out = append(*out, newDifference(path, agg, scan))
			return
		}

		if len(a) == len(s) {
			for i := range a {
				walk(fmt.Sprintf("%s[%d]", path, i), field, a[i], s[i], tolerances, out)
			}

			return
		}

		// lists of different lengths: one missing or extra entry shifts every
		// later index, so only the lengths and the first differing index are
		// reported
		*out = append(*out, Difference{
			Path:       path + ".length",
			Aggregator: strconv.Itoa(len(a)),
			Scan:       strconv.Itoa(len(s)),
		})

		for i := range min(len(a), len(s)) {
			var first []Difference
			walk(fmt.Sprintf("%s[%d]", path, i), field, a[i], s[i], tolerances, &first)
			if len(first) > 0 {
				*out = append(*out, first...)

				return
			}
		}
	case json.Number:
		s, ok := scan.(json.Number)
		if !ok || !numbersEqual(a, s, tolerance(field, tolerances)) {
			*out = append(*out, newDifference(path, agg, scan))
		}
	default:
		// strings, booleans and null: the other side is never a map or a
		// slice of the same type here, so == cannot panic
		if agg != scan {
			*out = append(*out, newDifference(path, agg, scan))
		}
	}
}

func tolerance(field string, tolerances map[string]float64) float64 {
	if t, ok := tolerances[field]; ok {
		return t
	}

	return defaultTolerance
}

func numbersEqual(a, b json.Number, tol float64) bool {
	if a == b {
		return true
	}

	af, err := a.Float64()
	if err != nil {
		return false
	}

	bf, err := b.Float64()
	if err != nil {
		return false
	}

	if af == bf {
		return true
	}

	return math.Abs(af-bf) <= tol*math.Max(math.Abs(af), math.Abs(bf))
}

func unionKeys(a, b map[string]any) []string {
	keys := make([]string, 0, len(a)+len(b))
	for k := range a {
		keys = append(keys, k)
	}

	for k := range b {
		if _, ok := a[k]; !ok {
			keys = append(keys, k)
		}
	}

	slices.Sort(keys)

	return keys
}

func newDifference(path string, agg, scan any) Difference {
	return Difference{Path: path, Aggregator: show(agg), Scan: show(scan)}
}

func show(v any) string {
	b, err := json.Marshal(v)
	if err != nil {
		return fmt.Sprintf("%v", v)
	}

	return clip(string(b))
}

func clip(s string) string {
	if len(s) <= maxValueLen {
		return s
	}

	return s[:maxValueLen] + "…"
}
