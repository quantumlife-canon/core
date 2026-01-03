# Real Keys Local Runbook V1

Phase 19.3: Azure OpenAI Shadow Provider - Local Setup Guide

## Overview

This runbook describes how to run the QuantumLife shadow-mode LLM pipeline with
real Azure OpenAI credentials locally. The shadow mode observes and analyzes
obligations WITHOUT affecting any canon behavior.

**CRITICAL INVARIANTS:**
- Canon loop remains deterministic and rule-based
- Shadow does NOT affect behavior - observation ONLY
- No PII in shadow inputs or outputs (privacy guard enforced)
- Secrets never stored in storelog or files
- Single request only - NO auto-retries

## Prerequisites

1. Gmail OAuth configured (Phase 18.8)
2. Azure OpenAI resource with a deployed model
3. Go 1.21+ installed
4. quantumlife-canon repository cloned

## Environment Variables

### Azure OpenAI (Required for real provider)

```bash
# Azure OpenAI endpoint (your resource URL)
export AZURE_OPENAI_ENDPOINT="https://your-resource.openai.azure.com"

# Model deployment name (e.g., "gpt-4o-mini")
export AZURE_OPENAI_DEPLOYMENT="gpt-4o-mini"

# API key (from Azure Portal > Keys and Endpoint)
export AZURE_OPENAI_API_KEY="your-api-key-here"

# API version (optional, defaults to 2024-02-15-preview)
export AZURE_OPENAI_API_VERSION="2024-02-15-preview"
```

### Shadow Mode Configuration

```bash
# Enable real providers (CRITICAL: default is false)
export QL_SHADOW_REAL_ALLOWED="true"

# Set provider kind to azure_openai (default is stub)
export QL_SHADOW_PROVIDER_KIND="azure_openai"

# Set mode to observe (default is off)
export QL_SHADOW_MODE="observe"
```

## Quick Start

### 1. Set Environment Variables

```bash
# Required for Azure OpenAI
export AZURE_OPENAI_ENDPOINT="https://your-resource.openai.azure.com"
export AZURE_OPENAI_DEPLOYMENT="gpt-4o-mini"
export AZURE_OPENAI_API_KEY="your-api-key"

# Enable real shadow provider
export QL_SHADOW_REAL_ALLOWED="true"
export QL_SHADOW_PROVIDER_KIND="azure_openai"
export QL_SHADOW_MODE="observe"
```

### 2. Start the Web Server

```bash
make run-real-shadow
```

Or manually:

```bash
go run ./cmd/quantumlife-web -mock=false
```

The startup log will show:

```
Shadow provider: azure_openai (RealAllowed: true)
```

If environment variables are missing, it falls back to stub:

```
Shadow provider: stub (RealAllowed: false, fallback: missing AZURE_OPENAI_* env vars)
```

### 3. Connect Gmail

1. Open http://localhost:8080/connections
2. Click "Connect Gmail"
3. Complete OAuth flow
4. Verify connection shows "Connected"

### 4. Run Gmail Sync (Bounded)

```bash
# Sync messages (max 25, last 7 days)
curl -X POST "http://localhost:8080/run/gmail-sync?circle_id=personal"
```

Expected output: Redirect to /connections with sync receipt shown.

### 5. Run Shadow Analysis

```bash
# Run shadow mode analysis
curl -X POST "http://localhost:8080/run/shadow?circle_id=personal"
```

Expected output: Redirect to /shadow/report with receipt hash shown.

### 6. Compute Shadow Diff

```bash
# Compare canon vs shadow signals
curl -X POST "http://localhost:8080/run/shadow-diff?circle_id=personal"
```

Expected output: Redirect to /shadow/report showing diff count.

### 7. View Shadow Report

```bash
# Get shadow calibration report
curl -s "http://localhost:8080/shadow/report"
```

Expected output: HTML page showing:
- Total diffs computed
- Agreement rate
- Conflict count
- Canon-only and shadow-only novelty counts

## Command Reference

### Gmail Sync

```bash
# POST /run/gmail-sync
# Query params:
#   circle_id (required) - Circle to sync
#
# Constraints:
#   - Max 25 messages
#   - Last 7 days only
#   - Read-only scope

curl -X POST "http://localhost:8080/run/gmail-sync?circle_id=personal"
```

### Shadow Run

```bash
# POST /run/shadow
# Query params:
#   circle_id (required) - Circle to analyze
#
# Returns: Shadow receipt with suggestions (abstract only)

curl -X POST "http://localhost:8080/run/shadow?circle_id=personal"
```

