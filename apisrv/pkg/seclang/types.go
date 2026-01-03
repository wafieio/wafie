package seclang

import (
	"fmt"
	"strings"
)

// SecRule represents a parsed ModSecurity SecRule directive
type SecRule struct {
	Variables []Variable `json:"variables"`
	Operator  Operator   `json:"operator"`
	Actions   []Action   `json:"actions,omitempty"`
}

// Variable represents a ModSecurity variable (what to inspect)
type Variable struct {
	Name           string   `json:"name"`           // e.g., "ARGS", "REQUEST_HEADERS"
	Collection     string   `json:"collection"`     // e.g., "ARGS:username"
	Transformations []string `json:"transformations,omitempty"` // e.g., ["t:lowercase", "t:htmlEntityDecode"]
}

// Operator represents a ModSecurity operator (how to evaluate)
type Operator struct {
	Type      string `json:"type"`      // e.g., "@rx", "@eq", "@contains"
	Parameter string `json:"parameter"` // The operator parameter/pattern
	Negated   bool   `json:"negated"`   // Whether operator is negated with !
}

// Action represents a ModSecurity action (what to do when rule matches)
type Action struct {
	Name      string `json:"name"`                // e.g., "id", "msg", "log", "deny"
	Parameter string `json:"parameter,omitempty"` // Action parameter, if any
}

// ActionType constants for common actions
const (
	ActionID       = "id"
	ActionMsg      = "msg"
	ActionLog      = "log"
	ActionDeny     = "deny"
	ActionPass     = "pass"
	ActionBlock    = "block"
	ActionPhase    = "phase"
	ActionSeverity = "severity"
	ActionTag      = "tag"
	ActionStatus   = "status"
)

// OperatorType constants for common operators
const (
	OperatorRx        = "@rx"
	OperatorEq        = "@eq"
	OperatorContains  = "@contains"
	OperatorDetectSQLi = "@detectSQLi"
	OperatorDetectXSS = "@detectXSS"
	OperatorPM        = "@pm"
	OperatorIPMatch   = "@ipMatch"
)

// VariableType constants for common variables
const (
	VarArgs            = "ARGS"
	VarArgsNames       = "ARGS_NAMES"
	VarRequestHeaders  = "REQUEST_HEADERS"
	VarRequestBody     = "REQUEST_BODY"
	VarResponseBody    = "RESPONSE_BODY"
	VarRequestFilename = "REQUEST_FILENAME"
	VarRemoteAddr      = "REMOTE_ADDR"
	VarXML             = "XML"
)

// String returns a string representation of the SecRule
func (sr *SecRule) String() string {
	var vars []string
	for _, v := range sr.Variables {
		vars = append(vars, v.String())
	}

	var actions []string
	for _, a := range sr.Actions {
		actions = append(actions, a.String())
	}

	return fmt.Sprintf("SecRule [%s] %s [%s]",
		strings.Join(vars, " "),
		sr.Operator.String(),
		strings.Join(actions, " "))
}

// ToModSecurityRule converts the SecRule back to valid ModSecurity rule syntax
func (sr *SecRule) ToModSecurityRule() string {
	// Build variables section (pipe-separated)
	var vars []string
	for _, v := range sr.Variables {
		vars = append(vars, v.String())
	}
	variablesStr := strings.Join(vars, "|")

	// Build operator section
	operatorStr := sr.buildOperatorString()

	// Build actions section (comma-separated, quoted)
	actionsStr := ""
	if len(sr.Actions) > 0 {
		var actions []string
		for _, a := range sr.Actions {
			actions = append(actions, a.toModSecurityString())
		}
		actionsStr = fmt.Sprintf("\"%s\"", strings.Join(actions, ","))
	}

	// Combine all parts
	if actionsStr != "" {
		return fmt.Sprintf("SecRule %s %s %s", variablesStr, operatorStr, actionsStr)
	}
	return fmt.Sprintf("SecRule %s %s", variablesStr, operatorStr)
}

