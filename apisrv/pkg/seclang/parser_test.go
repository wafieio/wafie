package seclang

import (
	"testing"
)

func TestParseRule(t *testing.T) {
	testCases := []struct {
		name        string
		ruleText    string
		expected    *SecRule
		expectError bool
	}{
		{
			name:     "Basic attack detection rule",
			ruleText: `SecRule ARGS "@rx attack" "phase:1,log,deny,id:1"`,
			expected: &SecRule{
				Variables: []Variable{{Name: "ARGS"}},
				Operator:  Operator{Type: "@rx", Parameter: "attack"},
				Actions: []Action{
					{Name: "phase", Parameter: "1"},
					{Name: "log"},
					{Name: "deny"},
					{Name: "id", Parameter: "1"},
				},
			},
			expectError: false,
		},
		{
			name:     "SQL injection detection",
			ruleText: `SecRule ARGS "@detectSQLi" "id:2,phase:2,block,msg:'SQL Injection Attack'"`,
			expected: &SecRule{
				Variables: []Variable{{Name: "ARGS"}},
				Operator:  Operator{Type: "@detectSQLi"},
				Actions: []Action{
					{Name: "id", Parameter: "2"},
					{Name: "phase", Parameter: "2"},
					{Name: "block"},
					{Name: "msg", Parameter: "SQL Injection Attack"},
				},
			},
			expectError: false,
		},
		{
			name:     "Multiple variables with pipe separator",
			ruleText: `SecRule REQUEST_FILENAME|ARGS_NAMES|ARGS "@rx <script" "id:3,phase:2,log,deny"`,
			expected: &SecRule{
				Variables: []Variable{
					{Name: "REQUEST_FILENAME"},
					{Name: "ARGS_NAMES"},
					{Name: "ARGS"},
				},
				Operator: Operator{Type: "@rx", Parameter: "<script"},
				Actions: []Action{
					{Name: "id", Parameter: "3"},
					{Name: "phase", Parameter: "2"},
					{Name: "log"},
					{Name: "deny"},
				},
			},
			expectError: false,
		},
		{
			name:     "Variable with collection",
			ruleText: `SecRule ARGS:username "@contains admin" "id:4,deny"`,
			expected: &SecRule{
				Variables: []Variable{{Name: "ARGS", Collection: "ARGS:username"}},
				Operator:  Operator{Type: "@contains", Parameter: "admin"},
				Actions: []Action{
					{Name: "id", Parameter: "4"},
					{Name: "deny"},
				},
			},
			expectError: false,
		},
		{
			name:     "Negated operator",
			ruleText: `SecRule ARGS !"@pm attack" "id:5,pass"`,
			expected: &SecRule{
				Variables: []Variable{{Name: "ARGS"}},
				Operator:  Operator{Type: "@pm", Parameter: "attack", Negated: true},
				Actions: []Action{
					{Name: "id", Parameter: "5"},
					{Name: "pass"},
				},
			},
			expectError: false,
		},
		{
			name:     "Rule without actions",
			ruleText: `SecRule ARGS "@rx test"`,
			expected: &SecRule{
				Variables: []Variable{{Name: "ARGS"}},
				Operator:  Operator{Type: "@rx", Parameter: "test"},
				Actions:   []Action{},
			},
			expectError: false,
		},
		{
			name:     "Rule with regex pattern (no @rx prefix)",
			ruleText: `SecRule ARGS "attack" "id:6,log"`,
			expected: &SecRule{
				Variables: []Variable{{Name: "ARGS"}},
				Operator:  Operator{Type: "@rx", Parameter: "attack"},
				Actions: []Action{
					{Name: "id", Parameter: "6"},
					{Name: "log"},
				},
			},
			expectError: false,
		},
		{
			name:     "Complex rule with status action",
			ruleText: `SecRule ARGS "@rx <script" "id:7,phase:2,log,deny,status:403,msg:'XSS Attack',tag:'WEB_ATTACK/XSS'"`,
			expected: &SecRule{
				Variables: []Variable{{Name: "ARGS"}},
				Operator:  Operator{Type: "@rx", Parameter: "<script"},
				Actions: []Action{
					{Name: "id", Parameter: "7"},
					{Name: "phase", Parameter: "2"},
					{Name: "log"},
					{Name: "deny"},
					{Name: "status", Parameter: "403"},
					{Name: "msg", Parameter: "XSS Attack"},
					{Name: "tag", Parameter: "WEB_ATTACK/XSS"},
				},
			},
			expectError: false,
		},
		{
			name:     "Unquoted operator parameter with complex actions",
			ruleText: `SecRule REQUEST_URI @streq /foo/bar id:10001,phase:1,pass,nolog,redirect:'sys?body=recaptcha&status=200'`,
			expected: &SecRule{
				Variables: []Variable{{Name: "REQUEST_URI"}},
				Operator:  Operator{Type: "@streq", Parameter: "/foo/bar"},
				Actions: []Action{
					{Name: "id", Parameter: "10001"},
					{Name: "phase", Parameter: "1"},
					{Name: "pass"},
					{Name: "nolog"},
					{Name: "redirect", Parameter: "sys?body=recaptcha&status=200"},
				},
			},
			expectError: false,
		},
		{
			name:        "Empty rule",
			ruleText:    "",
			expected:    nil,
			expectError: true,
		},
		{
			name:        "Invalid format - missing variables",
			ruleText:    `SecRule "@rx test"`,
			expected:    nil,
			expectError: true,
		},
		{
			name:        "Invalid format - missing operator",
			ruleText:    `SecRule ARGS`,
			expected:    nil,
			expectError: true,
		},
	}

	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			result, err := ParseRule(tc.ruleText)

			if tc.expectError {
				if err == nil {
					t.Errorf("Expected error but got none")
				}
				return
			}

			if err != nil {
				t.Errorf("Unexpected error: %v", err)
				return
			}

			if result == nil {
				t.Errorf("Expected result but got nil")
				return
			}

			// Compare variables
			if len(result.Variables) != len(tc.expected.Variables) {
				t.Errorf("Variables count mismatch: got %d, expected %d",
					len(result.Variables), len(tc.expected.Variables))
			}

			for i, variable := range result.Variables {
				if i < len(tc.expected.Variables) {
					expectedVar := tc.expected.Variables[i]
					if variable.Name != expectedVar.Name {
						t.Errorf("Variable %d name mismatch: got %s, expected %s",
							i, variable.Name, expectedVar.Name)
					}
					if variable.Collection != expectedVar.Collection {
						t.Errorf("Variable %d collection mismatch: got %s, expected %s",
							i, variable.Collection, expectedVar.Collection)
					}
				}
			}

			// Compare operator
			if result.Operator.Type != tc.expected.Operator.Type {
				t.Errorf("Operator type mismatch: got %s, expected %s",
					result.Operator.Type, tc.expected.Operator.Type)
			}
			if result.Operator.Parameter != tc.expected.Operator.Parameter {
				t.Errorf("Operator parameter mismatch: got %s, expected %s",
					result.Operator.Parameter, tc.expected.Operator.Parameter)
			}
			if result.Operator.Negated != tc.expected.Operator.Negated {
				t.Errorf("Operator negation mismatch: got %v, expected %v",
					result.Operator.Negated, tc.expected.Operator.Negated)
			}

			// Compare actions
			if len(result.Actions) != len(tc.expected.Actions) {
				t.Errorf("Actions count mismatch: got %d, expected %d",
					len(result.Actions), len(tc.expected.Actions))
			}

			for i, action := range result.Actions {
				if i < len(tc.expected.Actions) {
					expectedAction := tc.expected.Actions[i]
					if action.Name != expectedAction.Name {
						t.Errorf("Action %d name mismatch: got %s, expected %s",
							i, action.Name, expectedAction.Name)
					}
					if action.Parameter != expectedAction.Parameter {
						t.Errorf("Action %d parameter mismatch: got %s, expected %s",
							i, action.Parameter, expectedAction.Parameter)
					}
				}
			}
		})
	}
}

