package parser

import (
	"github.com/rohith0456/mattermost-support-package-repro/internal/ingestion"
	"github.com/rohith0456/mattermost-support-package-repro/pkg/models"
)

// Parser orchestrates all sub-parsers and produces a SupportPackage model.
type Parser struct{}

// NewParser creates a Parser.
func NewParser() *Parser {
	return &Parser{}
}

// Parse runs all sub-parsers on the normalized package and returns a SupportPackage.
func (p *Parser) Parse(np *ingestion.NormalizedPackage, srcPath string) *models.SupportPackage {
	sp := &models.SupportPackage{
		SourcePath: srcPath,
	}

	sp.Version = ParseVersion(np)
	sp.Topology = ParseTopology(np)
	sp.Database = ParseDatabase(np)
	sp.FileStorage = ParseFileStorage(np)
	sp.Auth = ParseAuth(np)
	sp.Plugins = ParsePlugins(np)
	sp.Integrations = ParseIntegrations(np)
	sp.Observability = ParseObservability(np)

	// Carry forward parse warnings
	sp.ParseWarnings = append(sp.ParseWarnings, np.Warnings...)

	return sp
}
