# Change: mcp-oidc

## Problem
Production requires enforcing an inbound **OIDC Bearer token** and using it for Kubernetes calls (no impersonation).

## Goals
- When `AUTH_MODE=OIDC_REQUIRED`:
  - Reject if `Authorization: Bearer <token>` is missing.
  - Build `rest.Config` using the kubeconfig for cluster endpoint/TLS, but set **`BearerToken=<inbound token>`** for all Kubernetes calls.
- Re-run authentication tests and confirm K8s RBAC is enforced server-side.

## Notes
- MCP Authorization spec defines Bearer usage for HTTP transports.