func TestParseVariables(t *testing.T) {
	testCases := []struct {
		name        string
		input       string
		expected    []Variable
		expectError bool
	}{
		{
			name:  "Single variable",
			input: "ARGS",
			expected: []Variable{
				{Name: "ARGS"},
			},
			expectError: false,
		},
		{
			name:  "Multiple variables",
			input: "ARGS|REQUEST_HEADERS|REQUEST_BODY",
			expected: []Variable{
				{Name: "ARGS"},
				{Name: "REQUEST_HEADERS"},
				{Name: "REQUEST_BODY"},
			},
			expectError: false,
		},
		{
			name:  "Variable with collection",
			input: "ARGS:username",
			expected: []Variable{
				{Name: "ARGS", Collection: "ARGS:username"},
			},
			expectError: false,
		},
		{
			name:        "Empty input",
			input:       "",
			expected:    nil,
			expectError: true,
		},
	}

	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			result, err := parseVariables(tc.input)

			if tc.expectError {
				if err == nil {
					t.Errorf("Expected error but got none")
				}
				return
			}

			if err != nil {
				t.Errorf("Unexpected error: %v", err)
				return
			}

			if len(result) != len(tc.expected) {
				t.Errorf("Result count mismatch: got %d, expected %d",
					len(result), len(tc.expected))
				return
			}

			for i, variable := range result {
				expected := tc.expected[i]
				if variable.Name != expected.Name {
					t.Errorf("Variable %d name mismatch: got %s, expected %s",
						i, variable.Name, expected.Name)
				}
				if variable.Collection != expected.Collection {
					t.Errorf("Variable %d collection mismatch: got %s, expected %s",
						i, variable.Collection, expected.Collection)
				}
			}
		})
	}
}

