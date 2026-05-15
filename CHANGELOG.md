# Changelog

All notable changes to this project will be documented in this file.

The format is based on [Keep a Changelog](https://keepachangelog.com/en/1.1.0/),
and this project adheres to [Semantic Versioning](https://semver.org/spec/v2.0.0.html).

---

## [Unreleased] — P2 — Advanced Management

16 new MCP handlers across 3 batches (read-only, writes, sources). Brings the tool catalog from 23 (P0+P1) to 39 total.

### Added

#### Batch 1 — Read-only diagnostics & module/node introspection (6 handlers)

- `deckhouse_GetNodeEvents` — list Kubernetes events scoped to a single node
- `deckhouse_GetPodLogs` — fetch container logs with `tail` and `since` parameters
- `deckhouse_GetStaticInstance` — get a single `StaticInstance` with labels and last-update time
- `deckhouse_ListModules` — list `Module` CRDs (status, weight, source)
- `deckhouse_CordonNode` — mark a node unschedulable; idempotent (returns `previousState`)
- `deckhouse_GetStaticClusterConfiguration` — read `static-cluster-configuration.yaml` from the `d8-cluster-configuration` Secret

#### Batch 2 — Write operations & cluster configuration (6 handlers)

- `deckhouse_UpdateModuleSettings` — RFC 7396 JSON Merge Patch on `ModuleConfig.spec.settings`; explicit `null` deletes keys
- `deckhouse_UncordonNode` — mark a node schedulable; idempotent skip if already schedulable
- `deckhouse_DrainNode` — composite: cordon → list non-DaemonSet/non-mirror pods → eviction loop with PDB awareness; 30s polling, default 300s timeout
- `deckhouse_DeleteSSHCredentials` — delete `SSHCredentials` CRD
- `deckhouse_DeleteNodeGroup` — delete `NodeGroup` CRD
- `deckhouse_UpdateKubernetesVersion` — patch `kubernetesVersion` in `d8-cluster-configuration` Secret with retry-on-conflict (up to 3 attempts), YAML round-trip via `sigs.k8s.io/yaml`

#### Batch 3 — Module sources & update policies (4 handlers)

- `deckhouse_ListModuleSources` — list `ModuleSource` CRDs with registry and status
- `deckhouse_CreateModuleSource` — create `ModuleSource` CRD with registry repo
- `deckhouse_ListModuleUpdatePolicies` — list `ModuleUpdatePolicy` CRDs with update mode
- `deckhouse_CreateModuleUpdatePolicy` — create `ModuleUpdatePolicy` CRD with update mode

#### Infrastructure

- New proto file `proto/deckhouse/v1/sources.proto` (`SourcesAPI` service)
- `k8s.Client` interface: +13 methods (`ListNodeEvents`, `GetPodLogs`, `GetSecret`, `GetModuleConfig`, `UpdateModuleConfig`, `GetNode`, `CordonNode`, `ListModules`, `UncordonNode`, `EvictPod`, `UpdateSecret`, `DeleteSSHCredentials`, `DeleteNodeGroup`, `ListModuleSources`, `CreateModuleSource`, `ListModuleUpdatePolicies`, `CreateModuleUpdatePolicy`)
- 2 new GVR constants: `ModuleSourceGVR`, `ModuleUpdatePolicyGVR` (deckhouse.io/v1alpha1)
- New handler file `internal/handler/sources.go` (`SourcesHandler`)
- Server registration: `pb.RegisterSourcesAPITools` (6th `Register*APITools` in `cmd/deckhouse-mcp/main.go`)
- Integration CRDs: `modulesources.deckhouse.io`, `moduleupdatepolicies.deckhouse.io` in `tests/integration/crds.yaml`

#### RBAC (least-privilege expansion)

- Read: `events`, `pods/log`, `modules`, `modulesources`, `moduleupdatepolicies`
- Write: `secrets` update on `d8-cluster-configuration`; `nodes` update/patch; `pods/eviction` create; `moduleconfigs` update; `deckhouserelease` patch; `staticinstances`/`sshcredentials`/`nodegroups` delete; `nodegroups` create; `modulesources`/`moduleupdatepolicies` create

#### Tests

- 77 new unit tests (115 total, up from 38 in P0)
- Polling tests (`DrainNode_PDBBlocksThenSucceeds`, `DrainNode_Timeout`) — ~30s each
- Mock `k8s.Client` extended with 17 new function-fields

---

## [0.1.0] — 2026-04-13

Initial release — MVP (P0) feature set. MCP server for managing Deckhouse Kubernetes Platform (Community Edition) over SSE transport, deployed as a Pod in `d8-system`.

### Added

#### MCP Tools (10 handlers)

**Block A — Diagnostics (read-only)**

- `deckhouse_GetClusterStatus` — aggregated cluster status: node counts, NodeGroup readiness, errored modules, pending releases, unhealthy pod count, current DKP version
- `deckhouse_ListNodes` — list all cluster nodes with filters by NodeGroup, status, and role
- `deckhouse_ListNodeGroups` — list all NodeGroup resources with readiness and condition info
- `deckhouse_ListStaticInstances` — list StaticInstance resources with phase filtering
- `deckhouse_ListUnhealthyPods` — list pods not in Running/Completed state across any namespace

**Block B — Modules**

- `deckhouse_ListModuleConfigs` — list all ModuleConfig resources with enabled/disabled filter

**Block C — Releases**

- `deckhouse_ListDeckhouseReleases` — list DeckhouseRelease resources with phase filter

**Block D — Nodes (write)**

- `deckhouse_CreateSSHCredentials` — create SSHCredentials CRD (base64-encodes private key internally)
- `deckhouse_CreateStaticInstance` — create StaticInstance CRD with credential reference and labels
- `deckhouse_AddWorkerNode` — composite handler: creates SSHCredentials + StaticInstance, then polls until `Running` or timeout

#### Infrastructure

- **Proto-first design**: services and MCP tool schemas defined in `.proto` files, Go bindings generated via `protoc-gen-mcp` + `easyp`
- **Four proto services**: `DiagnosticsAPI`, `ModulesAPI`, `ReleasesAPI`, `NodesAPI` (10 RPCs total); `ConfigAPI` and `SourcesAPI` stubs for future phases
- **SSE transport**: HTTP server with `mcp.NewSSEHandler`, listens on `:8080` (configurable via `LISTEN_ADDR`)
- **In-cluster auth**: `rest.InClusterConfig()` + ServiceAccount `deckhouse-mcp` in `d8-system`
- **k8s.Client interface** with typed client for core resources (`nodes`, `pods`) and dynamic client for Deckhouse CRDs (`NodeGroup`, `StaticInstance`, `SSHCredentials`, `ModuleConfig`, `DeckhouseRelease`)
- **Graceful shutdown**: `signal.NotifyContext(SIGINT, SIGTERM)` + `http.Server.Shutdown()`
- **Multi-stage Dockerfile**: `golang:1.26` build → `distroless` runtime image
- **Kubernetes manifests** in `deploy/`: `Deployment`, `Service`, `ServiceAccount`, `ClusterRole`, `ClusterRoleBinding`
- **RBAC** (least-privilege): read `nodes`, `pods`, `events`, `pods/log`, `nodegroups`, `staticinstances`, `moduleconfigs`, `deckhouserelease`; create `staticinstances`, `sshcredentials`
- **Taskfile** with tasks: `generate`, `lint`, `build`, `test`, `docker:build`, `docker:load`, `integration`
- **Integration test scaffolding**: `tests/integration/` with setup/teardown scripts, CRD fixtures

#### Tests

- 38 unit tests across 5 files in `internal/handler/`
- Mock `k8s.Client` using function fields (no external mock library)
- Coverage: `DiagnosticsHandler` (19 tests), `ModulesHandler` (3), `ReleasesHandler` (2), `NodesHandler` (11), error helpers (3)

[0.1.0]: https://github.com/sipki-tech/deckhouse-mcp/releases/tag/v0.1.0
