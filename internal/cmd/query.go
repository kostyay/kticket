package cmd

import (
	"encoding/json"
	"os"

	"github.com/spf13/cobra"
)

var queryCmd = &cobra.Command{
	Use:   "query",
	Short: "Output tickets as JSON (for piping to jq)",
	Long:  `Output all tickets as JSON array. Use with jq for filtering, e.g.: kt query | jq '.[] | select(.status == "open")'`,
	RunE:  runQuery,
}

func init() {
	rootCmd.AddCommand(queryCmd)
}

func runQuery(cmd *cobra.Command, args []string) error {
	tickets, err := Store.List()
	if err != nil {
		return err
	}

	enc := json.NewEncoder(os.Stdout)
	enc.SetIndent("", "  ")
	return enc.Encode(tickets)
}
