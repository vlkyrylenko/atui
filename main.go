package main

import (
	"context"
	"encoding/json"
	"fmt"
	"log"
	"net/url"
	"os"
	"os/exec"
	"regexp"
	"strings"
	"time"

	"github.com/aws/aws-sdk-go-v2/aws"
	"github.com/aws/aws-sdk-go-v2/config"
	"github.com/aws/aws-sdk-go-v2/service/iam"
	"github.com/aws/aws-sdk-go-v2/service/sts"
	"github.com/charmbracelet/bubbles/key"
	"github.com/charmbracelet/bubbles/list"
	"github.com/charmbracelet/bubbles/spinner"
	"github.com/charmbracelet/bubbles/viewport"
	tea "github.com/charmbracelet/bubbletea"
	"github.com/charmbracelet/lipgloss"

	appconfig "atui/config"
)

// Theme holds all styles for the application
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

// Styles
var (
	appTheme Theme
)

// Model holds the application state
type model struct {
	rolesList      list.Model
	policiesList   list.Model
	loading        bool
	spinner        spinner.Model
	selectedRole   *RoleItem
	policyView     viewport.Model
	selectedPolicy *PolicyItem
	policyDocument string
	currentScreen  string
	err            error
	width, height  int
	statusMsg      string
}

// RoleItem represents an IAM role
type RoleItem struct {
	roleName       string
	roleArn        string
	description    string
	policies       []PolicyItem
	policiesLoaded bool
	policyCount    int // Add count of policies
}

// PolicyItem represents an IAM policy
type PolicyItem struct {
	policyName     string
	policyArn      string
	policyType     string // Added policy type (AWS managed vs Customer managed)
	policyDocument string
	documentLoaded bool
}

func (i RoleItem) Title() string { return i.roleName }
func (i RoleItem) Description() string {
	desc := i.description
	if i.policiesLoaded {
		desc += fmt.Sprintf(" | %d policies attached", len(i.policies))
	}
	return desc
}
func (i RoleItem) FilterValue() string { return i.roleName }

func (i PolicyItem) Title() string {
	// Make the title more prominent by adding a symbol
	return "📄 " + i.policyName
}

func (i PolicyItem) Description() string {
	desc := ""
	if i.policyType == "AWS" {
		desc = "[AWS Managed] "
	} else if i.policyType == "Customer" {
		desc = "[Customer Managed] "
	}

	// Show shortened ARN for better readability
	if i.policyArn != "" {
		parts := strings.Split(i.policyArn, "/")
		if len(parts) > 1 {
			// Just show the last part of the ARN for cleaner display
			desc += parts[len(parts)-1]
		} else {
			// If no slash, show the last part after colon
			colonParts := strings.Split(i.policyArn, ":")
			if len(colonParts) > 1 {
				desc += colonParts[len(colonParts)-1]
			} else {
				desc += i.policyArn
			}
		}
	}
	return desc
}
func (i PolicyItem) FilterValue() string { return i.policyName }

// Key mappings
type keyMap struct {
	Up       key.Binding
	Down     key.Binding
	Enter    key.Binding
	Back     key.Binding
	OpenJSON key.Binding
	Quit     key.Binding
}

var keys = keyMap{
	Up: key.NewBinding(
		key.WithKeys("up", "k"),
		key.WithHelp("↑/k", "move up"),
	),
	Down: key.NewBinding(
		key.WithKeys("down", "j"),
		key.WithHelp("↓/j", "move down"),
	),
	Enter: key.NewBinding(
		key.WithKeys("enter"),
		key.WithHelp("enter", "select"),
	),
	Back: key.NewBinding(
		key.WithKeys("esc", "backspace"),
		key.WithHelp("esc/backspace", "back"),
	),
	OpenJSON: key.NewBinding(
		key.WithKeys("o"),
		key.WithHelp("o", "open JSON"),
	),
	Quit: key.NewBinding(
		key.WithKeys("q", "ctrl+c"),
		key.WithHelp("q/ctrl+c", "quit"),
	),
}

