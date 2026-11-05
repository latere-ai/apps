// SPDX-FileCopyrightText: 2026 Latere AI
// SPDX-License-Identifier: Apache-2.0

package manifest

import (
	"encoding/json/v2"
	"testing"
)

// A package.json with a repeated key, a different letter case and an invalid
// UTF-8 byte still yields its scripts.
func TestPackageJSONForeignInput(t *testing.T) {
	in := []byte("{\"scripts\":{\"x\":\"1\"},\"Scripts\":{\"build\":\"vite \xff\"}}")
	var pkg packageJSON
	if err := json.Unmarshal(in, &pkg, foreignJSON); err != nil {
		t.Fatalf("Unmarshal: %v", err)
	}
	if _, ok := pkg.Scripts["build"]; !ok {
		t.Fatalf("scripts = %v, want build", pkg.Scripts)
	}
}
