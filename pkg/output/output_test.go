package output

import (
	"net/http"
	"net/url"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/projectdiscovery/katana/pkg/navigation"
	"github.com/projectdiscovery/katana/pkg/utils/extensions"
	"github.com/stretchr/testify/require"
)

func createTestResult(rawURL, rawReq, rawResp, body string) *Result {
	parsedURL, _ := url.Parse(rawURL)
	return &Result{
		Request: &navigation.Request{
			Method: http.MethodGet,
			URL:    rawURL,
			Raw:    rawReq,
		},
		Response: &navigation.Response{
			Resp: &http.Response{
				Status:     "200 OK",
				StatusCode: 200,
				Request: &http.Request{
					Method: http.MethodGet,
					URL:    parsedURL,
				},
			},
			Raw:  rawResp,
			Body: body,
		},
	}
}

func TestStoreResponseDirPreservedWithNoClobber(t *testing.T) {
	tempDir, err := os.MkdirTemp("", "katana-test-ncb-*")
	require.NoError(t, err)
	defer os.RemoveAll(tempDir)

	sentinelFile := filepath.Join(tempDir, "sentinel.txt")
	require.NoError(t, os.WriteFile(sentinelFile, []byte("SENTINEL"), 0644))

	hostDir := filepath.Join(tempDir, "example.com")
	require.NoError(t, os.MkdirAll(hostDir, 0755))
	oldResponseFile := filepath.Join(hostDir, "old-response.txt")
	require.NoError(t, os.WriteFile(oldResponseFile, []byte("OLD_CONTENT"), 0644))

	indexFilePath := filepath.Join(tempDir, indexFile)
	require.NoError(t, os.WriteFile(indexFilePath, []byte("existing-index-line\n"), 0644))

	validator := extensions.NewValidator(nil, nil, true)
	writer, err := New(Options{
		StoreResponse:      true,
		StoreResponseDir:   tempDir,
		NoClobber:          true,
		ExtensionValidator: validator,
	})
	require.NoError(t, err)
	defer writer.Close()

	stdWriter, ok := writer.(*StandardWriter)
	require.True(t, ok)
	require.Equal(t, tempDir, stdWriter.storeResponseDir)

	sentinelContent, err := os.ReadFile(sentinelFile)
	require.NoError(t, err)
	require.Equal(t, "SENTINEL", string(sentinelContent))

	oldResponseContent, err := os.ReadFile(oldResponseFile)
	require.NoError(t, err)
	require.Equal(t, "OLD_CONTENT", string(oldResponseContent))

	indexContent, err := os.ReadFile(indexFilePath)
	require.NoError(t, err)
	require.True(t, strings.HasPrefix(string(indexContent), "existing-index-line\n"))
}

func TestStoreResponseDirWipedWithoutNoClobber(t *testing.T) {
	tempDir, err := os.MkdirTemp("", "katana-test-clobber-*")
	require.NoError(t, err)
	defer os.RemoveAll(tempDir)

	sentinelFile := filepath.Join(tempDir, "sentinel.txt")
	require.NoError(t, os.WriteFile(sentinelFile, []byte("SENTINEL"), 0644))

	validator := extensions.NewValidator(nil, nil, true)
	writer, err := New(Options{
		StoreResponse:      true,
		StoreResponseDir:   tempDir,
		NoClobber:          false,
		ExtensionValidator: validator,
	})
	require.NoError(t, err)
	defer writer.Close()

	_, err = os.Stat(sentinelFile)
	require.True(t, os.IsNotExist(err))

	indexFilePath := filepath.Join(tempDir, indexFile)
	indexContent, err := os.ReadFile(indexFilePath)
	require.NoError(t, err)
	require.Empty(t, string(indexContent))
}

func TestStoreResponseEmptyDirWithNoClobber(t *testing.T) {
	tempDir, err := os.MkdirTemp("", "katana-test-empty-*")
	require.NoError(t, err)
	defer os.RemoveAll(tempDir)

	validator := extensions.NewValidator(nil, nil, true)
	writer, err := New(Options{
		StoreResponse:      true,
		StoreResponseDir:   tempDir,
		NoClobber:          true,
		ExtensionValidator: validator,
	})
	require.NoError(t, err)
	defer writer.Close()

	stdWriter, ok := writer.(*StandardWriter)
	require.True(t, ok)
	require.Equal(t, tempDir, stdWriter.storeResponseDir)

	result := createTestResult(
		"https://example.com/test",
		"GET /test HTTP/1.1\r\nHost: example.com\r\n\r\n",
		"HTTP/1.1 200 OK\r\n\r\nHello",
		"Hello",
	)
	err = writer.Write(result)
	require.NoError(t, err)

	require.NotEmpty(t, result.Response.StoredResponsePath)
	require.True(t, strings.HasPrefix(result.Response.StoredResponsePath, tempDir))

	storedContent, err := os.ReadFile(result.Response.StoredResponsePath)
	require.NoError(t, err)
	require.Contains(t, string(storedContent), "Hello")

	indexContent, err := os.ReadFile(filepath.Join(tempDir, indexFile))
	require.NoError(t, err)
	require.Contains(t, string(indexContent), "https://example.com/test")
}