func TestParseOperator(t *testing.T) {
	testCases := []struct {
		name        string
		input       string
		negated     bool
		expected    Operator
		expectError bool
	}{
		{
			name:    "Regex operator",
			input:   "@rx attack",
			negated: false,
			expected: Operator{
				Type:      "@rx",
				Parameter: "attack",
				Negated:   false,
			},
			expectError: false,
		},
		{
			name:    "Operator without parameter",
			input:   "@detectSQLi",
			negated: false,
			expected: Operator{
				Type:    "@detectSQLi",
				Negated: false,
			},
			expectError: false,
		},
		{
			name:    "Negated operator",
			input:   "@pm test",
			negated: true,
			expected: Operator{
				Type:      "@pm",
				Parameter: "test",
				Negated:   true,
			},
			expectError: false,
		},
		{
			name:    "Plain string (defaults to @rx)",
			input:   "attack",
			negated: false,
			expected: Operator{
				Type:      "@rx",
				Parameter: "attack",
				Negated:   false,
			},
			expectError: false,
		},
		{
			name:        "Empty operator",
			input:       "",
			negated:     false,
			expected:    Operator{},
			expectError: true,
		},
	}

	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			result, err := parseOperator(tc.input, tc.negated)

			if tc.expectError {
				if err == nil {
					t.Errorf("Expected error but got none")
				}
				return
			}

			if err != nil {
				t.Errorf("Unexpected error: %v", err)
				return
			}

			if result.Type != tc.expected.Type {
				t.Errorf("Operator type mismatch: got %s, expected %s",
					result.Type, tc.expected.Type)
			}
			if result.Parameter != tc.expected.Parameter {
				t.Errorf("Operator parameter mismatch: got %s, expected %s",
					result.Parameter, tc.expected.Parameter)
			}
			if result.Negated != tc.expected.Negated {
				t.Errorf("Operator negation mismatch: got %v, expected %v",
					result.Negated, tc.expected.Negated)
			}
		})
	}
}

