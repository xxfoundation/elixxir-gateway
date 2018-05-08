package cmd

import (
	"fmt"
	"github.com/spf13/cobra"
)

//go:generate go run gen.go
// The above generates: GITVERSION, GLIDEDEPS, and SEMVER

func init() {
	RootCmd.AddCommand(versionCmd)
}

func printVersion() {
	fmt.Printf("Privategrity Gateway v%s -- %s\n\n", SEMVER,
		GITVERSION)
	fmt.Printf("Dependencies:\n\n%s\n", GLIDEDEPS)
}

var versionCmd = &cobra.Command{
	Use:   "version",
	Short: "Print the version number of Privategrity Gateway",
	Long: `Print the version number of Privategrity Gateway. This
also prints the glide cache versions of all of its dependencies.`,
	Run: func(cmd *cobra.Command, args []string) {
		printVersion()
	},
}
