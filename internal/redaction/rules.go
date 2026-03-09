package redaction

import "github.com/rohith0456/mattermost-support-package-repro/pkg/models"

// DefaultRules returns the full set of redaction rules applied by default.
func DefaultRules() []models.RedactionRule {
	return []models.RedactionRule{
		{
			ID:          "database-dsn",
			Name:        "Database Connection String",
			Description: "Full database connection strings containing credentials",
			Severity:    "high",
			Patterns: []string{
				"DataSource", "DataSourceReplicas", "datasource",
				"SearchDataSource", "ReplicaDataSource",
			},
			Replacement: models.PlaceholderDSN,
		},
		{
			ID:          "db-password",
			Name:        "Database Password",
			Description: "Database passwords",
			Severity:    "high",
			Patterns: []string{
				"password", "Password", "passwd",
				"DBPassword", "db_password",
			},
			Replacement: models.PlaceholderPassword,
		},
		{
			ID:          "ldap-bind-password",
			Name:        "LDAP Bind Password",
			Description: "LDAP bind user password",
			Severity:    "high",
			Patterns: []string{
				"BindPassword", "bind_password", "LdapPassword",
				"ldap_password", "LDAPBindPassword",
			},
			Replacement: models.PlaceholderPassword,
		},
		{
			ID:          "saml-private-key",
			Name:        "SAML Private Key",
			Description: "SAML private key and certificate",
			Severity:    "high",
			Patterns: []string{
				"PrivateKeyFile", "private_key", "PrivateKey",
				"ServiceProviderPrivateKey", "IdpCertificateFile",
				"PublicCertificateFile", "certificate", "Certificate",
			},
			Replacement: models.PlaceholderCertificate,
		},
		{
			ID:          "oauth-secret",
			Name:        "OAuth Client Secret",
			Description: "OAuth app client secrets and tokens",
			Severity:    "high",
			Patterns: []string{
				"Secret", "secret", "ClientSecret", "client_secret",
				"AppSecret", "app_secret", "OAuthSecret",
			},
			Replacement: models.PlaceholderSecret,
		},
		{
			ID:          "smtp-password",
			Name:        "SMTP Password",
			Description: "SMTP server authentication credentials",
			Severity:    "high",
			Patterns: []string{
				"SMTPPassword", "smtp_password", "SMTPUsername",
				"smtp_username", "EmailSmtpPassword",
			},
			Replacement: models.PlaceholderSMTP,
		},
		{
			ID:          "push-notification-key",
			Name:        "Push Notification Key",
			Description: "Push notification server keys and tokens",
			Severity:    "high",
			Patterns: []string{
				"PushNotificationContents", "ApplePushCertPrivate",
				"ApplePushCertPublic", "AndroidPushNotificationsProxyUrl",
			},
			Replacement: models.PlaceholderKey,
		},
		{
			ID:          "cloud-storage-key",
			Name:        "Cloud Storage Credentials",
			Description: "S3/Azure/GCS storage credentials",
			Severity:    "high",
			Patterns: []string{
				"AmazonS3AccessKeyId", "AmazonS3SecretAccessKey",
				"amazons3secretaccesskey", "s3accesskey", "s3secretkey",
				"StorageAccessKey", "storage_access_key",
				"AzureAccountName", "AzureAccountKey",
			},
			Replacement: models.PlaceholderKey,
		},
		{
			ID:          "license",
			Name:        "License File",
			Description: "Mattermost license file contents",
			Severity:    "high",
			Patterns: []string{
				"LicenseFileLocation", "License", "license",
				"LicenseId", "license_id",
			},
			Replacement: models.PlaceholderLicense,
		},
		{
			ID:          "encryption-key",
			Name:        "Encryption Key",
			Description: "At-rest encryption keys",
			Severity:    "high",
			Patterns: []string{
				"AtRestEncryptKey", "at_rest_encrypt_key",
				"EncryptionKey", "encryption_key",
				"PublicLinkSalt", "public_link_salt",
				"InviteSalt", "invite_salt",
				"PasswordResetSalt", "password_reset_salt",
			},
			Replacement: models.PlaceholderKey,
		},
		{
			ID:          "webhook-secret",
			Name:        "Webhook Secret",
			Description: "Webhook signing secrets and tokens",
			Severity:    "medium",
			Patterns: []string{
				"WebhookSecret", "webhook_secret", "SigningSecret",
				"signing_secret", "HookToken", "hook_token",
			},
			Replacement: models.PlaceholderSecret,
		},
		{
			ID:          "plugin-secret",
			Name:        "Plugin Secret",
			Description: "Plugin-specific secrets and API keys",
			Severity:    "medium",
			Patterns: []string{
				"api_key", "apikey", "ApiKey", "API_KEY",
				"access_token", "AccessToken", "bot_token", "BotToken",
				"integration_key", "IntegrationKey",
			},
			Replacement: models.PlaceholderSecret,
		},
		{
			ID:          "external-url",
			Name:        "External Service URL with Credentials",
			Description: "URLs containing credentials or tokens",
			Severity:    "medium",
			Patterns: []string{
				"ServiceUrl", "service_url", "RedirectURI",
				"redirect_uri", "CallbackURL", "callback_url",
			},
			Replacement: models.PlaceholderURL,
		},
	}
}

// StrictRules returns additional rules applied with --redact-strict flag.
func StrictRules() []models.RedactionRule {
	base := DefaultRules()
	strict := []models.RedactionRule{
		{
			ID:          "server-address",
			Name:        "Server Address",
			Description: "Internal server hostnames and IP addresses",
			Severity:    "low",
			Patterns: []string{
				"LdapServer", "SMTPServer", "smtp_server",
				"ElasticsearchConnectionUrl", "SearchConnectionUrl",
			},
			Replacement: "REDACTED_SERVER_ADDRESS",
		},
		{
			ID:          "email-address",
			Name:        "Email Address",
			Description: "User and admin email addresses",
			Severity:    "low",
			Patterns: []string{
				"FeedbackEmail", "feedback_email",
				"SupportEmail", "support_email",
				"ReplyToAddress", "reply_to_address",
			},
			Replacement: models.PlaceholderEmail,
		},
	}
	return append(base, strict...)
}
