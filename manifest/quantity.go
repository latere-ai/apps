// SPDX-FileCopyrightText: 2026 Latere AI
// SPDX-License-Identifier: Apache-2.0

package manifest

import (
	"strconv"
	"strings"
)

// memoryBytes reads a Kubernetes memory quantity: a number with an
// optional binary (Ki, Mi, Gi, Ti) or decimal (k, M, G, T) suffix.
func memoryBytes(q string) (float64, bool) {
	for _, s := range []struct {
		suffix string
		scale  float64
	}{
		{"Ki", 1 << 10}, {"Mi", 1 << 20}, {"Gi", 1 << 30}, {"Ti", 1 << 40},
		{"k", 1e3}, {"M", 1e6}, {"G", 1e9}, {"T", 1e12},
	} {
		if n, ok := strings.CutSuffix(q, s.suffix); ok {
			return number(n, s.scale)
		}
	}
	return number(q, 1)
}

// cpuMillis reads a Kubernetes CPU quantity in millicores: "250m" or a
// number of cores such as "0.5" or "2".
func cpuMillis(q string) (float64, bool) {
	if n, ok := strings.CutSuffix(q, "m"); ok {
		return number(n, 1)
	}
	return number(q, 1000)
}

func number(s string, scale float64) (float64, bool) {
	f, err := strconv.ParseFloat(s, 64)
	if err != nil || f < 0 || s == "" || strings.ContainsAny(s, "eE+") {
		return 0, false
	}
	return f * scale, true
}
