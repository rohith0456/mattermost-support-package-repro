# Security Model

## Overview

`mm-repro` is designed with a strict security model. The tool processes potentially sensitive customer support packages and must never leak, store, reuse, or expose customer secrets.

This document describes:
- What assets the tool protects
- The threat model it addresses
- How each mitigation works
- The complete redaction rule set
- Trust boundaries enforced at runtime

---

## Threat Model

### Assets to Protect

1. Customer database passwords and connection strings
2. Customer LDAP/SAML/OAuth credentials and certificates
3. Customer cloud storage access keys
4. Customer license files
5. Customer webhook secrets and plugin API tokens
6. Customer server addresses and infrastructure topology
7. Customer email addresses and personally identifiable information

### Threat Actors

| Actor | Description |
|-------|-------------|
| Accidental exposure | Engineer accidentally commits support package or generated `.env` to git |
| Tool misuse | Tool misconfigured to use real production credentials in the generated environment |
| Malicious package | Support package contains path traversal sequences or a zip bomb |
| Secret log leakage | Sensitive values are printed to the console or written to log files |
| Clipboard leakage | Secret values surfaced in terminal output and captured by clipboard managers |

---

## Mitigations

### 1. Redaction Before Storage

All sensitive config values are detected and replaced with clearly labeled placeholder strings **before** any storage, templating, or reporting. Original values exist only in memory for the minimum time needed to detect the pattern and are immediately discarded.

The redaction pipeline runs as the very first step after parsing the raw config bytes — no downstream layer ever receives unredacted secrets.

```
Raw bytes (from ZIP)
       │
       ▼
 JSON parse (in-memory only)
       │
       ▼
 Redaction engine ──► replaces secret values with REDACTED_* placeholders
       │
       ▼
 Redacted struct (safe for all further use)
       │
       ├──► Templates
       ├──► Reports
       └──► repro-plan.json
```

### 2. Defense-in-Depth Credential Isolation

The tool enforces a strict separation between "what the customer had" and "what the local environment uses":

- Generated `.env` files always contain **locally generated credentials**, never customer credentials
- The tool never reads customer DB passwords to use them in the local database container
- Connection string patterns (DSNs) are detected as a category and redacted wholesale — the tool does not parse out or reuse individual components (host, user, pass) from DSNs
- All local service passwords follow a naming convention (`*_local_repro_only`) that makes them visually distinct from production secrets

### 3. Path Sanitization

The ZIP extraction logic defends against malicious or malformed archives:

- Strips `../` path traversal sequences from all extracted file paths
- Verifies all extracted paths remain within the designated target directory using `filepath.Clean` + prefix checking
- Limits individual file extraction size to 500 MB to defend against zip bombs
- Sets extracted file permissions to `0600` (owner read/write only)
- Symlinks in ZIP archives are ignored and logged as warnings

Example of path traversal defense in pseudo-code:
```go
func sanitizePath(base, archivePath string) (string, error) {
    clean := filepath.Join(base, filepath.Clean("/"+archivePath))
    if !strings.HasPrefix(clean, filepath.Clean(base)+string(os.PathSeparator)) {
        return "", fmt.Errorf("path traversal detected: %s", archivePath)
    }
    return clean, nil
}
```

### 4. Email Capture

All generated environments configure MailHog as the SMTP server. This ensures:

- No email is ever sent to real recipients, even if an engineer accidentally re-enables email sending
- All outbound email is captured and visible at `http://localhost:8025`
- The generated config hard-codes `SMTPServer=mailhog` and `SMTPPort=1025` regardless of what the support package contained
- Customer SMTP server addresses and credentials are always redacted

### 5. No Outbound Connections

The generated Docker Compose environment is fully isolated by design:

| What the customer had | What the repro uses |
|----------------------|---------------------|
| Real S3/Azure storage | MinIO on localhost |
| Real LDAP server | OpenLDAP stub on localhost |
| Real SAML IdP | Keycloak on localhost |
| Real SMTP server | MailHog on localhost |
| Real Elasticsearch | OpenSearch on localhost |
| Real RTCD cluster | Local RTCD container |

The `docker-compose.yml` uses an isolated bridge network. No service has a host-mode network or external port bindings beyond the minimum needed for local browser access.

### 6. `.gitignore` Defaults

Both the tool repository and every generated project include `.gitignore` rules that prevent accidental commits:

**Repository `.gitignore`:**
```gitignore
# Support packages (contain sensitive customer data)
*.zip
*.tar.gz
testdata/fixtures/real/

# Generated repro projects
generated/
generated-repro/

# Environment files
.env
*.env

# Certificates and keys
*.pem
*.key
*.crt
*.p12
*.pfx
```

