// SPDX-FileCopyrightText: 2026 Latere AI
// SPDX-License-Identifier: Apache-2.0

package manifest

import (
	"math"
	"regexp"
	"strconv"
)

// quantityForm is the Kubernetes quantity grammar: a sign, a decimal
// number, then a binary suffix, a decimal suffix, or a decimal exponent.
// "E" alone is exa and "E3" an exponent, so the exponent form is tried
// after the suffix and the match must end the string.
var quantityForm = regexp.MustCompile(`^([+-]?)(\d+(?:\.\d*)?|\.\d+)(Ki|Mi|Gi|Ti|Pi|Ei|[numkMGTPE]|[eE][+-]?\d+)?$`)

// suffixScale is the one table of quantity suffixes.
var suffixScale = map[string]float64{
	"":   1,
	"Ki": 1 << 10, "Mi": 1 << 20, "Gi": 1 << 30, "Ti": 1 << 40, "Pi": 1 << 50, "Ei": 1 << 60,
	"n": 1e-9, "u": 1e-6, "m": 1e-3, "k": 1e3, "M": 1e6, "G": 1e9, "T": 1e12, "P": 1e15, "E": 1e18,
}

// quantity reads a Kubernetes quantity as a number of base units. It
// refuses a negative value and any form Kubernetes refuses (hex floats,
// digit underscores, Inf, NaN).
func quantity(q string) (float64, bool) {
	m := quantityForm.FindStringSubmatch(q)
	if m == nil {
		return 0, false
	}
	scale, known := suffixScale[m[3]]
	digits := m[2]
	if !known {
		// A decimal exponent: let ParseFloat apply it.
		digits += m[3]
		scale = 1
	}
	f, err := strconv.ParseFloat(digits, 64)
	if err != nil || m[1] == "-" {
		return 0, false
	}
	f *= scale
	if math.IsInf(f, 0) || math.IsNaN(f) {
		return 0, false
	}
	return f, true
}

// memoryBytes reads a Kubernetes memory quantity in bytes: a number with
// an optional binary (Ki to Ei) or decimal (n to E) suffix or exponent.
func memoryBytes(q string) (float64, bool) {
	return quantity(q)
}

// cpuMillis reads a Kubernetes CPU quantity in millicores: "250m" or a
// number of cores such as "0.5" or "2".
func cpuMillis(q string) (float64, bool) {
	f, ok := quantity(q)
	return f * 1000, ok
}
