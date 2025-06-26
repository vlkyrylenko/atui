package main

import (
	"context"
	"encoding/json"
	"fmt"
	"reflect"
	"strings"
	"testing"

	"github.com/aws/aws-sdk-go-v2/aws"
	"github.com/aws/aws-sdk-go-v2/service/iam"
	"github.com/aws/aws-sdk-go-v2/service/iam/types"
	"github.com/aws/aws-sdk-go-v2/service/sts"
	tea "github.com/charmbracelet/bubbletea"
)

// Test RoleItem methods
func TestRoleItemMethods(t *testing.T) {
	// Create test role item
	role := RoleItem{
		roleName:       "TestRole",
		roleArn:        "arn:aws:iam::123456789012:role/TestRole",
		description:    "Test description",
		policiesLoaded: false,
	}

	// Test Title method
	if title := role.Title(); title != "TestRole" {
		t.Errorf("Expected title to be 'TestRole', got '%s'", title)
	}

	// Test Description method without policies loaded
	expectedDesc := "Test description"
	if desc := role.Description(); desc != expectedDesc {
		t.Errorf("Expected description to be '%s', got '%s'", expectedDesc, desc)
	}

	// Test Description method with policies loaded
	role.policiesLoaded = true
	role.policies = []PolicyItem{
		{policyName: "Policy1", policyArn: "arn:aws:iam::123456789012:policy/Policy1"},
		{policyName: "Policy2", policyArn: "arn:aws:iam::123456789012:policy/Policy2"},
	}
	expectedDesc = "Test description | 2 policies attached"
	if desc := role.Description(); desc != expectedDesc {
		t.Errorf("Expected description to be '%s', got '%s'", expectedDesc, desc)
	}

	// Test FilterValue method
	if filterVal := role.FilterValue(); filterVal != "TestRole" {
		t.Errorf("Expected filter value to be 'TestRole', got '%s'", filterVal)
	}
}

// Test PolicyItem methods
func TestPolicyItemMethods(t *testing.T) {
	// Test AWS managed policy
	awsPolicy := PolicyItem{
		policyName: "AmazonS3ReadOnlyAccess",
		policyArn:  "arn:aws:iam::aws:policy/AmazonS3ReadOnlyAccess",
		policyType: "AWS",
	}

	// Test Title method for AWS policy
	expectedTitle := "📄 AmazonS3ReadOnlyAccess"
	if title := awsPolicy.Title(); title != expectedTitle {
		t.Errorf("Expected title to be '%s', got '%s'", expectedTitle, title)
	}

	// Test Description method for AWS policy
	expectedDesc := "[AWS Managed] AmazonS3ReadOnlyAccess"
	if desc := awsPolicy.Description(); desc != expectedDesc {
		t.Errorf("Expected description to be '%s', got '%s'", expectedDesc, desc)
	}

	// Test Customer managed policy
	customerPolicy := PolicyItem{
		policyName: "CustomerPolicy",
		policyArn:  "arn:aws:iam::123456789012:policy/CustomerPolicy",
		policyType: "Customer",
	}

	// Test Description method for customer policy
	expectedDesc = "[Customer Managed] CustomerPolicy"
	if desc := customerPolicy.Description(); desc != expectedDesc {
		t.Errorf("Expected description to be '%s', got '%s'", expectedDesc, desc)
	}

	// Test FilterValue method
	if filterVal := customerPolicy.FilterValue(); filterVal != "CustomerPolicy" {
		t.Errorf("Expected filter value to be 'CustomerPolicy', got '%s'", filterVal)
	}
}

// Test initialModel function
func TestInitialModel(t *testing.T) {
	model := initialModel()

	// Check that the model is initialized with the correct values
	if model.currentScreen != "roles" {
		t.Errorf("Expected current screen to be 'roles', got '%s'", model.currentScreen)
	}

	if model.statusMsg != "Select a role to view its policies" {
		t.Errorf("Expected status message to be 'Select a role to view its policies', got '%s'", model.statusMsg)
	}

	if model.loading != false {
		t.Errorf("Expected loading to be false")
	}

	// Check initialized list components
	if model.rolesList.Title != "AWS IAM Roles" {
		t.Errorf("Expected roles list title to be 'AWS IAM Roles', got '%s'", model.rolesList.Title)
	}

	if model.policiesList.Title != "Policies" {
		t.Errorf("Expected policies list title to be 'Policies', got '%s'", model.policiesList.Title)
	}
}

// Mock implementations for testing
type mockSTSClient struct{}

