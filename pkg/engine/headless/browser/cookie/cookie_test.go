package cookie

import (
	"testing"

	"github.com/go-rod/rod/lib/proto"
)

func TestShouldBlockRequest(t *testing.T) {
	tests := []struct {
		name            string
		url             string
		resourceType    proto.NetworkResourceType
		initiatorDomain string
		want            bool
	}{
		{
			name:            "should block trustarc consent notice",
			url:             "https://consent.trustarc.com/notice?domain=hackerone.com",
			resourceType:    proto.NetworkResourceTypeScript,
			initiatorDomain: "hackerone.com",
			want:            true,
		},
		{
			name:            "should not block trustarc for excluded domain",
			url:             "https://consent.trustarc.com/notice",
			resourceType:    proto.NetworkResourceTypeScript,
			initiatorDomain: "forbes.com",
			want:            false,
		},
		{
			name:            "should not block non-matching URL",
			url:             "https://example.com/other-path",
			resourceType:    proto.NetworkResourceTypeScript,
			initiatorDomain: "hackerone.com",
			want:            false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := ShouldBlockRequest(tt.url, tt.resourceType, tt.initiatorDomain)
			if got != tt.want {
				t.Errorf("ShouldBlockRequest() = %v, want %v", got, tt.want)
			}
		})
	}
}
