package generator

// generateKeycloakRealm writes keycloak/repro-realm.json — a Keycloak realm
// export that configures the 'repro' realm with:
//   - OIDC client for Mattermost "Sign in with GitLab" (no license needed)
//   - SAML client for Mattermost SAML SSO (requires Enterprise license)
//   - 8 test users matching the LDAP stub users (password: Repro1234!)
//
// Keycloak imports this file on startup via 'start-dev --import-realm'.
func (g *Generator) generateKeycloakRealm() (string, error) {
	content := `{
  "realm": "repro",
  "enabled": true,
  "displayName": "Mattermost Repro",
  "displayNameHtml": "<b>Mattermost Repro</b>",
  "sslRequired": "none",
  "registrationAllowed": false,
  "loginWithEmailAllowed": true,
  "duplicateEmailsAllowed": false,
  "resetPasswordAllowed": false,
  "editUsernameAllowed": false,
  "bruteForceProtected": false,
  "accessTokenLifespan": 300,
  "clients": [
    {
      "clientId": "mattermost-client",
      "name": "Mattermost OIDC (Entra ID simulation)",
      "description": "Used by MM_GITLABSETTINGS — no license required",
      "enabled": true,
      "clientAuthenticatorType": "client-secret",
      "secret": "mattermost-secret-local-repro-only",
      "redirectUris": [
        "http://localhost:8065/*",
        "http://localhost:8065/signup/gitlab/complete",
        "http://localhost:8065/login/gitlab/complete"
      ],
      "webOrigins": ["http://localhost:8065"],
      "standardFlowEnabled": true,
      "implicitFlowEnabled": false,
      "directAccessGrantsEnabled": true,
      "serviceAccountsEnabled": false,
      "publicClient": false,
      "frontchannelLogout": false,
      "protocol": "openid-connect",
      "attributes": {
        "saml.assertion.signature": "false",
        "saml.force.post.binding": "false",
        "saml.multivalued.roles": "false",
        "saml.encrypt": "false",
        "saml.server.signature": "false",
        "saml.server.signature.keyinfo.ext": "false",
        "exclude.session.state.from.auth.response": "false",
        "saml_force_name_id_format": "false",
        "saml.client.signature": "false",
        "tls.client.certificate.bound.access.tokens": "false",
        "saml.authnstatement": "false",
        "display.on.consent.screen": "false",
        "saml.onetimeuse.condition": "false"
      },
      "protocolMappers": [
        {
          "name": "username",
          "protocol": "openid-connect",
          "protocolMapper": "oidc-usermodel-property-mapper",
          "consentRequired": false,
          "config": {
            "userinfo.token.claim": "true",
            "user.attribute": "username",
            "id.token.claim": "true",
            "access.token.claim": "true",
            "claim.name": "preferred_username",
            "jsonType.label": "String"
          }
        },
        {
          "name": "email",
          "protocol": "openid-connect",
          "protocolMapper": "oidc-usermodel-property-mapper",
          "consentRequired": false,
          "config": {
            "userinfo.token.claim": "true",
            "user.attribute": "email",
            "id.token.claim": "true",
            "access.token.claim": "true",
            "claim.name": "email",
            "jsonType.label": "String"
          }
        },
        {
          "name": "given_name",
          "protocol": "openid-connect",
          "protocolMapper": "oidc-usermodel-property-mapper",
          "consentRequired": false,
          "config": {
            "userinfo.token.claim": "true",
            "user.attribute": "firstName",
            "id.token.claim": "true",
            "access.token.claim": "true",
            "claim.name": "given_name",
            "jsonType.label": "String"
          }
        },
        {
          "name": "family_name",
          "protocol": "openid-connect",
          "protocolMapper": "oidc-usermodel-property-mapper",
          "consentRequired": false,
          "config": {
            "userinfo.token.claim": "true",
            "user.attribute": "lastName",
            "id.token.claim": "true",
            "access.token.claim": "true",
            "claim.name": "family_name",
            "jsonType.label": "String"
          }
        }
      ],
      "defaultClientScopes": ["web-origins", "profile", "roles", "email"],
      "optionalClientScopes": ["address", "phone", "offline_access", "microprofile-jwt"]
    },
    {
      "clientId": "http://localhost:8065",
      "name": "Mattermost SAML (Azure AD simulation)",
      "description": "Used by MM_SAMLSETTINGS — requires Enterprise license",
      "enabled": true,
      "protocol": "saml",
      "adminUrl": "http://localhost:8065",
      "baseUrl": "http://localhost:8065",
      "redirectUris": ["http://localhost:8065/login/sso/saml"],
      "attributes": {
        "saml.authnstatement": "true",
        "saml.server.signature": "true",
        "saml.assertion.signature": "true",
        "saml.force.post.binding": "true",
        "saml.multivalued.roles": "false",
        "saml.encrypt": "false",
        "saml.client.signature": "false",
        "saml_assertion_consumer_url_post": "http://localhost:8065/login/sso/saml",
        "saml_name_id_format": "urn:oasis:names:tc:SAML:1.1:nameid-format:emailAddress",
        "saml.signature.algorithm": "RSA_SHA256",
        "saml_idp_initiated_sso_url_name": "mattermost"
      },
      "protocolMappers": [
        {
          "name": "email",
          "protocol": "saml",
          "protocolMapper": "saml-user-property-mapper",
          "consentRequired": false,
          "config": {
            "attribute.value": "email",
            "friendly.name": "email",
            "attribute.nameformat": "Basic",
            "user.attribute": "email"
          }
        },
        {
          "name": "preferred_username",
          "protocol": "saml",
          "protocolMapper": "saml-user-property-mapper",
          "consentRequired": false,
          "config": {
            "attribute.value": "preferred_username",
            "friendly.name": "preferred_username",
            "attribute.nameformat": "Basic",
            "user.attribute": "username"
          }
        },
        {
          "name": "given_name",
          "protocol": "saml",
          "protocolMapper": "saml-user-property-mapper",
          "consentRequired": false,
          "config": {
            "attribute.value": "given_name",
            "friendly.name": "given_name",
            "attribute.nameformat": "Basic",
            "user.attribute": "firstName"
          }
        },
        {
          "name": "family_name",
          "protocol": "saml",
          "protocolMapper": "saml-user-property-mapper",
          "consentRequired": false,
          "config": {
            "attribute.value": "family_name",
            "friendly.name": "family_name",
            "attribute.nameformat": "Basic",
            "user.attribute": "lastName"
          }
        }
      ]
    }
  ],
  "users": [
    {
      "username": "alice.johnson",
      "email": "alice.johnson@repro.local",
      "firstName": "Alice",
      "lastName": "Johnson",
      "enabled": true,
      "emailVerified": true,
      "credentials": [{"type": "password", "value": "Repro1234!", "temporary": false}]
    },
    {
      "username": "bob.smith",
      "email": "bob.smith@repro.local",
      "firstName": "Bob",
      "lastName": "Smith",
      "enabled": true,
      "emailVerified": true,
      "credentials": [{"type": "password", "value": "Repro1234!", "temporary": false}]
    },
    {
      "username": "carol.white",
      "email": "carol.white@repro.local",
      "firstName": "Carol",
      "lastName": "White",
      "enabled": true,
      "emailVerified": true,
      "credentials": [{"type": "password", "value": "Repro1234!", "temporary": false}]
    },
    {
      "username": "dave.brown",
      "email": "dave.brown@repro.local",
      "firstName": "Dave",
      "lastName": "Brown",
      "enabled": true,
      "emailVerified": true,
      "credentials": [{"type": "password", "value": "Repro1234!", "temporary": false}]
    },
    {
      "username": "eve.davis",
      "email": "eve.davis@repro.local",
      "firstName": "Eve",
      "lastName": "Davis",
      "enabled": true,
      "emailVerified": true,
      "credentials": [{"type": "password", "value": "Repro1234!", "temporary": false}]
    },
    {
      "username": "frank.miller",
      "email": "frank.miller@repro.local",
      "firstName": "Frank",
      "lastName": "Miller",
      "enabled": true,
      "emailVerified": true,
      "credentials": [{"type": "password", "value": "Repro1234!", "temporary": false}]
    },
    {
      "username": "grace.wilson",
      "email": "grace.wilson@repro.local",
      "firstName": "Grace",
      "lastName": "Wilson",
      "enabled": true,
      "emailVerified": true,
      "credentials": [{"type": "password", "value": "Repro1234!", "temporary": false}]
    },
    {
      "username": "henry.moore",
      "email": "henry.moore@repro.local",
      "firstName": "Henry",
      "lastName": "Moore",
      "enabled": true,
      "emailVerified": true,
      "credentials": [{"type": "password", "value": "Repro1234!", "temporary": false}]
    }
  ]
}
`
	return g.writeFile("keycloak/repro-realm.json", content)
}
