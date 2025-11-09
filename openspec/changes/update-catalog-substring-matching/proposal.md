# Change: Update catalog list filtering to support substring matches

## Why
- Operators search the catalog by entering partial app names (e.g., "nginx") but `k0rdent.catalog.serviceTemplates.list(app="nginx")` currently only matches exact slugs, so entries such as `ingress-nginx` are never returned.
- This forces agents to fetch the full catalog and perform their own filtering, which is inefficient and contradicts user expectations for fuzzy lookups.

## What Changes
- Expand the catalog list requirement so that the optional `app` filter performs a case-insensitive substring match against app slugs.
- Update the catalog manager/database query to implement substring matching while keeping performance acceptable with SQLite indices.
- Add regression tests covering partial matches ("nginx" → `ingress-nginx`) and ensure responses remain empty (not errors) when nothing matches.

## Impact
- Improves catalog discovery UX for agents and users without affecting existing workflows.
- No API shape change; behavior of the optional filter simply broadens to include substring matches.
- Minimal risk: only affects read-only catalog queries.