**Generated project `.gitignore`:**
```gitignore
# Never commit this .env — it contains local credentials
.env

# Never commit support packages
*.zip

# Docker volumes
volumes/
data/
```

---

## Redaction Rules

The redaction engine applies rules in priority order. Higher-severity rules are applied first.

### High Severity (always redacted)

These values are redacted in all modes, including `--dry-run` and `plan` output.

| Rule ID | Config Key Patterns | Placeholder |
|---------|---------------------|-------------|
| `dsn` | `DataSource`, `DataSourceReplicas`, `DataSourceSearchReplicas` | `REDACTED_DSN_SEE_REDACTION_REPORT` |
| `db-password` | `Password`, `password`, `passwd`, `DBPassword` | `REDACTED_PASSWORD_SEE_REDACTION_REPORT` |
| `ldap-bind-password` | `BindPassword`, `LdapPassword`, `LDAPPassword` | `REDACTED_PASSWORD_SEE_REDACTION_REPORT` |
| `saml-keys` | `PrivateKeyFile`, `IdpCertificateFile`, `ServiceProviderPrivateKeyFile`, `ServiceProviderPublicCertificateFile` | `REDACTED_CERTIFICATE_SEE_REDACTION_REPORT` |
| `oauth-secret` | `ClientSecret`, `AppSecret`, `OAuthClientSecret` | `REDACTED_SECRET_SEE_REDACTION_REPORT` |
| `smtp-credentials` | `SMTPPassword`, `SMTPUsername` | `REDACTED_SMTP_SEE_REDACTION_REPORT` |
| `cloud-storage-keys` | `AmazonS3SecretAccessKey`, `AzureAccountKey`, `AmazonS3AccessKeyId`, `AzureAccountName` | `REDACTED_KEY_SEE_REDACTION_REPORT` |
| `license` | `License`, `LicenseId`, `LicenseFileContents` | `REDACTED_LICENSE_SEE_REDACTION_REPORT` |
| `encryption-keys` | `AtRestEncryptKey`, `PublicLinkSalt`, `PasswordResetSalt`, `InviteSalt` | `REDACTED_KEY_SEE_REDACTION_REPORT` |
| `push-notification-key` | `PushNotificationServerKey`, `AndroidPushNotificationFirebaseKey` | `REDACTED_KEY_SEE_REDACTION_REPORT` |

### Medium Severity (always redacted)

| Rule ID | Config Key Patterns | Placeholder |
|---------|---------------------|-------------|
| `webhook-secret` | `WebhookSecret`, `SigningSecret`, `OutgoingWebhookSecret` | `REDACTED_SECRET_SEE_REDACTION_REPORT` |
| `plugin-secrets` | `api_key`, `access_token`, `bot_token`, `secret_key`, `client_secret` (plugin settings) | `REDACTED_SECRET_SEE_REDACTION_REPORT` |
| `rtcd-credentials` | `RTCDServiceURL` (if contains credentials in URL), `RTCDAuthToken` | `REDACTED_SECRET_SEE_REDACTION_REPORT` |

### Low Severity (redacted only with `--redact-strict`)

These values are used by the inference engine to make topology decisions (e.g., detecting what LDAP server was used) before `--redact-strict` suppresses them. With `--redact-strict`, all downstream processing uses placeholder values.

| Rule ID | Config Key Patterns | Placeholder |
|---------|---------------------|-------------|
| `server-addresses` | `LdapServer`, `SMTPServer`, `ElasticsearchConnectionUrl`, `AmazonS3Endpoint` | `REDACTED_SERVER_ADDRESS` |
| `email-addresses` | `FeedbackEmail`, `SupportEmail`, `AdminEmail` | `REDACTED_EMAIL_SEE_REDACTION_REPORT` |
| `site-url` | `SiteURL` | `REDACTED_SITE_URL` |

### Pattern-Based Detection

In addition to exact key matching, the redaction engine also applies heuristic detection for values that look like secrets regardless of their key name:

| Pattern | Description |
|---------|-------------|
| `^postgres://.*:.*@` | PostgreSQL DSN with embedded credentials |
| `^mysql://.*:.*@` | MySQL DSN with embedded credentials |
| `-----BEGIN.*PRIVATE KEY-----` | PEM private key blocks |
| `-----BEGIN CERTIFICATE-----` | PEM certificate blocks |
| `^[A-Za-z0-9+/]{40,}={0,2}$` in a `*Secret*` or `*Key*` field | Base64-encoded secret |

---

## What Is Never Reused

The following are **never** taken from a support package and reused in the generated environment:

