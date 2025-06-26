package config

import (
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"

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
	PolicyNameFg    string `json:"policyNameFg"`    // Policy name foreground color
	PolicyNameBg    string `json:"policyNameBg"`    // Policy name background color
	PolicyMetadata  string `json:"policyMetadata"`  // Policy metadata color (Type & ARN)
	JsonKey         string `json:"jsonKey"`         // JSON key color
	JsonServiceName string `json:"jsonServiceName"` // JSON AWS service name color
	Debug           string `json:"debug"`           // Debug message color
}

// Config holds application configuration
type Config struct {
	Colors ThemeColors `json:"colors"`
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
		PolicyNameFg:    "39",  // Bright cyan
		PolicyNameBg:    "236", // Dark background
		PolicyMetadata:  "220", // Yellow
		JsonKey:         "32",  // Green
		JsonServiceName: "35",  // Pink
		Debug:           "#FF00FF",
	},
}

// Load reads config from file or creates a default if not exist
func Load() (*Config, error) {
	// Get config path
	configDir, err := getConfigDir()
	if err != nil {
		return nil, err
	}

	configPath := filepath.Join(configDir, "config.json")

	// Check if config file exists
	if _, err := os.Stat(configPath); os.IsNotExist(err) {
		// Create default config
		err = os.MkdirAll(configDir, 0755)
		if err != nil {
			return nil, fmt.Errorf("failed to create config directory: %w", err)
		}

		// Save default config
		err = DefaultConfig.Save()
		if err != nil {
			return nil, fmt.Errorf("failed to save default config: %w", err)
		}

		return &DefaultConfig, nil
	}

	// Read config file
	data, err := os.ReadFile(configPath)
	if err != nil {
		return nil, fmt.Errorf("failed to read config file: %w", err)
	}

	var config Config
	err = json.Unmarshal(data, &config)
	if err != nil {
		return nil, fmt.Errorf("failed to parse config file: %w", err)
	}

	return &config, nil
}

// Save writes config to file
func (c *Config) Save() error {
	configDir, err := getConfigDir()
	if err != nil {
		return err
	}

	// Create config directory if not exists
	err = os.MkdirAll(configDir, 0755)
	if err != nil {
		return fmt.Errorf("failed to create config directory: %w", err)
	}

	configPath := filepath.Join(configDir, "config.json")

	data, err := json.MarshalIndent(c, "", "  ")
	if err != nil {
		return fmt.Errorf("failed to marshal config: %w", err)
	}

	err = os.WriteFile(configPath, data, 0644)
	if err != nil {
		return fmt.Errorf("failed to write config file: %w", err)
	}

	return nil
}

// getConfigDir returns the path to config directory
func getConfigDir() (string, error) {
	homeDir, err := os.UserHomeDir()
	if err != nil {
		return "", fmt.Errorf("failed to get home directory: %w", err)
	}

	return filepath.Join(homeDir, ".config", "atui"), nil
}

// GetForegroundColor returns a lipgloss color from the config
func GetForegroundColor(colorString string) lipgloss.AdaptiveColor {
	if colorString == "" {
		// If empty, don't color
		return lipgloss.AdaptiveColor{}
	}

	// If it starts with #, it's a hex color
	if colorString[0] == '#' {
		return lipgloss.AdaptiveColor{Light: colorString, Dark: colorString}
	}

	// Otherwise, it's an ANSI color or special keyword
	return lipgloss.AdaptiveColor{Light: colorString, Dark: colorString}
}
