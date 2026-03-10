package hybrid

import (
	"errors"
	"net/http"
	"testing"

	"github.com/projectdiscovery/katana/pkg/navigation"
	"github.com/stretchr/testify/require"
)

func TestGetOutputBodyFallsBackToCapturedResponse(t *testing.T) {
	resp := &navigation.Response{
		Resp: &http.Response{},
		Body: "<html>fallback</html>",
	}

	body, err := getOutputBody(func() (string, error) {
		return "", errors.New("could not get dom")
	}, resp)

	require.NoError(t, err)
	require.Equal(t, "<html>fallback</html>", body)
}

func TestGetOutputBodyErrorsWithoutFallback(t *testing.T) {
	body, err := getOutputBody(func() (string, error) {
		return "", errors.New("could not get dom")
	}, &navigation.Response{})

	require.Error(t, err)
	require.Empty(t, body)
}

func TestGetOutputBodyReturnsCapturedEmptyBody(t *testing.T) {
	resp := &navigation.Response{
		Resp: &http.Response{},
		Body: "",
	}

	body, err := getOutputBody(func() (string, error) {
		return "", errors.New("could not get dom")
	}, resp)

	require.NoError(t, err)
	require.Empty(t, body)
}
