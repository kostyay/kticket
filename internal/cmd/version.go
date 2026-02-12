package cmd

import (
	"fmt"

	"github.com/kostyay/kticket/internal/version"
	"github.com/spf13/cobra"
)

var versionCmd = &cobra.Command{
	Use:   "version",
	Short: "Print kt version",
	Run: func(cmd *cobra.Command, args []string) {
		if IsJSON() {
			_ = PrintJSON(map[string]string{
				"version": version.Version,
				"commit":  version.Commit,
				"date":    version.Date,
			})
			return
		}
		fmt.Printf("kt %s (commit: %s, built: %s)\n", version.Version, version.Commit, version.Date)
	},
}

func init() {
	rootCmd.AddCommand(versionCmd)
}
