package parser

import (
	"strings"

	"github.com/rohith0456/mattermost-support-package-repro/internal/ingestion"
	"github.com/rohith0456/mattermost-support-package-repro/pkg/models"
)

// ParseAuth extracts authentication backend information.
func ParseAuth(np *ingestion.NormalizedPackage) models.AuthInfo {
	info := models.AuthInfo{}

	// LDAP
	ldapSettings := getNestedMap(np.Config, "LdapSettings")
	if ldapSettings != nil {
		if enabled, _ := ldapSettings["Enable"].(bool); enabled {
			info.HasLDAP = true
		}
		if enabledStr := getNestedString(ldapSettings, "Enable"); enabledStr == "true" {
			info.HasLDAP = true
		}
		if info.HasLDAP {
			// Try to detect LDAP type from server string (never store the actual server address)
			server := getNestedString(ldapSettings, "LdapServer")
			if server != "" {
				serverLower := strings.ToLower(server)
				if strings.Contains(serverLower, "ad.") || strings.Contains(serverLower, "ldap.") {
					info.LDAPType = "ad"
				} else {
					info.LDAPType = "openldap"
				}
			} else {
				info.LDAPType = "unknown"
			}
		}
	}

	// SAML
	samlSettings := getNestedMap(np.Config, "SamlSettings")
	if samlSettings != nil {
		if enabled, _ := samlSettings["Enable"].(bool); enabled {
			info.HasSAML = true
		}
		if enabledStr := getNestedString(samlSettings, "Enable"); enabledStr == "true" {
			info.HasSAML = true
		}
		if info.HasSAML {
			// Detect provider from IdP URL hint (never store the actual URL)
			idpURL := getNestedString(samlSettings, "IdpURL")
			if idpURL != "" {
				idpLower := strings.ToLower(idpURL)
				switch {
				case strings.Contains(idpLower, "okta"):
					info.SAMLProvider = "okta"
				case strings.Contains(idpLower, "onelogin"):
					info.SAMLProvider = "onelogin"
				case strings.Contains(idpLower, "ping"):
					info.SAMLProvider = "ping"
				case strings.Contains(idpLower, "azure"), strings.Contains(idpLower, "microsoft"):
					info.SAMLProvider = "azure-ad"
				case strings.Contains(idpLower, "keycloak"):
					info.SAMLProvider = "keycloak"
				default:
					info.SAMLProvider = "unknown"
				}
			}
		}
	}

	// OIDC / OpenID Connect (via GitLab SSO or generic OAuth)
	gitlabSettings := getNestedMap(np.Config, "GitLabSettings")
	if gitlabSettings != nil {
		if enabled, _ := gitlabSettings["Enable"].(bool); enabled {
			info.HasGitLab = true
			info.HasOIDC = true
		}
	}

	googleSettings := getNestedMap(np.Config, "GoogleSettings")
	if googleSettings != nil {
		if enabled, _ := googleSettings["Enable"].(bool); enabled {
			info.HasGoogle = true
		}
	}

	office365Settings := getNestedMap(np.Config, "Office365Settings")
	if office365Settings != nil {
		if enabled, _ := office365Settings["Enable"].(bool); enabled {
			info.HasOffice365 = true
		}
	}

	// MFA
	serviceSettings := getNestedMap(np.Config, "ServiceSettings")
	if serviceSettings != nil {
		if enabled, _ := serviceSettings["EnableMultifactorAuthentication"].(bool); enabled {
			info.MFA = true
		}
	}

	// Guest accounts
	guestSettings := getNestedMap(np.Config, "GuestAccountsSettings")
	if guestSettings != nil {
		if enabled, _ := guestSettings["Enable"].(bool); enabled {
			info.GuestAccounts = true
		}
	}

	return info
}
