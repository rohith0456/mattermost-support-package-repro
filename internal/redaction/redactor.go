// Package redaction detects and replaces sensitive values in parsed config data.
// IMPORTANT: Original values are never stored, logged, or returned.
// The redactor only keeps track of which keys were redacted and their replacement placeholders.
package redaction

import (
	"fmt"
	"strings"
	"time"

	"github.com/rohith0456/mattermost-support-package-repro/pkg/models"
)

// Redactor applies redaction rules to a config map.
type Redactor struct {
	rules        []models.RedactionRule
	strictMode   bool
}

// NewRedactor creates a Redactor with default rules.
func NewRedactor(strict bool) *Redactor {
	rules := DefaultRules()
	if strict {
		rules = StrictRules()
	}
	return &Redactor{
		rules:      rules,
		strictMode: strict,
	}
}

// RedactConfig applies redaction rules to a config map in place and returns a report.
// The original config map is modified — sensitive values are replaced with placeholders.
func (r *Redactor) RedactConfig(config map[string]interface{}, srcPackage, location string) *models.RedactionReport {
	report := &models.RedactionReport{
		GeneratedAt:   time.Now(),
		SourcePackage: srcPackage,
	}

	categoryMap := make(map[string]*models.RedactionCategory)
	for i := range r.rules {
		rule := &r.rules[i]
		cat := &models.RedactionCategory{
			Name:        rule.Name,
			Description: rule.Description,
			Severity:    rule.Severity,
		}
		categoryMap[rule.ID] = cat
	}

	r.redactMap(config, location, "$", categoryMap)

	for _, cat := range categoryMap {
		if cat.Count > 0 {
			report.Categories = append(report.Categories, *cat)
			report.TotalRedacted += cat.Count
			if cat.Severity == "high" {
				report.HighSeverityCount += cat.Count
			}
		}
	}

	if r.strictMode {
		report.Notes = append(report.Notes, "Strict redaction mode enabled: additional fields were redacted")
	}

	return report
}

// redactMap recursively walks a map and replaces sensitive values.
func (r *Redactor) redactMap(m map[string]interface{}, location, jsonPath string, cats map[string]*models.RedactionCategory) {
	for key, val := range m {
		currentPath := jsonPath + "." + key

		switch v := val.(type) {
		case map[string]interface{}:
			r.redactMap(v, location, currentPath, cats)
		case []interface{}:
			r.redactSlice(v, key, location, currentPath, cats)
		case string:
			if v == "" {
				continue
			}
			if ruleID, replacement := r.matchesRule(key); ruleID != "" {
				// Replace in place — original value is immediately discarded
				m[key] = replacement
				if cat, ok := cats[ruleID]; ok {
					cat.Count++
					cat.Items = append(cat.Items, models.RedactedItem{
						ConfigKey:     key,
						Location:      location,
						JsonPath:      currentPath,
						Replacement:   replacement,
						DetectionRule: ruleID,
						WasEmpty:      false,
					})
				}
			}
		}
	}
}

func (r *Redactor) redactSlice(arr []interface{}, parentKey, location, jsonPath string, cats map[string]*models.RedactionCategory) {
	// Check if the parent key itself is a sensitive list (e.g., DataSourceReplicas)
	if ruleID, replacement := r.matchesRule(parentKey); ruleID != "" {
		// Replace all string items in the slice
		for i, item := range arr {
			if s, ok := item.(string); ok && s != "" {
				arr[i] = replacement
				if cat, ok := cats[ruleID]; ok {
					cat.Count++
					cat.Items = append(cat.Items, models.RedactedItem{
						ConfigKey:     parentKey,
						Location:      location,
						JsonPath:      jsonPath + fmt.Sprintf("[%d]", i),
						Replacement:   replacement,
						DetectionRule: ruleID,
						WasEmpty:      false,
					})
				}
			}
		}
		return
	}

	// Otherwise recurse into maps in the slice
	for _, item := range arr {
		if m, ok := item.(map[string]interface{}); ok {
			r.redactMap(m, location, jsonPath+"[]", cats)
		}
	}
}

// matchesRule checks if a key matches any redaction rule.
// Returns (ruleID, replacement) or ("", "") if no match.
func (r *Redactor) matchesRule(key string) (string, string) {
	keyLower := strings.ToLower(key)
	for _, rule := range r.rules {
		for _, pattern := range rule.Patterns {
			if strings.ToLower(pattern) == keyLower ||
				strings.Contains(keyLower, strings.ToLower(pattern)) {
				return rule.ID, rule.Replacement
			}
		}
	}
	return "", ""
}