// Initialize the model
func initialModel() model {
	s := spinner.New()
	s.Spinner = spinner.Dot
	s.Style = lipgloss.NewStyle().Foreground(lipgloss.Color("205"))

	roleDelegate := list.NewDefaultDelegate()
	rolesList := list.New([]list.Item{}, roleDelegate, 0, 0)
	rolesList.Title = "AWS IAM Roles"
	rolesList.SetShowStatusBar(false)
	rolesList.SetFilteringEnabled(true)
	rolesList.Styles.Title = appTheme.titleStyle
	rolesList.Styles.PaginationStyle = appTheme.paginationStyle
	rolesList.Styles.HelpStyle = appTheme.helpStyle

	// Create a custom delegate for policies with more visible styling
	policyDelegate := list.NewDefaultDelegate()
	policyDelegate.ShowDescription = true
	policyDelegate.SetHeight(3) // Increase height for better visibility
	policyDelegate.Styles.SelectedTitle = appTheme.selectedItemStyle.Copy().Bold(true)
	policyDelegate.Styles.SelectedDesc = appTheme.selectedItemStyle.Copy().Foreground(lipgloss.Color("240"))
	policyDelegate.Styles.NormalTitle = appTheme.itemStyle.Copy().Bold(true)
	policyDelegate.Styles.NormalDesc = appTheme.itemStyle.Copy().Foreground(lipgloss.Color("240"))

	policiesList := list.New([]list.Item{}, policyDelegate, 0, 0)
	policiesList.Title = "Policies"
	policiesList.SetShowStatusBar(false)
	policiesList.SetFilteringEnabled(true)
	policiesList.Styles.Title = appTheme.titleStyle
	policiesList.Styles.PaginationStyle = appTheme.paginationStyle
	policiesList.Styles.HelpStyle = appTheme.helpStyle

	policyView := viewport.New(0, 0)
	policyView.Style = lipgloss.NewStyle().Padding(1, 2)

	return model{
		rolesList:     rolesList,
		policiesList:  policiesList,
		spinner:       s,
		loading:       false,
		policyView:    policyView,
		currentScreen: "roles",
		statusMsg:     "Select a role to view its policies",
	}
}

func (m model) Init() tea.Cmd {
	return tea.Batch(
		spinner.Tick,
		loadIAMRolesCmd(),
	)
}

