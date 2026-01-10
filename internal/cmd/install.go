package cmd

import (
	"bufio"
	"fmt"
	"os"
	"path/filepath"
	"strings"

	"github.com/Jeffail/gabs/v2"
	"github.com/spf13/cobra"
)

const ktMdContent = `## kt - ticket tracker

Tickets in ` + "`.ktickets/`" + ` (markdown+YAML). Hierarchy: epic>task>subtask.

` + "```sh" + `
kt create "title" -d "desc" --parent <id>  # -t bug|feature|task|epic|chore -p 0-4
kt ls [--status=open] [--parent=<id>]      # or: kt ready, kt blocked
kt show <id>                               # partial ID ok: a1b2 → kt-a1b2c3d4
kt start|pass|close <id>                   # workflow transitions
kt add-note <id> "text"
kt dep add|rm|tree <id> [dep-id]
kt link add|rm <id> <id>
` + "```" + `

Create flags: ` + "`--design --acceptance --tests --external-ref`" + `
Output: JSON when piped/--json.
`

var installCmd = &cobra.Command{
	Use:   "install",
	Short: "Create kt.md instructions file in current directory",
	Long:  "Creates a kt.md file containing usage instructions for AI agents",
	RunE: func(cmd *cobra.Command, args []string) error {
		cwd, err := os.Getwd()
		if err != nil {
			return fmt.Errorf("get working directory: %w", err)
		}

		reader := bufio.NewReader(os.Stdin)
		path := filepath.Join(cwd, "kt.md")

		// Check if file exists
		if _, err := os.Stat(path); err == nil {
			if !promptYesNo(reader, "kt.md already exists. Regenerate?") {
				fmt.Println("Aborted")
				return nil
			}
		}

		if err := os.WriteFile(path, []byte(ktMdContent), 0644); err != nil {
			return fmt.Errorf("write kt.md: %w", err)
		}

		fmt.Println("Created kt.md")

		// Ask to register kt permission in Claude settings
		if promptYesNo(reader, "Add kt permission to .claude/settings.local.json? (allows Claude to run kt commands without prompting)") {
			if err := registerKtPermission(); err != nil {
				fmt.Fprintf(os.Stderr, "warning: %v\n", err)
			}
		}

		return nil
	},
}

func init() {
	rootCmd.AddCommand(installCmd)
}

// promptYesNo asks a yes/no question and returns true if user answers yes.
func promptYesNo(reader *bufio.Reader, prompt string) bool {
	fmt.Print(prompt + " [y/N] ")
	answer, _ := reader.ReadString('\n')
	answer = strings.TrimSpace(strings.ToLower(answer))
	return answer == "y" || answer == "yes"
}

// registerKtPermission adds "Bash(kt:*)" to .claude/settings.local.json if not present.
func registerKtPermission() error {
	return registerKtPermissionAt(".claude/settings.local.json")
}

// registerKtPermissionAt adds "Bash(kt:*)" to the specified settings file if not present.
func registerKtPermissionAt(settingsPath string) error {
	const permission = "Bash(kt:*)"

	var settings *gabs.Container
	data, err := os.ReadFile(settingsPath)
	if err != nil {
		if os.IsNotExist(err) {
			// Create new settings with permission
			settings = gabs.New()
			if _, err := settings.SetP([]string{permission}, "permissions.allow"); err != nil {
				return fmt.Errorf("set permission: %w", err)
			}
		} else {
			return fmt.Errorf("read settings: %w", err)
		}
	} else {
		settings, err = gabs.ParseJSON(data)
		if err != nil {
			return fmt.Errorf("parse settings: %w", err)
		}

		// Check if already registered
		if allow := settings.Path("permissions.allow"); allow != nil {
			for _, p := range allow.Children() {
				if p.Data().(string) == permission {
					return nil // Already exists
				}
			}
			// Append to existing array
			if err := settings.ArrayAppendP(permission, "permissions.allow"); err != nil {
				return fmt.Errorf("append permission: %w", err)
			}
		} else {
			// Create permissions.allow with our permission
			if _, err := settings.SetP([]string{permission}, "permissions.allow"); err != nil {
				return fmt.Errorf("set permission: %w", err)
			}
		}
	}

	// Ensure .claude directory exists
	if dir := filepath.Dir(settingsPath); dir != "." {
		if err := os.MkdirAll(dir, 0755); err != nil {
			return fmt.Errorf("create directory: %w", err)
		}
	}

	if err := os.WriteFile(settingsPath, settings.BytesIndent("", "  "), 0644); err != nil {
		return fmt.Errorf("write settings: %w", err)
	}

	fmt.Println("Registered kt in .claude/settings.local.json")
	return nil
}
