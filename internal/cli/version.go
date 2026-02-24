package cli

import (
	"fmt"

	"github.com/spf13/cobra"
)

var versionCmd = &cobra.Command{
	Use:   "version",
	Short: "Print the version",
	Run: func(_ *cobra.Command, _ []string) {
		fmt.Printf("lcg version %s\n", Version)
	},
}

func init() {
	rootCmd.AddCommand(versionCmd)
}