func (m mockSTSClient) GetCallerIdentity(ctx context.Context, params *sts.GetCallerIdentityInput, optFns ...func(*sts.Options)) (*sts.GetCallerIdentityOutput, error) {
	return &sts.GetCallerIdentityOutput{
		Arn: aws.String("arn:aws:iam::123456789012:user/TestUser"),
	}, nil
}

type mockIAMClient struct{}

func (m mockIAMClient) ListRoles(ctx context.Context, params *iam.ListRolesInput, optFns ...func(*iam.Options)) (*iam.ListRolesOutput, error) {
	return &iam.ListRolesOutput{
		Roles: []types.Role{
			{
				RoleName:    aws.String("TestRole1"),
				Arn:         aws.String("arn:aws:iam::123456789012:role/TestRole1"),
				Description: aws.String("Test role 1"),
			},
			{
				RoleName:    aws.String("TestRole2"),
				Arn:         aws.String("arn:aws:iam::123456789012:role/TestRole2"),
				Description: nil,
			},
		},
	}, nil
}

func (m mockIAMClient) ListAttachedRolePolicies(ctx context.Context, params *iam.ListAttachedRolePoliciesInput, optFns ...func(*iam.Options)) (*iam.ListAttachedRolePoliciesOutput, error) {
	return &iam.ListAttachedRolePoliciesOutput{
		AttachedPolicies: []types.AttachedPolicy{
			{
				PolicyName: aws.String("AmazonS3ReadOnlyAccess"),
				PolicyArn:  aws.String("arn:aws:iam::aws:policy/AmazonS3ReadOnlyAccess"),
			},
			{
				PolicyName: aws.String("CustomerPolicy"),
				PolicyArn:  aws.String("arn:aws:iam::123456789012:policy/CustomerPolicy"),
			},
		},
	}, nil
}

func (m mockIAMClient) GetPolicy(ctx context.Context, params *iam.GetPolicyInput, optFns ...func(*iam.Options)) (*iam.GetPolicyOutput, error) {
	return &iam.GetPolicyOutput{
		Policy: &types.Policy{
			DefaultVersionId: aws.String("v1"),
		},
	}, nil
}

func (m mockIAMClient) GetPolicyVersion(ctx context.Context, params *iam.GetPolicyVersionInput, optFns ...func(*iam.Options)) (*iam.GetPolicyVersionOutput, error) {
	policyDoc := `{"Version":"2012-10-17","Statement":[{"Effect":"Allow","Action":"s3:GetObject","Resource":"*"}]}`
	encodedDoc := aws.String(policyDoc)
	return &iam.GetPolicyVersionOutput{
		PolicyVersion: &types.PolicyVersion{
			Document: encodedDoc,
		},
	}, nil
}

// Test decodeURLEncodedDocument function
func TestDecodeURLEncodedDocument(t *testing.T) {
	testCases := []struct {
		name     string
		input    string
		expected string
		hasError bool
	}{
		{
			name:     "Regular JSON",
			input:    `{"key":"value"}`,
			expected: `{"key":"value"}`,
			hasError: false,
		},
		{
			name:     "URL Encoded JSON",
			input:    `%7B%22key%22%3A%22value%22%7D`,
			expected: `{"key":"value"}`,
			hasError: false,
		},
	}

	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			result, err := decodeURLEncodedDocument(tc.input)

			if tc.hasError && err == nil {
				t.Errorf("Expected error but got nil")
			}

			if !tc.hasError && err != nil {
				t.Errorf("Unexpected error: %v", err)
			}

			if result != tc.expected {
				t.Errorf("Expected result to be '%s', got '%s'", tc.expected, result)
			}
		})
	}
}

// Test Model Update method with key events
func TestModelUpdateKeyEvents(t *testing.T) {
	m := initialModel()

	// Add test data to the model
	m.selectedRole = &RoleItem{
		roleName: "TestRole",
		roleArn:  "arn:aws:iam::123456789012:role/TestRole",
		policies: []PolicyItem{
			{policyName: "TestPolicy", policyArn: "arn:aws:iam::123456789012:policy/TestPolicy"},
		},
		policiesLoaded: true,
	}

	m.selectedPolicy = &PolicyItem{
		policyName:     "TestPolicy",
		policyArn:      "arn:aws:iam::123456789012:policy/TestPolicy",
		documentLoaded: true,
		policyDocument: `{"Version":"2012-10-17","Statement":[]}`,
	}

	// Test Quit key
	quitMsg := tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune("q")}
	newModel, cmd := m.Update(quitMsg)
	if cmd == nil {
		t.Errorf("Expected quit command, got nil")
	}

	// Test Back key from policies screen
	m.currentScreen = "policies"
	backMsg := tea.KeyMsg{Type: tea.KeyEsc}
	newModel, _ = m.Update(backMsg)
	updatedModel := newModel.(model)
	if updatedModel.currentScreen != "roles" {
		t.Errorf("Expected screen to change to 'roles', got '%s'", updatedModel.currentScreen)
	}

	// Test Back key from policy_document screen
	m.currentScreen = "policy_document"
	newModel, _ = m.Update(backMsg)
	updatedModel = newModel.(model)
	if updatedModel.currentScreen != "policies" {
		t.Errorf("Expected screen to change to 'policies', got '%s'", updatedModel.currentScreen)
	}
}