// buildOperatorString creates the operator string for ModSecurity format
func (sr *SecRule) buildOperatorString() string {
	negation := ""
	if sr.Operator.Negated {
		negation = "!"
	}

	// If operator has spaces or special characters, quote it
	if sr.Operator.Parameter != "" {
		operatorWithParam := fmt.Sprintf("%s %s", sr.Operator.Type, sr.Operator.Parameter)
		if strings.Contains(operatorWithParam, " ") || needsQuoting(operatorWithParam) {
			return fmt.Sprintf("%s\"%s\"", negation, operatorWithParam)
		}
		return fmt.Sprintf("%s%s", negation, operatorWithParam)
	}

	// Operator without parameter
	if needsOperatorQuoting(sr.Operator.Type) {
		return fmt.Sprintf("%s\"%s\"", negation, sr.Operator.Type)
	}
	return fmt.Sprintf("%s%s", negation, sr.Operator.Type)
}

// toModSecurityString formats an action for ModSecurity rule syntax
func (a *Action) toModSecurityString() string {
	if a.Parameter != "" {
		// Escape single quotes in parameter
		escapedParam := strings.ReplaceAll(a.Parameter, "'", "\\'")

		// If parameter contains spaces, special chars, or commas, use single quotes
		if needsQuoting(a.Parameter) {
			return fmt.Sprintf("%s:'%s'", a.Name, escapedParam)
		}
		return fmt.Sprintf("%s:%s", a.Name, a.Parameter)
	}
	return a.Name
}

// needsQuoting determines if a string needs to be quoted in ModSecurity syntax
func needsQuoting(s string) bool {
	return strings.ContainsAny(s, " \t\n\r,;\"'<>(){}[]|&$*?\\/")
}

// needsOperatorQuoting determines if an operator needs to be quoted
func needsOperatorQuoting(operator string) bool {
	// @ operators generally don't need quoting unless they have special characters
	if strings.HasPrefix(operator, "@") {
		return strings.ContainsAny(operator, " \t\n\r,;\"'<>(){}[]|&$*?\\/")
	}
	return needsQuoting(operator)
}

// String returns a string representation of the Variable
func (v *Variable) String() string {
	if v.Collection != "" {
		return v.Collection
	}
	return v.Name
}

// String returns a string representation of the Operator
func (op *Operator) String() string {
	negation := ""
	if op.Negated {
		negation = "!"
	}
	if op.Parameter != "" {
		return fmt.Sprintf("%s%s %s", negation, op.Type, op.Parameter)
	}
	return fmt.Sprintf("%s%s", negation, op.Type)
}

// String returns a string representation of the Action
func (a *Action) String() string {
	if a.Parameter != "" {
		return fmt.Sprintf("%s:%s", a.Name, a.Parameter)
	}
	return a.Name
}

// GetActions returns the actions slice
func (sr *SecRule) GetActions() []Action {
	return sr.Actions
}

// SetActions sets the actions slice
func (sr *SecRule) SetActions(actions []Action) {
	sr.Actions = actions
}

// AddAction adds a single action to the SecRule
// If an action with the same name already exists, it will be overwritten
func (sr *SecRule) AddAction(action Action) {
	for i, existingAction := range sr.Actions {
		if existingAction.Name == action.Name {
			sr.Actions[i] = action
			return
		}
	}
	// Action not found, append it
	sr.Actions = append(sr.Actions, action)
}

// RemoveAction removes an action by name from the SecRule
// Returns true if the action was found and removed, false otherwise
func (sr *SecRule) RemoveAction(actionName string) bool {
	for i, action := range sr.Actions {
		if action.Name == actionName {
			// Remove the action at index i
			sr.Actions = append(sr.Actions[:i], sr.Actions[i+1:]...)
			return true
		}
	}
	return false
}

// GetActionByName returns the first action with the specified name
// Returns the action and true if found, empty Action and false otherwise
func (sr *SecRule) GetActionByName(actionName string) (Action, bool) {
	for _, action := range sr.Actions {
		if action.Name == actionName {
			return action, true
		}
	}
	return Action{}, false
}

// HasAction returns true if the SecRule has an action with the specified name
func (sr *SecRule) HasAction(actionName string) bool {
	_, found := sr.GetActionByName(actionName)
	return found
}

// UpdateAction updates an existing action or adds it if it doesn't exist
// This method now behaves identically to AddAction
func (sr *SecRule) UpdateAction(action Action) {
	sr.AddAction(action)
}