func TestParseActions(t *testing.T) {
	testCases := []struct {
		name        string
		input       string
		expected    []Action
		expectError bool
	}{
		{
			name:  "Simple actions",
			input: "log,deny,id:1",
			expected: []Action{
				{Name: "log"},
				{Name: "deny"},
				{Name: "id", Parameter: "1"},
			},
			expectError: false,
		},
		{
			name:  "Actions with quoted parameters",
			input: "msg:'SQL Injection',tag:'WEB_ATTACK'",
			expected: []Action{
				{Name: "msg", Parameter: "SQL Injection"},
				{Name: "tag", Parameter: "WEB_ATTACK"},
			},
			expectError: false,
		},
		{
			name:     "Empty actions",
			input:    "",
			expected: nil,
			expectError: false,
		},
	}

	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			result, err := parseActions(tc.input)

			if tc.expectError {
				if err == nil {
					t.Errorf("Expected error but got none")
				}
				return
			}

			if err != nil {
				t.Errorf("Unexpected error: %v", err)
				return
			}

			if len(result) != len(tc.expected) {
				t.Errorf("Result count mismatch: got %d, expected %d",
					len(result), len(tc.expected))
				return
			}

			for i, action := range result {
				if i < len(tc.expected) {
					expected := tc.expected[i]
					if action.Name != expected.Name {
						t.Errorf("Action %d name mismatch: got %s, expected %s",
							i, action.Name, expected.Name)
					}
					if action.Parameter != expected.Parameter {
						t.Errorf("Action %d parameter mismatch: got %s, expected %s",
							i, action.Parameter, expected.Parameter)
					}
				}
			}
		})
	}
}

func TestValidateRule(t *testing.T) {
	testCases := []struct {
		name        string
		rule        *SecRule
		expectError bool
		errorType   error
	}{
		{
			name: "Valid rule",
			rule: &SecRule{
				Variables: []Variable{{Name: "ARGS"}},
				Operator:  Operator{Type: "@rx", Parameter: "attack"},
				Actions:   []Action{{Name: "id", Parameter: "1"}},
			},
			expectError: false,
		},
		{
			name:        "Nil rule",
			rule:        nil,
			expectError: true,
		},
		{
			name: "Missing variables",
			rule: &SecRule{
				Operator: Operator{Type: "@rx", Parameter: "attack"},
				Actions:  []Action{{Name: "id", Parameter: "1"}},
			},
			expectError: true,
		},
		{
			name: "Missing operator",
			rule: &SecRule{
				Variables: []Variable{{Name: "ARGS"}},
				Actions:   []Action{{Name: "id", Parameter: "1"}},
			},
			expectError: true,
		},
		{
			name: "Missing ID action",
			rule: &SecRule{
				Variables: []Variable{{Name: "ARGS"}},
				Operator:  Operator{Type: "@rx", Parameter: "attack"},
				Actions:   []Action{{Name: "log"}},
			},
			expectError: true,
		},
		{
			name: "Invalid ID (non-numeric)",
			rule: &SecRule{
				Variables: []Variable{{Name: "ARGS"}},
				Operator:  Operator{Type: "@rx", Parameter: "attack"},
				Actions:   []Action{{Name: "id", Parameter: "abc"}},
			},
			expectError: true,
		},
	}

	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			err := ValidateRule(tc.rule)

			if tc.expectError {
				if err == nil {
					t.Errorf("Expected error but got none")
				}
				return
			}

			if err != nil {
				t.Errorf("Unexpected error: %v", err)
			}
		})
	}
}