### Shadow Diff

```bash
# POST /run/shadow-diff
# Query params:
#   circle_id (required) - Circle to diff
#
# Compares canon signals vs shadow suggestions
# Returns: Diff results with agreement/conflict/novelty

curl -X POST "http://localhost:8080/run/shadow-diff?circle_id=personal"
```

### Shadow Report

```bash
# GET /shadow/report
#
# Returns: Calibration statistics page

curl -s "http://localhost:8080/shadow/report"
```

## Expected Outputs

### Successful Shadow Run

When shadow mode runs successfully:

1. Receipt is created with provenance:
   - ProviderKind: `azure_openai`
   - ModelOrDeployment: your deployment name
   - LatencyBucket: `fast` | `medium` | `slow`
   - Status: `success`

2. Suggestions (0-5) with abstract fields only:
   - Category: `money` | `time` | `people` | `work` | `home`
   - Horizon: `now` | `soon` | `later` | `someday`
   - Magnitude: `nothing` | `a_few` | `several`
   - Confidence: `low` | `medium` | `high`

3. No PII or content in outputs

### Successful Diff

When shadow diff computes successfully:

- Total diffs: count of compared items
- Matches: items where canon and shadow agree
- Conflicts: items where canon and shadow disagree
- Canon-only: items only canon surfaced
- Shadow-only: items only shadow surfaced

## Troubleshooting

### 401 Unauthorized

```
Shadow provider error: http_unauthorized
```

**Cause:** Invalid API key

**Fix:**
1. Verify `AZURE_OPENAI_API_KEY` is correct
2. Check key hasn't expired in Azure Portal
3. Ensure key is for the correct resource

### 403 Forbidden

```
Shadow provider error: http_forbidden
```

**Cause:** API key doesn't have access to deployment

**Fix:**
1. Verify deployment exists in Azure Portal
2. Check deployment name matches `AZURE_OPENAI_DEPLOYMENT`
3. Ensure API key has access to the deployment

### 404 Not Found

```
Shadow provider error: http_not_found
```

**Cause:** Wrong endpoint or deployment name

**Fix:**
1. Verify `AZURE_OPENAI_ENDPOINT` is correct (no trailing slash)
2. Check `AZURE_OPENAI_DEPLOYMENT` matches exactly
3. Ensure deployment is active in Azure Portal

### Missing Environment Variables

```
Shadow provider: stub (RealAllowed: false, fallback: missing AZURE_OPENAI_* env vars)
```

**Cause:** Required Azure environment variables not set

**Fix:**
```bash
export AZURE_OPENAI_ENDPOINT="https://your-resource.openai.azure.com"
export AZURE_OPENAI_DEPLOYMENT="your-deployment"
export AZURE_OPENAI_API_KEY="your-key"
```

### RealAllowed is False

```
Shadow provider: stub (RealAllowed: false)
```

**Cause:** Real providers not enabled

**Fix:**
```bash
export QL_SHADOW_REAL_ALLOWED="true"
```

### Gmail Not Connected

```
Gmail: not connected
```

**Cause:** OAuth not completed

**Fix:**
1. Go to http://localhost:8080/connections
2. Click "Connect Gmail"
3. Complete OAuth flow

### Sync Limit Reached

Gmail sync is bounded:
- Max 25 messages per sync
- Only last 7 days
- Read-only (gmail.readonly scope)

This is by design for privacy and safety.

## Security Notes

1. **Never commit API keys** - Use environment variables only
2. **Keys not in storelog** - Only hashes stored
3. **No PII in shadow** - Privacy guard enforced
4. **Single requests** - No retry loops that could leak timing
5. **Bounded sync** - 25 messages, 7 days max

## Verification Checklist

After setup, verify:

- [ ] Startup shows `Shadow provider: azure_openai`
- [ ] Gmail shows "Connected" on /connections
- [ ] `/run/gmail-sync` creates sync receipt
- [ ] `/run/shadow` creates shadow receipt with azure_openai provenance
- [ ] `/run/shadow-diff` shows diff count > 0
- [ ] `/shadow/report` shows statistics

## Reference

- Phase 19.2: Shadow Mode Contract (ADR-0043)
- Phase 19.3: Azure OpenAI Shadow Provider (ADR-0044)
- Phase 19.4: Shadow Diff + Calibration (ADR-0045)
- Phase 18.8: Real OAuth Gmail (ADR-0041)
