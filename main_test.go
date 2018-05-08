////////////////////////////////////////////////////////////////////////////////
// Copyright © 2018 Privategrity Corporation                                   /
//                                                                             /
// All rights reserved.                                                        /
////////////////////////////////////////////////////////////////////////////////

package main

import (
	"os/exec"
	"testing"

	"gitlab.com/privategrity/gateway/cmd"
)

// Smoke test for main
func TestMainSmoke(t *testing.T) {
	cmd.RootCmd.SetArgs([]string{"--version"})
	main()
	cmd.RootCmd.SetArgs([]string{"--version", "--config", "sampleconfig.yaml"})
	main()
	cmd := exec.Command("go", "run", "main.go", "--version")
	err := cmd.Run()
	if e, ok := err.(*exec.ExitError); ok && !e.Success() {
		t.Errorf("Smoke test failed with %v", e)
	}
}