// Update handles all the application logic and events
func (m model) Update(msg tea.Msg) (tea.Model, tea.Cmd) {
	var (
		cmd  tea.Cmd
		cmds []tea.Cmd
	)

	switch msg := msg.(type) {
	case tea.KeyMsg:
		switch {
		case key.Matches(msg, keys.Quit):
			return m, tea.Quit

		case key.Matches(msg, keys.Back):
			if m.currentScreen == "policies" {
				m.currentScreen = "roles"
				m.selectedPolicy = nil
				m.statusMsg = ""
				return m, nil
			} else if m.currentScreen == "policy_document" {
				m.currentScreen = "policies"
				m.statusMsg = ""
				return m, nil
			}

		case key.Matches(msg, keys.OpenJSON):
			if m.currentScreen == "policy_document" && m.selectedPolicy != nil {
				return m, openInEditorCmd(m.policyDocument)
			}

		case key.Matches(msg, keys.Enter):
			if m.currentScreen == "roles" {
				if m.rolesList.SelectedItem() == nil {
					return m, nil
				}

				if selected, ok := m.rolesList.SelectedItem().(*RoleItem); ok {
					m.selectedRole = selected
					m.currentScreen = "policies"
					m.policiesList.Title = fmt.Sprintf("Policies for %s", m.selectedRole.roleName)
					m.statusMsg = ""

					if !m.selectedRole.policiesLoaded {
						m.loading = true
						m.statusMsg = fmt.Sprintf("Loading policies for %s...", m.selectedRole.roleName)
						return m, loadRolePoliciesCmd(m.selectedRole.roleName)
					} else {
						// Update policy list with existing policies
						items := []list.Item{}
						for _, p := range m.selectedRole.policies {
							pCopy := p // Create a copy to avoid issues with loop variables in closures
							items = append(items, &pCopy)
						}
						m.policiesList.SetItems(items)

						// Ensure the policy list is properly selected and focused
						if len(items) > 0 {
							m.policiesList.Select(0) // Select first item
						}

						// Clear the status message to avoid seeing "Found X policies" text
						m.statusMsg = ""
					}
				}
				return m, nil
			} else if m.currentScreen == "policies" {
				if m.policiesList.SelectedItem() == nil {
					return m, nil
				}

				if selected, ok := m.policiesList.SelectedItem().(*PolicyItem); ok {
					m.selectedPolicy = selected
					m.currentScreen = "policy_document"
					m.statusMsg = ""

					if !m.selectedPolicy.documentLoaded {
						m.loading = true
						m.statusMsg = fmt.Sprintf("Loading policy document for %s...", m.selectedPolicy.policyName)
						return m, loadPolicyDocumentCmd(m.selectedPolicy.policyArn)
					} else {
						m.policyDocument = m.selectedPolicy.policyDocument
						m.policyView.SetContent(m.policyDocument)
					}
				}
				return m, nil
			}
		}

	case tea.WindowSizeMsg:
		m.height = msg.Height
		m.width = msg.Width

		headerHeight := 6
		footerHeight := 3
		verticalMarginHeight := headerHeight + footerHeight

		// Always resize all list components to ensure they're properly initialized
		m.rolesList.SetSize(msg.Width, msg.Height-verticalMarginHeight)
		m.policiesList.SetSize(msg.Width, msg.Height-verticalMarginHeight)
		m.policyView.Width = msg.Width
		m.policyView.Height = msg.Height - verticalMarginHeight

		return m, nil
	case rolesLoadedMsg:
		m.loading = false
		items := []list.Item{}
		for _, role := range msg {
			roleCopy := role // Create a copy to avoid issues with loop variables in closures
			items = append(items, &roleCopy)
		}
		m.rolesList.SetItems(items)
		return m, nil

	case policiesLoadedMsg:
		m.loading = false
		items := []list.Item{}
		for _, policy := range msg.policies {
			policyCopy := policy // Create a copy to avoid issues with loop variables in closures
			items = append(items, &policyCopy)
		}
		m.policiesList.SetItems(items)

		// Update the selected role's policies
		m.selectedRole.policies = msg.policies
		m.selectedRole.policiesLoaded = true

		// Clear status message so we just see the policies directly
		m.statusMsg = ""

		return m, nil

	case policyDocumentLoadedMsg:
		m.loading = false
		m.policyDocument = msg.document

		// Pretty format the JSON
		var jsonObj interface{}
		if err := json.Unmarshal([]byte(msg.document), &jsonObj); err != nil {
			m.policyDocument = "Error parsing JSON: " + err.Error()
		} else {
			prettyJSON, err := json.MarshalIndent(jsonObj, "", "  ")
			if err != nil {
				m.policyDocument = "Error formatting JSON: " + err.Error()
			} else {
				// Apply color formatting to the pretty-printed JSON
				m.policyDocument = colorizeJSON(string(prettyJSON))
			}
		}

		m.policyView.SetContent(m.policyDocument)

		// Update the selected policy
		m.selectedPolicy.policyDocument = m.policyDocument
		m.selectedPolicy.documentLoaded = true
		return m, nil

	case spinner.TickMsg:
		var spinnerCmd tea.Cmd
		m.spinner, spinnerCmd = m.spinner.Update(msg)
		if m.loading {
			return m, spinnerCmd
		}

	case errorMsg:
		m.loading = false
		m.err = msg
		return m, nil
	}

	// Handle list updates
	switch m.currentScreen {
	case "roles":
		m.rolesList, cmd = m.rolesList.Update(msg)
		cmds = append(cmds, cmd)
	case "policies":
		m.policiesList, cmd = m.policiesList.Update(msg)
		cmds = append(cmds, cmd)
	case "policy_document":
		m.policyView, cmd = m.policyView.Update(msg)
		cmds = append(cmds, cmd)
	}

	return m, tea.Batch(cmds...)
}

// View renders the UI based on the current state
func (m model) View() string {
	if m.loading {
		return fmt.Sprintf("\n\n   %s Loading...\n\n", m.spinner.View())
	}

	if m.err != nil {
		// Wrap error message to fit screen width, leaving some margin
		wrappedErrorMsg := wordWrap(m.err.Error(), m.width-10)
		return fmt.Sprintf("\n\n   Error: %s\n\n", appTheme.errorMessageStyle(wrappedErrorMsg))
	}

	var view string

	switch m.currentScreen {
	case "roles":
		view = "\n" + m.rolesList.View()
		if m.statusMsg != "" {
			view += "\n  " + appTheme.statusMessageStyle(m.statusMsg)
		}
	case "policies":
		if m.selectedRole != nil {
			view = "\n" + m.policiesList.View()
			if m.statusMsg != "" {
				view += "\n  " + appTheme.statusMessageStyle(m.statusMsg)
			}
			view += "\n  press enter to view policy details • esc to go back • q to quit"
		}
	case "policy_document":
		if m.selectedPolicy != nil {
			// Use the highlighted style for the policy name
			headerStr := fmt.Sprintf("\n  %s\n", appTheme.policyNameHighlightStyle(m.selectedPolicy.policyName))
			if m.selectedPolicy.policyType != "" {
				headerStr += fmt.Sprintf("  %s\n", appTheme.policyMetadataStyle("Type: "+m.selectedPolicy.policyType))
			}
			if m.selectedPolicy.policyArn != "" {
				headerStr += fmt.Sprintf("  %s\n", appTheme.policyMetadataStyle("ARN: "+m.selectedPolicy.policyArn))
			}
			headerStr += "\n"
			helpStr := "\n\n  press o to open in editor • esc to go back • q to quit\n"
			view = headerStr + m.policyView.View() + helpStr
		}
	}

	return view
}

