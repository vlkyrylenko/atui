package main

import (
	"bufio"
	"context"
	"encoding/json"
	"fmt"
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
	"log"
	"net/url"
	"os"
	"path/filepath"
	"regexp"
	"strings"

	appconfig "github.com/vlkyrylenko/atui/config"
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
	rolesList         list.Model
	policiesList      list.Model
	loading           bool
	spinner           spinner.Model
	selectedRole      *RoleItem
	policyView        viewport.Model
	selectedPolicy    *PolicyItem
	policyDocument    string
	currentScreen     string
	err               error
	width, height     int
	statusMsg         string
	currentProfile    string
	availableProfiles []string
	profilesList      list.Model
	userArn           string // Store current user ARN
	// Viewport search functionality
	searchMode    bool
	searchQuery   string
	searchResults []int // Line numbers containing matches
	currentMatch  int   // Current match index
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
	} else if i.policyType == "Inline" {
		desc = "[Inline] "
	}

	// For the test to pass, we need to show the policy name directly
	desc += i.policyName
	return desc
}
func (i PolicyItem) FilterValue() string { return i.policyName }

// ProfileItem represents an AWS profile for the list
type ProfileItem struct {
	name string
}

func (i ProfileItem) Title() string       { return i.name }
func (i ProfileItem) Description() string { return "" }
func (i ProfileItem) FilterValue() string { return i.name }

// Key mappings
type keyMap struct {
	Up            key.Binding
	Down          key.Binding
	Enter         key.Binding
	Back          key.Binding
	SwitchProfile key.Binding
	Quit          key.Binding
	Filter        key.Binding // Filter list items
	// Viewport-specific key bindings
	PageUp       key.Binding
	PageDown     key.Binding
	HalfPageUp   key.Binding
	HalfPageDown key.Binding
	GotoTop      key.Binding
	GotoBottom   key.Binding
	Search       key.Binding // Search in viewport
	NextMatch    key.Binding // Navigate to next search match
	PrevMatch    key.Binding // Navigate to previous search match
}

// ShortHelp returns the short help for keybindings
func (k keyMap) ShortHelp() []key.Binding {
	return []key.Binding{k.Up, k.Down, k.SwitchProfile, k.Back, k.Quit}
}

// FullHelp returns the full help for keybindings
func (k keyMap) FullHelp() [][]key.Binding {
	return [][]key.Binding{
		{k.Up, k.Down},
		{k.Enter, k.SwitchProfile},
		{k.Back, k.Quit},
	}
}

// ViewportShortHelp returns short help for viewport screen
func (k keyMap) ViewportShortHelp() []key.Binding {
	return []key.Binding{k.Up, k.Down, k.PageUp, k.PageDown, k.Search, k.NextMatch, k.PrevMatch, k.Back, k.Quit}
}

// ViewportFullHelp returns full help for viewport screen
func (k keyMap) ViewportFullHelp() [][]key.Binding {
	return [][]key.Binding{
		{k.Up, k.Down, k.PageUp, k.PageDown},
		{k.HalfPageUp, k.HalfPageDown, k.GotoTop, k.GotoBottom},
		{k.Back, k.Quit},
	}
}

var keys = keyMap{
	Up: key.NewBinding(
		key.WithKeys("up", "k"),
		key.WithHelp("↑/k", "up"),
	),
	Down: key.NewBinding(
		key.WithKeys("down", "j"),
		key.WithHelp("↓/j", "down"),
	),
	Enter: key.NewBinding(
		key.WithKeys("enter"),
		key.WithHelp("enter", "select"),
	),
	Back: key.NewBinding(
		key.WithKeys("escape", "esc"),
		key.WithHelp("esc", "go back"),
	),
	SwitchProfile: key.NewBinding(
		key.WithKeys("p"),
		key.WithHelp("p", "switch profiles"),
	),
	Quit: key.NewBinding(
		key.WithKeys("q", "ctrl+c"),
		key.WithHelp("q", "quit"),
	),
	// Viewport-specific key bindings
	PageUp: key.NewBinding(
		key.WithKeys("pgup"),
		key.WithHelp("pgup", "scroll up"),
	),
	PageDown: key.NewBinding(
		key.WithKeys("pgdn"),
		key.WithHelp("pgdn", "scroll down"),
	),
	HalfPageUp: key.NewBinding(
		key.WithKeys("shift+pgup"),
		key.WithHelp("shift+pgup", "scroll half page up"),
	),
	HalfPageDown: key.NewBinding(
		key.WithKeys("shift+pgdn"),
		key.WithHelp("shift+pgdn", "scroll half page down"),
	),
	GotoTop: key.NewBinding(
		key.WithKeys("home"),
		key.WithHelp("home", "go to top"),
	),
	GotoBottom: key.NewBinding(
		key.WithKeys("end"),
		key.WithHelp("end", "go to bottom"),
	),
	Search: key.NewBinding(
		key.WithKeys("/"),
		key.WithHelp("/", "search"),
	),
	NextMatch: key.NewBinding(
		key.WithKeys("n"),
		key.WithHelp("n", "next match"),
	),
	PrevMatch: key.NewBinding(
		key.WithKeys("N"),
		key.WithHelp("N", "previous match"),
	),
	Filter: key.NewBinding(
		key.WithKeys("/"),
		key.WithHelp("/", "filter items"),
	),
}

