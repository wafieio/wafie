package seclang

import (
	"errors"
	"fmt"
	"strconv"
	"strings"
)

var (
	// ErrInvalidRule represents an invalid SecRule format
	ErrInvalidRule = errors.New("invalid SecRule format")

	// ErrMissingOperator represents a missing operator in SecRule
	ErrMissingOperator = errors.New("missing operator in SecRule")

	// ErrMissingVariables represents missing variables in SecRule
	ErrMissingVariables = errors.New("missing variables in SecRule")
)

// ParseRule parses a ModSecurity SecRule string and returns a SecRule struct
func ParseRule(ruleText string) (*SecRule, error) {
	if ruleText == "" {
		return nil, ErrInvalidRule
	}

	// Clean up the rule text
	ruleText = strings.TrimSpace(ruleText)

	// Handle line continuations - remove backslash followed by newline/whitespace
	ruleText = cleanLineContinations(ruleText)

	// Remove "SecRule" prefix if present
	if strings.HasPrefix(ruleText, "SecRule") {
		ruleText = strings.TrimSpace(ruleText[7:])
	}

	// Parse the rule components
	variables, operator, actions, err := parseRuleComponents(ruleText)
	if err != nil {
		return nil, fmt.Errorf("failed to parse rule components: %w", err)
	}

	return &SecRule{
		Variables: variables,
		Operator:  operator,
		Actions:   actions,
	}, nil
}

// parseRuleComponents parses the three main components of a SecRule
func parseRuleComponents(ruleText string) ([]Variable, Operator, []Action, error) {
	// Split the rule into parts, handling quoted strings
	parts := splitRuleIntoComponents(ruleText)

	if len(parts) < 2 {
		return nil, Operator{}, nil, ErrInvalidRule
	}

	// Parse variables (first part)
	variablesStr := strings.TrimSpace(parts[0])
	if variablesStr == "" {
		return nil, Operator{}, nil, ErrMissingVariables
	}
	variables, err := parseVariables(variablesStr)
	if err != nil {
		return nil, Operator{}, nil, fmt.Errorf("failed to parse variables: %w", err)
	}

	// Parse operator (second part)
	operatorStr := strings.TrimSpace(parts[1])
	if operatorStr == "" {
		return nil, Operator{}, nil, ErrMissingOperator
	}

	// Check for negation
	var negated bool
	if strings.HasPrefix(operatorStr, "!") {
		negated = true
		operatorStr = strings.TrimSpace(operatorStr[1:])
	}

	// Remove quotes if present
	if strings.HasPrefix(operatorStr, "\"") && strings.HasSuffix(operatorStr, "\"") {
		operatorStr = operatorStr[1 : len(operatorStr)-1]
	}

	operator, err := parseOperator(operatorStr, negated)
	if err != nil {
		return nil, Operator{}, nil, fmt.Errorf("failed to parse operator: %w", err)
	}

	// Parse actions (third part, optional)
	var actions []Action
	if len(parts) > 2 {
		actionsStr := strings.TrimSpace(parts[2])
		// Remove quotes if present
		if strings.HasPrefix(actionsStr, "\"") && strings.HasSuffix(actionsStr, "\"") {
			actionsStr = actionsStr[1 : len(actionsStr)-1]
		}

		if actionsStr != "" {
			actions, err = parseActions(actionsStr)
			if err != nil {
				return nil, Operator{}, nil, fmt.Errorf("failed to parse actions: %w", err)
			}
		}
	}

	return variables, operator, actions, nil
}

// splitRuleIntoComponents splits a rule string into variables, operator, and actions parts
func splitRuleIntoComponents(ruleText string) []string {
	var parts []string
	var current strings.Builder
	var inQuotes bool
	var i int

	for i < len(ruleText) {
		char := ruleText[i]

		switch {
		case char == '"':
			inQuotes = !inQuotes
			current.WriteByte(char)
		case char == ' ' && !inQuotes:
			if current.Len() > 0 {
				parts = append(parts, current.String())
				current.Reset()

				// Skip multiple spaces
				for i+1 < len(ruleText) && ruleText[i+1] == ' ' {
					i++
				}

				// If we find a quote after spaces, collect the entire quoted string
				if i+1 < len(ruleText) && ruleText[i+1] == '"' {
					i++ // Move to the quote
					var quoted strings.Builder
					quoted.WriteByte('"') // Add the opening quote
					i++                   // Move past the opening quote

					// Collect everything until the closing quote
					for i < len(ruleText) && ruleText[i] != '"' {
						quoted.WriteByte(ruleText[i])
						i++
					}

					if i < len(ruleText) {
						quoted.WriteByte('"') // Add the closing quote
						parts = append(parts, quoted.String())
					}
				}
			}
		default:
			current.WriteByte(char)
		}
		i++
	}

	// Add the last part if there's content
	if current.Len() > 0 {
		parts = append(parts, current.String())
	}

	return parts
}

