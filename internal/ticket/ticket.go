package ticket

import (
	"bufio"
	"bytes"
	"fmt"
	"os"
	"path/filepath"
	"strings"

	"github.com/goccy/go-yaml"
)

type Status string

const (
	StatusOpen       Status = "open"
	StatusInProgress Status = "in_progress"
	StatusClosed     Status = "closed"
)

type Type string

const (
	TypeBug     Type = "bug"
	TypeFeature Type = "feature"
	TypeTask    Type = "task"
	TypeEpic    Type = "epic"
	TypeChore   Type = "chore"
)

type Ticket struct {
	// Frontmatter fields (YAML)
	ID          string   `yaml:"id" json:"id"`
	Status      Status   `yaml:"status" json:"status"`
	Deps        []string `yaml:"deps,omitempty" json:"deps,omitempty"`
	Links       []string `yaml:"links,omitempty" json:"links,omitempty"`
	Created     string   `yaml:"created" json:"created"`
	Type        Type     `yaml:"type" json:"type"`
	Priority    int      `yaml:"priority" json:"priority"`
	Assignee    string   `yaml:"assignee,omitempty" json:"assignee,omitempty"`
	ExternalRef string   `yaml:"external-ref,omitempty" json:"external_ref,omitempty"`
	Parent      string   `yaml:"parent,omitempty" json:"parent,omitempty"`
	TestsPassed bool     `yaml:"tests_passed" json:"tests_passed"`

	// Parsed from markdown body
	Title              string `yaml:"-" json:"title"`
	Description        string `yaml:"-" json:"description,omitempty"`
	Design             string `yaml:"-" json:"design,omitempty"`
	AcceptanceCriteria string `yaml:"-" json:"acceptance_criteria,omitempty"`
	Tests              string `yaml:"-" json:"tests,omitempty"`
	Notes              string `yaml:"-" json:"notes,omitempty"`
}

// CanClose checks if the ticket can be closed based on test requirements.
func (t *Ticket) CanClose() error {
	if t.Tests != "" && !t.TestsPassed {
		return fmt.Errorf("cannot close %s: tests not passed (run 'kt pass %s' first)", t.ID, t.ID)
	}
	return nil
}

// ParseFile reads a ticket from a markdown file with YAML frontmatter.
func ParseFile(path string) (*Ticket, error) {
	data, err := os.ReadFile(path)
	if err != nil {
		return nil, err
	}
	return Parse(data)
}

// Parse parses a ticket from raw markdown bytes.
func Parse(data []byte) (*Ticket, error) {
	frontmatter, body, err := splitFrontmatter(data)
	if err != nil {
		return nil, err
	}

	t := &Ticket{}
	if err := yaml.Unmarshal(frontmatter, t); err != nil {
		return nil, fmt.Errorf("parse frontmatter: %w", err)
	}

	parseBody(t, body)
	return t, nil
}

// WriteFile writes a ticket to a markdown file.
func WriteFile(path string, t *Ticket) error {
	data, err := Marshal(t)
	if err != nil {
		return err
	}
	return atomicWrite(path, data, 0644)
}

func atomicWrite(path string, data []byte, perm os.FileMode) error {
	dir := filepath.Dir(path)
	tmp, err := os.CreateTemp(dir, ".kt-*.tmp")
	if err != nil {
		return fmt.Errorf("create temp file: %w", err)
	}
	tmpPath := tmp.Name()

	if _, err := tmp.Write(data); err != nil {
		tmp.Close()
		os.Remove(tmpPath)
		return fmt.Errorf("write temp file: %w", err)
	}
	if err := tmp.Close(); err != nil {
		os.Remove(tmpPath)
		return fmt.Errorf("close temp file: %w", err)
	}
	if err := os.Chmod(tmpPath, perm); err != nil {
		os.Remove(tmpPath)
		return fmt.Errorf("chmod temp file: %w", err)
	}
	if err := os.Rename(tmpPath, path); err != nil {
		os.Remove(tmpPath)
		return fmt.Errorf("rename temp file: %w", err)
	}
	return nil
}