func TestSecRuleActionMethods(t *testing.T) {
	// Create a test rule
	rule := &SecRule{
		Variables: []Variable{{Name: "ARGS"}},
		Operator:  Operator{Type: "@rx", Parameter: "test"},
		Actions: []Action{
			{Name: "id", Parameter: "1"},
			{Name: "log"},
			{Name: "deny"},
		},
	}

	t.Run("GetActions", func(t *testing.T) {
		actions := rule.GetActions()
		if len(actions) != 3 {
			t.Errorf("Expected 3 actions, got %d", len(actions))
		}

		expected := []string{"id", "log", "deny"}
		for i, action := range actions {
			if action.Name != expected[i] {
				t.Errorf("Expected action %d to be %s, got %s", i, expected[i], action.Name)
			}
		}
	})

	t.Run("SetActions", func(t *testing.T) {
		newActions := []Action{
			{Name: "phase", Parameter: "2"},
			{Name: "block"},
		}

		rule.SetActions(newActions)
		actions := rule.GetActions()

		if len(actions) != 2 {
			t.Errorf("Expected 2 actions after set, got %d", len(actions))
		}

		if actions[0].Name != "phase" || actions[0].Parameter != "2" {
			t.Errorf("Expected first action to be phase:2, got %s:%s", actions[0].Name, actions[0].Parameter)
		}

		if actions[1].Name != "block" {
			t.Errorf("Expected second action to be block, got %s", actions[1].Name)
		}
	})

	t.Run("AddAction", func(t *testing.T) {
		// Test adding a new action
		initialCount := len(rule.GetActions())

		newAction := Action{Name: "msg", Parameter: "Test message"}
		rule.AddAction(newAction)

		actions := rule.GetActions()
		if len(actions) != initialCount+1 {
			t.Errorf("Expected %d actions after add, got %d", initialCount+1, len(actions))
		}

		// Verify the action was added
		msgAction, found := rule.GetActionByName("msg")
		if !found {
			t.Error("Expected to find 'msg' action after adding")
		}
		if msgAction.Parameter != "Test message" {
			t.Errorf("Expected msg parameter to be 'Test message', got '%s'", msgAction.Parameter)
		}

		// Test overwriting an existing action
		currentCount := len(rule.GetActions())
		overwriteAction := Action{Name: "msg", Parameter: "Updated message"}
		rule.AddAction(overwriteAction)

		actions = rule.GetActions()
		if len(actions) != currentCount {
			t.Errorf("Expected %d actions after overwrite (no change), got %d", currentCount, len(actions))
		}

		// Verify the action was overwritten
		msgAction, found = rule.GetActionByName("msg")
		if !found {
			t.Error("Expected to find 'msg' action after overwriting")
		}
		if msgAction.Parameter != "Updated message" {
			t.Errorf("Expected msg parameter to be 'Updated message', got '%s'", msgAction.Parameter)
		}
	})

	t.Run("HasAction", func(t *testing.T) {
		if !rule.HasAction("phase") {
			t.Error("Expected rule to have 'phase' action")
		}

		if !rule.HasAction("msg") {
			t.Error("Expected rule to have 'msg' action")
		}

		if rule.HasAction("nonexistent") {
			t.Error("Expected rule to not have 'nonexistent' action")
		}
	})

	t.Run("GetActionByName", func(t *testing.T) {
		action, found := rule.GetActionByName("phase")
		if !found {
			t.Error("Expected to find 'phase' action")
		}
		if action.Name != "phase" || action.Parameter != "2" {
			t.Errorf("Expected phase:2, got %s:%s", action.Name, action.Parameter)
		}

		_, found = rule.GetActionByName("nonexistent")
		if found {
			t.Error("Expected not to find 'nonexistent' action")
		}
	})

	t.Run("RemoveAction", func(t *testing.T) {
		initialCount := len(rule.GetActions())

		// Remove existing action
		removed := rule.RemoveAction("msg")
		if !removed {
			t.Error("Expected to successfully remove 'msg' action")
		}

		actions := rule.GetActions()
		if len(actions) != initialCount-1 {
			t.Errorf("Expected %d actions after removal, got %d", initialCount-1, len(actions))
		}

		if rule.HasAction("msg") {
			t.Error("Expected 'msg' action to be removed")
		}

		// Try to remove non-existent action
		removed = rule.RemoveAction("nonexistent")
		if removed {
			t.Error("Expected not to remove 'nonexistent' action")
		}
	})

	t.Run("UpdateAction", func(t *testing.T) {
		// Update existing action
		updatedAction := Action{Name: "phase", Parameter: "3"}
		rule.UpdateAction(updatedAction)

		action, found := rule.GetActionByName("phase")
		if !found {
			t.Error("Expected to find updated 'phase' action")
		}
		if action.Parameter != "3" {
			t.Errorf("Expected phase parameter to be '3', got '%s'", action.Parameter)
		}

		// Add new action via update
		initialCount := len(rule.GetActions())
		newAction := Action{Name: "severity", Parameter: "2"}
		rule.UpdateAction(newAction)

		if len(rule.GetActions()) != initialCount+1 {
			t.Errorf("Expected %d actions after update-add, got %d", initialCount+1, len(rule.GetActions()))
		}

		action, found = rule.GetActionByName("severity")
		if !found {
			t.Error("Expected to find new 'severity' action")
		}
		if action.Parameter != "2" {
			t.Errorf("Expected severity parameter to be '2', got '%s'", action.Parameter)
		}
	})
}

