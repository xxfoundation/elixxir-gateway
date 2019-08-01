////////////////////////////////////////////////////////////////////////////////
// Copyright © 2018 Privategrity Corporation                                   /
//                                                                             /
// All rights reserved.                                                        /
////////////////////////////////////////////////////////////////////////////////

package main

import (
	"os/exec"
	"testing"
)

// Smoke test for main
func TestMainSmoke(t *testing.T) {
	command := exec.Command("go", "run", "main.go", "--version")
	err := command.Run()
	if e, ok := err.(*exec.ExitError); ok && !e.Success() {
		t.Errorf("Smoke test failed with %v", e)
	}
}