func TestStoreResponseFileWriteBehaviorWithNoClobber(t *testing.T) {
	tempDir, err := os.MkdirTemp("", "katana-test-filewrite-ncb-*")
	require.NoError(t, err)
	defer os.RemoveAll(tempDir)

	pageURL := "https://example.com/page1"
	domain, err := getResponseHost(pageURL)
	require.NoError(t, err)
	preExistingFile := getResponseFileName(tempDir, domain, pageURL)
	require.NoError(t, os.MkdirAll(filepath.Dir(preExistingFile), 0755))
	require.NoError(t, os.WriteFile(preExistingFile, []byte("PREEXISTING_CONTENT\n"), 0644))

	indexFilePath := filepath.Join(tempDir, indexFile)
	initialIndex := preExistingFile + " " + pageURL + " (200 OK)\n"
	require.NoError(t, os.WriteFile(indexFilePath, []byte(initialIndex), 0644))

	validator := extensions.NewValidator(nil, nil, true)
	writer, err := New(Options{
		StoreResponse:      true,
		StoreResponseDir:   tempDir,
		NoClobber:          true,
		ExtensionValidator: validator,
	})
	require.NoError(t, err)
	defer writer.Close()

	clobberAttemptResult := createTestResult(
		pageURL,
		"GET /page1 HTTP/1.1\r\nHost: example.com\r\n\r\n",
		"HTTP/1.1 200 OK\r\n\r\nOVERWRITTEN_ATTEMPT",
		"OVERWRITTEN_ATTEMPT",
	)
	err = writer.Write(clobberAttemptResult)
	require.NoError(t, err)

	contentAfter, err := os.ReadFile(preExistingFile)
	require.NoError(t, err)
	require.Equal(t, "PREEXISTING_CONTENT\n", string(contentAfter))

	indexContent, err := os.ReadFile(indexFilePath)
	require.NoError(t, err)
	require.Equal(t, initialIndex, string(indexContent))

	newPageURL := "https://example.com/page2"
	newResult := createTestResult(
		newPageURL,
		"GET /page2 HTTP/1.1\r\nHost: example.com\r\n\r\n",
		"HTTP/1.1 200 OK\r\n\r\nNEW_CONTENT",
		"NEW_CONTENT",
	)
	err = writer.Write(newResult)
	require.NoError(t, err)

	newFile := getResponseFileName(tempDir, domain, newPageURL)
	newContent, err := os.ReadFile(newFile)
	require.NoError(t, err)
	require.Contains(t, string(newContent), "NEW_CONTENT")

	updatedIndex, err := os.ReadFile(indexFilePath)
	require.NoError(t, err)
	require.Contains(t, string(updatedIndex), newPageURL)
}

func TestStoreResponseFileWriteBehaviorWithoutNoClobber(t *testing.T) {
	tempDir, err := os.MkdirTemp("", "katana-test-filewrite-clobber-*")
	require.NoError(t, err)
	defer os.RemoveAll(tempDir)

	validator := extensions.NewValidator(nil, nil, true)
	writer, err := New(Options{
		StoreResponse:      true,
		StoreResponseDir:   tempDir,
		NoClobber:          false,
		ExtensionValidator: validator,
	})
	require.NoError(t, err)
	defer writer.Close()

	pageURL := "https://example.com/page"
	result1 := createTestResult(
		pageURL,
		"GET /page HTTP/1.1\r\nHost: example.com\r\n\r\n",
		"HTTP/1.1 200 OK\r\n\r\nFIRST_CONTENT",
		"FIRST_CONTENT",
	)
	err = writer.Write(result1)
	require.NoError(t, err)

	domain, err := getResponseHost(pageURL)
	require.NoError(t, err)
	responseFile := getResponseFileName(tempDir, domain, pageURL)

	content1, err := os.ReadFile(responseFile)
	require.NoError(t, err)
	require.Contains(t, string(content1), "FIRST_CONTENT")

	result2 := createTestResult(
		pageURL,
		"GET /page HTTP/1.1\r\nHost: example.com\r\n\r\n",
		"HTTP/1.1 200 OK\r\n\r\nUPDATED_CONTENT",
		"UPDATED_CONTENT",
	)
	err = writer.Write(result2)
	require.NoError(t, err)

	content2, err := os.ReadFile(responseFile)
	require.NoError(t, err)
	require.Contains(t, string(content2), "UPDATED_CONTENT")
	require.NotContains(t, string(content2), "FIRST_CONTENT")
}