func TestToModSecurityRule(t *testing.T) {
	testCases := []struct {
		name     string
		rule     *SecRule
		expected string
	}{
		{
			name: "Basic rule with simple actions",
			rule: &SecRule{
				Variables: []Variable{{Name: "ARGS"}},
				Operator:  Operator{Type: "@rx", Parameter: "attack"},
				Actions: []Action{
					{Name: "id", Parameter: "1001"},
					{Name: "log"},
					{Name: "deny"},
				},
			},
			expected: `SecRule ARGS "@rx attack" "id:1001,log,deny"`,
		},
		{
			name: "Multiple variables",
			rule: &SecRule{
				Variables: []Variable{
					{Name: "REQUEST_FILENAME"},
					{Name: "ARGS_NAMES"},
					{Name: "ARGS"},
				},
				Operator: Operator{Type: "@rx", Parameter: "<script"},
				Actions: []Action{
					{Name: "id", Parameter: "2001"},
					{Name: "phase", Parameter: "2"},
				},
			},
			expected: `SecRule REQUEST_FILENAME|ARGS_NAMES|ARGS "@rx <script" "id:2001,phase:2"`,
		},
		{
			name: "Variable collection",
			rule: &SecRule{
				Variables: []Variable{{Name: "ARGS", Collection: "ARGS:username"}},
				Operator:  Operator{Type: "@contains", Parameter: "admin"},
				Actions: []Action{
					{Name: "id", Parameter: "3001"},
					{Name: "deny"},
				},
			},
			expected: `SecRule ARGS:username "@contains admin" "id:3001,deny"`,
		},
		{
			name: "Negated operator",
			rule: &SecRule{
				Variables: []Variable{{Name: "ARGS"}},
				Operator:  Operator{Type: "@pm", Parameter: "attack", Negated: true},
				Actions: []Action{
					{Name: "id", Parameter: "4001"},
					{Name: "pass"},
				},
			},
			expected: `SecRule ARGS !"@pm attack" "id:4001,pass"`,
		},
		{
			name: "Operator without parameter",
			rule: &SecRule{
				Variables: []Variable{{Name: "ARGS"}},
				Operator:  Operator{Type: "@detectSQLi"},
				Actions: []Action{
					{Name: "id", Parameter: "5001"},
					{Name: "block"},
				},
			},
			expected: `SecRule ARGS @detectSQLi "id:5001,block"`,
		},
		{
			name: "Actions with special characters",
			rule: &SecRule{
				Variables: []Variable{{Name: "ARGS"}},
				Operator:  Operator{Type: "@rx", Parameter: "test"},
				Actions: []Action{
					{Name: "id", Parameter: "6001"},
					{Name: "msg", Parameter: "SQL Injection Attack"},
					{Name: "tag", Parameter: "WEB_ATTACK/SQLI"},
				},
			},
			expected: `SecRule ARGS "@rx test" "id:6001,msg:'SQL Injection Attack',tag:'WEB_ATTACK/SQLI'"`,
		},
		{
			name: "Rule without actions",
			rule: &SecRule{
				Variables: []Variable{{Name: "ARGS"}},
				Operator:  Operator{Type: "@rx", Parameter: "test"},
				Actions:   []Action{},
			},
			expected: `SecRule ARGS "@rx test"`,
		},
		{
			name: "Complex rule with quoted parameters",
			rule: &SecRule{
				Variables: []Variable{{Name: "ARGS"}},
				Operator:  Operator{Type: "@rx", Parameter: "(?i)((union(.*?)select)|((union(.*?)all(.*?)select)))"},
				Actions: []Action{
					{Name: "id", Parameter: "7001"},
					{Name: "msg", Parameter: "SQL Injection Attack: UNION query detected"},
					{Name: "severity", Parameter: "2"},
					{Name: "tag", Parameter: "WEB_ATTACK"},
				},
			},
			expected: `SecRule ARGS "@rx (?i)((union(.*?)select)|((union(.*?)all(.*?)select)))" "id:7001,msg:'SQL Injection Attack: UNION query detected',severity:2,tag:WEB_ATTACK"`,
		},
		{
			name: "Action parameter with single quotes",
			rule: &SecRule{
				Variables: []Variable{{Name: "ARGS"}},
				Operator:  Operator{Type: "@rx", Parameter: "test"},
				Actions: []Action{
					{Name: "id", Parameter: "8001"},
					{Name: "msg", Parameter: "It's a test message"},
				},
			},
			expected: `SecRule ARGS "@rx test" "id:8001,msg:'It\'s a test message'"`,
		},
		{
			name: "Operator with unquoted parameter and complex actions",
			rule: &SecRule{
				Variables: []Variable{{Name: "REQUEST_URI"}},
				Operator:  Operator{Type: "@streq", Parameter: "/foo/bar"},
				Actions: []Action{
					{Name: "id", Parameter: "10001"},
					{Name: "phase", Parameter: "1"},
					{Name: "pass"},
					{Name: "nolog"},
					{Name: "redirect", Parameter: "sys?body=recaptcha&status=200"},
				},
			},
			expected: `SecRule REQUEST_URI "@streq /foo/bar" "id:10001,phase:1,pass,nolog,redirect:'sys?body=recaptcha&status=200'"`,
		},
	}

	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			result := tc.rule.ToModSecurityRule()
			if result != tc.expected {
				t.Errorf("Expected:\n%s\nGot:\n%s", tc.expected, result)
			}
		})
	}
}

