package ingestion_test

import (
	"archive/zip"
	"os"
	"path/filepath"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/rohith0456/mattermost-support-package-repro/internal/ingestion"
)

func TestIngest_MinimalPackage(t *testing.T) {
	// Create a minimal in-memory ZIP for testing
	tmpZip := createTestZip(t, map[string]string{
		"config.json": `{"ServiceSettings": {"Version": "8.1.0"}}`,
	})

	workDir := t.TempDir()
	ingestor := ingestion.NewIngestor(workDir)
	pkg, err := ingestor.Ingest(tmpZip)
	require.NoError(t, err)
	defer pkg.Cleanup()

	assert.NotEmpty(t, pkg.ExtractDir)
	assert.Equal(t, "standard", pkg.Format)
	assert.NotEmpty(t, pkg.RawFiles)

	// Should find config.json
	configPath := pkg.FindFile("config.json")
	assert.NotEmpty(t, configPath)
	assert.FileExists(t, configPath)
}

func TestIngest_MissingFile(t *testing.T) {
	ingestor := ingestion.NewIngestor(t.TempDir())
	_, err := ingestor.Ingest("./nonexistent.zip")
	assert.Error(t, err)
}

func TestIngest_UnknownFormat(t *testing.T) {
	tmpZip := createTestZip(t, map[string]string{
		"some-random-file.txt": "random content",
	})

	ingestor := ingestion.NewIngestor(t.TempDir())
	pkg, err := ingestor.Ingest(tmpZip)
	require.NoError(t, err)
	defer pkg.Cleanup()

	assert.Equal(t, "unknown", pkg.Format)
}

func TestIngest_FindFileByPattern(t *testing.T) {
	tmpZip := createTestZip(t, map[string]string{
		"logs/mattermost.log": "2024-01-01 Starting Server version=8.1.0",
		"config.json":         `{"ServiceSettings": {"Version": "8.1.0"}}`,
	})

	ingestor := ingestion.NewIngestor(t.TempDir())
	pkg, err := ingestor.Ingest(tmpZip)
	require.NoError(t, err)
	defer pkg.Cleanup()

	logFiles := pkg.FindFilesByPattern("mattermost.log")
	assert.NotEmpty(t, logFiles)
}

func TestNormalizer_WithConfig(t *testing.T) {
	tmpZip := createTestZip(t, map[string]string{
		"config.json": `{
			"ServiceSettings": {"Version": "8.1.0", "SiteURL": "http://localhost:8065"},
			"SqlSettings": {"DriverName": "postgres"}
		}`,
	})

	ingestor := ingestion.NewIngestor(t.TempDir())
	pkg, err := ingestor.Ingest(tmpZip)
	require.NoError(t, err)
	defer pkg.Cleanup()

	normalizer := ingestion.NewNormalizer()
	np := normalizer.Normalize(pkg)

	assert.NotEmpty(t, np.Config)
	assert.Contains(t, np.Config, "ServiceSettings")
}

func TestNormalizer_MissingFiles(t *testing.T) {
	// Package with no known files — should not panic
	tmpZip := createTestZip(t, map[string]string{
		"unknown.txt": "some content",
	})

	ingestor := ingestion.NewIngestor(t.TempDir())
	pkg, err := ingestor.Ingest(tmpZip)
	require.NoError(t, err)
	defer pkg.Cleanup()

	normalizer := ingestion.NewNormalizer()
	np := normalizer.Normalize(pkg)

	// Should return empty maps, not nil
	assert.NotNil(t, np.Config)
	assert.NotNil(t, np.Diagnostics)
}

// createTestZip creates a temporary ZIP file with the given file contents.
func createTestZip(t *testing.T, files map[string]string) string {
	t.Helper()
	tmpDir := t.TempDir()
	zipPath := filepath.Join(tmpDir, "test-package.zip")

	w, err := os.Create(zipPath)
	require.NoError(t, err)
	defer w.Close()

	zw := zip.NewWriter(w)
	defer zw.Close()

	for name, content := range files {
		f, err := zw.Create(name)
		require.NoError(t, err)
		_, err = f.Write([]byte(content))
		require.NoError(t, err)
	}

	return zipPath
}
