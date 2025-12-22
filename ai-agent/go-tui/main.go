package main

import (
	"context"
	"encoding/json"
	"fmt"
	"os"
	"strings"

	tea "github.com/charmbracelet/bubbletea"
	"github.com/charmbracelet/lipgloss"
)

// Styles
var (
	titleStyle = lipgloss.NewStyle().
			Bold(true).
			Foreground(lipgloss.Color("#7D56F4")).
			Padding(0, 1)

	userMessageStyle = lipgloss.NewStyle().
				Background(lipgloss.Color("#7D56F4")).
				Foreground(lipgloss.Color("#FFFFFF")).
				Padding(0, 1).
				MarginRight(20).
				Align(lipgloss.Right)

	assistantMessageStyle = lipgloss.NewStyle().
				Background(lipgloss.Color("#3C3C3C")).
				Foreground(lipgloss.Color("#FFFFFF")).
				Padding(0, 1).
				MarginLeft(2)

	systemMessageStyle = lipgloss.NewStyle().
				Foreground(lipgloss.Color("#888888")).
				Italic(true)

	inputStyle = lipgloss.NewStyle().
			Foreground(lipgloss.Color("#FF79C6"))

	toolCallStyle = lipgloss.NewStyle().
			Foreground(lipgloss.Color("#50FA7B")).
			Italic(true)
)

// Message represents a chat message
type Message struct {
	Role    string
	Content string
}

// Model represents the application state
type model struct {
	messages     []Message
	input        string
	provider     Provider
	tools        *ToolRegistry
	vault        *ObsidianVault
	width        int
	height       int
	cursorPos    int
	providerType string
}

// Initial model
func initialModel(vaultPath string) model {
	vault, err := NewObsidianVault(vaultPath)
	if err != nil {
		fmt.Printf("Warning: Could not load vault: %v\n", err)
		vault = nil
	}

	tools := NewToolRegistry()
	if vault != nil {
		RegisterObsidianTools(tools, vault)
	}

	return model{
		messages:     []Message{{Role: "system", Content: "AI Agent ready. Provider: Not connected"}},
		input:        "",
		provider:     nil,
		tools:        tools,
		vault:        vault,
		providerType: "openai",
	}
}

// Init initializes the application
func (m model) Init() tea.Cmd {
	return nil
}

// Update handles messages
func (m model) Update(msg tea.Msg) (tea.Model, tea.Cmd) {
	switch msg := msg.(type) {
	case tea.WindowSizeMsg:
		m.width = msg.Width
		m.height = msg.Height
		return m, nil

	case tea.KeyMsg:
		switch msg.String() {
		case "ctrl+c", "esc":
			return m, tea.Quit

		case "ctrl+p":
			// Cycle through providers
			switch m.providerType {
			case "openai":
				m.providerType = "anthropic"
			case "anthropic":
				m.providerType = "ollama"
			case "ollama":
				m.providerType = "openai"
			}
			m.messages = append(m.messages, Message{
				Role:    "system",
				Content: fmt.Sprintf("Switched to provider: %s", m.providerType),
			})
			return m, nil

		case "ctrl+n":
			// Connect to provider
			provider, err := CreateProvider(m.providerType)
			if err != nil {
				m.messages = append(m.messages, Message{
					Role:    "system",
					Content: fmt.Sprintf("Error connecting: %v", err),
				})
			} else {
				m.provider = provider
				m.messages = append(m.messages, Message{
					Role:    "system",
					Content: fmt.Sprintf("Connected to %s", m.providerType),
				})
			}
			return m, nil

		case "enter":
			if m.input == "" {
				return m, nil
			}
			return m, m.sendMessage()

		case "backspace":
			if len(m.input) > 0 {
				m.input = m.input[:len(m.input)-1]
			}

		default:
			if msg.Type == tea.KeyRunes {
				m.input += string(msg.Runes)
			}
		}

	case responseMsg:
		m.messages = append(m.messages, Message{
			Role:    "assistant",
			Content: msg.content,
		})
		if len(msg.toolCalls) > 0 {
			toolNames := make([]string, len(msg.toolCalls))
			for i, tc := range msg.toolCalls {
				toolNames[i] = tc.Name
			}
			m.messages = append(m.messages, Message{
				Role:    "system",
				Content: fmt.Sprintf("🔧 Tools used: %s", strings.Join(toolNames, ", ")),
			})
		}

	case errorMsg:
		m.messages = append(m.messages, Message{
			Role:    "system",
			Content: fmt.Sprintf("Error: %v", msg.err),
		})
	}

	return m, nil
}