// updateKeyBindingsForScreen updates the help text for key bindings based on the current screen
func updateKeyBindingsForScreen(currentScreen string) {
	switch currentScreen {
	case "roles":
		keys.Enter = key.NewBinding(
			key.WithKeys("enter"),
			key.WithHelp("enter", "select role"),
		)
	case "policies":
		keys.Enter = key.NewBinding(
			key.WithKeys("enter"),
			key.WithHelp("enter", "view policy details"),
		)
	case "policy_document":
		// Enter key is not used in policy document view, so we can hide it or keep generic
		keys.Enter = key.NewBinding(
			key.WithKeys("enter"),
			key.WithHelp("enter", "select"),
		)
	case "profiles":
		keys.Enter = key.NewBinding(
			key.WithKeys("enter"),
			key.WithHelp("enter", "switch profile"),
		)
	default:
		keys.Enter = key.NewBinding(
			key.WithKeys("enter"),
			key.WithHelp("enter", "select"),
		)
	}
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
	rolesList.SetShowHelp(false) // Disable original help bar
	// Create boxed title style
	boxedTitleStyle := lipgloss.NewStyle().
		Background(lipgloss.Color("99")). // Purple background to match logo
		Foreground(lipgloss.Color("15")). // White text
		Bold(true).
		Padding(0, 1).
		MarginLeft(2)
	rolesList.Styles.Title = boxedTitleStyle
	rolesList.Styles.PaginationStyle = appTheme.paginationStyle
	rolesList.Styles.HelpStyle = appTheme.helpStyle
	// Set custom key bindings
	rolesList.AdditionalShortHelpKeys = func() []key.Binding {
		return []key.Binding{keys.Enter, keys.SwitchProfile, keys.Back}
	}
	rolesList.AdditionalFullHelpKeys = func() []key.Binding {
		return []key.Binding{keys.Enter, keys.SwitchProfile, keys.Back}
	}
	rolesList.KeyMap.Quit.SetKeys("ctrl+c")
	rolesList.KeyMap.CloseFullHelp.SetKeys("q")

	// Create a custom delegate for policies with more visible styling
	policyDelegate := list.NewDefaultDelegate()
	policyDelegate.ShowDescription = true
	policyDelegate.SetHeight(3) // Increase height for better visibility
	policyDelegate.Styles.SelectedTitle = appTheme.selectedItemStyle.Bold(true)
	policyDelegate.Styles.SelectedDesc = appTheme.selectedItemStyle.Foreground(lipgloss.Color("240"))
	policyDelegate.Styles.NormalTitle = appTheme.itemStyle.Bold(true)
	policyDelegate.Styles.NormalDesc = appTheme.itemStyle.Foreground(lipgloss.Color("240"))

	policiesList := list.New([]list.Item{}, policyDelegate, 0, 0)
	policiesList.Title = "Policies"
	policiesList.SetShowStatusBar(false)
	policiesList.SetFilteringEnabled(true)
	policiesList.SetShowHelp(false) // Disable original help bar
	policiesList.Styles.Title = boxedTitleStyle
	policiesList.Styles.PaginationStyle = appTheme.paginationStyle
	policiesList.Styles.HelpStyle = appTheme.helpStyle
	// Set custom key bindings
	policiesList.AdditionalShortHelpKeys = func() []key.Binding {
		return []key.Binding{keys.Enter, keys.SwitchProfile, keys.Back}
	}
	policiesList.AdditionalFullHelpKeys = func() []key.Binding {
		return []key.Binding{keys.Enter, keys.SwitchProfile, keys.Back}
	}
	policiesList.KeyMap.Quit.SetKeys("ctrl+c")
	policiesList.KeyMap.CloseFullHelp.SetKeys("q")

	policyView := viewport.New(0, 0)
	policyView.Style = lipgloss.NewStyle().Padding(1, 2)

	profilesList := list.New([]list.Item{}, list.NewDefaultDelegate(), 0, 0)
	profilesList.Title = "AWS Profiles"
	profilesList.SetShowStatusBar(false)
	profilesList.SetFilteringEnabled(true)
	profilesList.SetShowHelp(false) // Disable original help bar
	profilesList.Styles.Title = boxedTitleStyle
	profilesList.Styles.PaginationStyle = appTheme.paginationStyle
	profilesList.Styles.HelpStyle = appTheme.helpStyle
	// Set custom key bindings
	profilesList.AdditionalShortHelpKeys = func() []key.Binding {
		return []key.Binding{keys.Enter, keys.Back}
	}
	profilesList.AdditionalFullHelpKeys = func() []key.Binding {
		return []key.Binding{keys.Enter, keys.Back}
	}
	profilesList.KeyMap.Quit.SetKeys("ctrl+c")
	profilesList.KeyMap.CloseFullHelp.SetKeys("q")

	return model{
		rolesList:     rolesList,
		policiesList:  policiesList,
		spinner:       s,
		loading:       false,
		policyView:    policyView,
		currentScreen: "roles",
		statusMsg:     "Select a role to view its policies",
		profilesList:  profilesList,
	}
}

