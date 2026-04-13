# Changelog

All notable changes to this project will be documented in this file.

The format is based on [Keep a Changelog](https://keepachangelog.com/en/1.1.0/),
and this project adheres to [Semantic Versioning](https://semver.org/spec/v2.0.0.html).

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
