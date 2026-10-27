package output

import (
	"os"
	"path/filepath"
	"testing"

	"github.com/stretchr/testify/require"
)

func TestCreateDirNameNoClobber_EmptyDirectory(t *testing.T) {
	tempDir := t.TempDir()
	emptySubDir := filepath.Join(tempDir, "archive")
	err := os.Mkdir(emptySubDir, 0755)
	require.NoError(t, err)

	// An empty directory should not increment suffix
	resultDir := createDirNameNoClobber(emptySubDir)
	require.Equal(t, emptySubDir, resultDir)

	// After adding a file, it should increment suffix to archive1
	err = os.WriteFile(filepath.Join(emptySubDir, "sample.txt"), []byte("test"), 0644)
	require.NoError(t, err)

	resultDirWithFile := createDirNameNoClobber(emptySubDir)
	require.Equal(t, filepath.Join(tempDir, "archive1"), resultDirWithFile)
}

func TestStoreResponse_PreserveCustomDirectory(t *testing.T) {
	tempDir := t.TempDir()
	customStoreDir := filepath.Join(tempDir, "my_archive")
	err := os.Mkdir(customStoreDir, 0755)
	require.NoError(t, err)

	sentinelFile := filepath.Join(customStoreDir, "sentinel.txt")
	err = os.WriteFile(sentinelFile, []byte("SENTINEL"), 0644)
	require.NoError(t, err)

	indexFile := filepath.Join(customStoreDir, "index.txt")
	err = os.WriteFile(indexFile, []byte("existing_entry\n"), 0644)
	require.NoError(t, err)

	options := Options{
		StoreResponse:    true,
		StoreResponseDir: customStoreDir,
		NoClobber:        false,
	}

	writer, err := New(options)
	require.NoError(t, err)
	require.NotNil(t, writer)

	// Sentinel file must still exist
	require.FileExists(t, sentinelFile)
	content, err := os.ReadFile(sentinelFile)
	require.NoError(t, err)
	require.Equal(t, "SENTINEL", string(content))

	// Index file must not be truncated
	indexContent, err := os.ReadFile(indexFile)
	require.NoError(t, err)
	require.Contains(t, string(indexContent), "existing_entry")
}
