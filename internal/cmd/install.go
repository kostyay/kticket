package cmd

import (
	"bufio"
	"embed"
	"fmt"
	"os"
	"path/filepath"
	"strconv"
	"strings"

	"github.com/Jeffail/gabs/v2"
	"github.com/spf13/cobra"
)

//go:embed templates/*
var templatesFS embed.FS

var installCmd = &cobra.Command{
	Use:   "install",
	Short: "Install kt.md and Claude slash commands",
	Long:  "Creates kt.md file and optionally installs Claude slash commands and permissions",
	RunE: func(cmd *cobra.Command, args []string) error {
		cwd, err := os.Getwd()
		if err != nil {
			return fmt.Errorf("get working directory: %w", err)
		}

		reader := bufio.NewReader(os.Stdin)

		// Install kt.md
		ktMdPath := filepath.Join(cwd, "kt.md")
		if _, err := os.Stat(ktMdPath); err == nil {
			if !promptYesNo(reader, "kt.md already exists. Regenerate?") {
				fmt.Println("Skipped kt.md")
			} else {
				if err := writeKtMd(ktMdPath); err != nil {
					return err
				}
			}
		} else {
			if err := writeKtMd(ktMdPath); err != nil {
				return err
			}
		}

		// Install slash commands
		globalDir := getClaudeConfigDir()
		cmdChoice := promptChoice(reader, "Install slash commands (/kt-create, /kt-run, /kt-run-all)?", []string{
			fmt.Sprintf("Global (%s/commands/)", globalDir),
			"Project (.claude/commands/)",
			"Skip",
		})
		if cmdChoice != 3 {
			global := cmdChoice == 1
			if err := installSlashCommands(global); err != nil {
				fmt.Fprintf(os.Stderr, "warning: %v\n", err)
			}
		}

		// Install kt permission
		permChoice := promptChoice(reader, "Add kt permission (allows Claude to run kt commands without prompting)?", []string{
			fmt.Sprintf("Global (%s/settings.json)", globalDir),
			"Project (.claude/settings.local.json)",
			"Skip",
		})
		if permChoice != 3 {
			global := permChoice == 1
			if err := registerKtPermission(global); err != nil {
				fmt.Fprintf(os.Stderr, "warning: %v\n", err)
			}
		}

		return nil
	},
}

func init() {
	rootCmd.AddCommand(installCmd)
}

// writeKtMd writes kt.md from embedded template.
func writeKtMd(path string) error {
	content, err := templatesFS.ReadFile("templates/kt.md")
	if err != nil {
		return fmt.Errorf("read template: %w", err)
	}
	if err := os.WriteFile(path, content, 0644); err != nil {
		return fmt.Errorf("write kt.md: %w", err)
	}
	fmt.Println("Created kt.md")
	return nil
}

// promptYesNo asks a yes/no question and returns true if user answers yes.
func promptYesNo(reader *bufio.Reader, prompt string) bool {
	fmt.Print(prompt + " [y/N] ")
	answer, _ := reader.ReadString('\n')
	answer = strings.TrimSpace(strings.ToLower(answer))
	return answer == "y" || answer == "yes"
}

// promptChoice presents numbered options and returns 1-indexed selection.
func promptChoice(reader *bufio.Reader, prompt string, options []string) int {
	fmt.Println(prompt)
	for i, opt := range options {
		fmt.Printf("  %d. %s\n", i+1, opt)
	}
	fmt.Print("> ")
	answer, _ := reader.ReadString('\n')
	answer = strings.TrimSpace(answer)
	choice, err := strconv.Atoi(answer)
	if err != nil || choice < 1 || choice > len(options) {
		return len(options) // Default to last option (Skip)
	}
	return choice
}

// getClaudeConfigDir returns the Claude config directory, respecting CLAUDE_CONFIG_DIR env var.
func getClaudeConfigDir() string {
	if dir := os.Getenv("CLAUDE_CONFIG_DIR"); dir != "" {
		return dir
	}
	home, _ := os.UserHomeDir()
	return filepath.Join(home, ".claude")
}

// installSlashCommands installs kt-create.md and kt-run.md commands.
func installSlashCommands(global bool) error {
	var commandsDir string
	if global {
		commandsDir = filepath.Join(getClaudeConfigDir(), "commands")
	} else {
		commandsDir = ".claude/commands"
	}

	if err := os.MkdirAll(commandsDir, 0755); err != nil {
		return fmt.Errorf("create commands directory: %w", err)
	}

	commands := []string{"kt-create.md", "kt-run.md", "kt-run-all.md"}
	for _, cmd := range commands {
		content, err := templatesFS.ReadFile("templates/" + cmd)
		if err != nil {
			return fmt.Errorf("read template %s: %w", cmd, err)
		}
		path := filepath.Join(commandsDir, cmd)
		if err := os.WriteFile(path, content, 0644); err != nil {
			return fmt.Errorf("write %s: %w", cmd, err)
		}
	}

	scope := "project"
	if global {
		scope = "global"
	}
	fmt.Printf("Installed /kt-create, /kt-run, /kt-run-all (%s)\n", scope)
	return nil
}

// registerKtPermission adds "Bash(kt:*)" to Claude settings.
func registerKtPermission(global bool) error {
	var settingsPath string
	if global {
		settingsPath = filepath.Join(getClaudeConfigDir(), "settings.json")
	} else {
		settingsPath = ".claude/settings.local.json"
	}
	return registerKtPermissionAt(settingsPath, global)
}

// registerKtPermissionAt adds "Bash(kt:*)" to the specified settings file if not present.
func registerKtPermissionAt(settingsPath string, global bool) error {
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
					scope := "project"
					if global {
						scope = "global"
					}
					fmt.Printf("kt permission already registered (%s)\n", scope)
					return nil
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

	// Ensure directory exists
	if dir := filepath.Dir(settingsPath); dir != "." {
		if err := os.MkdirAll(dir, 0755); err != nil {
			return fmt.Errorf("create directory: %w", err)
		}
	}

	if err := os.WriteFile(settingsPath, settings.BytesIndent("", "  "), 0644); err != nil {
		return fmt.Errorf("write settings: %w", err)
	}

	scope := "project"
	if global {
		scope = "global"
	}
	fmt.Printf("Registered kt permission (%s)\n", scope)
	return nil
}
