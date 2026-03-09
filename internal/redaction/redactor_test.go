package redaction_test

import (
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/rohith0456/mattermost-support-package-repro/internal/redaction"
	"github.com/rohith0456/mattermost-support-package-repro/pkg/models"
)

func TestRedactConfig_BasicPassword(t *testing.T) {
	r := redaction.NewRedactor(false)
	config := map[string]interface{}{
		"SqlSettings": map[string]interface{}{
			"DriverName": "postgres",
			"DataSource": "postgres://user:secret_password@db:5432/mm",
			"Password":   "my-secret-password",
		},
	}

	report := r.RedactConfig(config, "./test.zip", "config.json")

	// Verify values are replaced
	sqlSettings := config["SqlSettings"].(map[string]interface{})
	assert.Equal(t, models.PlaceholderPassword, sqlSettings["Password"],
		"Password should be redacted")
	assert.Equal(t, models.PlaceholderDSN, sqlSettings["DataSource"],
		"DataSource should be redacted")

	// Verify original value NOT in report
	assert.NotContains(t, report.Categories[0].Items[0].Replacement, "secret_password",
		"Replacement should not contain original value")

	// Verify report counts
	assert.Greater(t, report.TotalRedacted, 0)
	assert.Greater(t, report.HighSeverityCount, 0)
}

func TestRedactConfig_LDAPPassword(t *testing.T) {
	r := redaction.NewRedactor(false)
	config := map[string]interface{}{
		"LdapSettings": map[string]interface{}{
			"Enable":       true,
			"LdapServer":   "ldap.internal.corp.com",
			"BindPassword": "ldap-super-secret",
		},
	}

	_ = r.RedactConfig(config, "./test.zip", "config.json")

	ldap := config["LdapSettings"].(map[string]interface{})
	assert.Equal(t, models.PlaceholderPassword, ldap["BindPassword"],
		"BindPassword should be redacted")
	// LdapServer should NOT be redacted in default mode
	assert.Equal(t, "ldap.internal.corp.com", ldap["LdapServer"],
		"LdapServer should not be redacted in default mode")
}

func TestRedactConfig_StrictMode(t *testing.T) {
	r := redaction.NewRedactor(true) // strict mode
	config := map[string]interface{}{
		"LdapSettings": map[string]interface{}{
			"Enable":       true,
			"LdapServer":   "ldap.internal.corp.com",
			"BindPassword": "ldap-super-secret",
		},
		"EmailSettings": map[string]interface{}{
			"FeedbackEmail": "admin@customer.com",
			"SMTPServer":    "smtp.customer.com",
		},
	}

	_ = r.RedactConfig(config, "./test.zip", "config.json")

	ldap := config["LdapSettings"].(map[string]interface{})
	// In strict mode, LdapServer should be redacted
	assert.Equal(t, "REDACTED_SERVER_ADDRESS", ldap["LdapServer"],
		"LdapServer should be redacted in strict mode")

	email := config["EmailSettings"].(map[string]interface{})
	assert.Equal(t, models.PlaceholderEmail, email["FeedbackEmail"],
		"FeedbackEmail should be redacted in strict mode")
}

func TestRedactConfig_EmptyValues(t *testing.T) {
	r := redaction.NewRedactor(false)
	config := map[string]interface{}{
		"SqlSettings": map[string]interface{}{
			"Password": "", // empty — should not be counted
		},
	}

	report := r.RedactConfig(config, "./test.zip", "config.json")

	// Empty values should not be counted as redacted
	_ = report // just ensure no panic
}

