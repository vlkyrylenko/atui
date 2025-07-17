package config

import (
	"encoding/json"
	"os"
	"path/filepath"
	"reflect"
	"testing"
)

// Test DefaultConfig values
func TestDefaultConfig(t *testing.T) {
	// Test that DefaultConfig has expected values
	if DefaultConfig.Colors.Title != "bold" {
		t.Errorf("Expected default title color to be 'bold', got '%s'", DefaultConfig.Colors.Title)
	}

	if DefaultConfig.Colors.SelectedItem != "170" {
		t.Errorf("Expected default selected item color to be '170', got '%s'", DefaultConfig.Colors.SelectedItem)
	}

	if DefaultConfig.Colors.Status != "#04B575" {
		t.Errorf("Expected default status color to be '#04B575', got '%s'", DefaultConfig.Colors.Status)
	}

	if DefaultConfig.Colors.Error != "#FF0000" {
		t.Errorf("Expected default error color to be '#FF0000', got '%s'", DefaultConfig.Colors.Error)
	}

	if DefaultConfig.Colors.JsonKey != "32" {
		t.Errorf("Expected default JSON key color to be '32', got '%s'", DefaultConfig.Colors.JsonKey)
	}

	if DefaultConfig.Colors.JsonServiceName != "35" {
		t.Errorf("Expected default JSON service name color to be '35', got '%s'", DefaultConfig.Colors.JsonServiceName)
	}
}

// Test ThemeColors struct
func TestThemeColors(t *testing.T) {
	colors := ThemeColors{
		Title:           "test-title",
		Item:            "test-item",
		SelectedItem:    "test-selected",
		Status:          "test-status",
		Error:           "test-error",
		PolicyInfo:      "test-policy-info",
		PolicyNameFg:    "test-policy-name-fg",
		PolicyNameBg:    "test-policy-name-bg",
		PolicyMetadata:  "test-policy-metadata",
		JsonKey:         "test-json-key",
		JsonServiceName: "test-json-service",
		Debug:           "test-debug",
	}

	// Test that all fields are properly set
	if colors.Title != "test-title" {
		t.Errorf("Expected title to be 'test-title', got '%s'", colors.Title)
	}

	if colors.JsonKey != "test-json-key" {
		t.Errorf("Expected JsonKey to be 'test-json-key', got '%s'", colors.JsonKey)
	}

	if colors.JsonServiceName != "test-json-service" {
		t.Errorf("Expected JsonServiceName to be 'test-json-service', got '%s'", colors.JsonServiceName)
	}
}

// Test Config struct
func TestConfig(t *testing.T) {
	config := Config{
		Colors: ThemeColors{
			Title: "test",
			Item:  "test-item",
		},
	}

	if config.Colors.Title != "test" {
		t.Errorf("Expected config title to be 'test', got '%s'", config.Colors.Title)
	}

	if config.Colors.Item != "test-item" {
		t.Errorf("Expected config item to be 'test-item', got '%s'", config.Colors.Item)
	}
}

// Test JSON marshaling/unmarshaling of Config
func TestConfigJSONSerialization(t *testing.T) {
	// Create a test config
	original := Config{
		Colors: ThemeColors{
			Title:           "bold",
			Item:            "white",
			SelectedItem:    "170",
			Status:          "#04B575",
			Error:           "#FF0000",
			PolicyInfo:      "#AAAAAA",
			PolicyNameFg:    "39",
			PolicyNameBg:    "236",
			PolicyMetadata:  "220",
			JsonKey:         "32",
			JsonServiceName: "35",
			Debug:           "#FF00FF",
		},
	}

	// Marshal to JSON
	jsonData, err := json.Marshal(original)
	if err != nil {
		t.Errorf("Failed to marshal config to JSON: %v", err)
	}

	// Unmarshal back to Config
	var unmarshaled Config
	err = json.Unmarshal(jsonData, &unmarshaled)
	if err != nil {
		t.Errorf("Failed to unmarshal config from JSON: %v", err)
	}

	// Compare original and unmarshaled
	if !reflect.DeepEqual(original, unmarshaled) {
		t.Errorf("Original and unmarshaled configs don't match")
	}
}