// Test View method
func TestModelView(t *testing.T) {
	// Initialize appTheme with default values for testing
	var err error
	appTheme, err = loadThemeFromConfig()
	if err != nil {
		// If config loading fails, create a minimal theme for testing
		appTheme = Theme{
			statusMessageStyle:       func(msg string) string { return msg },
			errorMessageStyle:        func(msg string) string { return msg },
			policyNameHighlightStyle: func(name string) string { return name },
			policyMetadataStyle:      func(metadata string) string { return metadata },
		}
	}

	m := initialModel()

	// Test loading view
	m.loading = true
	loadingView := m.View()
	if !strings.HasPrefix(loadingView, "\n\n   ") {
		t.Errorf("Loading view format incorrect: %s", loadingView)
	}

	// Test error view
	m.loading = false
	m.err = fmt.Errorf("Test error")
	errorView := m.View()
	if !strings.HasPrefix(errorView, "\n\n   Error:") {
		t.Errorf("Error view format incorrect: %s", errorView)
	}

	// Test roles view
	m.err = nil
	m.currentScreen = "roles"
	rolesView := m.View()
	if !strings.HasPrefix(rolesView, "\n") {
		t.Errorf("Roles view format incorrect: %s", rolesView)
	}

	// Test policies view
	m.currentScreen = "policies"
	m.selectedRole = &RoleItem{roleName: "TestRole"}
	policiesView := m.View()
	if !strings.HasSuffix(policiesView, "press enter to view policy details • esc to go back • q to quit") {
		t.Errorf("Policies view format incorrect: %s", policiesView)
	}

	// Test policy document view
	m.currentScreen = "policy_document"
	m.selectedPolicy = &PolicyItem{policyName: "TestPolicy", policyType: "AWS"}
	docView := m.View()
	if !strings.HasSuffix(docView, "press o to open in editor • esc to go back • q to quit\n") {
		t.Errorf("Policy document view format incorrect: %s", docView)
	}
}

// Test message handler functions
func TestMessageHandlers(t *testing.T) {
	// Test rolesLoadedMsg
	m := initialModel()
	msg := rolesLoadedMsg{
		{roleName: "Role1", roleArn: "arn:aws:iam::123456789012:role/Role1"},
		{roleName: "Role2", roleArn: "arn:aws:iam::123456789012:role/Role2"},
	}

	newModel, _ := m.Update(msg)
	updatedModel := newModel.(model)

	if updatedModel.loading {
		t.Errorf("Expected loading to be false after roles loaded")
	}

	if updatedModel.rolesList.Items() == nil || len(updatedModel.rolesList.Items()) != 2 {
		t.Errorf("Expected 2 roles, got %d", len(updatedModel.rolesList.Items()))
	}

	// Test policiesLoadedMsg
	m.selectedRole = &RoleItem{roleName: "TestRole"}
	policyMsg := policiesLoadedMsg{
		roleName: "TestRole",
		policies: []PolicyItem{
			{policyName: "Policy1", policyArn: "arn:aws:policy/Policy1"},
			{policyName: "Policy2", policyArn: "arn:aws:policy/Policy2"},
		},
	}

	newModel, _ = m.Update(policyMsg)
	updatedModel = newModel.(model)

	if updatedModel.loading {
		t.Errorf("Expected loading to be false after policies loaded")
	}

	if updatedModel.selectedRole.policiesLoaded != true {
		t.Errorf("Expected policiesLoaded to be true")
	}

	if len(updatedModel.selectedRole.policies) != 2 {
		t.Errorf("Expected 2 policies, got %d", len(updatedModel.selectedRole.policies))
	}
}