func TestParseMultilineRule(t *testing.T) {
	// Test case for the specific bug that was reported
	multilineRule := `SecRule REQUEST_URI ".*" \
    "id:99999, \
    phase:1, \
    pass, \
    nolog, \
    msg:'DEBUG: The current REQUEST_URI is: %{REQUEST_URI}'"`

	parsed, err := ParseRule(multilineRule)
	if err != nil {
		t.Fatalf("Failed to parse multiline rule: %v", err)
	}

	// Verify all actions were parsed correctly
	expectedActions := []Action{
		{Name: "id", Parameter: "99999"},
		{Name: "phase", Parameter: "1"},
		{Name: "pass"},
		{Name: "nolog"},
		{Name: "msg", Parameter: "DEBUG: The current REQUEST_URI is: %{REQUEST_URI}"},
	}

	if len(parsed.Actions) != len(expectedActions) {
		t.Errorf("Expected %d actions, got %d", len(expectedActions), len(parsed.Actions))
	}

	for i, expectedAction := range expectedActions {
		if i >= len(parsed.Actions) {
			t.Errorf("Missing action %d: %+v", i, expectedAction)
			continue
		}
		actualAction := parsed.Actions[i]
		if actualAction.Name != expectedAction.Name || actualAction.Parameter != expectedAction.Parameter {
			t.Errorf("Action %d mismatch: expected %+v, got %+v", i, expectedAction, actualAction)
		}
	}

	// Test round-trip conversion
	modSecRule := parsed.ToModSecurityRule()

	// Parse the generated rule again
	reparsed, err := ParseRule(modSecRule)
	if err != nil {
		t.Fatalf("Failed to parse regenerated rule: %v", err)
	}

	// Verify all actions are preserved
	if len(reparsed.Actions) != len(expectedActions) {
		t.Errorf("Round-trip failed: expected %d actions, got %d", len(expectedActions), len(reparsed.Actions))
	}
}

