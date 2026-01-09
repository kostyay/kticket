package cmd

import (
	"encoding/json"
	"fmt"
	"os"

	"github.com/kostyay/kticket/internal/store"
	"github.com/spf13/cobra"
	"golang.org/x/term"
)

var (
	jsonFlag bool
	Store    *store.Store
)

// OutputMode returns "json" or "text" based on flags and TTY detection.
func OutputMode() string {
	if jsonFlag {
		return "json"
	}
	if !term.IsTerminal(int(os.Stdout.Fd())) {
		return "json" // Piped → JSON
	}
	return "text"
}

// IsJSON returns true if output should be JSON.
func IsJSON() bool {
	return OutputMode() == "json"
}

// PrintJSON marshals v to JSON and prints it.
func PrintJSON(v any) error {
	enc := json.NewEncoder(os.Stdout)
	enc.SetIndent("", "  ")
	return enc.Encode(v)
}

// Errorf prints an error message to stderr.
func Errorf(format string, args ...any) {
	fmt.Fprintf(os.Stderr, "error: "+format+"\n", args...)
}

var rootCmd = &cobra.Command{
	Use:   "kt",
	Short: "Git-backed issue tracker",
	Long:  `kt stores tickets as markdown files with YAML frontmatter in .tickets/`,
	PersistentPreRun: func(cmd *cobra.Command, args []string) {
		Store = store.New("")
	},
}

// Execute runs the root command.
func Execute() {
	if err := rootCmd.Execute(); err != nil {
		os.Exit(1)
	}
}

func init() {
	rootCmd.PersistentFlags().BoolVar(&jsonFlag, "json", false, "Output JSON format")
}
