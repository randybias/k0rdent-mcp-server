## Tasks
1. [ ] Update `tools-catalog` spec to state that the `app` filter performs case-insensitive substring matching and document an example scenario (`nginx` → `ingress-nginx`).
2. [ ] Implement substring matching in the catalog manager/database (SQLite query) so list calls honor the spec.
3. [ ] Extend catalog list unit tests (and supporting test data) to cover partial matches and verify no errors are thrown for empty results.
4. [ ] Run `go test ./...` (or targeted packages) to confirm regressions are not introduced.