// View renders the UI
func (m model) View() string {
	var b strings.Builder

	// Header
	b.WriteString(titleStyle.Render("🤖 AI Agent - Obsidian Assistant"))
	b.WriteString("\n")
	b.WriteString(systemMessageStyle.Render(fmt.Sprintf("Provider: %s | Ctrl+P: Switch | Ctrl+N: Connect | Ctrl+C: Quit", m.providerType)))
	b.WriteString("\n\n")

	// Messages
	chatHeight := m.height - 8
	visibleMessages := m.messages
	if len(visibleMessages) > chatHeight {
		visibleMessages = visibleMessages[len(visibleMessages)-chatHeight:]
	}

	for _, msg := range visibleMessages {
		switch msg.Role {
		case "user":
			b.WriteString(userMessageStyle.Render("You: " + msg.Content))
		case "assistant":
			b.WriteString(assistantMessageStyle.Render("AI: " + msg.Content))
		case "system":
			if strings.Contains(msg.Content, "🔧") {
				b.WriteString(toolCallStyle.Render(msg.Content))
			} else {
				b.WriteString(systemMessageStyle.Render(msg.Content))
			}
		}
		b.WriteString("\n")
	}

	// Input
	b.WriteString("\n")
	b.WriteString(strings.Repeat("─", m.width))
	b.WriteString("\n")
	b.WriteString(inputStyle.Render("> " + m.input + "▊"))

	return b.String()
}

// Message types
type responseMsg struct {
	content   string
	toolCalls []ToolCall
}

type errorMsg struct {
	err error
}

// sendMessage sends a message to the AI
func (m model) sendMessage() tea.Cmd {
	userMsg := m.input
	m.input = ""

	m.messages = append(m.messages, Message{
		Role:    "user",
		Content: userMsg,
	})

	return func() tea.Msg {
		if m.provider == nil {
			return errorMsg{err: fmt.Errorf("not connected to provider")}
		}

		ctx := context.Background()

		// Convert messages
		chatMessages := make([]ChatMessage, 0, len(m.messages))
		for _, msg := range m.messages {
			if msg.Role != "system" || strings.Contains(msg.Content, "AI Agent ready") {
				chatMessages = append(chatMessages, ChatMessage{
					Role:    msg.Role,
					Content: msg.Content,
				})
			}
		}

		// Get tool definitions
		var tools []Tool
		if m.tools != nil {
			tools = m.tools.GetToolDefinitions()
		}

		// Call provider
		response, err := m.provider.Chat(ctx, chatMessages, tools)
		if err != nil {
			return errorMsg{err: err}
		}

		// Execute tools if needed
		if len(response.ToolCalls) > 0 && m.tools != nil {
			for i := range response.ToolCalls {
				result, err := m.tools.ExecuteTool(
					response.ToolCalls[i].Name,
					response.ToolCalls[i].Arguments,
				)
				if err != nil {
					response.ToolCalls[i].Result = fmt.Sprintf("Error: %v", err)
				} else {
					resultJSON, _ := json.Marshal(result)
					response.ToolCalls[i].Result = string(resultJSON)
				}
			}

			// Get final response after tool execution
			toolMessages := append(chatMessages, ChatMessage{
				Role:      "assistant",
				Content:   response.Content,
				ToolCalls: response.ToolCalls,
			})

			for _, tc := range response.ToolCalls {
				toolMessages = append(toolMessages, ChatMessage{
					Role:       "tool",
					Content:    tc.Result,
					ToolCallID: tc.ID,
				})
			}

			finalResponse, err := m.provider.Chat(ctx, toolMessages, nil)
			if err == nil {
				response = finalResponse
			}
		}

		return responseMsg{
			content:   response.Content,
			toolCalls: response.ToolCalls,
		}
	}
}

func main() {
	vaultPath := os.Getenv("OBSIDIAN_VAULT_PATH")
	if vaultPath == "" {
		vaultPath = os.Getenv("HOME") + "/Documents/Obsidian"
	}

	p := tea.NewProgram(
		initialModel(vaultPath),
		tea.WithAltScreen(),
	)

	if _, err := p.Run(); err != nil {
		fmt.Printf("Error: %v\n", err)
		os.Exit(1)
	}
}