// parseVariables parses the variables section (pipe-separated)
func parseVariables(variablesStr string) ([]Variable, error) {
	if variablesStr == "" {
		return nil, ErrMissingVariables
	}

	var variables []Variable
	varParts := strings.Split(variablesStr, "|")

	for _, varPart := range varParts {
		varPart = strings.TrimSpace(varPart)
		if varPart == "" {
			continue
		}

		variable := Variable{}

		// Check if it's a collection variable (contains colon)
		if strings.Contains(varPart, ":") {
			variable.Collection = varPart
			parts := strings.SplitN(varPart, ":", 2)
			variable.Name = parts[0]
		} else {
			variable.Name = varPart
		}

		variables = append(variables, variable)
	}

	if len(variables) == 0 {
		return nil, ErrMissingVariables
	}

	return variables, nil
}

// parseOperator parses the operator section
func parseOperator(operatorStr string, negated bool) (Operator, error) {
	if operatorStr == "" {
		return Operator{}, ErrMissingOperator
	}

	operator := Operator{
		Negated: negated,
	}

	// Check if it starts with @ (named operator)
	if strings.HasPrefix(operatorStr, "@") {
		parts := strings.SplitN(operatorStr, " ", 2)
		operator.Type = parts[0]
		if len(parts) > 1 {
			operator.Parameter = strings.TrimSpace(parts[1])
		}
	} else {
		// It's a regular expression or string match
		operator.Type = "@rx" // Default to regex
		operator.Parameter = operatorStr
	}

	return operator, nil
}

// parseActions parses the actions section (comma-separated)
func parseActions(actionsStr string) ([]Action, error) {
	if actionsStr == "" {
		return nil, nil
	}

	var actions []Action

	// Split by comma, but handle quoted values
	actionParts := splitActions(actionsStr)

	for _, actionPart := range actionParts {
		actionPart = strings.TrimSpace(actionPart)
		if actionPart == "" {
			continue
		}

		action := Action{}

		// Check if action has a parameter (contains colon)
		if strings.Contains(actionPart, ":") {
			colonIndex := strings.Index(actionPart, ":")
			action.Name = strings.TrimSpace(actionPart[:colonIndex])
			parameter := strings.TrimSpace(actionPart[colonIndex+1:])

			// Remove quotes if present
			if strings.HasPrefix(parameter, "'") && strings.HasSuffix(parameter, "'") {
				parameter = parameter[1 : len(parameter)-1]
			}
			action.Parameter = parameter
		} else {
			action.Name = actionPart
		}

		actions = append(actions, action)
	}

	return actions, nil
}

// splitActions splits action string by comma, respecting quoted values
func splitActions(actionsStr string) []string {
	var parts []string
	var current strings.Builder
	var inQuotes bool
	var quoteChar rune

	for i, r := range actionsStr {
		switch {
		case (r == '\'' || r == '"') && !inQuotes:
			inQuotes = true
			quoteChar = r
			current.WriteRune(r)
		case r == quoteChar && inQuotes:
			inQuotes = false
			current.WriteRune(r)
		case r == ',' && !inQuotes:
			if current.Len() > 0 {
				parts = append(parts, current.String())
				current.Reset()
			}
		default:
			current.WriteRune(r)
		}

		// Handle last character
		if i == len(actionsStr)-1 && current.Len() > 0 {
			parts = append(parts, current.String())
		}
	}

	return parts
}

// cleanLineContinations removes line continuation characters (backslash followed by newline/whitespace)
func cleanLineContinations(ruleText string) string {
	// Split by lines and process each line
	lines := strings.Split(ruleText, "\n")
	var cleanedLines []string

	for _, line := range lines {
		line = strings.TrimSpace(line)

		// If line ends with backslash, it's a continuation
		if strings.HasSuffix(line, "\\") {
			// Remove the backslash and add space for proper separation
			line = strings.TrimSuffix(line, "\\")
			line = strings.TrimSpace(line)

			// Only add non-empty lines
			if line != "" {
				cleanedLines = append(cleanedLines, line)
			}
		} else {
			// Regular line, add it
			if line != "" {
				cleanedLines = append(cleanedLines, line)
			}
		}
	}

	// Join all lines with space
	return strings.Join(cleanedLines, " ")
}

// ValidateRule validates a parsed SecRule for completeness and correctness
func ValidateRule(rule *SecRule) error {
	if rule == nil {
		return ErrInvalidRule
	}

	// Check that we have variables
	if len(rule.Variables) == 0 {
		return ErrMissingVariables
	}

	// Check that operator is valid
	if rule.Operator.Type == "" {
		return ErrMissingOperator
	}

	// Check for required ID action
	hasID := false
	for _, action := range rule.Actions {
		if action.Name == ActionID {
			hasID = true
			// Validate ID is numeric
			if action.Parameter != "" {
				if _, err := strconv.Atoi(action.Parameter); err != nil {
					return fmt.Errorf("invalid rule ID '%s': must be numeric", action.Parameter)
				}
			}
			break
		}
	}

	if !hasID {
		return fmt.Errorf("rule missing required 'id' action")
	}

	return nil
}
