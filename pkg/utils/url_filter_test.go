package utils

import (
	"testing"
)

func TestParseCondition(t *testing.T) {
	tests := []struct {
		name      string
		input     string
		wantOp    string
		wantValue int
		wantErr   bool
	}{
		{
			name:      "greater than or equal",
			input:     ">=3",
			wantOp:    ">=",
			wantValue: 3,
			wantErr:   false,
		},
		{
			name:      "less than or equal",
			input:     "<=5",
			wantOp:    "<=",
			wantValue: 5,
			wantErr:   false,
		},
		{
			name:      "equal",
			input:     "==2",
			wantOp:    "==",
			wantValue: 2,
			wantErr:   false,
		},
		{
			name:      "greater than",
			input:     ">10",
			wantOp:    ">",
			wantValue: 10,
			wantErr:   false,
		},
		{
			name:      "less than",
			input:     "<1",
			wantOp:    "<",
			wantValue: 1,
			wantErr:   false,
		},
		{
			name:    "invalid format",
			input:   "invalid",
			wantErr: true,
		},
		{
			name:    "invalid value",
			input:   ">=abc",
			wantErr: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			cond, err := ParseCondition(tt.input)
			if tt.wantErr {
				if err == nil {
					t.Errorf("ParseCondition() expected error but got none")
				}
				return
			}
			if err != nil {
				t.Errorf("ParseCondition() unexpected error: %v", err)
				return
			}
			if cond.Operator != tt.wantOp {
				t.Errorf("ParseCondition() operator = %v, want %v", cond.Operator, tt.wantOp)
			}
			if cond.Value != tt.wantValue {
				t.Errorf("ParseCondition() value = %v, want %v", cond.Value, tt.wantValue)
			}
		})
	}
}

func TestConditionEvaluate(t *testing.T) {
	tests := []struct {
		name     string
		operator string
		value    int
		actual   int
		want     bool
	}{
		{
			name:     "greater than or equal - true",
			operator: ">=",
			value:    3,
			actual:   5,
			want:     true,
		},
		{
			name:     "greater than or equal - equal",
			operator: ">=",
			value:    3,
			actual:   3,
			want:     true,
		},
		{
			name:     "greater than or equal - false",
			operator: ">=",
			value:    3,
			actual:   2,
			want:     false,
		},
		{
			name:     "less than or equal - true",
			operator: "<=",
			value:    5,
			actual:   3,
			want:     true,
		},
		{
			name:     "equal - true",
			operator: "==",
			value:    3,
			actual:   3,
			want:     true,
		},
		{
			name:     "equal - false",
			operator: "==",
			value:    3,
			actual:   4,
			want:     false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			cond := &Condition{
				Operator: tt.operator,
				Value:    tt.value,
			}
			if got := cond.Evaluate(tt.actual); got != tt.want {
				t.Errorf("Condition.Evaluate() = %v, want %v", got, tt.want)
			}
		})
	}
}

func TestCountQueryParams(t *testing.T) {
	tests := []struct {
		name    string
		url     string
		want    int
		wantErr bool
	}{
		{
			name:    "no query params",
			url:     "https://example.com/path",
			want:    0,
			wantErr: false,
		},
		{
			name:    "one query param",
			url:     "https://example.com/path?key=value",
			want:    1,
			wantErr: false,
		},
		{
			name:    "three query params",
			url:     "https://example.com/path?a=1&b=2&c=3",
			want:    3,
			wantErr: false,
		},
		{
			name:    "multiple values for same key",
			url:     "https://example.com/path?key=value1&key=value2",
			want:    1,
			wantErr: false,
		},
		{
			name:    "five query params",
			url:     "https://example.com/path?a=1&b=2&c=3&d=4&e=5",
			want:    5,
			wantErr: false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got, err := CountQueryParams(tt.url)
			if (err != nil) != tt.wantErr {
				t.Errorf("CountQueryParams() error = %v, wantErr %v", err, tt.wantErr)
				return
			}
			if got != tt.want {
				t.Errorf("CountQueryParams() = %v, want %v", got, tt.want)
			}
		})
	}
}