// Test Load function with non-existent config file
func TestLoadNonExistentConfig(t *testing.T) {
	// Create a temporary directory for testing
	tempDir, err := os.MkdirTemp("", "atui-config-test")
	if err != nil {
		t.Fatalf("Failed to create temp directory: %v", err)
	}
	defer os.RemoveAll(tempDir)

	// Temporarily change HOME to our temp directory
	originalHome := os.Getenv("HOME")
	os.Setenv("HOME", tempDir)
	defer os.Setenv("HOME", originalHome)

	// Load config (should create default config file)
	config, err := Load()
	if err != nil {
		t.Errorf("Expected Load to succeed with default config, got error: %v", err)
	}

	// Should return default config
	if !reflect.DeepEqual(*config, DefaultConfig) {
		t.Errorf("Expected default config, got different config")
	}

	// Check that config file was created
	configPath := filepath.Join(tempDir, ".config", "atui", "config.json")
	if _, err := os.Stat(configPath); os.IsNotExist(err) {
		t.Errorf("Expected config file to be created at %s", configPath)
	}
}

// Test Load function with existing valid config file
func TestLoadExistingValidConfig(t *testing.T) {
	// Create a temporary directory for testing
	tempDir, err := os.MkdirTemp("", "atui-config-test")
	if err != nil {
		t.Fatalf("Failed to create temp directory: %v", err)
	}
	defer os.RemoveAll(tempDir)

	// Create config directory
	configDir := filepath.Join(tempDir, ".config", "atui")
	err = os.MkdirAll(configDir, 0755)
	if err != nil {
		t.Fatalf("Failed to create config directory: %v", err)
	}

	// Create a custom config
	customConfig := Config{
		Colors: ThemeColors{
			Title:           "custom-title",
			Item:            "custom-item",
			SelectedItem:    "custom-selected",
			Status:          "custom-status",
			Error:           "custom-error",
			PolicyInfo:      "custom-policy-info",
			PolicyNameFg:    "custom-policy-name-fg",
			PolicyNameBg:    "custom-policy-name-bg",
			PolicyMetadata:  "custom-policy-metadata",
			JsonKey:         "custom-json-key",
			JsonServiceName: "custom-json-service",
			Debug:           "custom-debug",
		},
	}

	// Write custom config to file
	configPath := filepath.Join(configDir, "config.json")
	configData, err := json.MarshalIndent(customConfig, "", "  ")
	if err != nil {
		t.Fatalf("Failed to marshal custom config: %v", err)
	}

	err = os.WriteFile(configPath, configData, 0644)
	if err != nil {
		t.Fatalf("Failed to write config file: %v", err)
	}

	// Temporarily change HOME to our temp directory
	originalHome := os.Getenv("HOME")
	os.Setenv("HOME", tempDir)
	defer os.Setenv("HOME", originalHome)

	// Load config
	config, err := Load()
	if err != nil {
		t.Errorf("Expected Load to succeed, got error: %v", err)
	}

	// Should return custom config
	if !reflect.DeepEqual(*config, customConfig) {
		t.Errorf("Expected custom config, got different config")
	}
}

// Test Load function with invalid JSON config file
func TestLoadInvalidJSONConfig(t *testing.T) {
	// Create a temporary directory for testing
	tempDir, err := os.MkdirTemp("", "atui-config-test")
	if err != nil {
		t.Fatalf("Failed to create temp directory: %v", err)
	}
	defer os.RemoveAll(tempDir)

	// Create config directory
	configDir := filepath.Join(tempDir, ".config", "atui")
	err = os.MkdirAll(configDir, 0755)
	if err != nil {
		t.Fatalf("Failed to create config directory: %v", err)
	}

	// Write invalid JSON to config file
	configPath := filepath.Join(configDir, "config.json")
	invalidJSON := `{"colors": {"title": "bold", "item": }` // Invalid JSON
	err = os.WriteFile(configPath, []byte(invalidJSON), 0644)
	if err != nil {
		t.Fatalf("Failed to write invalid config file: %v", err)
	}

	// Temporarily change HOME to our temp directory
	originalHome := os.Getenv("HOME")
	os.Setenv("HOME", tempDir)
	defer os.Setenv("HOME", originalHome)

	// Load config (should fail with JSON error)
	_, err = Load()
	if err == nil {
		t.Errorf("Expected Load to fail with invalid JSON, got no error")
	}
}

