package main

import (
	"fmt"
	"os"
	"path/filepath"
	"regexp"
	"sort"
	"strings"
	"time"
)

// ObsidianVault represents an Obsidian vault
type ObsidianVault struct {
	Path string
}

// NoteInfo contains information about a note
type NoteInfo struct {
	Path     string    `json:"path"`
	Title    string    `json:"title"`
	Size     int64     `json:"size,omitempty"`
	Modified time.Time `json:"modified,omitempty"`
	Preview  string    `json:"preview,omitempty"`
	Content  string    `json:"content,omitempty"`
}

// NewObsidianVault creates a new Obsidian vault interface
func NewObsidianVault(path string) (*ObsidianVault, error) {
	// Expand home directory
	if strings.HasPrefix(path, "~/") {
		home, err := os.UserHomeDir()
		if err != nil {
			return nil, err
		}
		path = filepath.Join(home, path[2:])
	}

	// Check if path exists
	if _, err := os.Stat(path); os.IsNotExist(err) {
		return nil, fmt.Errorf("vault path does not exist: %s", path)
	}

	return &ObsidianVault{Path: path}, nil
}

// SearchNotes searches for notes containing query
func (v *ObsidianVault) SearchNotes(query string, caseSensitive bool) ([]NoteInfo, error) {
	var results []NoteInfo
	flags := 0
	if !caseSensitive {
		flags = regexp.FlagCaseInsensitive
	}

	pattern, err := regexp.Compile(fmt.Sprintf("(?%s)%s", getRegexpFlags(flags), regexp.QuoteMeta(query)))
	if err != nil {
		return nil, err
	}

	err = filepath.Walk(v.Path, func(path string, info os.FileInfo, err error) error {
		if err != nil {
			return nil // Skip errors
		}

		if !info.IsDir() && strings.HasSuffix(path, ".md") {
			content, err := os.ReadFile(path)
			if err != nil {
				return nil // Skip files we can't read
			}

			if pattern.Match(content) {
				relPath, _ := filepath.Rel(v.Path, path)
				lines := strings.Split(string(content), "\n")
				preview := ""

				for _, line := range lines {
					if pattern.MatchString(line) {
						if preview != "" {
							preview += "\n"
						}
						preview += line
						if len(preview) > 200 {
							break
						}
					}
				}

				if len(preview) > 200 {
					preview = preview[:200] + "..."
				}

				results = append(results, NoteInfo{
					Path:    relPath,
					Title:   strings.TrimSuffix(info.Name(), ".md"),
					Preview: preview,
				})
			}
		}
		return nil
	})

	return results, err
}

// ReadNote reads a complete note
func (v *ObsidianVault) ReadNote(notePath string) (*NoteInfo, error) {
	fullPath := filepath.Join(v.Path, notePath)

	content, err := os.ReadFile(fullPath)
	if err != nil {
		return nil, fmt.Errorf("note not found: %s", notePath)
	}

	info, err := os.Stat(fullPath)
	if err != nil {
		return nil, err
	}

	return &NoteInfo{
		Path:     notePath,
		Title:    strings.TrimSuffix(filepath.Base(notePath), ".md"),
		Content:  string(content),
		Size:     info.Size(),
		Modified: info.ModTime(),
	}, nil
}

// CreateNote creates a new note
func (v *ObsidianVault) CreateNote(title, content, folder string, tags []string) (string, error) {
	// Sanitize filename
	filename := sanitizeFilename(title)
	if !strings.HasSuffix(filename, ".md") {
		filename += ".md"
	}

	targetDir := v.Path
	if folder != "" {
		targetDir = filepath.Join(v.Path, folder)
		if err := os.MkdirAll(targetDir, 0755); err != nil {
			return "", err
		}
	}

	filePath := filepath.Join(targetDir, filename)

	// Build frontmatter
	var fullContent strings.Builder
	fullContent.WriteString("---\n")
	fullContent.WriteString(fmt.Sprintf("created: %s\n", time.Now().Format(time.RFC3339)))
	if len(tags) > 0 {
		fullContent.WriteString("tags:\n")
		for _, tag := range tags {
			fullContent.WriteString(fmt.Sprintf("  - %s\n", tag))
		}
	}
	fullContent.WriteString("---\n\n")
	fullContent.WriteString(content)

	if err := os.WriteFile(filePath, []byte(fullContent.String()), 0644); err != nil {
		return "", err
	}

	relPath, _ := filepath.Rel(v.Path, filePath)
	return relPath, nil
}