// Marshal serializes a ticket to markdown bytes.
func Marshal(t *Ticket) ([]byte, error) {
	var buf bytes.Buffer

	// Write frontmatter
	buf.WriteString("---\n")
	fm, err := yaml.Marshal(t)
	if err != nil {
		return nil, fmt.Errorf("marshal frontmatter: %w", err)
	}
	buf.Write(fm)
	buf.WriteString("---\n")

	// Write title
	buf.WriteString("# ")
	buf.WriteString(t.Title)
	buf.WriteString("\n")

	// Write description
	if t.Description != "" {
		buf.WriteString("\n")
		buf.WriteString(t.Description)
		buf.WriteString("\n")
	}

	// Write sections
	if t.Design != "" {
		buf.WriteString("\n## Design\n\n")
		buf.WriteString(t.Design)
		buf.WriteString("\n")
	}

	if t.AcceptanceCriteria != "" {
		buf.WriteString("\n## Acceptance Criteria\n\n")
		buf.WriteString(t.AcceptanceCriteria)
		buf.WriteString("\n")
	}

	if t.Tests != "" {
		buf.WriteString("\n## Tests\n\n")
		buf.WriteString(t.Tests)
		buf.WriteString("\n")
	}

	if t.Notes != "" {
		buf.WriteString("\n## Notes\n\n")
		buf.WriteString(t.Notes)
		buf.WriteString("\n")
	}

	return buf.Bytes(), nil
}

func splitFrontmatter(data []byte) ([]byte, []byte, error) {
	scanner := bufio.NewScanner(bytes.NewReader(data))

	// Expect first line to be "---"
	if !scanner.Scan() {
		return nil, nil, fmt.Errorf("empty file")
	}
	if strings.TrimSpace(scanner.Text()) != "---" {
		return nil, nil, fmt.Errorf("missing frontmatter delimiter")
	}

	// Read until closing "---"
	var frontmatter bytes.Buffer
	for scanner.Scan() {
		line := scanner.Text()
		if strings.TrimSpace(line) == "---" {
			break
		}
		frontmatter.WriteString(line)
		frontmatter.WriteString("\n")
	}

	// Rest is body
	var body bytes.Buffer
	for scanner.Scan() {
		body.WriteString(scanner.Text())
		body.WriteString("\n")
	}

	return frontmatter.Bytes(), body.Bytes(), scanner.Err()
}

func parseBody(t *Ticket, body []byte) {
	lines := strings.Split(string(body), "\n")

	var currentSection string
	var sectionContent strings.Builder

	flushSection := func() {
		content := strings.TrimSpace(sectionContent.String())
		switch currentSection {
		case "title":
			t.Title = content
		case "description":
			t.Description = content
		case "design":
			t.Design = content
		case "acceptance":
			t.AcceptanceCriteria = content
		case "tests":
			t.Tests = content
		case "notes":
			t.Notes = content
		}
		sectionContent.Reset()
	}

	for _, line := range lines {
		trimmed := strings.TrimSpace(line)

		// Check for section headers
		if strings.HasPrefix(trimmed, "# ") && currentSection == "" {
			// Title line
			flushSection()
			currentSection = "title"
			sectionContent.WriteString(strings.TrimPrefix(trimmed, "# "))
			continue
		}

		if strings.HasPrefix(trimmed, "## ") {
			flushSection()
			header := strings.ToLower(strings.TrimPrefix(trimmed, "## "))
			switch {
			case strings.Contains(header, "design"):
				currentSection = "design"
			case strings.Contains(header, "acceptance"):
				currentSection = "acceptance"
			case strings.Contains(header, "test"):
				currentSection = "tests"
			case strings.Contains(header, "note"):
				currentSection = "notes"
			default:
				currentSection = "description"
			}
			continue
		}

		// After title, before first section is description
		if currentSection == "title" && trimmed == "" {
			flushSection()
			currentSection = "description"
			continue
		}

		sectionContent.WriteString(line)
		sectionContent.WriteString("\n")
	}

	flushSection()
}
