package cmd

import (
	"bufio"
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"strings"

	"github.com/spf13/cobra"
)

const ktMdContent = `## Task Tracking (kt)

Git-backed issue tracker for AI agents. Stores tickets as markdown files with YAML frontmatter in ` + "`.ktickets/`" + `.

### Quick Reference
- ` + "`kt help`" + ` for full CLI reference
- Structure: epic > task > subtask (keep subtasks atomic)
- Start work: ` + "`kt ready`" + ` → pick top, execute, update status
- Creating: break features into testable chunks

### Common Commands

` + "```sh" + `
# Create tickets
kt create "title" -d "description" --parent <epic-id>
kt create "Feature X" -t feature -p 1

# List/query
kt ls --status=open       # List open tickets
kt ready                  # Open with deps resolved
kt blocked                # Open with unresolved deps
kt show <id>              # Display ticket details

# Workflow
kt start <id>             # Set to in_progress
kt pass <id>              # Mark tests passed
kt close <id>             # Set to closed
kt add-note <id> "text"   # Append timestamped note

# Dependencies
kt dep add <id> <dep-id>  # Add dependency
kt dep tree <id>          # Show dependency tree
kt link add <id> <id>     # Link tickets (symmetric)
` + "```" + `

### Create Options
- ` + "`-t, --type`" + `: bug|feature|task|epic|chore (default: task)
- ` + "`-p, --priority`" + `: 0-4, 0=highest (default: 2)
- ` + "`-d, --description`" + `: Description text
- ` + "`--design`" + `: Design notes
- ` + "`--acceptance`" + `: Acceptance criteria
- ` + "`--tests`" + `: Test requirements (requires ` + "`kt pass`" + ` before close)
- ` + "`--parent`" + `: Parent ticket ID
- ` + "`--external-ref`" + `: External reference (e.g., gh-123)

### Output Modes
- Terminal: human-readable text
- Piped/--json: JSON for scripting

### ID Format
Partial ID matching supported: ` + "`kt show a1b2`" + ` matches ` + "`kt-a1b2c3d4`" + `.
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

		path := filepath.Join(cwd, "kt.md")

		// Check if file exists
		if _, err := os.Stat(path); err == nil {
			fmt.Print("kt.md already exists. Regenerate? [y/N] ")
			reader := bufio.NewReader(os.Stdin)
			answer, _ := reader.ReadString('\n')
			answer = strings.TrimSpace(strings.ToLower(answer))
			if answer != "y" && answer != "yes" {
				fmt.Println("Aborted")
				return nil
			}
		}

		if err := os.WriteFile(path, []byte(ktMdContent), 0644); err != nil {
			return fmt.Errorf("write kt.md: %w", err)
		}

		fmt.Println("Created kt.md")

		// Register kt permission in Claude settings
		if err := registerKtPermission(); err != nil {
			fmt.Fprintf(os.Stderr, "warning: %v\n", err)
		}

		return nil
	},
}

func init() {
	rootCmd.AddCommand(installCmd)
}

// registerKtPermission adds "Bash(kt:*)" to .claude/settings.local.json if not present.
func registerKtPermission() error {
	return registerKtPermissionAt(".claude/settings.local.json")
}

// registerKtPermissionAt adds "Bash(kt:*)" to the specified settings file if not present.
func registerKtPermissionAt(settingsPath string) error {
	data, err := os.ReadFile(settingsPath)
	if err != nil {
		if os.IsNotExist(err) {
			return nil // File doesn't exist - skip
		}
		return fmt.Errorf("read settings: %w", err)
	}

	var settings map[string]any
	if err := json.Unmarshal(data, &settings); err != nil {
		return fmt.Errorf("parse settings: %w", err)
	}

	perms, ok := settings["permissions"].(map[string]any)
	if !ok {
		return nil // No permissions section
	}
	allowRaw, ok := perms["allow"].([]any)
	if !ok {
		return nil // No allow array
	}

	// Check if already registered
	permission := "Bash(kt:*)"
	for _, p := range allowRaw {
		if s, ok := p.(string); ok && s == permission {
			return nil // Already exists
		}
	}

	// Append permission
	perms["allow"] = append(allowRaw, permission)

	out, err := json.MarshalIndent(settings, "", "  ")
	if err != nil {
		return fmt.Errorf("marshal settings: %w", err)
	}

	if err := os.WriteFile(settingsPath, append(out, '\n'), 0644); err != nil {
		return fmt.Errorf("write settings: %w", err)
	}

	fmt.Println("Registered kt in .claude/settings.local.json")
	return nil
}
