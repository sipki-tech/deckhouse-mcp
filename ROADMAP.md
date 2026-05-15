# Deckhouse MCP Server — Roadmap

MCP server for managing [Deckhouse Kubernetes Platform](https://deckhouse.ru/docs) (Community Edition).
Full handler specification: [mcp-handlers-full.md](mcp-handlers-full.md).

## Priority Legend

| Priority | Label | Description |
|----------|-------|-------------|
| 🔴 P0 | **MVP** | Without these the MCP server is useless. **Done.** |
| 🟠 P1 | **Core Operations** | Covers 80 % of real-world management tasks |
| 🟡 P2 | **Advanced Management** | Useful but not urgent |
| ⚪ P3 | **Edge Cases** | Niche / debugging helpers |

---

## Implementation Order

Handlers are implemented in priority order. Within a phase, dependencies are respected (e.g., `RemoveNode` requires `DeleteStaticInstance` first).

| Phase | Handlers | Status |
|-------|----------|--------|
| P0 — MVP | A1, A2, A5, A7, A9, B1, C1, D1, D2, D4 | ✅ Done (10/10) |
| P1 — Core Operations | A3, A6, A11, B2, B3, B4, C2, C3, D5, D6, D10, D12, E1 | ✅ Done (13/13) |
| P2 — Advanced Management | A4, A8, A10, B5, B7, D3, D7, D8, D9, D11, E2, E3, F1, F2, F4, F5 | ✅ Done (16/16) |
| P3 — Edge Cases | B6, D13, F3, F6 | 0/4 |

---

## Summary Matrix

| Block | Domain | P0 | P1 | P2 | P3 | Total |
|-------|--------|:--:|:--:|:--:|:--:|:-----:|
| **A** Diagnostics | Cluster status, nodes, pods, logs | ✅ 5 | 3 | 3 | — | 11 |
| **B** Modules | ModuleConfig CRUD, enable/disable | ✅ 1 | 3 | 2 | 1 | 7 |
| **C** Releases | DeckhouseRelease, approve | ✅ 1 | 2 | — | — | 3 |
| **D** Nodes | StaticInstance, SSH, drain, NodeGroup | ✅ 3 | 4 | 5 | 1 | 13 |
| **E** Configuration | ClusterConfiguration, K8s version | — | 1 | 2 | — | 3 |
| **F** Sources | ModuleSource, ModuleUpdatePolicy | — | — | 4 | 2 | 6 |
| | **Total** | **10** | **13** | **16** | **4** | **43** |

---

## P0 — MVP ✅ (Done)

All 10 handlers implemented, tested (38 unit tests), and deployed.

| ID | Handler | Block | Type | Proto RPC |
|----|---------|-------|------|-----------|
| A1 | `GetClusterStatus` | Diagnostics | read | `DiagnosticsAPI.GetClusterStatus` |
| A2 | `ListNodes` | Diagnostics | read | `DiagnosticsAPI.ListNodes` |
| A5 | `ListNodeGroups` | Diagnostics | read | `DiagnosticsAPI.ListNodeGroups` |
| A7 | `ListStaticInstances` | Diagnostics | read | `DiagnosticsAPI.ListStaticInstances` |
| A9 | `ListUnhealthyPods` | Diagnostics | read | `DiagnosticsAPI.ListUnhealthyPods` |
| B1 | `ListModuleConfigs` | Modules | read | `ModulesAPI.ListModuleConfigs` |
| C1 | `ListDeckhouseReleases` | Releases | read | `ReleasesAPI.ListDeckhouseReleases` |
| D1 | `AddWorkerNode` | Nodes | write (composite) | `NodesAPI.AddWorkerNode` |
| D2 | `CreateSSHCredentials` | Nodes | write | `NodesAPI.CreateSSHCredentials` |
| D4 | `CreateStaticInstance` | Nodes | write | `NodesAPI.CreateStaticInstance` |

### P0 Infrastructure (delivered)

- **Proto**: 4 services with 10 RPCs (`diagnostics.proto`, `modules.proto`, `releases.proto`, `nodes.proto`)
- **k8s.Client**: 9 methods — `ListNodes`, `ListPods`, `ListNodeGroups`, `ListStaticInstances`, `GetStaticInstance`, `CreateStaticInstance`, `ListModuleConfigs`, `ListDeckhouseReleases`, `CreateSSHCredentials`
- **RBAC**: read `nodes`, `pods`, `nodegroups`, `staticinstances`, `moduleconfigs`, `deckhouserelease`; create `staticinstances`, `sshcredentials`

---

## P1 — Core Operations (13 handlers)

Unlocks: single-resource detail views, module enable/disable, release approval, node lifecycle management, cluster configuration read.

| ID | Handler | Block | Type | Proto RPC |
|----|---------|-------|------|-----------|
| A3 | `GetNode` | Diagnostics | read | `DiagnosticsAPI.GetNode` |
| A6 | `GetNodeGroup` | Diagnostics | read | `DiagnosticsAPI.GetNodeGroup` |
| A11 | `GetDeckhouseLogs` | Diagnostics | read | `DiagnosticsAPI.GetDeckhouseLogs` |
| B2 | `GetModuleConfig` | Modules | read | `ModulesAPI.GetModuleConfig` |
| B3 | `EnableModule` | Modules | write | `ModulesAPI.EnableModule` |
| B4 | `DisableModule` | Modules | write | `ModulesAPI.DisableModule` |
| C2 | `GetDeckhouseRelease` | Releases | read | `ReleasesAPI.GetDeckhouseRelease` |
| C3 | `ApproveRelease` | Releases | write | `ReleasesAPI.ApproveRelease` |
| D5 | `DeleteStaticInstance` | Nodes | write | `NodesAPI.DeleteStaticInstance` |
| D6 | `RemoveNode` | Nodes | write (composite) | `NodesAPI.RemoveNode` |
| D10 | `CreateNodeGroup` | Nodes | write | `NodesAPI.CreateNodeGroup` |
| D12 | `WaitNodeReady` | Nodes | read (polling) | `NodesAPI.WaitNodeReady` |
| E1 | `GetClusterConfiguration` | Config | read | `ConfigAPI.GetClusterConfiguration` |

### P1 Infrastructure

**Proto — new RPCs:**

| Service | New RPCs | File |
|---------|----------|------|
| `DiagnosticsAPI` | `GetNode`, `GetNodeGroup`, `GetDeckhouseLogs` | `diagnostics.proto` |
| `ModulesAPI` | `GetModuleConfig`, `EnableModule`, `DisableModule` | `modules.proto` |
| `ReleasesAPI` | `GetDeckhouseRelease`, `ApproveRelease` | `releases.proto` |
| `NodesAPI` | `DeleteStaticInstance`, `RemoveNode`, `WaitNodeReady`, `CreateNodeGroup` | `nodes.proto` |
| `ConfigAPI` | `GetClusterConfiguration` | `config.proto` |

**k8s.Client — new methods:**

| Method | Purpose |
|--------|---------|
| `GetNode(ctx, name) → *corev1.Node` | Detailed node info (A3) |
| `ListEvents(ctx, fieldSelector) → []corev1.Event` | Events for node (A3) |
| `GetPodLogs(ctx, namespace, name, opts) → string` | Deckhouse controller logs (A11) |
| `GetNodeGroup(ctx, name) → *unstructured.Unstructured` | Single NodeGroup detail (A6) |
| `GetModuleConfig(ctx, name) → *unstructured.Unstructured` | Single ModuleConfig (B2) |
| `UpdateModuleConfig(ctx, obj) → *unstructured.Unstructured` | Enable/disable module (B3, B4) |
| `GetDeckhouseRelease(ctx, name) → *unstructured.Unstructured` | Single release detail (C2) |
| `UpdateDeckhouseRelease(ctx, obj) → *unstructured.Unstructured` | Approve release (C3) |
| `DeleteStaticInstance(ctx, name) → error` | Delete SI (D5) |
| `CreateNodeGroup(ctx, obj) → *unstructured.Unstructured` | Create NG (D10) |
| `GetClusterConfiguration(ctx) → *unstructured.Unstructured` | Read cluster config secret (E1) |

**RBAC — new permissions:**

| Resource | Verbs | Reason |
|----------|-------|--------|
| `events` (core) | get, list | Node events (A3) |
| `pods/log` (core) | get | Deckhouse logs (A11) |
| `moduleconfigs` (deckhouse.io) | update, patch | Enable/disable module (B3, B4) |
| `deckhouserelease` (deckhouse.io) | update, patch | Approve release (C3) |
| `staticinstances` (deckhouse.io) | delete | Delete SI (D5) |
| `nodegroups` (deckhouse.io) | create | Create NG (D10) |
| `secrets` (core) | get | Read cluster config (E1) |

---

## P2 — Advanced Management (16 handlers)

Unlocks: pod/node event logs, module settings editing, node drain/cordon operations, cluster version management, module sources & update policies.

| ID | Handler | Block | Type | Proto RPC |
|----|---------|-------|------|-----------|
| A4 | `GetNodeEvents` | Diagnostics | read | `DiagnosticsAPI.GetNodeEvents` |
| A8 | `GetStaticInstance` | Diagnostics | read | `DiagnosticsAPI.GetStaticInstance` |
| A10 | `GetPodLogs` | Diagnostics | read | `DiagnosticsAPI.GetPodLogs` |
| B5 | `UpdateModuleSettings` | Modules | write | `ModulesAPI.UpdateModuleSettings` |
| B7 | `ListModules` | Modules | read | `ModulesAPI.ListModules` |
| D3 | `DeleteSSHCredentials` | Nodes | write | `NodesAPI.DeleteSSHCredentials` |
| D7 | `CordonNode` | Nodes | write | `NodesAPI.CordonNode` |
| D8 | `UncordonNode` | Nodes | write | `NodesAPI.UncordonNode` |
| D9 | `DrainNode` | Nodes | write | `NodesAPI.DrainNode` |
| D11 | `DeleteNodeGroup` | Nodes | write | `NodesAPI.DeleteNodeGroup` |
| E2 | `GetStaticClusterConfiguration` | Config | read | `ConfigAPI.GetStaticClusterConfiguration` |
| E3 | `UpdateKubernetesVersion` | Config | write | `ConfigAPI.UpdateKubernetesVersion` |
| F1 | `ListModuleSources` | Sources | read | `SourcesAPI.ListModuleSources` |
| F2 | `CreateModuleSource` | Sources | write | `SourcesAPI.CreateModuleSource` |
| F4 | `ListModuleUpdatePolicies` | Sources | read | `SourcesAPI.ListModuleUpdatePolicies` |
| F5 | `CreateModuleUpdatePolicy` | Sources | write | `SourcesAPI.CreateModuleUpdatePolicy` |

### P2 Infrastructure

**Proto — new RPCs:**

| Service | New RPCs | File |
|---------|----------|------|
| `DiagnosticsAPI` | `GetNodeEvents`, `GetStaticInstance`, `GetPodLogs` | `diagnostics.proto` |
| `ModulesAPI` | `UpdateModuleSettings`, `ListModules` | `modules.proto` |
| `NodesAPI` | `DeleteSSHCredentials`, `CordonNode`, `UncordonNode`, `DrainNode`, `DeleteNodeGroup` | `nodes.proto` |
| `ConfigAPI` | `GetStaticClusterConfiguration`, `UpdateKubernetesVersion` | `config.proto` |
| `SourcesAPI` | `ListModuleSources`, `CreateModuleSource`, `ListModuleUpdatePolicies`, `CreateModuleUpdatePolicy` | `sources.proto` |

**k8s.Client — new methods:**

| Method | Purpose |
|--------|---------|
| `ListNodeEvents(ctx, name, opts) → []corev1.Event` | Filtered node events (A4) |
| `GetStaticInstanceDetail(ctx, name) → *unstructured.Unstructured` | SI detail + linked node (A8) |
| `GetPodLogs(ctx, ns, name, opts) → string` | Pod logs (A10) — may reuse A11 method |
| `PatchModuleConfig(ctx, name, patch) → *unstructured.Unstructured` | Settings update (B5) |
| `ListModules(ctx) → []unstructured.Unstructured` | Module objects (B7) |
| `DeleteSSHCredentials(ctx, name) → error` | Delete SSH creds (D3) |
| `PatchNode(ctx, name, patch) → *corev1.Node` | Cordon/uncordon (D7, D8) |
| `EvictPod(ctx, ns, name) → error` | Drain support (D9) |
| `DeleteNodeGroup(ctx, name) → error` | Delete NG (D11) |
| `GetStaticClusterConfiguration(ctx) → *unstructured.Unstructured` | Static cluster config (E2) |
| `UpdateClusterConfiguration(ctx, obj) → error` | K8s version update (E3) |
| `ListModuleSources(ctx) → []unstructured.Unstructured` | Module sources (F1) |
| `CreateModuleSource(ctx, obj) → *unstructured.Unstructured` | Create source (F2) |
| `ListModuleUpdatePolicies(ctx) → []unstructured.Unstructured` | Update policies (F4) |
| `CreateModuleUpdatePolicy(ctx, obj) → *unstructured.Unstructured` | Create policy (F5) |

**RBAC — new permissions:**

| Resource | Verbs | Reason |
|----------|-------|--------|
| `pods/eviction` (core) | create | Drain node (D9) |
| `sshcredentials` (deckhouse.io) | delete | Delete SSH creds (D3) |
| `nodegroups` (deckhouse.io) | delete | Delete NG (D11) |
| `nodes` (core) | patch | Cordon/uncordon (D7, D8) |
| `modulesources` (deckhouse.io) | get, list, create | Module sources (F1, F2) |
| `moduleupdatepolicies` (deckhouse.io) | get, list, create | Update policies (F4, F5) |
| `modules` (deckhouse.io) | get, list | Module objects (B7) |

---

## P3 — Edge Cases (4 handlers)

Unlocks: module maintenance mode, node group configuration scripts, module source cleanup, module release listing.

| ID | Handler | Block | Type | Proto RPC |
|----|---------|-------|------|-----------|
| B6 | `SetModuleMaintenance` | Modules | write | `ModulesAPI.SetModuleMaintenance` |
| D13 | `CreateNodeGroupConfiguration` | Nodes | write | `NodesAPI.CreateNodeGroupConfiguration` |
| F3 | `DeleteModuleSource` | Sources | write | `SourcesAPI.DeleteModuleSource` |
| F6 | `ListModuleReleases` | Sources | read | `SourcesAPI.ListModuleReleases` |

### P3 Infrastructure

**Proto — new RPCs:**

| Service | New RPCs | File |
|---------|----------|------|
| `ModulesAPI` | `SetModuleMaintenance` | `modules.proto` |
| `NodesAPI` | `CreateNodeGroupConfiguration` | `nodes.proto` |
| `SourcesAPI` | `DeleteModuleSource`, `ListModuleReleases` | `sources.proto` |

**k8s.Client — new methods:**

| Method | Purpose |
|--------|---------|
| `PatchModuleConfig(ctx, name, patch)` | Maintenance mode (B6) — may reuse P2 method |
| `CreateNodeGroupConfiguration(ctx, obj)` | NGC creation (D13) |
| `DeleteModuleSource(ctx, name)` | Source deletion (F3) |
| `ListModuleReleases(ctx, moduleName)` | Module releases (F6) |

**RBAC — new permissions:**

| Resource | Verbs | Reason |
|----------|-------|--------|
| `nodegroupconfigurations` (deckhouse.io) | create | NGC (D13) |
| `modulesources` (deckhouse.io) | delete | Delete source (F3) |
| `modulereleases` (deckhouse.io) | get, list | Module releases (F6) |

---

## Handler Dependencies

Some handlers depend on others or share infrastructure.

**Key dependency chains:**

| Chain | Handlers | Notes |
|-------|----------|-------|
| SSH → StaticInstance → AddWorkerNode | D2 → D4 → D1 | P0, all done |
| Delete SI + Drain → RemoveNode | D5 + D9 → D6 | P1 composite, D9 can be deferred to P2 (drain optional) |
| Cordon → Drain | D7 → D9 | Both P2, cordon is a prerequisite for safe drain |
| Enable ↔ Disable module | B3 ↔ B4 | P1, share same `UpdateModuleConfig` k8s method |
| GetDeckhouseRelease → ApproveRelease | C2 → C3 | P1, approve needs release lookup first |
| WaitNodeReady | D12 | Reusable by D1 (already has built-in polling), D10 |

---

## Implementation Checklist

### Per-handler workflow

For each handler:

1. **Proto**: Add RPC + request/response messages to the relevant `.proto` file
2. **Generate**: `task generate` (easyp mod download → easyp generate)
3. **k8s.Client**: Add new method(s) to `internal/k8s/client.go` interface + implementation
4. **Handler**: Implement the generated `*ToolHandler` method in `internal/handler/`
5. **Tests**: Add unit tests with mock `k8s.Client`
6. **RBAC**: Update `deploy/rbac.yaml`
7. **Register**: Handler auto-registered via generated `pb.Register{Service}Tools()` — no changes needed in `main.go`

### Phase progress tracker

- [x] **P0 — MVP** (10/10 handlers) — shipped
- [x] **P1 — Core Operations** (13/13 handlers) — shipped (`feat(p1)`, commit `4a23933`)
- [x] **P2 — Advanced Management** (16/16 handlers) — shipped (`feat(p2)`, commit `ce83857`)
- [ ] **P3 — Edge Cases** (0/4 handlers)
