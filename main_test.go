///////////////////////////////////////////////////////////////////////////////
// Copyright © 2020 xx network SEZC                                          //
//                                                                           //
// Use of this source code is governed by a license that can be found in the //
// LICENSE file                                                              //
///////////////////////////////////////////////////////////////////////////////

package main

import (
	"os/exec"
	"testing"
)

// Smoke test for version
func TestMainVersion(t *testing.T) {
	command := exec.Command("go", "run", "main.go", "version")
	err := command.Run()
	if e, ok := err.(*exec.ExitError); ok && !e.Success() {
		t.Errorf("Smoke test failed with %v", e)
	}
}
