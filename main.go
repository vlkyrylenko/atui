package main

import (
	"context"
	"encoding/json"
	"fmt"
	"log"
	"net/url"
	"os"
	"os/exec"
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
)

// Styles
var (
	titleStyle         = lipgloss.NewStyle().MarginLeft(2).Bold(true)
	itemStyle          = lipgloss.NewStyle().PaddingLeft(4)
	selectedItemStyle  = lipgloss.NewStyle().PaddingLeft(2).Foreground(lipgloss.Color("170"))
	paginationStyle    = list.DefaultStyles().PaginationStyle.PaddingLeft(4)
	helpStyle          = list.DefaultStyles().HelpStyle.PaddingLeft(4).PaddingBottom(1)
	statusMessageStyle = lipgloss.NewStyle().
		Foreground(lipgloss.AdaptiveColor{Light: "#04B575", Dark: "#04B575"}).
		Render
	errorMessageStyle = lipgloss.NewStyle().
		Foreground(lipgloss.AdaptiveColor{Light: "#FF0000", Dark: "#FF0000"}).
		Render
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
}

// RoleItem represents an IAM role
type RoleItem struct {
	roleName       string
	roleArn        string
	description    string
	policies       []PolicyItem
	policiesLoaded bool
}

// PolicyItem represents an IAM policy
type PolicyItem struct {
	policyName     string
	policyArn      string
	policyDocument string
	documentLoaded bool
}

func (i RoleItem) Title() string       { return i.roleName }
func (i RoleItem) Description() string { return i.description }
func (i RoleItem) FilterValue() string { return i.roleName }

func (i PolicyItem) Title() string       { return i.policyName }
func (i PolicyItem) Description() string { return i.policyArn }
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
	rolesList.SetFilteringEnabled(false)
	rolesList.Styles.Title = titleStyle
	rolesList.Styles.PaginationStyle = paginationStyle
	rolesList.Styles.HelpStyle = helpStyle

	policyDelegate := list.NewDefaultDelegate()
	policiesList := list.New([]list.Item{}, policyDelegate, 0, 0)
	policiesList.Title = "Policies"
	policiesList.SetShowStatusBar(false)
	policiesList.SetFilteringEnabled(false)
	policiesList.Styles.Title = titleStyle
	policiesList.Styles.PaginationStyle = paginationStyle
	policiesList.Styles.HelpStyle = helpStyle

	policyView := viewport.New(0, 0)
	policyView.Style = lipgloss.NewStyle().Padding(1, 2)

	return model{
		rolesList:     rolesList,
		policiesList:  policiesList,
		spinner:       s,
		loading:       false,
		policyView:    policyView,
		currentScreen: "roles",
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
				return m, nil
			} else if m.currentScreen == "policy_document" {
				m.currentScreen = "policies"
				return m, nil
			}

		case key.Matches(msg, keys.OpenJSON):
			if m.currentScreen == "policy_document" && m.selectedPolicy != nil {
				return m, openInEditorCmd(m.policyDocument)
			}

		case key.Matches(msg, keys.Enter):
			if m.currentScreen == "roles" {
				if selected, ok := m.rolesList.SelectedItem().(*RoleItem); ok {
					m.selectedRole = selected
					m.currentScreen = "policies"
					m.policiesList.Title = fmt.Sprintf("Policies for %s", m.selectedRole.roleName)

					if !m.selectedRole.policiesLoaded {
						m.loading = true
						return m, loadRolePoliciesCmd(m.selectedRole.roleName)
					} else {
						// Update policy list with existing policies
						items := []list.Item{}
						for _, p := range m.selectedRole.policies {
							pCopy := p // Create a copy to avoid issues with loop variables in closures
							items = append(items, &pCopy)
						}
						m.policiesList.SetItems(items)
					}
				}
				return m, nil
			} else if m.currentScreen == "policies" {
				if selected, ok := m.policiesList.SelectedItem().(*PolicyItem); ok {
					m.selectedPolicy = selected
					m.currentScreen = "policy_document"

					if !m.selectedPolicy.documentLoaded {
						m.loading = true
						return m, loadPolicyDocumentCmd(m.selectedPolicy.policyArn)
					} else {
						m.policyDocument = m.selectedPolicy.policyDocument
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

		if m.currentScreen == "roles" {
			m.rolesList.SetSize(msg.Width, msg.Height-verticalMarginHeight)
		} else if m.currentScreen == "policies" {
			m.policiesList.SetSize(msg.Width, msg.Height-verticalMarginHeight)
		} else if m.currentScreen == "policy_document" {
			m.policyView.Width = msg.Width
			m.policyView.Height = msg.Height - verticalMarginHeight
		}

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
				m.policyDocument = string(prettyJSON)
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
		return fmt.Sprintf("\n\n   Error: %s\n\n", errorMessageStyle(m.err.Error()))
	}

	switch m.currentScreen {
	case "roles":
		return "\n" + m.rolesList.View()
	case "policies":
		if m.selectedRole != nil {
			return "\n" + m.policiesList.View()
		}
	case "policy_document":
		if m.selectedPolicy != nil {
			headerStr := fmt.Sprintf("\n  %s\n\n", titleStyle.Render(m.selectedPolicy.policyName))
			helpStr := "\n\n  press o to open in editor • esc to go back • q to quit\n"
			return headerStr + m.policyView.View() + helpStr
		}
	}

	return "Something went wrong!"
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
			return errorMsg(fmt.Errorf("error loading AWS configuration: %w", err))
		}

		// Create IAM client
		iamClient := iam.NewFromConfig(cfg)

		// Get attached role policies
		var policies []PolicyItem
		paginator := iam.NewListAttachedRolePoliciesPaginator(iamClient, &iam.ListAttachedRolePoliciesInput{
			RoleName: aws.String(roleName),
		})

		for paginator.HasMorePages() {
			page, err := paginator.NextPage(ctx)
			if err != nil {
				return errorMsg(fmt.Errorf("error listing policies for role %s: %w", roleName, err))
			}

			for _, policy := range page.AttachedPolicies {
				policies = append(policies, PolicyItem{
					policyName: aws.ToString(policy.PolicyName),
					policyArn:  aws.ToString(policy.PolicyArn),
				})
			}
		}

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

func main() {
	p := tea.NewProgram(initialModel(), tea.WithAltScreen())
	if _, err := p.Run(); err != nil {
		log.Fatal(err)
	}
}