// Test SaveDefaultConfig function
func TestSaveDefaultConfig(t *testing.T) {
	// Create a temporary directory for testing
	tempDir, err := os.MkdirTemp("", "atui-config-test")
	if err != nil {
		t.Fatalf("Failed to create temp directory: %v", err)
	}
	defer os.RemoveAll(tempDir)

	// Temporarily change HOME to our temp directory
	originalHome := os.Getenv("HOME")
	os.Setenv("HOME", tempDir)
	defer os.Setenv("HOME", originalHome)

	// Save default config
	err = SaveDefaultConfig()
	if err != nil {
		t.Errorf("Expected SaveDefaultConfig to succeed, got error: %v", err)
	}

	// Check that config file was created
	configPath := filepath.Join(tempDir, ".config", "atui", "config.json")
	if _, err := os.Stat(configPath); os.IsNotExist(err) {
		t.Errorf("Expected config file to be created at %s", configPath)
	}

	// Load the saved config and verify it matches default
	config, err := Load()
	if err != nil {
		t.Errorf("Failed to load saved config: %v", err)
	}

	if !reflect.DeepEqual(*config, DefaultConfig) {
		t.Errorf("Saved config doesn't match default config")
	}
}

// Test config file permissions
func TestConfigFilePermissions(t *testing.T) {
	// Create a temporary directory for testing
	tempDir, err := os.MkdirTemp("", "atui-config-test")
	if err != nil {
		t.Fatalf("Failed to create temp directory: %v", err)
	}
	defer os.RemoveAll(tempDir)

	// Temporarily change HOME to our temp directory
	originalHome := os.Getenv("HOME")
	os.Setenv("HOME", tempDir)
	defer os.Setenv("HOME", originalHome)

	// Save default config
	err = SaveDefaultConfig()
	if err != nil {
		t.Errorf("Expected SaveDefaultConfig to succeed, got error: %v", err)
	}

	// Check file permissions
	configPath := filepath.Join(tempDir, ".config", "atui", "config.json")
	fileInfo, err := os.Stat(configPath)
	if err != nil {
		t.Errorf("Failed to stat config file: %v", err)
	}

	// Check that file has reasonable permissions (readable by owner)
	mode := fileInfo.Mode()
	if mode&0400 == 0 {
		t.Errorf("Config file is not readable by owner")
	}
}

// Test partial config loading (missing fields should use defaults)
func TestPartialConfigLoading(t *testing.T) {
	// Create a temporary directory for testing
	tempDir, err := os.MkdirTemp("", "atui-config-test")
	if err != nil {
		t.Fatalf("Failed to create temp directory: %v", err)
	}
	defer os.RemoveAll(tempDir)

	// Create config directory
	configDir := filepath.Join(tempDir, ".config", "atui")
	err = os.MkdirAll(configDir, 0755)
	if err != nil {
		t.Fatalf("Failed to create config directory: %v", err)
	}

	// Create a partial config (only some fields)
	partialConfigJSON := `{
		"colors": {
			"title": "custom-title",
			"selectedItem": "custom-selected"
		}
	}`

	// Write partial config to file
	configPath := filepath.Join(configDir, "config.json")
	err = os.WriteFile(configPath, []byte(partialConfigJSON), 0644)
	if err != nil {
		t.Fatalf("Failed to write partial config file: %v", err)
	}

	// Temporarily change HOME to our temp directory
	originalHome := os.Getenv("HOME")
	os.Setenv("HOME", tempDir)
	defer os.Setenv("HOME", originalHome)

	// Load config
	config, err := Load()
	if err != nil {
		t.Errorf("Expected Load to succeed with partial config, got error: %v", err)
	}

	// Check that specified fields are loaded
	if config.Colors.Title != "custom-title" {
		t.Errorf("Expected title to be 'custom-title', got '%s'", config.Colors.Title)
	}

	if config.Colors.SelectedItem != "custom-selected" {
		t.Errorf("Expected selectedItem to be 'custom-selected', got '%s'", config.Colors.SelectedItem)
	}

	// Check that unspecified fields use defaults (empty strings in this case due to JSON unmarshaling)
	// Note: JSON unmarshaling into struct will set missing fields to zero values
	if config.Colors.Status != "" {
		t.Errorf("Expected status to be empty (zero value), got '%s'", config.Colors.Status)
	}
}