// Custom messages for handling asynchronous operations
type rolesLoadedMsg []RoleItem

type policiesLoadedMsg struct {
	roleName string
	policies []PolicyItem
}

type policyDocumentLoadedMsg struct {
	policyArn string
	document  string
}

type errorMsg error

// Load IAM roles from AWS
func loadIAMRolesCmd() tea.Cmd {
	return func() tea.Msg {
		ctx := context.Background()

		// Load AWS configuration with shared config
		cfg, err := config.LoadDefaultConfig(ctx)
		if err != nil {
			return errorMsg(fmt.Errorf("error loading AWS configuration: %w", err))
		}

		// Create clients
		iamClient := iam.NewFromConfig(cfg)
		stsClient := sts.NewFromConfig(cfg)

		// Get caller identity to determine current user/role
		identity, err := stsClient.GetCallerIdentity(ctx, &sts.GetCallerIdentityInput{})
		if err != nil {
			return errorMsg(fmt.Errorf("error getting caller identity: %w", err))
		}

		userArn := aws.ToString(identity.Arn)
		fmt.Println("Current user ARN:", userArn)

		// List IAM roles
		var roles []RoleItem
		paginator := iam.NewListRolesPaginator(iamClient, &iam.ListRolesInput{})
		for paginator.HasMorePages() {
			page, err := paginator.NextPage(ctx)
			if err != nil {
				return errorMsg(fmt.Errorf("error listing IAM roles: %w", err))
			}

			for _, role := range page.Roles {
				description := fmt.Sprintf("ARN: %s", *role.Arn)
				if role.Description != nil {
					description = aws.ToString(role.Description)
				}

				roles = append(roles, RoleItem{
					roleName:    aws.ToString(role.RoleName),
					roleArn:     aws.ToString(role.Arn),
					description: description,
				})
			}
		}

		return rolesLoadedMsg(roles)
	}
}

// Load policies attached to a role
func loadRolePoliciesCmd(roleName string) tea.Cmd {
	return func() tea.Msg {
		ctx := context.Background()

		// Load AWS configuration
		cfg, err := config.LoadDefaultConfig(ctx)
		if err != nil {
			fmt.Printf("Error loading AWS configuration: %v\n", err)
			return errorMsg(fmt.Errorf("error loading AWS configuration: %w", err))
		}

		// Create IAM client
		iamClient := iam.NewFromConfig(cfg)

		// Debug info
		fmt.Printf("Fetching policies for role: %s\n", roleName)

		// Get attached role policies
		var policies []PolicyItem
		paginator := iam.NewListAttachedRolePoliciesPaginator(iamClient, &iam.ListAttachedRolePoliciesInput{
			RoleName: aws.String(roleName),
		})

		for paginator.HasMorePages() {
			page, err := paginator.NextPage(ctx)
			if err != nil {
				fmt.Printf("Error listing policies for role %s: %v\n", roleName, err)
				return errorMsg(fmt.Errorf("error listing policies for role %s: %w", roleName, err))
			}

			fmt.Printf("Found %d policies on this page\n", len(page.AttachedPolicies))

			for _, policy := range page.AttachedPolicies {
				policyArn := aws.ToString(policy.PolicyArn)
				policyType := "Customer"

				// Check if it's an AWS managed policy
				if strings.Contains(policyArn, "arn:aws:iam::aws:") {
					policyType = "AWS"
				}

				policies = append(policies, PolicyItem{
					policyName: aws.ToString(policy.PolicyName),
					policyArn:  policyArn,
					policyType: policyType,
				})

				fmt.Printf("Added policy: %s (%s)\n", aws.ToString(policy.PolicyName), policyType)
			}
		}

		fmt.Printf("Total policies found for role %s: %d\n", roleName, len(policies))

		return policiesLoadedMsg{
			roleName: roleName,
			policies: policies,
		}
	}
}

