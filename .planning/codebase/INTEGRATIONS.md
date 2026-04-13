# External Integrations

**Analysis Date:** 2025-04-13

## APIs & External Services

**System SSH:**
- OpenSSH binary - Direct system integration via `/usr/bin/ssh`
- No API calls - Executes system SSH command
- Connection parameters passed as command-line arguments

**Local File System:**
- SSH config file reading - `~/.ssh/config`
- SSH key file detection - `~/.ssh/id_rsa.pub`, `~/.ssh/id_ed25519.pub`
- File system operations - Reading, writing, backup creation

**GitHub Integration:**
- GitHub Releases - Binary downloads via `curl` and `jq`
- GitHub Actions - CI/CD pipeline for releases
- GitHub API - Fetching latest release information

## Data Storage

**Databases:**
- None detected - Uses flat file storage (SSH config)

**File Storage:**
- Local filesystem only
- SSH config file management with atomic writes
- Backup rotation with timestamped files

**Caching:**
- None detected - Real-time SSH config reading

## Authentication & Identity

**SSH Authentication:**
- System SSH authentication - Uses existing SSH agent, keys, passwords
- No additional auth layer - Passes through to system SSH
- Supports all SSH authentication methods

**GitHub Authentication:**
- GitHub Personal Access Token - For releases workflow
- Environment variable: `GH_TOKEN` - GitHub API access for releases

## Monitoring & Observability

**Error Tracking:**
- None detected - Uses zap structured logging
- Errors logged to console via tracing

**Logs:**
- Zap structured logging - JSON-formatted log output
- Log levels: Info, Error, Warn
- Contextual logging with structured fields

## CI/CD & Deployment

**Hosting:**
- GitHub Releases - Primary distribution channel
- No cloud hosting detected

**CI Pipeline:**
- GitHub Actions - Automated testing and releases
- Workflow: `.github/workflows/release.yml`
- Semantic PR enforcement via GitHub Action

## Environment Configuration

**Required env vars:**
- `GH_TOKEN` - GitHub personal access token for releases
- `LAZYSSH` - Logger identifier (optional)

**Secrets location:**
- GitHub secrets - `GH_TOKEN` stored as repository secret
- No other external API keys detected

## Webhooks & Callbacks

**Incoming:**
- None detected - No webhook endpoints

**Outgoing:**
- GitHub API calls - For fetching release information
- System SSH commands - For establishing connections

---

*Integration audit: 2025-04-13*
```