// UpdateNote updates an existing note
func (v *ObsidianVault) UpdateNote(notePath, content string, append bool) error {
	fullPath := filepath.Join(v.Path, notePath)

	if append {
		existing, err := os.ReadFile(fullPath)
		if err != nil {
			return fmt.Errorf("note not found: %s", notePath)
		}
		content = string(existing) + "\n\n" + content
	}

	return os.WriteFile(fullPath, []byte(content), 0644)
}

// ListNotes lists all notes in the vault or a folder
func (v *ObsidianVault) ListNotes(folder string) ([]NoteInfo, error) {
	searchPath := v.Path
	if folder != "" {
		searchPath = filepath.Join(v.Path, folder)
	}

	var notes []NoteInfo

	err := filepath.Walk(searchPath, func(path string, info os.FileInfo, err error) error {
		if err != nil {
			return nil
		}

		if !info.IsDir() && strings.HasSuffix(path, ".md") {
			relPath, _ := filepath.Rel(v.Path, path)
			notes = append(notes, NoteInfo{
				Path:     relPath,
				Title:    strings.TrimSuffix(info.Name(), ".md"),
				Size:     info.Size(),
				Modified: info.ModTime(),
			})
		}
		return nil
	})

	// Sort by modified time, newest first
	sort.Slice(notes, func(i, j int) bool {
		return notes[i].Modified.After(notes[j].Modified)
	})

	return notes, err
}

// GetBacklinks finds all notes that link to the specified note
func (v *ObsidianVault) GetBacklinks(notePath string) ([]NoteInfo, error) {
	noteName := strings.TrimSuffix(filepath.Base(notePath), ".md")
	var backlinks []NoteInfo

	// Compile patterns
	patterns := []*regexp.Regexp{
		regexp.MustCompile(fmt.Sprintf(`\[\[%s\]\]`, regexp.QuoteMeta(noteName))),
		regexp.MustCompile(fmt.Sprintf(`\[\[%s\|.*?\]\]`, regexp.QuoteMeta(noteName))),
		regexp.MustCompile(fmt.Sprintf(`\[.*?\]\(%s\)`, regexp.QuoteMeta(notePath))),
	}

	err := filepath.Walk(v.Path, func(path string, info os.FileInfo, err error) error {
		if err != nil {
			return nil
		}

		if !info.IsDir() && strings.HasSuffix(path, ".md") {
			// Skip the note itself
			if strings.TrimSuffix(info.Name(), ".md") == noteName {
				return nil
			}

			content, err := os.ReadFile(path)
			if err != nil {
				return nil
			}

			for _, pattern := range patterns {
				if pattern.Match(content) {
					relPath, _ := filepath.Rel(v.Path, path)

					// Find context
					lines := strings.Split(string(content), "\n")
					context := ""
					for _, line := range lines {
						if pattern.MatchString(line) {
							context = line
							if len(context) > 200 {
								context = context[:200]
							}
							break
						}
					}

					backlinks = append(backlinks, NoteInfo{
						Path:    relPath,
						Title:   strings.TrimSuffix(info.Name(), ".md"),
						Preview: context,
					})
					break
				}
			}
		}
		return nil
	})

	return backlinks, err
}

// GetTags returns all tags used in the vault
func (v *ObsidianVault) GetTags() (map[string]int, error) {
	tags := make(map[string]int)
	hashtagPattern := regexp.MustCompile(`#([\w/\-]+)`)

	err := filepath.Walk(v.Path, func(path string, info os.FileInfo, err error) error {
		if err != nil {
			return nil
		}

		if !info.IsDir() && strings.HasSuffix(path, ".md") {
			content, err := os.ReadFile(path)
			if err != nil {
				return nil
			}

			// Find hashtags
			matches := hashtagPattern.FindAllStringSubmatch(string(content), -1)
			for _, match := range matches {
				if len(match) > 1 {
					tags[match[1]]++
				}
			}
		}
		return nil
	})

	return tags, err
}

// Helper functions

func getRegexpFlags(flags int) string {
	if flags == regexp.FlagCaseInsensitive {
		return "i"
	}
	return ""
}

func sanitizeFilename(name string) string {
	// Remove invalid characters
	re := regexp.MustCompile(`[<>:"/\\|?*]`)
	return re.ReplaceAllString(name, "")
}

