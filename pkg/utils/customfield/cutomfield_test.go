package customfield

import "testing"

func TestPart_ToString(t *testing.T) {
	tests := []struct {
		name string
		p    Part
		want string
	}{
		{
			name: "Test Header",
			p:    0,
			want: "header",
		},
		{
			name: "Test Body",
			p:    1,
			want: "body",
		},
		{
			name: "Test Header",
			p:    2,
			want: "response",
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got := tt.p.ToString(); got != tt.want {
				t.Errorf("Part.ToString() = %v, want %v", got, tt.want)
			}
		})
	}
}
