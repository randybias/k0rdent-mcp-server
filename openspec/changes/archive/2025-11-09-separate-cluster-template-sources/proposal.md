# Change: Differentiate catalog vs. installed ClusterTemplates

## Why
- Catalog entries now include ClusterDeployment samples and ClusterTemplate definitions that still need to be installed into the management cluster, yet `k0rdent.mgmt.clusterTemplates.list` mixes concerns by reporting what is already installed.
- Agents need to browse catalog-provided ClusterTemplates (to install) separately from the set already running in the management plane.
- Upcoming namespace refactors move catalog operations under `k0rdent.catalog.*`, so we need a spec defining the separation of responsibilities before changing code.

## What Changes
- Extend the catalog tooling surface to enumerate ClusterTemplate artifacts that live only in the Git repo (and require installation).
- Clarify that `k0rdent.mgmt.clusterTemplates.list` returns **installed** ClusterTemplates sourced from the management cluster via the Kubernetes API.
- Add metadata linking installed ClusterTemplates back to their catalog origin (slug/version) so operators can see whether an installed resource is up-to-date with the catalog.

## Impact
- Users and agents get a clear flow: discover templates via `k0rdent.catalog.*`, install them into the management cluster, and verify installation via `k0rdent.mgmt.clusterTemplates.*`.
- Prevents accidental assumptions that a catalog entry already exists on the cluster, reducing “not found” errors when referencing ClusterTemplates that were never applied.
