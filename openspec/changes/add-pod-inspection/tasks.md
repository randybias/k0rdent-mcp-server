# Implementation Tasks

## 1. Runtime pod access
- [ ] 1.1 Add helper(s) for computing pod readiness summary (ready vs total containers, restart totals, age) using the existing Kubernetes client
- [ ] 1.2 Ensure helpers respect the active namespace filter before returning data

## 2. `k0rdent.pods.list`
- [ ] 2.1 Register the new tool alongside the other core tools
- [ ] 2.2 Implement the handler to list pods in a namespace and return summary fields (phase, ready/total, restarts, node, age)
- [ ] 2.3 Add unit tests covering populated namespace, filtered namespace, and pods with multiple containers (ready vs not ready)

## 3. `k0rdent.pods.inspect`
- [ ] 3.1 Register the inspect tool in the core tool set
- [ ] 3.2 Implement the handler to fetch a pod and return metadata, conditions, and per-container status (current state, last termination, restart count)
- [ ] 3.3 Add unit tests for happy path, pod not found, filtered namespace, and multi-container pods

## 4. Documentation & validation
- [ ] 4.1 Update developer docs (e.g., tool catalog) with examples for the new pod tools
- [ ] 4.2 Run `openspec validate add-pod-inspection --strict` and ensure tests/docs references are updated accordingly
