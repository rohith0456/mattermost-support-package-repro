// Package ingestion handles unpacking and normalizing Mattermost support packages.
package ingestion

import (
	"archive/zip"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"strings"
	"time"
)

// PackageInfo holds metadata about an unpacked support package.
type PackageInfo struct {
	SourcePath   string
	ExtractDir   string
	Format       string // "standard", "cloud", "legacy", "unknown"
	ExtractedAt  time.Time
	FileIndex    map[string]string // logical name -> actual path
	RawFiles     []string          // all file paths found
}

// Ingestor handles unpacking and file discovery for support packages.
type Ingestor struct {
	workDir string
}

// NewIngestor creates an Ingestor that extracts into workDir.
func NewIngestor(workDir string) *Ingestor {
	return &Ingestor{workDir: workDir}
}

// Ingest unpacks the support package at srcPath and returns a PackageInfo.
func (i *Ingestor) Ingest(srcPath string) (*PackageInfo, error) {
	absPath, err := filepath.Abs(srcPath)
	if err != nil {
		return nil, fmt.Errorf("resolving path: %w", err)
	}

	// Create unique extraction directory
	ts := time.Now().Format("20060102-150405")
	extractDir := filepath.Join(i.workDir, "extract-"+ts)
	if err := os.MkdirAll(extractDir, 0o700); err != nil {
		return nil, fmt.Errorf("creating extract dir: %w", err)
	}

	if err := unzip(absPath, extractDir); err != nil {
		return nil, fmt.Errorf("extracting package: %w", err)
	}

	pkg := &PackageInfo{
		SourcePath:  absPath,
		ExtractDir:  extractDir,
		ExtractedAt: time.Now(),
		FileIndex:   make(map[string]string),
	}

	if err := pkg.walkFiles(); err != nil {
		return nil, fmt.Errorf("indexing files: %w", err)
	}

	pkg.Format = detectFormat(pkg.FileIndex)
	return pkg, nil
}

// walkFiles walks the extract directory and builds the file index.
func (p *PackageInfo) walkFiles() error {
	return filepath.Walk(p.ExtractDir, func(path string, info os.FileInfo, err error) error {
		if err != nil {
			// Non-fatal: log to stderr and continue with remaining files
			fmt.Fprintf(os.Stderr, "warning: skipping unreadable file %s: %v\n", path, err)
			return nil
		}
		if info.IsDir() {
			return nil
		}
		rel, err := filepath.Rel(p.ExtractDir, path)
		if err != nil {
			return nil
		}
		// Store with forward slashes for portability
		rel = filepath.ToSlash(rel)
		p.RawFiles = append(p.RawFiles, rel)

		// Build logical name index (basename -> path, shortest/root path wins)
		base := filepath.Base(rel)
		if _, exists := p.FileIndex[base]; !exists {
			p.FileIndex[base] = path
		}

		// Also index by relative path (always stored — full path is always unique)
		p.FileIndex[rel] = path
		return nil
	})
}

// FindFile returns the absolute path for a logical or relative file name.
// Returns empty string if not found.
func (p *PackageInfo) FindFile(name string) string {
	if v, ok := p.FileIndex[name]; ok {
		return v
	}
	// Try case-insensitive match
	nameLower := strings.ToLower(name)
	for k, v := range p.FileIndex {
		if strings.ToLower(k) == nameLower {
			return v
		}
	}
	return ""
}

// FindFilesByPattern returns all files whose relative paths contain the pattern.
func (p *PackageInfo) FindFilesByPattern(pattern string) []string {
	var results []string
	patLower := strings.ToLower(pattern)
	for _, rel := range p.RawFiles {
		if strings.Contains(strings.ToLower(rel), patLower) {
			results = append(results, filepath.Join(p.ExtractDir, filepath.FromSlash(rel)))
		}
	}
	return results
}

// Cleanup removes the extraction directory.
func (p *PackageInfo) Cleanup() error {
	if p.ExtractDir == "" {
		return nil
	}
	return os.RemoveAll(p.ExtractDir)
}

// detectFormat inspects known paths to classify the package format.
func detectFormat(index map[string]string) string {
	knownPaths := map[string]string{
		"config.json":              "standard",
		"mattermost.log":           "standard",
		"mattermost-cloud.json":    "cloud",
		"diagnostic.json":          "standard",
		"system_info.json":         "standard",
		"support_packet.yaml":      "standard",
		"support_packet.json":      "standard",
	}
	for name, format := range knownPaths {
		if _, ok := index[name]; ok {
			return format
		}
		// Check suffix matches
		for k := range index {
			if strings.HasSuffix(strings.ToLower(k), strings.ToLower(name)) {
				return format
			}
		}
	}
	return "unknown"
}

// unzip extracts a ZIP archive to destDir.
// It sanitizes paths to prevent directory traversal attacks.
func unzip(src, destDir string) error {
	r, err := zip.OpenReader(src)
	if err != nil {
		return fmt.Errorf("opening zip: %w", err)
	}
	defer r.Close()

	for _, f := range r.File {
		// Sanitize path
		cleanPath := filepath.Clean(f.Name)
		if strings.HasPrefix(cleanPath, "..") {
			continue // skip directory traversal attempts
		}

		target := filepath.Join(destDir, cleanPath)

		// Ensure target is within destDir
		if !strings.HasPrefix(filepath.Clean(target)+string(os.PathSeparator), filepath.Clean(destDir)+string(os.PathSeparator)) {
			continue
		}

		if f.FileInfo().IsDir() {
			if err := os.MkdirAll(target, 0o700); err != nil {
				return err
			}
			continue
		}

		if err := os.MkdirAll(filepath.Dir(target), 0o700); err != nil {
			return err
		}

		if err := extractFile(f, target); err != nil {
			// Log but don't fail on individual file errors
			continue
		}
	}
	return nil
}

func extractFile(f *zip.File, target string) error {
	rc, err := f.Open()
	if err != nil {
		return err
	}
	defer rc.Close()

	out, err := os.OpenFile(target, os.O_WRONLY|os.O_CREATE|os.O_TRUNC, 0o600)
	if err != nil {
		return err
	}

	// Limit file size to 500MB to prevent decompression bombs
	_, err = io.Copy(out, io.LimitReader(rc, 500*1024*1024))
	// Close explicitly (not deferred) so write errors surfaced at Close() are not lost
	if closeErr := out.Close(); closeErr != nil && err == nil {
		err = closeErr
	}
	return err
}