// RegisterObsidianTools registers Obsidian tools with the tool registry
func RegisterObsidianTools(registry *ToolRegistry, vault *ObsidianVault) {
	// Search notes
	registry.Register(Tool{
		Name:        "search_obsidian_notes",
		Description: "Search for notes in the Obsidian vault containing specific text",
		Parameters: map[string]interface{}{
			"type": "object",
			"properties": map[string]interface{}{
				"query": map[string]interface{}{
					"type":        "string",
					"description": "The search term to look for in notes",
				},
				"case_sensitive": map[string]interface{}{
					"type":        "boolean",
					"description": "Whether the search should be case sensitive",
					"default":     false,
				},
			},
			"required": []string{"query"},
		},
		Function: func(args map[string]interface{}) (interface{}, error) {
			query := args["query"].(string)
			caseSensitive := false
			if cs, ok := args["case_sensitive"].(bool); ok {
				caseSensitive = cs
			}
			return vault.SearchNotes(query, caseSensitive)
		},
	})

	// Read note
	registry.Register(Tool{
		Name:        "read_obsidian_note",
		Description: "Read the complete contents of a specific note",
		Parameters: map[string]interface{}{
			"type": "object",
			"properties": map[string]interface{}{
				"note_path": map[string]interface{}{
					"type":        "string",
					"description": "Path to the note relative to vault root",
				},
			},
			"required": []string{"note_path"},
		},
		Function: func(args map[string]interface{}) (interface{}, error) {
			notePath := args["note_path"].(string)
			return vault.ReadNote(notePath)
		},
	})

	// Create note
	registry.Register(Tool{
		Name:        "create_obsidian_note",
		Description: "Create a new note in the Obsidian vault",
		Parameters: map[string]interface{}{
			"type": "object",
			"properties": map[string]interface{}{
				"title": map[string]interface{}{
					"type":        "string",
					"description": "Title of the note",
				},
				"content": map[string]interface{}{
					"type":        "string",
					"description": "Content of the note in Markdown format",
				},
				"folder": map[string]interface{}{
					"type":        "string",
					"description": "Subfolder within vault (optional)",
					"default":     "",
				},
				"tags": map[string]interface{}{
					"type":        "array",
					"description": "List of tags to add to the note",
					"items": map[string]interface{}{
						"type": "string",
					},
					"default": []string{},
				},
			},
			"required": []string{"title", "content"},
		},
		Function: func(args map[string]interface{}) (interface{}, error) {
			title := args["title"].(string)
			content := args["content"].(string)
			folder := ""
			if f, ok := args["folder"].(string); ok {
				folder = f
			}
			var tags []string
			if t, ok := args["tags"].([]interface{}); ok {
				for _, tag := range t {
					if tagStr, ok := tag.(string); ok {
						tags = append(tags, tagStr)
					}
				}
			}
			return vault.CreateNote(title, content, folder, tags)
		},
	})

	// List notes
	registry.Register(Tool{
		Name:        "list_obsidian_notes",
		Description: "List all notes in the vault or a specific folder",
		Parameters: map[string]interface{}{
			"type": "object",
			"properties": map[string]interface{}{
				"folder": map[string]interface{}{
					"type":        "string",
					"description": "Subfolder to list (optional)",
					"default":     "",
				},
			},
		},
		Function: func(args map[string]interface{}) (interface{}, error) {
			folder := ""
			if f, ok := args["folder"].(string); ok {
				folder = f
			}
			return vault.ListNotes(folder)
		},
	})

	// Get backlinks
	registry.Register(Tool{
		Name:        "get_obsidian_backlinks",
		Description: "Find all notes that link to a specific note",
		Parameters: map[string]interface{}{
			"type": "object",
			"properties": map[string]interface{}{
				"note_path": map[string]interface{}{
					"type":        "string",
					"description": "Path to the note to find backlinks for",
				},
			},
			"required": []string{"note_path"},
		},
		Function: func(args map[string]interface{}) (interface{}, error) {
			notePath := args["note_path"].(string)
			return vault.GetBacklinks(notePath)
		},
	})

	// Get tags
	registry.Register(Tool{
		Name:        "get_obsidian_tags",
		Description: "Get all tags used in the vault with their frequencies",
		Parameters: map[string]interface{}{
			"type":       "object",
			"properties": map[string]interface{}{},
		},
		Function: func(args map[string]interface{}) (interface{}, error) {
			return vault.GetTags()
		},
	})
}
