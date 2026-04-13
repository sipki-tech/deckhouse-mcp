#!/usr/bin/env bash
# Integration tests for Deckhouse MCP server running in Kind.
#
# Usage: bash tests/integration/test.sh
#
# Requires: curl, jq, kubectl (port-forward to MCP server on localhost:8080).
set -euo pipefail

MCP_BASE_URL="${MCP_BASE_URL:-http://localhost:8080}"
KUBE_CONTEXT="${KUBE_CONTEXT:-kind-d8}"

# Counters.
PASSED=0
FAILED=0
TOTAL=0

# Temp dir for SSE stream files.
TMPDIR=$(mktemp -d)
trap 'cleanup' EXIT

cleanup() {
  # Kill background curl processes.
  jobs -p 2>/dev/null | xargs kill 2>/dev/null || true
  rm -rf "$TMPDIR"
}

# --- SSE/MCP Helpers ----------------------------------------------------------

# Connect to SSE endpoint and return the session message endpoint URL.
# Writes SSE stream to $TMPDIR/sse_stream in background.
mcp_connect() {
  local stream_file="$TMPDIR/sse_stream"
  > "$stream_file"

  # Start background SSE listener.
  curl -sN "${MCP_BASE_URL}/sse" > "$stream_file" 2>/dev/null &
  local curl_pid=$!
  echo "$curl_pid" > "$TMPDIR/curl_pid"

  # Wait for the endpoint event.
  local endpoint=""
  for i in $(seq 1 30); do
    if [ -s "$stream_file" ]; then
      endpoint=$(grep '^data:' "$stream_file" | head -1 | sed 's/^data: *//')
      if [ -n "$endpoint" ]; then
        break
      fi
    fi
    sleep 0.5
  done

  if [ -z "$endpoint" ]; then
    echo "ERROR: Failed to get endpoint from SSE stream" >&2
    return 1
  fi

  # Resolve relative endpoint to absolute URL.
  if [[ "$endpoint" == /* ]]; then
    endpoint="${MCP_BASE_URL}${endpoint}"
  fi

  echo "$endpoint"
}

# Disconnect from SSE (kill background curl).
mcp_disconnect() {
  if [ -f "$TMPDIR/curl_pid" ]; then
    kill "$(cat "$TMPDIR/curl_pid")" 2>/dev/null || true
    rm -f "$TMPDIR/curl_pid"
  fi
}

# Send a JSON-RPC request and wait for the response on the SSE stream.
# Usage: mcp_request <endpoint> <method> <params_json> [id]
mcp_request() {
  local endpoint="$1"
  local method="$2"
  local params="${3:-null}"
  local id="${4:-1}"

  local stream_file="$TMPDIR/sse_stream"
  local line_count_before
  line_count_before=$(wc -l < "$stream_file" | tr -d ' ')

  # POST the JSON-RPC request.
  local body
  body=$(jq -n --arg method "$method" --argjson params "$params" --argjson id "$id" \
    '{"jsonrpc":"2.0","method":$method,"params":$params,"id":$id}')

  curl -sf -X POST "$endpoint" \
    -H "Content-Type: application/json" \
    -d "$body" >/dev/null 2>&1

  # Wait for response on SSE stream (new message event after our request).
  local response=""
  for i in $(seq 1 200); do
    # Extract all 'data:' lines after the initial endpoint event.
    local new_data
    new_data=$(tail -n +"$((line_count_before + 1))" "$stream_file" 2>/dev/null | grep '^data:' | sed 's/^data: *//' || true)

    if [ -n "$new_data" ]; then
      # Find a JSON-RPC response with our id.
      while IFS= read -r line; do
        if echo "$line" | jq -e --argjson id "$id" 'select(.id == $id)' >/dev/null 2>&1; then
          response="$line"
          break 2
        fi
      done <<< "$new_data"
    fi
    sleep 0.5
  done

  if [ -z "$response" ]; then
    echo '{"error":{"code":-1,"message":"Timeout waiting for SSE response"}}' >&2
    return 1
  fi

  echo "$response"
}

# Initialize MCP session. Returns the endpoint URL.
mcp_initialize() {
  local endpoint
  endpoint=$(mcp_connect) || return 1

  local resp
  resp=$(mcp_request "$endpoint" "initialize" '{
    "protocolVersion": "2024-11-05",
    "capabilities": {},
    "clientInfo": {"name": "integration-test", "version": "1.0.0"}
  }' 0)

  # Check for error.
  if echo "$resp" | jq -e '.error' >/dev/null 2>&1; then
    echo "ERROR: Initialize failed: $(echo "$resp" | jq -r '.error.message')" >&2
    return 1
  fi

  # Send initialized notification (no id = notification).
  curl -sf -X POST "$endpoint" \
    -H "Content-Type: application/json" \
    -d '{"jsonrpc":"2.0","method":"notifications/initialized"}' >/dev/null 2>&1

  echo "$endpoint"
}

# Call an MCP tool. Returns the result JSON.
# Usage: mcp_call_tool <endpoint> <tool_name> [arguments_json] [request_id]
mcp_call_tool() {
  local endpoint="$1"
  local tool_name="$2"
  local arguments="${3:-}"
  if [ -z "$arguments" ]; then arguments='{}'; fi
  local id="${4:-$((RANDOM % 10000 + 100))}"

  local params
  params=$(jq -n --arg name "$tool_name" --argjson args "$arguments" \
    '{"name":$name,"arguments":$args}')

  local resp
  resp=$(mcp_request "$endpoint" "tools/call" "$params" "$id") || return 1

  # Check for JSON-RPC error.
  if echo "$resp" | jq -e '.error' >/dev/null 2>&1; then
    echo "TOOL ERROR: $(echo "$resp" | jq -r '.error.message')" >&2
    echo "$resp"
    return 1
  fi

  # Extract the text content from the result.
  echo "$resp" | jq -r '.result.content[0].text // .result'
}

# --- Assertions ---------------------------------------------------------------

assert_contains() {
  local text="$1"
  local pattern="$2"
  local msg="${3:-}"

  if echo "$text" | grep -q "$pattern"; then
    return 0
  else
    echo "  ASSERT FAILED: expected to contain '$pattern'${msg:+ ($msg)}" >&2
    echo "  Got: $(echo "$text" | head -5)" >&2
    return 1
  fi
}

assert_jq() {
  local json="$1"
  local expr="$2"
  local msg="${3:-}"

  if echo "$json" | jq -e "$expr" >/dev/null 2>&1; then
    return 0
  else
    echo "  ASSERT FAILED: jq '$expr' returned false${msg:+ ($msg)}" >&2
    echo "  Got: $(echo "$json" | head -5)" >&2
    return 1
  fi
}

# --- Test runner --------------------------------------------------------------

run_test() {
  local name="$1"
  TOTAL=$((TOTAL + 1))
  echo -n "[$TOTAL] $name ... "

  if "$name"; then
    PASSED=$((PASSED + 1))
    echo "PASS"
  else
    FAILED=$((FAILED + 1))
    echo "FAIL"
  fi
}

# --- Test cases ---------------------------------------------------------------

test_get_cluster_status() {
  local result
  result=$(mcp_call_tool "$ENDPOINT" "deckhouse_GetClusterStatus") || return 1

  assert_jq "$result" '.nodes.total >= 1' "at least 1 node" || return 1
  assert_jq "$result" '.nodes.ready >= 1' "at least 1 ready node" || return 1
  assert_jq "$result" '.deckhouseVersion | length > 0' "deckhouse version present" || return 1
}

test_list_nodes() {
  local result
  result=$(mcp_call_tool "$ENDPOINT" "deckhouse_ListNodes") || return 1

  assert_jq "$result" '.nodes | length >= 1' "at least 1 node" || return 1
  assert_contains "$result" "control-plane" "Kind node name" || return 1
}

test_list_nodes_filter_ready() {
  local result
  result=$(mcp_call_tool "$ENDPOINT" "deckhouse_ListNodes" '{"status":"NODE_STATUS_FILTER_READY"}') || return 1

  assert_jq "$result" '.nodes | length >= 1' "at least 1 ready node" || return 1
  # All returned nodes should be Ready.
  assert_jq "$result" '[.nodes[].status] | all(. == "Ready")' "all nodes Ready" || return 1
}

test_list_node_groups() {
  local result
  result=$(mcp_call_tool "$ENDPOINT" "deckhouse_ListNodeGroups") || return 1

  assert_jq "$result" '.nodeGroups | length >= 1' "at least 1 node group" || return 1
}

test_list_static_instances() {
  local result
  result=$(mcp_call_tool "$ENDPOINT" "deckhouse_ListStaticInstances") || return 1

  # Empty list is OK for Kind — just verify no error.
  assert_jq "$result" '.instances | type == "array"' "instances is array" || return 1
}

test_list_unhealthy_pods() {
  local result
  result=$(mcp_call_tool "$ENDPOINT" "deckhouse_ListUnhealthyPods") || return 1

  # May be empty or have pods — just verify structure.
  assert_jq "$result" '.pods | type == "array"' "pods is array" || return 1
}

test_list_module_configs() {
  local result
  result=$(mcp_call_tool "$ENDPOINT" "deckhouse_ListModuleConfigs") || return 1

  assert_jq "$result" '.modules | length >= 5' "at least 5 modules" || return 1
  # Deckhouse itself should be in the list.
  assert_contains "$result" "deckhouse" "deckhouse module present" || return 1
}

test_list_module_configs_filter_enabled() {
  local result
  result=$(mcp_call_tool "$ENDPOINT" "deckhouse_ListModuleConfigs" '{"enabled":true}') || return 1

  assert_jq "$result" '.modules | length >= 1' "at least 1 enabled module" || return 1
}

test_list_deckhouse_releases() {
  local result
  result=$(mcp_call_tool "$ENDPOINT" "deckhouse_ListDeckhouseReleases") || return 1

  assert_jq "$result" '.releases | length >= 1' "at least 1 release" || return 1
}

test_create_ssh_credentials() {
  local result
  result=$(mcp_call_tool "$ENDPOINT" "deckhouse_CreateSSHCredentials" '{
    "name": "integration-test-creds",
    "user": "testuser",
    "privateKey": "-----BEGIN OPENSSH PRIVATE KEY-----\ntest-key-data-for-integration\n-----END OPENSSH PRIVATE KEY-----"
  }') || return 1

  assert_jq "$result" '.name == "integration-test-creds"' "returned name matches" || return 1

  # Verify via kubectl.
  kubectl --context "$KUBE_CONTEXT" get sshcredentials integration-test-creds >/dev/null 2>&1 || {
    echo "  ASSERT FAILED: SSHCredentials not found via kubectl" >&2
    return 1
  }
}

test_create_static_instance() {
  local result
  result=$(mcp_call_tool "$ENDPOINT" "deckhouse_CreateStaticInstance" '{
    "name": "integration-test-si",
    "address": "192.168.1.100",
    "credentialsRef": "integration-test-creds",
    "labels": {"node.deckhouse.io/group": "worker"}
  }') || return 1

  assert_jq "$result" '.name == "integration-test-si"' "returned name matches" || return 1

  # Verify via kubectl.
  kubectl --context "$KUBE_CONTEXT" get staticinstances integration-test-si >/dev/null 2>&1 || {
    echo "  ASSERT FAILED: StaticInstance not found via kubectl" >&2
    return 1
  }
}

test_add_worker_node() {
  local result
  result=$(mcp_call_tool "$ENDPOINT" "deckhouse_AddWorkerNode" '{
    "address": "192.168.1.200",
    "sshUser": "testuser",
    "privateKey": "-----BEGIN OPENSSH PRIVATE KEY-----\ntest-key-data-for-add-worker\n-----END OPENSSH PRIVATE KEY-----",
    "nodeGroup": "worker",
    "nodeName": "integration-test-worker",
    "timeoutSeconds": 5,
    "waitReady": true
  }') || {
    # AddWorkerNode may return an error or timeout — check the response.
    true
  }

  # Even with timeout, the response should contain the resource names.
  assert_contains "$result" "integration-test-worker" "node name in response" || return 1

  # Verify SSHCredentials and StaticInstance were created.
  kubectl --context "$KUBE_CONTEXT" get sshcredentials integration-test-worker-creds >/dev/null 2>&1 || {
    echo "  ASSERT FAILED: SSHCredentials for worker not found via kubectl" >&2
    return 1
  }
  kubectl --context "$KUBE_CONTEXT" get staticinstances integration-test-worker >/dev/null 2>&1 || {
    echo "  ASSERT FAILED: StaticInstance for worker not found via kubectl" >&2
    return 1
  }
}

# --- Main ---------------------------------------------------------------------

main() {
  echo "========================================"
  echo "Deckhouse MCP Integration Tests"
  echo "========================================"
  echo "MCP server: $MCP_BASE_URL"
  echo "Kube context: $KUBE_CONTEXT"
  echo ""

  # Initialize MCP session.
  echo "Connecting to MCP server..."
  ENDPOINT=$(mcp_initialize) || {
    echo "FATAL: Failed to initialize MCP session."
    exit 1
  }
  echo "Connected. Session endpoint: $ENDPOINT"
  echo ""

  # Run all tests.
  # Read-only tests first.
  run_test test_get_cluster_status
  run_test test_list_nodes
  run_test test_list_nodes_filter_ready
  run_test test_list_node_groups
  run_test test_list_static_instances
  run_test test_list_unhealthy_pods
  run_test test_list_module_configs
  run_test test_list_module_configs_filter_enabled
  run_test test_list_deckhouse_releases

  # Write tests (create resources).
  run_test test_create_ssh_credentials
  run_test test_create_static_instance

  # Composite write test (takes ~60s due to timeout).
  echo ""
  echo "Note: AddWorkerNode test will wait ~60s for timeout..."
  run_test test_add_worker_node

  # Disconnect SSE.
  mcp_disconnect

  # Summary.
  echo ""
  echo "========================================"
  echo "Results: $PASSED passed, $FAILED failed, $TOTAL total"
  echo "========================================"

  if [ "$FAILED" -gt 0 ]; then
    exit 1
  fi
}

main "$@"
