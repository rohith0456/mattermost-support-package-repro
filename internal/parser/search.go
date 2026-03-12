package parser

import (
	"strings"

	"github.com/rohith0456/mattermost-support-package-repro/internal/ingestion"
	"github.com/rohith0456/mattermost-support-package-repro/pkg/models"
)

// ParseSearch extracts search backend configuration signals.
// Reads ElasticsearchSettings (covers both Elasticsearch and OpenSearch deployments).
func ParseSearch(np *ingestion.NormalizedPackage) models.SearchInfo {
	info := models.SearchInfo{Backend: "database"}

	esSettings := getNestedMap(np.Config, "ElasticsearchSettings")
	if esSettings == nil {
		return info
	}

	// Check if indexing is enabled — this means search backend is active
	indexingEnabled := false
	if v, _ := esSettings["EnableIndexing"].(bool); v {
		indexingEnabled = true
	}
	if s := getNestedString(esSettings, "EnableIndexing"); s == "true" {
		indexingEnabled = true
	}

	if !indexingEnabled {
		return info
	}

	// Detect backend type from connection URL
	connURL := getNestedString(esSettings, "ConnectionURL")
	connURLLower := strings.ToLower(connURL)
	switch {
	case strings.Contains(connURLLower, "opensearch"):
		info.Backend = "opensearch"
	case strings.Contains(connURLLower, "es.amazonaws.com"),
		strings.Contains(connURLLower, "elastic"):
		info.Backend = "elasticsearch"
	default:
		// Default: OpenSearch (Mattermost 9.x+ ships with OpenSearch support by default)
		info.Backend = "opensearch"
	}

	// Index prefix (safe to store — not a secret)
	info.IndexPrefix = getNestedString(esSettings, "IndexPrefix")

	// Live vs bulk indexing state
	if v, _ := esSettings["EnableIndexing"].(bool); v {
		info.BulkIndexing = true
	}
	if s := getNestedString(esSettings, "EnableIndexing"); s == "true" {
		info.BulkIndexing = true
	}
	if v, _ := esSettings["LiveIndexingBatchSize"].(float64); v > 0 {
		info.LiveIndexing = true
	}
	if s := getNestedString(esSettings, "LiveIndexingBatchSize"); s != "" && s != "0" {
		info.LiveIndexing = true
	}

	return info
}