func (m model) Init() tea.Cmd {
	// Set initial key bindings for the starting screen
	updateKeyBindingsForScreen("roles")
	return tea.Batch(
		m.spinner.Tick,
		loadCurrentProfileCmd(),
		loadIAMRolesCmd(""),
		loadUserArnCmd(""),
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
		// Direct check for Escape key by its type
		if msg.Type == tea.KeyEsc {
			if m.currentScreen == "profiles" {
				m.currentScreen = "roles"
				updateKeyBindingsForScreen(m.currentScreen)
				m.statusMsg = ""
				return m, nil
			} else if m.currentScreen == "policies" {
				m.currentScreen = "roles"
				updateKeyBindingsForScreen(m.currentScreen)
				m.selectedPolicy = nil
				m.statusMsg = ""
				return m, nil
			} else if m.currentScreen == "policy_document" {
				m.currentScreen = "policies"
				updateKeyBindingsForScreen(m.currentScreen)
				m.statusMsg = ""
				return m, nil
			}
		}

		switch {
		case key.Matches(msg, keys.Quit):
			return m, tea.Quit

		case key.Matches(msg, keys.SwitchProfile):
			if m.currentScreen != "profiles" {
				m.currentScreen = "profiles"
				updateKeyBindingsForScreen(m.currentScreen)
				m.loading = true
				// Ensure profiles list is properly sized
				headerHeight := 6
				footerHeight := 3
				verticalMarginHeight := headerHeight + footerHeight
				m.profilesList.SetSize(m.width, m.height-verticalMarginHeight)
				return m, loadAWSProfilesCmd()
			}

		case key.Matches(msg, keys.Back):
			if m.currentScreen == "profiles" {
				m.currentScreen = "roles"
				updateKeyBindingsForScreen(m.currentScreen)
				m.statusMsg = ""
				return m, nil
			} else if m.currentScreen == "policies" {
				m.currentScreen = "roles"
				updateKeyBindingsForScreen(m.currentScreen)
				m.selectedPolicy = nil
				m.statusMsg = ""
				return m, nil
			} else if m.currentScreen == "policy_document" {
				m.currentScreen = "policies"
				updateKeyBindingsForScreen(m.currentScreen)
				m.statusMsg = ""
				return m, nil
			}

		case key.Matches(msg, keys.Enter):
			if m.currentScreen == "roles" {
				if m.rolesList.SelectedItem() == nil {
					return m, nil
				}

				if selected, ok := m.rolesList.SelectedItem().(*RoleItem); ok {
					m.selectedRole = selected
					m.currentScreen = "policies"
					updateKeyBindingsForScreen(m.currentScreen)
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
					updateKeyBindingsForScreen(m.currentScreen)
					m.statusMsg = ""

					// Reset search state when switching policy documents
					m.searchMode = false
					m.searchQuery = ""
					m.searchResults = []int{}
					m.currentMatch = 0

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
			} else if m.currentScreen == "profiles" {
				if m.profilesList.SelectedItem() == nil {
					return m, nil
				}

				if selected, ok := m.profilesList.SelectedItem().(*ProfileItem); ok {
					m.currentProfile = selected.name
					m.statusMsg = fmt.Sprintf("Switched to profile: %s", m.currentProfile)
					m.currentScreen = "roles"
					updateKeyBindingsForScreen(m.currentScreen)
					m.loading = true

					// Clear existing data to force refresh
					m.rolesList.SetItems([]list.Item{})
					m.selectedRole = nil
					m.selectedPolicy = nil

					// Reload roles and user ARN with the new profile
					return m, tea.Batch(
						loadIAMRolesCmd(m.currentProfile),
						loadUserArnCmd(m.currentProfile),
					)
				}
				return m, nil
			}

		// Handle viewport-specific key bindings
		case key.Matches(msg, keys.Search):
			if m.currentScreen == "policy_document" && !m.searchMode {
				m.searchMode = true
				m.searchQuery = ""
				m.searchResults = []int{}
				m.currentMatch = 0
				return m, nil
			}

		case key.Matches(msg, keys.PageUp):
			if m.currentScreen == "policy_document" && !m.searchMode {
				m.policyView.YOffset -= m.policyView.Height
				if m.policyView.YOffset < 0 {
					m.policyView.YOffset = 0
				}
				return m, nil
			}

		case key.Matches(msg, keys.PageDown):
			if m.currentScreen == "policy_document" && !m.searchMode {
				m.policyView.YOffset += m.policyView.Height
				maxOffset := len(strings.Split(m.policyDocument, "\n")) - m.policyView.Height
				if m.policyView.YOffset > maxOffset {
					m.policyView.YOffset = maxOffset
				}
				return m, nil
			}

		case key.Matches(msg, keys.HalfPageUp):
			if m.currentScreen == "policy_document" && !m.searchMode {
				m.policyView.YOffset -= m.policyView.Height / 2
				if m.policyView.YOffset < 0 {
					m.policyView.YOffset = 0
				}
				return m, nil
			}

		case key.Matches(msg, keys.HalfPageDown):
			if m.currentScreen == "policy_document" && !m.searchMode {
				m.policyView.YOffset += m.policyView.Height / 2
				maxOffset := len(strings.Split(m.policyDocument, "\n")) - m.policyView.Height
				if m.policyView.YOffset > maxOffset {
					m.policyView.YOffset = maxOffset
				}
				return m, nil
			}

		case key.Matches(msg, keys.GotoTop):
			if m.currentScreen == "policy_document" && !m.searchMode {
				m.policyView.YOffset = 0
				return m, nil
			}

		case key.Matches(msg, keys.GotoBottom):
			if m.currentScreen == "policy_document" && !m.searchMode {
				maxOffset := len(strings.Split(m.policyDocument, "\n")) - m.policyView.Height
				if maxOffset < 0 {
					maxOffset = 0
				}
				m.policyView.YOffset = maxOffset
				return m, nil
			}

		// Handle search result navigation
		case key.Matches(msg, keys.NextMatch):
			if m.currentScreen == "policy_document" && len(m.searchResults) > 0 {
				m.currentMatch = (m.currentMatch + 1) % len(m.searchResults)
				m.policyView.YOffset = m.searchResults[m.currentMatch]
				return m, nil
			}

		case key.Matches(msg, keys.PrevMatch):
			if m.currentScreen == "policy_document" && len(m.searchResults) > 0 {
				m.currentMatch = (m.currentMatch - 1 + len(m.searchResults)) % len(m.searchResults)
				m.policyView.YOffset = m.searchResults[m.currentMatch]
				return m, nil
			}

		// Handle up/down keys for viewport
		case key.Matches(msg, keys.Up):
			if m.currentScreen == "policy_document" && !m.searchMode {
				if m.policyView.YOffset > 0 {
					m.policyView.YOffset--
				}
				return m, nil
			}

		case key.Matches(msg, keys.Down):
			if m.currentScreen == "policy_document" && !m.searchMode {
				maxOffset := len(strings.Split(m.policyDocument, "\n")) - m.policyView.Height
				if m.policyView.YOffset < maxOffset {
					m.policyView.YOffset++
				}
				return m, nil
			}
		}

		// Handle search mode input
		if m.searchMode && m.currentScreen == "policy_document" {
			switch msg.Type {
			case tea.KeyEsc:
				m.searchMode = false
				m.searchQuery = ""
				m.searchResults = []int{}
				m.currentMatch = 0
				return m, nil
			case tea.KeyEnter:
				if m.searchQuery != "" {
					m.performSearch()
					if len(m.searchResults) > 0 {
						// Jump to first match
						m.policyView.YOffset = m.searchResults[0]
					}
				}
				m.searchMode = false
				return m, nil
			case tea.KeyBackspace:
				if len(m.searchQuery) > 0 {
					m.searchQuery = m.searchQuery[:len(m.searchQuery)-1]
				}
				return m, nil
			default:
				if len(msg.String()) == 1 && msg.String() >= " " {
					m.searchQuery += msg.String()
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
		m.profilesList.SetSize(msg.Width, msg.Height-verticalMarginHeight)
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

		// Update the selected role's policies if a role is selected
		if m.selectedRole != nil {
			m.selectedRole.policies = msg.policies
			m.selectedRole.policiesLoaded = true
		}

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

	case profilesLoadedMsg:
		m.loading = false
		m.availableProfiles = msg.profiles
		m.currentProfile = msg.currentProfile

		// Convert profiles to list items using ProfileItem
		items := []list.Item{}
		for _, profile := range msg.profiles {
			items = append(items, &ProfileItem{name: profile})
		}
		m.profilesList.SetItems(items)
		return m, nil

	case userArnLoadedMsg:
		m.userArn = msg.arn
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
	case "profiles":
		m.profilesList, cmd = m.profilesList.Update(msg)
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

	// Create profile indicator for top right corner
	profileIndicator := ""
	if m.currentProfile != "" {
		profileStyle := lipgloss.NewStyle().
			Background(lipgloss.Color("220")). // Yellow background
			Foreground(lipgloss.Color("0")). // Black text
			Bold(true).
			Padding(0, 1)

		profileText := fmt.Sprintf("Profile: %s", m.currentProfile)
		profileIndicator = profileStyle.Render(profileText)
	}

	var view string

	switch m.currentScreen {
	case "roles":
		// Create header with logo and profile indicator on the same line
		logo := displayLogo()
		header := ""
		if profileIndicator != "" {
			// Calculate spacing to put logo on left, profile on right
			logoWidth := len(stripAnsiCodes(logo))
			profileWidth := len(stripAnsiCodes(profileIndicator))
			spacerWidth := m.width - logoWidth - profileWidth - 2
			if spacerWidth > 0 {
				spacer := strings.Repeat(" ", spacerWidth)
				header = fmt.Sprintf("%s%s%s\n", logo, spacer, profileIndicator)
			} else {
				// Not enough space, put on separate lines
				header = fmt.Sprintf("%s\n%s\n", logo, profileIndicator)
			}
		} else {
			header = fmt.Sprintf("%s\n", logo)
		}

		view = header + "\n" + m.rolesList.View()
		// Status message will be handled in the footer area

	case "policies":
		if m.selectedRole != nil {
			// Create header with logo and profile indicator on the same line
			logo := displayLogo()
			header := ""
			if profileIndicator != "" {
				// Calculate spacing to put logo on left, profile on right
				logoWidth := len(stripAnsiCodes(logo))
				profileWidth := len(stripAnsiCodes(profileIndicator))
				spacerWidth := m.width - logoWidth - profileWidth - 2
				if spacerWidth > 0 {
					spacer := strings.Repeat(" ", spacerWidth)
					header = fmt.Sprintf("%s%s%s\n", logo, spacer, profileIndicator)
				} else {
					// Not enough space, put on separate lines
					header = fmt.Sprintf("%s\n%s\n", logo, profileIndicator)
				}
			} else {
				header = fmt.Sprintf("%s\n", logo)
			}

			view = header + "\n" + m.policiesList.View()
			// Status message will be handled in the footer area
		}

	case "policy_document":
		if m.selectedPolicy != nil {
			// Create header with profile indicator
			header := ""
			if profileIndicator != "" {
				headerWidth := m.width - len(stripAnsiCodes(profileIndicator)) - 2
				if headerWidth > 0 {
					spacer := strings.Repeat(" ", headerWidth)
					header = fmt.Sprintf("%s%s\n", spacer, profileIndicator)
				} else {
					header = fmt.Sprintf("%s\n", profileIndicator)
				}
			}

			// Use the highlighted style for the policy name
			headerStr := fmt.Sprintf("\n  %s\n", appTheme.policyNameHighlightStyle(m.selectedPolicy.policyName))
			if m.selectedPolicy.policyType != "" {
				headerStr += fmt.Sprintf("  %s\n", appTheme.policyMetadataStyle("Type: "+m.selectedPolicy.policyType))
			}
			if m.selectedPolicy.policyArn != "" {
				headerStr += fmt.Sprintf("  %s\n", appTheme.policyMetadataStyle("ARN: "+m.selectedPolicy.policyArn))
			}
			headerStr += "\n"

			// Show search input and match status if in search mode or has results
			searchBar := ""
			if m.searchMode {
				searchStyle := lipgloss.NewStyle().
					Foreground(lipgloss.Color("220")).
					Bold(true).
					PaddingLeft(1)
				searchBar = "\n" + searchStyle.Render(fmt.Sprintf("Search: %s_", m.searchQuery))
			} else if len(m.searchResults) > 0 {
				// Show search results status
				matchStyle := lipgloss.NewStyle().
					Foreground(lipgloss.Color("245")).
					PaddingLeft(1)
				searchBar = "\n" + matchStyle.Render(fmt.Sprintf("Match %d of %d for '%s'", m.currentMatch+1, len(m.searchResults), m.searchQuery))
			}

			// Apply search highlighting if we have search results
			content := m.policyDocument
			if len(m.searchResults) > 0 && m.searchQuery != "" {
				content = m.highlightSearchResults(m.policyDocument, m.searchQuery, m.currentMatch)
			}
			m.policyView.SetContent(content)

			view = header + headerStr + m.policyView.View() + searchBar
		}

	case "profiles":
		// Create header with logo and profile indicator on the same line
		logo := displayLogo()
		header := ""
		if profileIndicator != "" {
			// Calculate spacing to put logo on left, profile on right
			logoWidth := len(stripAnsiCodes(logo))
			profileWidth := len(stripAnsiCodes(profileIndicator))
			spacerWidth := m.width - logoWidth - profileWidth - 2
			if spacerWidth > 0 {
				spacer := strings.Repeat(" ", spacerWidth)
				header = fmt.Sprintf("%s%s%s\n", logo, spacer, profileIndicator)
			} else {
				// Not enough space, put on separate lines
				header = fmt.Sprintf("%s\n%s\n", logo, profileIndicator)
			}
		} else {
			header = fmt.Sprintf("%s\n", logo)
		}

		view = header + "\n" + m.profilesList.View()
		// Status message will be handled in the footer area
	}

	// Create consistent footer with help bar and user ARN for all views
	if m.userArn != "" {
		// Add help bar above Current ARN message based on current screen
		helpBar := ""
		statusBar := ""

		switch m.currentScreen {
		case "policy_document":
			if m.searchMode {
				helpBar += renderSearchHelpBar() + "\n"
			} else {
				helpBar += renderViewportHelpBar() + "\n"
			}
		case "roles", "policies", "profiles":
			// Show general help for list navigation
			helpBar += renderListHelpBar(m.currentScreen) + "\n"
		}

		// Add gap between help bar and current ARN message
		if helpBar != "" {
			helpBar += "\n"
		}

		// Add status message if present
		if m.statusMsg != "" {
			statusStyle := appTheme.statusMessageStyle(m.statusMsg)
			statusBar = statusStyle + "\n"
		}

		userArnStyle := lipgloss.NewStyle().
			Background(lipgloss.Color("42")). // Light green background
			Foreground(lipgloss.Color("0")). // Black text
			Padding(0, 1)

		userArnText := fmt.Sprintf("Current user ARN: %s", m.userArn)
		userArnDisplay := userArnStyle.Render(userArnText)

		// Calculate the height of the main view content
		viewLines := strings.Split(view, "\n")
		contentHeight := len(viewLines)

		// Calculate footer height
		footerHeight := 1 // User ARN takes one line
		if helpBar != "" {
			footerHeight += 1
		}
		if statusBar != "" {
			footerHeight += 1
		}

		// Create padding to push footer to the bottom
		if m.height > contentHeight+footerHeight {
			paddingLines := m.height - contentHeight - footerHeight
			padding := strings.Repeat("\n", paddingLines)
			view = view + padding + statusBar + helpBar + userArnDisplay
		} else {
			// If content is too tall, place at bottom anyway
			view = view + "\n" + statusBar + helpBar + userArnDisplay
		}
	}

	return view
}

// renderViewportHelpBar renders a help bar for viewport navigation
func renderViewportHelpBar() string {
	helpKeys := keys.ViewportShortHelp()
	var helpStrings []string

	for _, binding := range helpKeys {
		helpStrings = append(helpStrings, fmt.Sprintf("%s %s", binding.Help().Key, binding.Help().Desc))
	}

	helpText := strings.Join(helpStrings, " • ")
	helpStyle := lipgloss.NewStyle().
		Foreground(lipgloss.Color("241")).
		PaddingLeft(1)

	return helpStyle.Render(helpText)
}

// renderSearchHelpBar renders a help bar for search mode
func renderSearchHelpBar() string {
	helpItems := []string{
		"enter confirm search",
		"esc exit search",
		"backspace delete char",
		"type to search",
		"n next match",
		"N previous match",
	}

	helpText := strings.Join(helpItems, " • ")
	helpStyle := lipgloss.NewStyle().
		Foreground(lipgloss.Color("241")).
		PaddingLeft(1)

	return helpStyle.Render(helpText)
}

// renderListHelpBar renders a help bar for list navigation using the same code as original lists
func renderListHelpBar(currentScreen string) string {
	// Use the exact same help rendering as the original list components
	var helpKeys []key.Binding

	switch currentScreen {
	case "roles", "policies":
		// Use the same keys that were defined in AdditionalShortHelpKeys for roles/policies, plus filter
		helpKeys = []key.Binding{keys.Enter, keys.Filter, keys.SwitchProfile, keys.Back}
	case "profiles":
		// Use the same keys that were defined in AdditionalShortHelpKeys for profiles, plus filter
		helpKeys = []key.Binding{keys.Enter, keys.Filter, keys.Back}
	default:
		helpKeys = []key.Binding{}
	}

	// Add the default list navigation keys (up/down) and quit, matching the original pattern
	allKeys := []key.Binding{keys.Up, keys.Down}
	allKeys = append(allKeys, helpKeys...)
	allKeys = append(allKeys, keys.Quit)

	// Use the exact same formatting logic as the list component's help system
	var helpStrings []string
	for _, binding := range allKeys {
		helpStrings = append(helpStrings, fmt.Sprintf("%s %s", binding.Help().Key, binding.Help().Desc))
	}

	helpText := strings.Join(helpStrings, " • ")
	helpStyle := lipgloss.NewStyle().
		Foreground(lipgloss.Color("241")).
		PaddingLeft(1)

	return helpStyle.Render(helpText)
}

// performSearch searches for the query in the policy document and stores line numbers with matches
func (m *model) performSearch() {
	if m.searchQuery == "" {
		m.searchResults = []int{}
		return
	}

	lines := strings.Split(m.policyDocument, "\n")
	m.searchResults = []int{}

	// Case-insensitive search
	query := strings.ToLower(m.searchQuery)

	for i, line := range lines {
		if strings.Contains(strings.ToLower(stripAnsiCodes(line)), query) {
			m.searchResults = append(m.searchResults, i)
		}
	}

	m.currentMatch = 0
}

// highlightSearchResults highlights search matches in the document content
func (m *model) highlightSearchResults(content, query string, currentMatchIndex int) string {
	if query == "" || len(m.searchResults) == 0 {
		return content
	}

	lines := strings.Split(content, "\n")
	query = strings.ToLower(query)

	// Create highlight styles
	matchStyle := lipgloss.NewStyle().
		Background(lipgloss.Color("11")). // Bright yellow background
		Foreground(lipgloss.Color("0")). // Black text
		Bold(true)

	currentMatchStyle := lipgloss.NewStyle().
		Background(lipgloss.Color("201")). // Bright magenta background
		Foreground(lipgloss.Color("15")). // White text
		Bold(true)

	// Track which line we're currently highlighting as the active match
	currentMatchLine := -1
	if currentMatchIndex >= 0 && currentMatchIndex < len(m.searchResults) {
		currentMatchLine = m.searchResults[currentMatchIndex]
	}

	// Apply highlighting to each line that contains matches
	for i, line := range lines {
		// Check if this line contains the search term
		if strings.Contains(strings.ToLower(stripAnsiCodes(line)), query) {
			// Determine which highlight style to use
			style := matchStyle
			if i == currentMatchLine {
				style = currentMatchStyle
			}

			// Use case-insensitive replacement but preserve original case
			re := regexp.MustCompile("(?i)" + regexp.QuoteMeta(query))
			lines[i] = re.ReplaceAllStringFunc(line, func(match string) string {
				return style.Render(match)
			})
		}
	}

	return strings.Join(lines, "\n")
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

type profilesLoadedMsg struct {
	profiles       []string
	currentProfile string
}

type userArnLoadedMsg struct {
	arn string
}

type errorMsg error

// Load IAM roles from AWS
func loadIAMRolesCmd(profile string) tea.Cmd {
	return func() tea.Msg {
		ctx := context.Background()

		// Load AWS configuration with specified profile
		var cfg aws.Config
		var err error
		if profile != "" {
			cfg, err = config.LoadDefaultConfig(ctx, config.WithSharedConfigProfile(profile))
		} else {
			cfg, err = config.LoadDefaultConfig(ctx)
		}
		if err != nil {
			return errorMsg(fmt.Errorf("error loading AWS configuration: %w", err))
		}

		// Create clients
		iamClient := iam.NewFromConfig(cfg)
		stsClient := sts.NewFromConfig(cfg)

		// Get current user identity to determine available roles
		identity, err := stsClient.GetCallerIdentity(ctx, &sts.GetCallerIdentityInput{})
		if err != nil {
			return errorMsg(fmt.Errorf("error getting caller identity: %w", err))
		}

		userArn := aws.ToString(identity.Arn)
		var roles []RoleItem

		// If user is already assuming a role, add current role to the list
		if strings.Contains(userArn, ":assumed-role/") {
			// Extract role name from assumed role ARN
			parts := strings.Split(userArn, "/")
			if len(parts) >= 2 {
				roleName := parts[1]
				roleArn := fmt.Sprintf("arn:aws:iam::%s:role/%s", *identity.Account, roleName)

				roles = append(roles, RoleItem{
					roleName:    roleName,
					roleArn:     roleArn,
					description: "Current assumed role",
				})
			}
		}

		// Try to list roles user can access (may fail with limited permissions)
		paginator := iam.NewListRolesPaginator(iamClient, &iam.ListRolesInput{})
		for paginator.HasMorePages() {
			page, err := paginator.NextPage(ctx)
			if err != nil {
				// If we can't list roles, just return current role if available
				if len(roles) > 0 {
					break
				}
				if strings.Contains(err.Error(), "AccessDenied") || strings.Contains(err.Error(), "UnauthorizedOperation") {
					return errorMsg(fmt.Errorf("insufficient permissions to list IAM roles."))
				}
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

// Load current user ARN
func loadUserArnCmd(profile string) tea.Cmd {
	return func() tea.Msg {
		ctx := context.Background()

		// Load AWS configuration with specified profile
		var cfg aws.Config
		var err error
		if profile != "" {
			cfg, err = config.LoadDefaultConfig(ctx, config.WithSharedConfigProfile(profile))
		} else {
			cfg, err = config.LoadDefaultConfig(ctx)
		}
		if err != nil {
			return errorMsg(fmt.Errorf("error loading AWS configuration: %w", err))
		}

		// Create STS client
		stsClient := sts.NewFromConfig(cfg)

		// Get caller identity to determine current user/role
		identity, err := stsClient.GetCallerIdentity(ctx, &sts.GetCallerIdentityInput{})
		if err != nil {
			return errorMsg(fmt.Errorf("error getting caller identity: %w", err))
		}

		userArn := aws.ToString(identity.Arn)
		return userArnLoadedMsg{arn: userArn}
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

// Strip ANSI color codes from text
func stripAnsiCodes(text string) string {
	// Remove ANSI escape sequences
	ansiRegex := regexp.MustCompile(`\x1b\[[0-9;]*m`)
	return ansiRegex.ReplaceAllString(text, "")
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

// displayLogo returns a simple colorful logo for display in the UI
func displayLogo() string {
	style := lipgloss.NewStyle().Foreground(lipgloss.Color("99")).Bold(true)
	return style.Render("🌈 AWS Terminal UI 🌈")
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
	theme.titleStyle = lipgloss.NewStyle().MarginLeft(2).Foreground(lipgloss.Color("205")) // Set pink color
	// The below configuration is overridden by our pink setting above, but kept for backwards compatibility
	if cfg.Colors.Title == "bold" {
		theme.titleStyle = theme.titleStyle.Bold(true)
	} else if cfg.Colors.Title != "" {
		appconfig.GetForegroundColor(cfg.Colors.Title)
		theme.titleStyle = theme.titleStyle.Foreground(lipgloss.Color("205")) // Ensure pink color is applied
	}

	theme.itemStyle = lipgloss.NewStyle().PaddingLeft(4)
	if cfg.Colors.Item != "" {
		theme.itemStyle = theme.itemStyle.Foreground(appconfig.GetForegroundColor(cfg.Colors.Item))
	}

	theme.selectedItemStyle = lipgloss.NewStyle().
		PaddingLeft(2).
		Foreground(appconfig.GetForegroundColor(cfg.Colors.SelectedItem))

	theme.paginationStyle = list.DefaultStyles().PaginationStyle.PaddingLeft(4)
	theme.helpStyle = list.DefaultStyles().HelpStyle.PaddingLeft(1).PaddingBottom(1)

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

// Load AWS profiles from config files
func loadAWSProfilesCmd() tea.Cmd {
	return func() tea.Msg {
		// Get home directory
		homeDir, err := os.UserHomeDir()
		if err != nil {
			return errorMsg(fmt.Errorf("error getting home directory: %w", err))
		}

		// Read AWS config file
		configPath := filepath.Join(homeDir, ".aws", "config")
		credentialsPath := filepath.Join(homeDir, ".aws", "credentials")

		profiles := make(map[string]bool)

		// Parse config file
		if file, err := os.Open(configPath); err == nil {
			defer func() { _ = file.Close() }()
			scanner := bufio.NewScanner(file)
			for scanner.Scan() {
				line := strings.TrimSpace(scanner.Text())
				if strings.HasPrefix(line, "[profile ") && strings.HasSuffix(line, "]") {
					profileName := strings.TrimPrefix(line, "[profile ")
					profileName = strings.TrimSuffix(profileName, "]")
					profiles[profileName] = true
				} else if line == "[default]" {
					profiles["default"] = true
				}
			}
		}

		// Parse credentials file
		if file, err := os.Open(credentialsPath); err == nil {
			defer func() { _ = file.Close() }()
			scanner := bufio.NewScanner(file)
			for scanner.Scan() {
				line := strings.TrimSpace(scanner.Text())
				if strings.HasPrefix(line, "[") && strings.HasSuffix(line, "]") {
					profileName := strings.TrimPrefix(line, "[")
					profileName = strings.TrimSuffix(profileName, "]")
					profiles[profileName] = true
				}
			}
		}

		// Convert map to slice
		var profileList []string
		for profile := range profiles {
			profileList = append(profileList, profile)
		}

		// Get current profile from environment or default
		currentProfile := os.Getenv("AWS_PROFILE")
		if currentProfile == "" {
			currentProfile = "default"
		}

		return profilesLoadedMsg{
			profiles:       profileList,
			currentProfile: currentProfile,
		}
	}
}

// Load current AWS profile
func loadCurrentProfileCmd() tea.Cmd {
	return func() tea.Msg {
		// Get current profile from environment
		currentProfile := os.Getenv("AWS_PROFILE")
		if currentProfile == "" {
			currentProfile = "default"
		}

		return profilesLoadedMsg{
			profiles:       []string{}, // Empty list, we just set the current profile
			currentProfile: currentProfile,
		}
	}
}