func TestRoundTripParsing(t *testing.T) {
	// Test that parsing a rule and then converting it back produces equivalent rule
	testCases := []string{
		`SecRule ARGS "@rx attack" "id:1001,log,deny"`,
		`SecRule REQUEST_FILENAME|ARGS "@rx <script" "id:2001,phase:2,block"`,
		`SecRule ARGS:username "@contains admin" "id:3001,deny"`,
		`SecRule ARGS !"@pm attack" "id:4001,pass"`,
		`SecRule ARGS "@detectSQLi" "id:5001,block,msg:'SQL Injection'"`,
		`SecRule ARGS "@rx test"`, // Rule without actions
	}

	for _, ruleText := range testCases {
		t.Run(ruleText, func(t *testing.T) {
			// Parse the rule
			parsed, err := ParseRule(ruleText)
			if err != nil {
				t.Fatalf("Failed to parse rule: %v", err)
			}

			// Convert back to ModSecurity format
			regenerated := parsed.ToModSecurityRule()

			// Parse the regenerated rule
			reparsed, err := ParseRule(regenerated)
			if err != nil {
				t.Fatalf("Failed to parse regenerated rule: %v\nOriginal: %s\nRegenerated: %s", err, ruleText, regenerated)
			}

			// Compare key components (we can't compare exact strings due to potential formatting differences)
			if len(parsed.Variables) != len(reparsed.Variables) {
				t.Errorf("Variable count mismatch: original %d, reparsed %d", len(parsed.Variables), len(reparsed.Variables))
			}

			for i, origVar := range parsed.Variables {
				if i < len(reparsed.Variables) {
					if origVar.Name != reparsed.Variables[i].Name || origVar.Collection != reparsed.Variables[i].Collection {
						t.Errorf("Variable %d mismatch: original %+v, reparsed %+v", i, origVar, reparsed.Variables[i])
					}
				}
			}

			if parsed.Operator.Type != reparsed.Operator.Type ||
				parsed.Operator.Parameter != reparsed.Operator.Parameter ||
				parsed.Operator.Negated != reparsed.Operator.Negated {
				t.Errorf("Operator mismatch: original %+v, reparsed %+v", parsed.Operator, reparsed.Operator)
			}

			if len(parsed.Actions) != len(reparsed.Actions) {
				t.Errorf("Action count mismatch: original %d, reparsed %d", len(parsed.Actions), len(reparsed.Actions))
			}

			for i, origAction := range parsed.Actions {
				if i < len(reparsed.Actions) {
					if origAction.Name != reparsed.Actions[i].Name || origAction.Parameter != reparsed.Actions[i].Parameter {
						t.Errorf("Action %d mismatch: original %+v, reparsed %+v", i, origAction, reparsed.Actions[i])
					}
				}
			}
		})
	}
}