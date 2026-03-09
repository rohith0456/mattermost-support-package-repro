package models

import "time"

// RedactionReport is the machine-readable + human-readable record of
// every value that was detected and redacted during support package parsing.
// Original values are NEVER stored here — only categories and counts.
type RedactionReport struct {
	GeneratedAt     time.Time          `json:"generated_at"`
	SourcePackage   string             `json:"source_package"`
	TotalRedacted   int                `json:"total_redacted"`
	Categories      []RedactionCategory `json:"categories"`
	HighSeverityCount int              `json:"high_severity_count"`
	Notes           []string           `json:"notes,omitempty"`
}

// RedactionCategory groups redacted items by type.
type RedactionCategory struct {
	Name        string           `json:"name"`
	Description string           `json:"description"`
	Severity    string           `json:"severity"` // "high", "medium", "low"
	Count       int              `json:"count"`
	Items       []RedactedItem   `json:"items"`
}

// RedactedItem describes a single redacted value.
// The original value is NEVER included.
type RedactedItem struct {
	ConfigKey      string `json:"config_key"`
	Location       string `json:"location"`       // file path within package
	JsonPath       string `json:"json_path,omitempty"`
	Replacement    string `json:"replacement"`    // what replaced it (e.g. "REDACTED_DB_PASSWORD")
	DetectionRule  string `json:"detection_rule"` // which rule triggered the redaction
	WasEmpty       bool   `json:"was_empty"`      // if true, original value was already empty
}

// RedactionRule defines a rule for detecting and redacting secrets.
type RedactionRule struct {
	ID          string   `json:"id"`
	Name        string   `json:"name"`
	Description string   `json:"description"`
	Severity    string   `json:"severity"`
	Patterns    []string `json:"patterns"`     // key name patterns (case-insensitive)
	Replacement string   `json:"replacement"`  // placeholder value to use
}

// Well-known placeholder values used by the redaction engine.
// These are safe constant strings, never original values.
const (
	PlaceholderPassword    = "REDACTED_PASSWORD_SEE_REDACTION_REPORT"
	PlaceholderSecret      = "REDACTED_SECRET_SEE_REDACTION_REPORT"
	PlaceholderKey         = "REDACTED_KEY_SEE_REDACTION_REPORT"
	PlaceholderCertificate = "REDACTED_CERTIFICATE_SEE_REDACTION_REPORT"
	PlaceholderToken       = "REDACTED_TOKEN_SEE_REDACTION_REPORT"
	PlaceholderDSN         = "REDACTED_DSN_SEE_REDACTION_REPORT"
	PlaceholderURL         = "REDACTED_URL_SEE_REDACTION_REPORT"
	PlaceholderLicense     = "REDACTED_LICENSE_SEE_REDACTION_REPORT"
	PlaceholderEmail       = "REDACTED_EMAIL_SEE_REDACTION_REPORT"
	PlaceholderSMTP        = "REDACTED_SMTP_SEE_REDACTION_REPORT"
)