// Load policy document
func loadPolicyDocumentCmd(policyArn string) tea.Cmd {
	return func() tea.Msg {
		ctx := context.Background()

		// Load AWS configuration
		cfg, err := config.LoadDefaultConfig(ctx)
		if err != nil {
			return errorMsg(fmt.Errorf("error loading AWS configuration: %w", err))
		}

		// Create IAM client
		iamClient := iam.NewFromConfig(cfg)

		// Get policy version
		policyResp, err := iamClient.GetPolicy(ctx, &iam.GetPolicyInput{
			PolicyArn: aws.String(policyArn),
		})
		if err != nil {
			return errorMsg(fmt.Errorf("error getting policy %s: %w", policyArn, err))
		}

		// Get default version of the policy
		versionResp, err := iamClient.GetPolicyVersion(ctx, &iam.GetPolicyVersionInput{
			PolicyArn: aws.String(policyArn),
			VersionId: policyResp.Policy.DefaultVersionId,
		})
		if err != nil {
			return errorMsg(fmt.Errorf("error getting policy version: %w", err))
		}

		// UrlDecode the document
		doc, err := decodeURLEncodedDocument(aws.ToString(versionResp.PolicyVersion.Document))
		if err != nil {
			return errorMsg(fmt.Errorf("error decoding policy document: %w", err))
		}

		return policyDocumentLoadedMsg{
			policyArn: policyArn,
			document:  doc,
		}
	}
}

// Decode URL-encoded JSON policy document
func decodeURLEncodedDocument(encoded string) (string, error) {
	decoded, err := url.QueryUnescape(encoded)
	if err != nil {
		return "", err
	}
	return decoded, nil
}

// Colorize JSON policy document with configured colors
func colorizeJSON(jsonStr string) string {
	// Get the configured colors from config
	cfg, err := appconfig.Load()
	if err != nil {
		// Fallback to default colors
		cfg = &appconfig.DefaultConfig
	}

	// Convert ANSI color numbers to escape codes
	keyColorCode := "32"         // Default: green
	serviceNameColorCode := "35" // Default: pink

	if cfg.Colors.JsonKey != "" {
		keyColorCode = cfg.Colors.JsonKey
	}
	if cfg.Colors.JsonServiceName != "" {
		serviceNameColorCode = cfg.Colors.JsonServiceName
	}

	// Remove ANSI prefix if it exists (some people might add the full escape code)
	if strings.HasPrefix(keyColorCode, "\033[") {
		keyColorCode = strings.TrimPrefix(keyColorCode, "\033[")
		keyColorCode = strings.TrimSuffix(keyColorCode, "m")
	}
	if strings.HasPrefix(serviceNameColorCode, "\033[") {
		serviceNameColorCode = strings.TrimPrefix(serviceNameColorCode, "\033[")
		serviceNameColorCode = strings.TrimSuffix(serviceNameColorCode, "m")
	}

	// Use regex to match JSON keys and their values in format: "key": value
	keyRegex := regexp.MustCompile(`"([^"]+)"(\s*:\s*)`)

	// Find service:action patterns in IAM permissions
	actionRegex := regexp.MustCompile(`"([a-zA-Z0-9]+):(.*?)"`)

	// First pass: Color the keys according to config
	coloredJSON := keyRegex.ReplaceAllString(jsonStr, fmt.Sprintf("\033[%sm\"$1\"\033[0m$2", keyColorCode))

	// Second pass: Color service names according to config
	coloredJSON = actionRegex.ReplaceAllString(coloredJSON, fmt.Sprintf("\"\033[%sm$1\033[0m:$2\"", serviceNameColorCode))

	return coloredJSON
}

// Open policy document in default editor
func openInEditorCmd(content string) tea.Cmd {
	return func() tea.Msg {
		// Create a temporary file
		tmpFile, err := os.CreateTemp("", "aws-policy-*.json")
		if err != nil {
			return errorMsg(fmt.Errorf("error creating temp file: %w", err))
		}
		defer tmpFile.Close()

		// Write the policy content to the file
		if _, err := tmpFile.WriteString(content); err != nil {
			return errorMsg(fmt.Errorf("error writing to temp file: %w", err))
		}

		filename := tmpFile.Name()

		// Determine which editor to use
		editor := os.Getenv("EDITOR")
		if editor == "" {
			// Default editors depending on platform
			editor = "nano" // Linux default
		}

		// Prepare editor command
		cmd := exec.Command(editor, filename)
		cmd.Stdin = os.Stdin
		cmd.Stdout = os.Stdout
		cmd.Stderr = os.Stderr

		// Execute the editor
		err = cmd.Run()
		if err != nil {
			return errorMsg(fmt.Errorf("error opening editor: %w", err))
		}

		// Wait a moment to let the user see the result when they exit the editor
		time.Sleep(500 * time.Millisecond)

		// Since we're returning to the TUI, we don't need to read the file back
		// The temp file will be cleaned up eventually by the OS

		return nil // No message needed when returning to the app
	}
}