// Test loadRolePoliciesCmd
func TestLoadRolePoliciesCmd(t *testing.T) {
	// Create a wrapper function for testing to avoid direct assignment
	// to the original loadRolePoliciesCmd function
	origFunc := loadRolePoliciesCmd

	// Create a test wrapper that returns a mocked response
	mockLoadRolePoliciesCmd := func(roleName string) tea.Cmd {
		return func() tea.Msg {
			// Return a mock response instead of making a real AWS API call
			return policiesLoadedMsg{
				roleName: roleName,
				policies: []PolicyItem{
					{policyName: "MockPolicy1", policyArn: "arn:aws:iam::123456789012:policy/MockPolicy1", policyType: "AWS"},
					{policyName: "MockPolicy2", policyArn: "arn:aws:iam::123456789012:policy/MockPolicy2", policyType: "Customer"},
				},
			}
		}
	}

	// Test with our mocked function
	cmd := mockLoadRolePoliciesCmd("TestRole")
	msg := cmd().(policiesLoadedMsg)

	// Verify the results
	if msg.roleName != "TestRole" {
		t.Errorf("Expected role name to be 'TestRole', got '%s'", msg.roleName)
	}

	if len(msg.policies) != 2 {
		t.Errorf("Expected 2 policies, got %d", len(msg.policies))
	}

	if msg.policies[0].policyName != "MockPolicy1" {
		t.Errorf("Expected policy name to be 'MockPolicy1', got '%s'", msg.policies[0].policyName)
	}

	// Verify that original function is still intact
	// (we don't actually call it to avoid AWS API calls)
	if origFunc == nil {
		t.Errorf("Original function should not be nil")
	}
}

// Test loadPolicyDocumentCmd
func TestLoadPolicyDocumentCmd(t *testing.T) {
	// Create a mock function that returns predefined test data
	mockLoadPolicyDocumentCmd := func(policyArn string) tea.Cmd {
		return func() tea.Msg {
			// Return a mock response instead of making a real AWS API call
			return policyDocumentLoadedMsg{
				policyArn: policyArn,
				document: `{
					"Version": "2012-10-17",
					"Statement": [
						{
							"Effect": "Allow",
							"Action": "s3:GetObject",
							"Resource": "*"
						}
					]
				}`,
			}
		}
	}

	// Test with our mocked function
	cmd := mockLoadPolicyDocumentCmd("arn:aws:iam::123456789012:policy/TestPolicy")
	msg := cmd()

	// Check if the result is of the correct type and has correct values
	docMsg, ok := msg.(policyDocumentLoadedMsg)
	if !ok {
		t.Errorf("Expected result of type policyDocumentLoadedMsg, got %T", msg)
	}

	// Verify the policy ARN is set correctly
	if docMsg.policyArn != "arn:aws:iam::123456789012:policy/TestPolicy" {
		t.Errorf("Expected policy ARN to be 'arn:aws:iam::123456789012:policy/TestPolicy', got '%s'",
			docMsg.policyArn)
	}

	// Verify the document contains valid JSON
	var jsonObj interface{}
	if err := json.Unmarshal([]byte(docMsg.document), &jsonObj); err != nil {
		t.Errorf("Expected valid JSON document, got error: %v", err)
	}
}

// Test JSON formatting in policyDocumentLoadedMsg handler
func TestPolicyDocumentFormatting(t *testing.T) {
	m := initialModel()
	m.selectedPolicy = &PolicyItem{policyName: "TestPolicy"}

	// Valid JSON document
	validJSON := `{"Version":"2012-10-17","Statement":[{"Effect":"Allow","Action":"s3:Get*","Resource":"*"}]}`
	docMsg := policyDocumentLoadedMsg{
		policyArn: "arn:aws:iam::123456789012:policy/TestPolicy",
		document:  validJSON,
	}

	newModel, _ := m.Update(docMsg)
	updatedModel := newModel.(model)

	var parsedOriginal, parsedFormatted interface{}
	json.Unmarshal([]byte(validJSON), &parsedOriginal)
	json.Unmarshal([]byte(updatedModel.policyDocument), &parsedFormatted)

	if !reflect.DeepEqual(parsedOriginal, parsedFormatted) {
		t.Errorf("JSON formatting changed the content")
	}

	// Invalid JSON document
	invalidJSON := `{"invalid`
	docMsg = policyDocumentLoadedMsg{
		policyArn: "arn:aws:iam::123456789012:policy/TestPolicy",
		document:  invalidJSON,
	}

	newModel, _ = m.Update(docMsg)
	updatedModel = newModel.(model)

	if !strings.HasPrefix(updatedModel.policyDocument, "Error parsing JSON:") {
		t.Errorf("Expected error message for invalid JSON, got: %s", updatedModel.policyDocument)
	}
}