func TestRedactConfig_NestedOAuthSecret(t *testing.T) {
	r := redaction.NewRedactor(false)
	config := map[string]interface{}{
		"GitLabSettings": map[string]interface{}{
			"Enable":       true,
			"ClientSecret": "gitlab-oauth-secret-12345",
			"ClientId":     "gitlab-client-id",
		},
	}

	_ = r.RedactConfig(config, "./test.zip", "config.json")

	gitlab := config["GitLabSettings"].(map[string]interface{})
	assert.Equal(t, models.PlaceholderSecret, gitlab["ClientSecret"])
	// ClientId should NOT be redacted
	assert.Equal(t, "gitlab-client-id", gitlab["ClientId"])
}

func TestRedactConfig_SAMLCertificate(t *testing.T) {
	r := redaction.NewRedactor(false)
	config := map[string]interface{}{
		"SamlSettings": map[string]interface{}{
			"Enable":                true,
			"IdpCertificateFile":    "-----BEGIN CERTIFICATE-----\nMIIB...\n-----END CERTIFICATE-----",
			"PrivateKeyFile":        "-----BEGIN PRIVATE KEY-----\nMIIE...\n-----END PRIVATE KEY-----",
			"PublicCertificateFile": "-----BEGIN CERTIFICATE-----\nMIIB...\n-----END CERTIFICATE-----",
		},
	}

	_ = r.RedactConfig(config, "./test.zip", "config.json")

	saml := config["SamlSettings"].(map[string]interface{})
	assert.Equal(t, models.PlaceholderCertificate, saml["IdpCertificateFile"])
	assert.Equal(t, models.PlaceholderCertificate, saml["PrivateKeyFile"])
}

func TestRedactConfig_DataSourceReplicas(t *testing.T) {
	r := redaction.NewRedactor(false)
	config := map[string]interface{}{
		"SqlSettings": map[string]interface{}{
			"DriverName": "postgres",
			"DataSourceReplicas": []interface{}{
				"postgres://user:replica_pass@replica1:5432/mm",
				"postgres://user:replica_pass@replica2:5432/mm",
			},
		},
	}

	report := r.RedactConfig(config, "./test.zip", "config.json")
	require.NotNil(t, report)

	// Verify replicas are redacted
	sqlSettings := config["SqlSettings"].(map[string]interface{})
	replicas := sqlSettings["DataSourceReplicas"].([]interface{})
	for _, r := range replicas {
		assert.Equal(t, models.PlaceholderDSN, r, "Replica DSN should be redacted")
	}
}

func TestDefaultRules_NotEmpty(t *testing.T) {
	rules := redaction.DefaultRules()
	assert.NotEmpty(t, rules)

	// Verify all rules have required fields
	for _, rule := range rules {
		assert.NotEmpty(t, rule.ID, "Rule ID should not be empty")
		assert.NotEmpty(t, rule.Name, "Rule Name should not be empty")
		assert.NotEmpty(t, rule.Patterns, "Rule Patterns should not be empty")
		assert.NotEmpty(t, rule.Replacement, "Rule Replacement should not be empty")
		assert.Contains(t, []string{"high", "medium", "low"}, rule.Severity,
			"Severity should be high, medium, or low")
	}
}

func TestRedactConfig_NeverStoresOriginal(t *testing.T) {
	r := redaction.NewRedactor(false)
	sensitiveValue := "TOP_SECRET_PASSWORD_12345_SHOULD_NEVER_APPEAR"
	config := map[string]interface{}{
		"SqlSettings": map[string]interface{}{
			"Password": sensitiveValue,
		},
	}

	report := r.RedactConfig(config, "./test.zip", "config.json")

	// Check that the original value NEVER appears in the report
	for _, cat := range report.Categories {
		for _, item := range cat.Items {
			assert.NotContains(t, item.Replacement, sensitiveValue,
				"Replacement should never contain original value")
			assert.NotContains(t, item.ConfigKey, sensitiveValue,
				"ConfigKey should never contain original value")
		}
	}

	// Verify it was replaced in config
	sqlSettings := config["SqlSettings"].(map[string]interface{})
	assert.NotEqual(t, sensitiveValue, sqlSettings["Password"],
		"Original value should be replaced in config")
}
