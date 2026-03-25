# ConductorOne Report Builder

> **Disclaimer:** This is an independent, community-built tool. It is **not** an official ConductorOne product, not supported by ConductorOne, and not affiliated with or endorsed by ConductorOne, Inc. Use at your own risk. For official ConductorOne support, visit [conductorone.com](https://www.conductorone.com).

Custom report builder for auditing ConductorOne data. Single binary with embedded web UI — no runtime dependencies.

## Install

### Download (recommended)

Download the latest binary for your platform from the [Releases page](https://github.com/shaun-nichols/c1-report-builder/releases).

### Install script

```bash
curl -fsSL https://raw.githubusercontent.com/shaun-nichols/c1-report-builder/master/install.sh | sh
```

### Homebrew (macOS/Linux)

```bash
brew install shaun-nichols/tap/c1-report-builder
```

### Go install

```bash
go install github.com/shaun-nichols/c1-report-builder@latest
```

### Build from source

```bash
git clone https://github.com/shaun-nichols/c1-report-builder.git
cd c1-report-builder
go build -o c1-report-builder .
```

## Usage

### Web UI (recommended)

Just run the binary with no arguments — it opens your browser automatically:

```bash
./c1-report-builder
```

1. Paste your ConductorOne API credentials (Client ID + Secret)
2. Pick a built-in report or create a custom one
3. Preview data, then generate and download

### CLI

```bash
# List available report templates
./c1-report-builder --list-templates

# Run a template
./c1-report-builder --template orphaned-accounts --app-id "2bMfQx1234abc"

# Override output format
./c1-report-builder --template all-users-entitlements --app-id "abc" --format excel
```

CLI mode requires `C1_CLIENT_ID` and `C1_CLIENT_SECRET` environment variables (or a `.env` file).

## Built-in Reports

### Data Audit
- All Users & Entitlements
- Orphaned Accounts
- Service Accounts Audit
- Entitlement Grants by App
- Users Without Grants
- Past / Revoked Grants

### Governance
- Access Request History
- Access Review Results
- Revocation Activity
- Admin Audit Log
- Grant Change Feed

### Operations
- App Health Overview
- Apps Without Owners
- Apps With Sync Errors
- Connector Sync Status
- App Owner Coverage

## Data Sources

| Source | Description | Requires App |
|--------|-----------|:---:|
| Applications | All connected apps | |
| App Overview (Enriched) | Apps with owners + sync status | |
| App Users | Accounts within an app | Yes |
| Entitlements | Roles, groups, permissions | Yes |
| Grants | Account-entitlement assignments | Yes |
| Connectors | Sync status and health | Yes |
| Tasks / Access Requests | Approvals, reviews, revocations | |
| System Log | OCSF audit trail | |
| Grant Change Feed | Grant additions/removals | Yes |
| Past / Revoked Grants | Historical grant periods | Yes |
| App Owners | Owner assignments | Yes |

## Output Formats

| Format | Flag |
|--------|------|
| CSV (default) | `--format csv` |
| JSON | `--format json` |
| Excel | `--format excel` |
| HTML | `--format html` |

Every output file includes a `.sha256` sidecar for integrity verification.

## Getting API Credentials

1. In ConductorOne, go to **Settings > Developers > Service principals**
2. Create a service principal and generate a credential
3. Copy the **Client ID** and **Client Secret**

The Client Secret starts with `secret-token:` — include the full value.

## Cross-compile

```bash
GOOS=darwin GOARCH=arm64 go build -ldflags "-X main.Version=v1.0.0" -o c1-report-builder-mac .
GOOS=linux GOARCH=amd64 go build -ldflags "-X main.Version=v1.0.0" -o c1-report-builder-linux .
GOOS=windows GOARCH=amd64 go build -ldflags "-X main.Version=v1.0.0" -o c1-report-builder.exe .
```
