package utils

import (
	"fmt"
	"net/url"
	"strconv"
	"strings"
)

// Condition represents a comparison condition.
type Condition struct {
	// Operator is the comparison operator (>=, <=, ==, >, <).
	Operator string
	// Value is the numeric value to compare against.
	Value int
}

// ParseCondition parses a condition string like ">=3", "==5", "<2".
func ParseCondition(condStr string) (*Condition, error) {
	condStr = strings.TrimSpace(condStr)

	operators := []string{">=", "<=", "==", ">", "<"}
	for _, op := range operators {
		if strings.HasPrefix(condStr, op) {
			valueStr := strings.TrimSpace(condStr[len(op):])
			value, err := strconv.Atoi(valueStr)
			if err != nil {
				return nil, fmt.Errorf("invalid value in condition %s: %v", condStr, err)
			}
			return &Condition{
				Operator: op,
				Value:    value,
			}, nil
		}
	}

	return nil, fmt.Errorf("invalid condition format: %s (expected format: >=3, ==5, <2, etc.)", condStr)
}

// Evaluate checks if the actual value satisfies the condition.
func (c *Condition) Evaluate(actual int) bool {
	switch c.Operator {
	case ">=":
		return actual >= c.Value
	case "<=":
		return actual <= c.Value
	case "==":
		return actual == c.Value
	case ">":
		return actual > c.Value
	case "<":
		return actual < c.Value
	default:
		return false
	}
}

// CountQueryParams counts the number of query parameters in a URL.
func CountQueryParams(rawURL string) (int, error) {
	parsedURL, err := url.Parse(rawURL)
	if err != nil {
		return 0, err
	}

	values := parsedURL.Query()
	return len(values), nil
}

// CountPathDepth counts the depth of the path in a URL.
// For example: /a/b/c has depth 3, / has depth 0.
func CountPathDepth(rawURL string) (int, error) {
	parsedURL, err := url.Parse(rawURL)
	if err != nil {
		return 0, err
	}

	path := strings.Trim(parsedURL.Path, "/")
	if path == "" {
		return 0, nil
	}

	segments := strings.Split(path, "/")
	return len(segments), nil
}

// CheckConditions checks if a value satisfies all conditions.
func CheckConditions(value int, conditionStrs []string) (bool, error) {
	if len(conditionStrs) == 0 {
		return true, nil
	}

	for _, condStr := range conditionStrs {
		cond, err := ParseCondition(condStr)
		if err != nil {
			return false, err
		}

		if !cond.Evaluate(value) {
			return false, nil
		}
	}

	return true, nil
}

// MatchesQueryParamCount checks if URL matches query parameter count conditions.
func MatchesQueryParamCount(rawURL string, conditions []string) (bool, error) {
	if len(conditions) == 0 {
		return true, nil
	}

	count, err := CountQueryParams(rawURL)
	if err != nil {
		return false, err
	}

	return CheckConditions(count, conditions)
}

// MatchesPathDepth checks if URL matches path depth conditions.
func MatchesPathDepth(rawURL string, conditions []string) (bool, error) {
	if len(conditions) == 0 {
		return true, nil
	}

	depth, err := CountPathDepth(rawURL)
	if err != nil {
		return false, err
	}

	return CheckConditions(depth, conditions)
}
