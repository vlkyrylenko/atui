package config

import (
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"

	"github.com/charmbracelet/bubbles/key"
	"github.com/charmbracelet/lipgloss"
)

// ThemeColors holds all the color settings for the application
type ThemeColors struct {
	Title           string `json:"title"`           // Title style color
	Item            string `json:"item"`            // Normal item color
	SelectedItem    string `json:"selectedItem"`    // Selected item color
	Status          string `json:"status"`          // Status message color
	Error           string `json:"error"`           // Error message color
	PolicyInfo      string `json:"policyInfo"`      // Policy info color
	HelpInfo        string `json:"helpInfo"`        // Help info color
	PolicyNameFg    string `json:"policyNameFg"`    // Policy name foreground color
	PolicyNameBg    string `json:"policyNameBg"`    // Policy name background color
	PolicyMetadata  string `json:"policyMetadata"`  // Policy metadata color (Type & ARN)
	JsonKey         string `json:"jsonKey"`         // JSON key color
	JsonServiceName string `json:"jsonServiceName"` // JSON AWS service name color
	Debug           string `json:"debug"`           // Debug message color
}

// Config holds application configuration
type Config struct {
	Colors              ThemeColors `json:"colors"`
	KeybindingSeparator string      `json:"keybindingSeparator"` // Separator between key and description in help text
}

// Default configuration
var DefaultConfig = Config{
	Colors: ThemeColors{
		Title:           "bold",
		Item:            "",
		SelectedItem:    "170",
		Status:          "#04B575",
		Error:           "#FF0000",
		PolicyInfo:      "#AAAAAA",
		HelpInfo:        "#FF00FF",
		PolicyNameFg:    "39",  // Bright cyan
		PolicyNameBg:    "236", // Dark background
		PolicyMetadata:  "220", // Yellow
		JsonKey:         "32",  // Green
		JsonServiceName: "35",  // Pink
		Debug:           "#FF00FF",
	},
	KeybindingSeparator: " - ", // Default separator
}

// Load reads config from file or creates a default if not exist
func Load() (*Config, error) {
	configPath, err := getConfigPath()
	if err != nil {
		return nil, fmt.Errorf("failed to get config path: %w", err)
	}

	// Check if config file exists
	if _, err := os.Stat(configPath); os.IsNotExist(err) {
		// Create default config file
		if err := SaveDefaultConfig(); err != nil {
			return nil, fmt.Errorf("failed to create default config: %w", err)
		}
		return &DefaultConfig, nil
	}

	// Read existing config file
	data, err := os.ReadFile(configPath)
	if err != nil {
		return nil, fmt.Errorf("failed to read config file: %w", err)
	}

	// Parse JSON
	var config Config
	if err := json.Unmarshal(data, &config); err != nil {
		return nil, fmt.Errorf("failed to parse config JSON: %w", err)
	}

	return &config, nil
}

// SaveDefaultConfig creates the default configuration file
func SaveDefaultConfig() error {
	configPath, err := getConfigPath()
	if err != nil {
		return fmt.Errorf("failed to get config path: %w", err)
	}

	// Create config directory if it doesn't exist
	configDir := filepath.Dir(configPath)
	if err := os.MkdirAll(configDir, 0755); err != nil {
		return fmt.Errorf("failed to create config directory: %w", err)
	}

	// Marshal default config to JSON
	data, err := json.MarshalIndent(DefaultConfig, "", "  ")
	if err != nil {
		return fmt.Errorf("failed to marshal default config: %w", err)
	}

	// Write to file
	if err := os.WriteFile(configPath, data, 0644); err != nil {
		return fmt.Errorf("failed to write config file: %w", err)
	}

	return nil
}

// getConfigPath returns the path to the configuration file
func getConfigPath() (string, error) {
	homeDir, err := os.UserHomeDir()
	if err != nil {
		return "", fmt.Errorf("failed to get user home directory: %w", err)
	}

	return filepath.Join(homeDir, ".config", "atui", "config.json"), nil
}

// GetTheme creates a lipgloss theme from the configuration
func (c *Config) GetTheme() *Theme {
	return &Theme{
		titleStyle:        lipgloss.NewStyle().Foreground(lipgloss.Color(c.Colors.Title)),
		itemStyle:         lipgloss.NewStyle().Foreground(lipgloss.Color(c.Colors.Item)),
		selectedItemStyle: lipgloss.NewStyle().Foreground(lipgloss.Color(c.Colors.SelectedItem)),
		paginationStyle:   lipgloss.NewStyle().Foreground(lipgloss.Color(c.Colors.PolicyInfo)),
		helpStyle:         lipgloss.NewStyle().Foreground(lipgloss.Color(c.Colors.HelpInfo)),
		statusMessageStyle: func(s string) string {
			return lipgloss.NewStyle().Foreground(lipgloss.Color(c.Colors.Status)).Render(s)
		},
		errorMessageStyle: func(s string) string { return lipgloss.NewStyle().Foreground(lipgloss.Color(c.Colors.Error)).Render(s) },
		policyInfoStyle:   lipgloss.NewStyle().Foreground(lipgloss.Color(c.Colors.PolicyInfo)),
		policyNameHighlightStyle: func(s string) string {
			return lipgloss.NewStyle().Foreground(lipgloss.Color(c.Colors.PolicyNameFg)).Background(lipgloss.Color(c.Colors.PolicyNameBg)).Render(s)
		},
		policyMetadataStyle: func(s string) string {
			return lipgloss.NewStyle().Foreground(lipgloss.Color(c.Colors.PolicyMetadata)).Render(s)
		},
		debugStyle: func(s string) string { return lipgloss.NewStyle().Foreground(lipgloss.Color(c.Colors.Debug)).Render(s) },
	}
}

// Theme holds all styles for the application (moved from main.go for better organization)
type Theme struct {
	titleStyle               lipgloss.Style
	itemStyle                lipgloss.Style
	selectedItemStyle        lipgloss.Style
	paginationStyle          lipgloss.Style
	helpStyle                lipgloss.Style
	statusMessageStyle       func(string) string
	errorMessageStyle        func(string) string
	policyInfoStyle          lipgloss.Style
	policyNameHighlightStyle func(string) string
	policyMetadataStyle      func(string) string
	debugStyle               func(string) string
}

// GetForegroundColor converts a color string to lipgloss color
func GetForegroundColor(colorStr string) lipgloss.Color {
	if colorStr == "" {
		return lipgloss.Color("")
	}
	return lipgloss.Color(colorStr)
}

// CreateKeybinding creates a key binding with the configured separator
func (c *Config) CreateKeybinding(keys []string, keyDisplay, description string) key.Binding {
	separator := c.KeybindingSeparator
	if separator == "" {
		separator = " - " // fallback to default
	}

	return key.NewBinding(
		key.WithKeys(keys...),
		key.WithHelp(keyDisplay, separator+description),
	)
}