// wordWrap wraps text to fit within maxWidth characters per line
func wordWrap(text string, maxWidth int) string {
	if maxWidth <= 0 || len(text) == 0 {
		return text
	}

	var result strings.Builder
	lineLen := 0
	words := strings.Fields(text)

	for i, word := range words {
		wordLen := len(word)

		// If this is not the first word and adding it would exceed maxWidth,
		// append a newline before the word
		if lineLen > 0 && lineLen+wordLen+1 > maxWidth {
			result.WriteString("\n")
			lineLen = 0
		} else if i > 0 {
			// Add space before non-first words on the same line
			result.WriteString(" ")
			lineLen++
		}

		result.WriteString(word)
		lineLen += wordLen
	}

	return result.String()
}

func main() {
	// Load the color theme from the config file
	var err error
	appTheme, err = loadThemeFromConfig()
	if err != nil {
		log.Fatalf("error loading theme from config: %v", err)
	}

	p := tea.NewProgram(initialModel(), tea.WithAltScreen())
	if _, err := p.Run(); err != nil {
		log.Fatal(err)
	}
}

// loadThemeFromConfig loads the color theme from the config package
func loadThemeFromConfig() (Theme, error) {
	var theme Theme

	// Load configuration from file or use defaults
	cfg, err := appconfig.Load()
	if err != nil {
		return theme, fmt.Errorf("error loading config: %w", err)
	}

	// Apply colors from config
	theme.titleStyle = lipgloss.NewStyle().MarginLeft(2)
	if cfg.Colors.Title == "bold" {
		theme.titleStyle = theme.titleStyle.Bold(true)
	} else {
		fg := appconfig.GetForegroundColor(cfg.Colors.Title)
		theme.titleStyle = theme.titleStyle.Foreground(fg)
	}

	theme.itemStyle = lipgloss.NewStyle().PaddingLeft(4)
	if cfg.Colors.Item != "" {
		theme.itemStyle = theme.itemStyle.Foreground(appconfig.GetForegroundColor(cfg.Colors.Item))
	}

	theme.selectedItemStyle = lipgloss.NewStyle().
		PaddingLeft(2).
		Foreground(appconfig.GetForegroundColor(cfg.Colors.SelectedItem))

	theme.paginationStyle = list.DefaultStyles().PaginationStyle.PaddingLeft(4)
	theme.helpStyle = list.DefaultStyles().HelpStyle.PaddingLeft(4).PaddingBottom(1)

	theme.statusMessageStyle = func(msg string) string {
		return lipgloss.NewStyle().
			Foreground(appconfig.GetForegroundColor(cfg.Colors.Status)).
			Render(msg)
	}

	theme.errorMessageStyle = func(msg string) string {
		return lipgloss.NewStyle().
			Foreground(appconfig.GetForegroundColor(cfg.Colors.Error)).
			Render(msg)
	}

	theme.policyInfoStyle = lipgloss.NewStyle().
		PaddingLeft(2).
		Foreground(appconfig.GetForegroundColor(cfg.Colors.PolicyInfo))

	theme.policyNameHighlightStyle = func(name string) string {
		return lipgloss.NewStyle().
			Bold(true).
			Foreground(lipgloss.Color(cfg.Colors.PolicyNameFg)).
			Background(lipgloss.Color(cfg.Colors.PolicyNameBg)).
			PaddingLeft(1).
			PaddingRight(1).
			Render(name)
	}

	theme.policyMetadataStyle = func(metadata string) string {
		return lipgloss.NewStyle().
			PaddingLeft(2).
			Foreground(lipgloss.Color(cfg.Colors.PolicyMetadata)).
			Render(metadata)
	}

	theme.debugStyle = func(msg string) string {
		return lipgloss.NewStyle().
			Foreground(appconfig.GetForegroundColor(cfg.Colors.Debug)).
			Render(msg)
	}

	return theme, nil
}