func TestCountPathDepth(t *testing.T) {
	tests := []struct {
		name    string
		url     string
		want    int
		wantErr bool
	}{
		{
			name:    "root path",
			url:     "https://example.com/",
			want:    0,
			wantErr: false,
		},
		{
			name:    "single segment",
			url:     "https://example.com/path",
			want:    1,
			wantErr: false,
		},
		{
			name:    "three segments",
			url:     "https://example.com/a/b/c",
			want:    3,
			wantErr: false,
		},
		{
			name:    "four segments",
			url:     "https://example.com/a/b/c/d",
			want:    4,
			wantErr: false,
		},
		{
			name:    "trailing slash",
			url:     "https://example.com/a/b/c/",
			want:    3,
			wantErr: false,
		},
		{
			name:    "no path",
			url:     "https://example.com",
			want:    0,
			wantErr: false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got, err := CountPathDepth(tt.url)
			if (err != nil) != tt.wantErr {
				t.Errorf("CountPathDepth() error = %v, wantErr %v", err, tt.wantErr)
				return
			}
			if got != tt.want {
				t.Errorf("CountPathDepth() = %v, want %v", got, tt.want)
			}
		})
	}
}

func TestMatchesQueryParamCount(t *testing.T) {
	tests := []struct {
		name       string
		url        string
		conditions []string
		want       bool
		wantErr    bool
	}{
		{
			name:       "no conditions",
			url:        "https://example.com/path?a=1",
			conditions: []string{},
			want:       true,
			wantErr:    false,
		},
		{
			name:       "matches >= condition",
			url:        "https://example.com/path?a=1&b=2&c=3",
			conditions: []string{">=3"},
			want:       true,
			wantErr:    false,
		},
		{
			name:       "does not match >= condition",
			url:        "https://example.com/path?a=1&b=2",
			conditions: []string{">=3"},
			want:       false,
			wantErr:    false,
		},
		{
			name:       "matches multiple conditions",
			url:        "https://example.com/path?a=1&b=2&c=3",
			conditions: []string{">=2", "<=5"},
			want:       true,
			wantErr:    false,
		},
		{
			name:       "does not match one condition",
			url:        "https://example.com/path?a=1&b=2&c=3&d=4&e=5&f=6",
			conditions: []string{">=2", "<=5"},
			want:       false,
			wantErr:    false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got, err := MatchesQueryParamCount(tt.url, tt.conditions)
			if (err != nil) != tt.wantErr {
				t.Errorf("MatchesQueryParamCount() error = %v, wantErr %v", err, tt.wantErr)
				return
			}
			if got != tt.want {
				t.Errorf("MatchesQueryParamCount() = %v, want %v", got, tt.want)
			}
		})
	}
}

func TestMatchesPathDepth(t *testing.T) {
	tests := []struct {
		name       string
		url        string
		conditions []string
		want       bool
		wantErr    bool
	}{
		{
			name:       "no conditions",
			url:        "https://example.com/a/b/c",
			conditions: []string{},
			want:       true,
			wantErr:    false,
		},
		{
			name:       "matches >= condition",
			url:        "https://example.com/a/b/c/d",
			conditions: []string{">=4"},
			want:       true,
			wantErr:    false,
		},
		{
			name:       "does not match >= condition",
			url:        "https://example.com/a/b",
			conditions: []string{">=4"},
			want:       false,
			wantErr:    false,
		},
		{
			name:       "matches == condition",
			url:        "https://example.com/a/b/c/d",
			conditions: []string{"==4"},
			want:       true,
			wantErr:    false,
		},
		{
			name:       "matches multiple conditions",
			url:        "https://example.com/a/b/c",
			conditions: []string{">=2", "<=5"},
			want:       true,
			wantErr:    false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got, err := MatchesPathDepth(tt.url, tt.conditions)
			if (err != nil) != tt.wantErr {
				t.Errorf("MatchesPathDepth() error = %v, wantErr %v", err, tt.wantErr)
				return
			}
			if got != tt.want {
				t.Errorf("MatchesPathDepth() = %v, want %v", got, tt.want)
			}
		})
	}
}
