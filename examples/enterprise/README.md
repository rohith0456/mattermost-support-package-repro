# Enterprise Example

This example shows a full-stack repro workflow for an enterprise Mattermost deployment.

## Scenario

Support ticket: Customer reports LDAP sync is failing and some users cannot log in.

Package: `enterprise-support-package.zip` (enterprise, 3-node cluster, PostgreSQL with replica,
LDAP, Elasticsearch, S3 storage, custom plugins)

## Steps

```bash
# From the repository root:

# 1. Validate the package — shows all detected signals
mm-repro validate --support-package ./examples/enterprise/enterprise-support-package.zip

# 2. Preview the full plan
mm-repro plan --support-package ./examples/enterprise/enterprise-support-package.zip \
  --with-ldap --json | jq .

# 3. Generate the repro with LDAP and OpenSearch
mm-repro init \
  --support-package ./examples/enterprise/enterprise-support-package.zip \
  --issue "mm-98765-ldap-sync" \
  --with-ldap \
  --with-opensearch \
  --with-minio \
  --with-grafana \
  --output ./generated-repro/enterprise-example

# 4. Review all reports
cat ./generated-repro/enterprise-example/REPRO_SUMMARY.md
cat ./generated-repro/enterprise-example/REDACTION_REPORT.md
cat ./generated-repro/enterprise-example/PLUGIN_REPORT.md

# 5. Start
cd ./generated-repro/enterprise-example
make run

# 6. Wait for all services (may take 2-3 minutes)
make ps

# 7. Services available at:
#    - Mattermost:  http://localhost:8065
#    - OpenLDAP UI: http://localhost:8089
#    - MinIO:       http://localhost:9001
#    - Grafana:     http://localhost:3000
#    - MailHog:     http://localhost:8025

# 8. Configure Mattermost LDAP settings to point to openldap:389
# 9. Test LDAP sync in System Console > Authentication > LDAP

# 10. Clean up
make reset
```

## Expected Output

- 2 Mattermost nodes (capped from 3) with nginx load balancer
- PostgreSQL 15 (original had replica — approximated)
- OpenSearch (approximating Elasticsearch)
- OpenLDAP with stub users (safe local replacement)
- MinIO (approximating S3 storage)
- Prometheus + Grafana
- MailHog

## Security Notes

- LDAP directory contains STUB USERS ONLY — not real customer directory
- All LDAP credentials are local-only
- S3 credentials replaced with local MinIO credentials
- Customer license not reused