- Database passwords
- Database connection strings
- LDAP server addresses or bind credentials
- SAML certificates or private keys
- OAuth client secrets
- SMTP server credentials
- S3/Azure access keys
- License files
- Webhook secrets
- Plugin API keys
- Push notification keys

### How "Never Reused" Is Enforced

The enforcement is structural, not just policy:

1. The redaction engine runs before any generator sees the config
2. All generators receive only the redacted struct — they have no access to original values
3. The `.env` template never references config values for credentials — it generates its own with `GENERATED_LOCAL_` prefixes
4. There is no "passthrough" code path that could forward a customer credential into the generated project

---

## Safe Defaults for Generated Credentials

All generated local credentials follow these patterns:

```env
# Postgres
POSTGRES_PASSWORD=mm_local_repro_only_postgres
POSTGRES_USER=mmuser_local_repro_only
POSTGRES_DB=mattermost_local_repro

# Mattermost admin
MM_ADMIN_USERNAME=admin_local_repro
MM_ADMIN_PASSWORD=Admin_local_repro_1!

# MinIO (when included)
MINIO_ROOT_USER=minioadmin_local_repro_only
MINIO_ROOT_PASSWORD=minioadmin_local_repro_only

# LDAP (when included)
LDAP_ADMIN_PASSWORD=admin_local_repro_only
LDAP_BIND_DN=cn=admin,dc=repro,dc=local
```

The `_local_repro_only` suffix makes these values visually distinct and unsearchable in credential scanners that key on known production patterns.

---

## Trust Boundaries

The diagram below shows what the tool trusts and what it treats as untrusted input:

```
┌─────────────────────────────────────────────────────┐
│              TRUSTED ZONE (local machine)            │
│                                                      │
│  mm-repro binary ──────────────────────────────►    │
│       │                                              │
│       ▼                                              │
│  Support Package ZIP                                 │
│  (READ ONLY, extracted to temp dir, then deleted)    │
│       │                                              │
│       │ Redaction applied immediately after parse    │
│       ▼                                              │
│  Redacted Config Struct (no secrets)                 │
│       │                                              │
│       ▼                                              │
│  Generated Project (local-only credentials)          │
│       │                                              │
│       ▼                                              │
│  Docker Compose (isolated bridge network)            │
│       │                                              │
│       └──► All services communicate internally       │
│                                                      │
└─────────────────────────────────────────────────────┘
               │
               │  NO OUTBOUND CONNECTIONS
               │
          ─────┼──────  UNTRUSTED ZONE
               │        (customer infrastructure)
               │
          ◄────┘  (blocked by design)
```

### What Is Trusted

- The local filesystem where the tool runs
- The Docker Hub registry (for pulling public Mattermost images)
- The Mattermost plugin marketplace API (for plugin metadata, if network is available)

### What Is Not Trusted

- The contents of the support package ZIP
- File names within the support package (path traversal risk)
- File sizes within the support package (zip bomb risk)
- Config values within the support package (credential leakage risk)
- Any server addresses found in the support package

---

## Redaction Report

After every `init` run, the tool writes `REDACTION_REPORT.md` to the generated project. This file:

- Lists every config key that was redacted
- Shows the placeholder value used (never the original value)
- Identifies the redaction rule that matched
- Notes any keys that triggered heuristic (pattern-based) detection

Example excerpt:
```markdown
## Redaction Report

Generated: 2026-03-09T14:23:01Z

| Config Key | Rule | Placeholder |
|------------|------|-------------|
| SqlSettings.DataSource | dsn | REDACTED_DSN_SEE_REDACTION_REPORT |
| LdapSettings.BindPassword | ldap-bind-password | REDACTED_PASSWORD_SEE_REDACTION_REPORT |
| FileSettings.AmazonS3SecretAccessKey | cloud-storage-keys | REDACTED_KEY_SEE_REDACTION_REPORT |
| EmailSettings.SMTPPassword | smtp-credentials | REDACTED_SMTP_SEE_REDACTION_REPORT |
| PluginSettings.Plugins.com.mattermost.plugin-xyz.api_key | plugin-secrets | REDACTED_SECRET_SEE_REDACTION_REPORT |

Total: 5 values redacted across 4 categories.
```

---

## Reporting Security Issues

If you discover a security issue in mm-repro, please report it responsibly:

- **Email:** security@mattermost.com
- **Do not** open a public GitHub issue for security vulnerabilities
- Include a description of the issue, steps to reproduce, and potential impact
- You will receive a response within 5 business days

For non-security bugs and feature requests, use the [GitHub issue tracker](https://github.com/rohith0456/mattermost-support-package-repro/issues).
