package normalizer

import (
	"regexp"
	"testing"
)

func TestTextNormalizer_Apply(t *testing.T) {
	type fields struct {
		patterns []*regexp.Regexp
	}
	type args struct {
		text string
	}
	tests := []struct {
		name   string
		fields fields
		args   args
		want   string
	}{
		{
			name: "test",
			fields: fields{
				patterns: []*regexp.Regexp{regexp.MustCompile(`[A-Za-z0-9._%+-]+@[A-Za-z0-9.-]+\.[A-Z|a-z]{2,}`)},
			},
			args: args{
				text: "<a href='nav'>nizamul@pd.io</a>",
			},
			want: "<a href='nav'></a>",
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			n := &TextNormalizer{
				patterns: tt.fields.patterns,
			}
			if got := n.Apply(tt.args.text); got != tt.want {
				t.Errorf("TextNormalizer.Apply() = %v, want %v", got, tt.want)
			}
		})
	}
}
