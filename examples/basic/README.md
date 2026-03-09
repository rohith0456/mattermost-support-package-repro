# Basic Example

This example shows a minimal repro workflow for a standard single-node Mattermost deployment.

## Scenario

Support ticket: Customer reports that they cannot send direct messages after upgrading to v8.1.

Package: `sample-support-package.zip` (minimal, single-node, PostgreSQL, no optional services)

## Steps

```bash
# From the repository root:

# 1. Validate the package
mm-repro validate --support-package ./examples/basic/sample-support-package.zip

# 2. Preview the plan
mm-repro plan --support-package ./examples/basic/sample-support-package.zip

# 3. Generate the repro
mm-repro init \
  --support-package ./examples/basic/sample-support-package.zip \
  --issue "mm-12345-dm-issue" \
  --output ./generated-repro/basic-example

# 4. Review
cat ./generated-repro/basic-example/REPRO_SUMMARY.md

# 5. Start
cd ./generated-repro/basic-example
make run

# 6. Open Mattermost at http://localhost:8065
# 7. Create a user, try to send a DM, reproduce the issue

# 8. Clean up when done
make reset
```

## Expected Output

- Single Mattermost container (v8.1.0)
- PostgreSQL 15
- MailHog for email capture
- No optional services

## Generated Files

```
generated-repro/basic-example/
├── docker-compose.yml
├── .env
├── Makefile
├── README.md
├── REPRO_SUMMARY.md
├── REDACTION_REPORT.md
├── PLUGIN_REPORT.md
├── repro-plan.json
└── scripts/
    ├── start.sh
    ├── stop.sh
    └── reset.sh
```
