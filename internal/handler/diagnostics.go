package handler

import (
	"context"
	"fmt"
	"strings"
	"time"

	emptypb "google.golang.org/protobuf/types/known/emptypb"
	corev1 "k8s.io/api/core/v1"

	"github.com/sipki-tech/deckhouse-mcp/internal/k8s"
	pb "github.com/sipki-tech/deckhouse-mcp/proto/deckhouse/v1"
)

// DiagnosticsHandler implements pb.DiagnosticsAPIToolHandler.
type DiagnosticsHandler struct {
	client k8s.Client
}

// NewDiagnosticsHandler creates a new DiagnosticsHandler.
func NewDiagnosticsHandler(client k8s.Client) *DiagnosticsHandler {
	return &DiagnosticsHandler{client: client}
}

func (h *DiagnosticsHandler) GetClusterStatus(ctx context.Context, _ *emptypb.Empty) (*pb.GetClusterStatusResponse, error) {
	nodes, err := h.client.ListNodes(ctx)
	if err != nil {
		return nil, fmt.Errorf("listing nodes: %w", err)
	}

	var ready, notReady int32
	for _, n := range nodes {
		if isNodeReady(&n) {
			ready++
		} else {
			notReady++
		}
	}

	nodeGroups, err := h.client.ListNodeGroups(ctx)
	if err != nil {
		return nil, fmt.Errorf("listing node groups: %w", err)
	}

	var ngStatuses []*pb.NodeGroupStatus
	for _, ng := range nodeGroups {
		name, _, _ := unstructuredNestedString(ng.Object, "metadata", "name")
		ngReady, _, _ := unstructuredNestedInt64(ng.Object, "status", "ready")
		ngTotal, _, _ := unstructuredNestedInt64(ng.Object, "status", "nodes")
		ngStatuses = append(ngStatuses, &pb.NodeGroupStatus{
			Name:  name,
			Ready: int32(ngReady),
			Total: int32(ngTotal),
		})
	}

	moduleConfigs, err := h.client.ListModuleConfigs(ctx)
	if err != nil {
		return nil, fmt.Errorf("listing module configs: %w", err)
	}

	var erroredModules []string
	for _, mc := range moduleConfigs {
		msg, _, _ := unstructuredNestedString(mc.Object, "status", "message")
		if msg != "" {
			name, _, _ := unstructuredNestedString(mc.Object, "metadata", "name")
			erroredModules = append(erroredModules, name)
		}
	}

	releases, err := h.client.ListDeckhouseReleases(ctx)
	if err != nil {
		return nil, fmt.Errorf("listing releases: %w", err)
	}

	var pendingReleases []*pb.PendingRelease
	var deckhouseVersion string
	for _, r := range releases {
		phase, _, _ := unstructuredNestedString(r.Object, "status", "phase")
		name, _, _ := unstructuredNestedString(r.Object, "metadata", "name")
		version, _, _ := unstructuredNestedString(r.Object, "spec", "version")
		if phase == "Pending" {
			pendingReleases = append(pendingReleases, &pb.PendingRelease{
				Name:    name,
				Version: version,
			})
		}
		if phase == "Deployed" {
			deckhouseVersion = version
		}
	}

	pods, err := h.client.ListPods(ctx, "d8-system")
	if err != nil {
		return nil, fmt.Errorf("listing pods: %w", err)
	}

	var unhealthyCount int32
	for _, p := range pods {
		if !isPodHealthy(&p) {
			unhealthyCount++
		}
	}

	return &pb.GetClusterStatusResponse{
		Nodes: &pb.NodeSummary{
			Total:    int32(len(nodes)),
			Ready:    ready,
			NotReady: notReady,
		},
		NodeGroups:         ngStatuses,
		ErroredModules:     erroredModules,
		PendingReleases:    pendingReleases,
		UnhealthyPodsCount: unhealthyCount,
		DeckhouseVersion:   deckhouseVersion,
	}, nil
}

func (h *DiagnosticsHandler) ListNodes(ctx context.Context, req *pb.ListNodesRequest) (*pb.ListNodesResponse, error) {
	nodes, err := h.client.ListNodes(ctx)
	if err != nil {
		return nil, fmt.Errorf("listing nodes: %w", err)
	}

	var result []*pb.NodeInfo
	for _, n := range nodes {
		info := nodeToInfo(&n)

		if req.NodeGroup != nil && info.NodeGroup != *req.NodeGroup {
			continue
		}
		if req.Status != nil {
			switch *req.Status {
			case pb.NodeStatusFilter_NODE_STATUS_FILTER_READY:
				if info.Status != "Ready" {
					continue
				}
			case pb.NodeStatusFilter_NODE_STATUS_FILTER_NOT_READY:
				if info.Status != "NotReady" {
					continue
				}
			}
		}
		if req.Role != nil && info.Role != *req.Role {
			continue
		}

		result = append(result, info)
	}

	return &pb.ListNodesResponse{Nodes: result}, nil
}

func (h *DiagnosticsHandler) ListNodeGroups(ctx context.Context, _ *emptypb.Empty) (*pb.ListNodeGroupsResponse, error) {
	nodeGroups, err := h.client.ListNodeGroups(ctx)
	if err != nil {
		return nil, fmt.Errorf("listing node groups: %w", err)
	}

	var result []*pb.NodeGroupInfo
	for _, ng := range nodeGroups {
		name, _, _ := unstructuredNestedString(ng.Object, "metadata", "name")
		nodeType, _, _ := unstructuredNestedString(ng.Object, "spec", "nodeType")
		ngReady, _, _ := unstructuredNestedInt64(ng.Object, "status", "ready")
		ngTotal, _, _ := unstructuredNestedInt64(ng.Object, "status", "nodes")
		upToDate, _, _ := unstructuredNestedInt64(ng.Object, "status", "upToDate")
		ngError, _, _ := unstructuredNestedString(ng.Object, "status", "error")

		var conditions []*pb.Condition
		condSlice, _, _ := unstructuredNestedSlice(ng.Object, "status", "conditions")
		for _, c := range condSlice {
			if cMap, ok := c.(map[string]interface{}); ok {
				conditions = append(conditions, &pb.Condition{
					Type:    getString(cMap, "type"),
					Status:  getString(cMap, "status"),
					Message: getString(cMap, "message"),
				})
			}
		}

		result = append(result, &pb.NodeGroupInfo{
			Name:       name,
			NodeType:   nodeType,
			Ready:      int32(ngReady),
			Total:      int32(ngTotal),
			UpToDate:   int32(upToDate),
			Conditions: conditions,
			Error:      ngError,
		})
	}

	return &pb.ListNodeGroupsResponse{NodeGroups: result}, nil
}

func (h *DiagnosticsHandler) ListStaticInstances(ctx context.Context, req *pb.ListStaticInstancesRequest) (*pb.ListStaticInstancesResponse, error) {
	instances, err := h.client.ListStaticInstances(ctx)
	if err != nil {
		return nil, fmt.Errorf("listing static instances: %w", err)
	}

	var result []*pb.StaticInstanceInfo
	for _, si := range instances {
		name, _, _ := unstructuredNestedString(si.Object, "metadata", "name")
		address, _, _ := unstructuredNestedString(si.Object, "spec", "address")
		phase, _, _ := unstructuredNestedString(si.Object, "status", "currentStatus", "phase")
		nodeRef, _, _ := unstructuredNestedString(si.Object, "status", "nodeRef", "name")
		lastUpdate, _, _ := unstructuredNestedString(si.Object, "status", "currentStatus", "lastUpdateTime")

		labels, _, _ := unstructuredNestedStringMap(si.Object, "metadata", "labels")
		nodeGroup := labels["node.deckhouse.io/group"]

		if req.NodeGroup != nil && nodeGroup != *req.NodeGroup {
			continue
		}
		if req.Phase != nil && *req.Phase != pb.StaticInstancePhase_STATIC_INSTANCE_PHASE_UNSPECIFIED {
			if !strings.EqualFold(phase, phaseToString(*req.Phase)) {
				continue
			}
		}

		result = append(result, &pb.StaticInstanceInfo{
			Name:           name,
			Address:        address,
			Phase:          phase,
			NodeRef:        nodeRef,
			LastUpdateTime: lastUpdate,
		})
	}

	return &pb.ListStaticInstancesResponse{Instances: result}, nil
}

func (h *DiagnosticsHandler) ListUnhealthyPods(ctx context.Context, req *pb.ListUnhealthyPodsRequest) (*pb.ListUnhealthyPodsResponse, error) {
	namespace := ""
	if req.Namespace != nil {
		namespace = *req.Namespace
	}

	pods, err := h.client.ListPods(ctx, namespace)
	if err != nil {
		return nil, fmt.Errorf("listing pods: %w", err)
	}

	excludeCompleted := req.ExcludeCompleted != nil && *req.ExcludeCompleted

	var result []*pb.UnhealthyPodInfo
	for _, p := range pods {
		phase := string(p.Status.Phase)
		if phase == string(corev1.PodRunning) || phase == string(corev1.PodSucceeded) {
			continue
		}
		if excludeCompleted && phase == string(corev1.PodSucceeded) {
			continue
		}

		var restartCount int32
		for _, cs := range p.Status.ContainerStatuses {
			restartCount += cs.RestartCount
		}

		reason := phase
		if p.Status.Reason != "" {
			reason = p.Status.Reason
		}

		age := time.Since(p.CreationTimestamp.Time).Truncate(time.Second).String()

		result = append(result, &pb.UnhealthyPodInfo{
			Name:         p.Name,
			Namespace:    p.Namespace,
			Status:       phase,
			Reason:       reason,
			RestartCount: restartCount,
			Age:          age,
		})
	}

	return &pb.ListUnhealthyPodsResponse{Pods: result}, nil
}

// Helper: determine if a node is Ready.
func isNodeReady(node *corev1.Node) bool {
	for _, c := range node.Status.Conditions {
		if c.Type == corev1.NodeReady {
			return c.Status == corev1.ConditionTrue
		}
	}
	return false
}

// Helper: determine if a pod is healthy.
func isPodHealthy(pod *corev1.Pod) bool {
	phase := pod.Status.Phase
	return phase == corev1.PodRunning || phase == corev1.PodSucceeded
}

// Helper: convert Node to NodeInfo proto.
func nodeToInfo(node *corev1.Node) *pb.NodeInfo {
	status := "NotReady"
	if isNodeReady(node) {
		status = "Ready"
	}

	var role string
	for label := range node.Labels {
		if strings.HasPrefix(label, "node-role.kubernetes.io/") {
			role = strings.TrimPrefix(label, "node-role.kubernetes.io/")
			break
		}
	}

	var internalIP string
	for _, addr := range node.Status.Addresses {
		if addr.Type == corev1.NodeInternalIP {
			internalIP = addr.Address
			break
		}
	}

	age := time.Since(node.CreationTimestamp.Time).Truncate(time.Second).String()
	nodeGroup := node.Labels["node.deckhouse.io/group"]

	return &pb.NodeInfo{
		Name:           node.Name,
		Status:         status,
		Role:           role,
		InternalIp:     internalIP,
		OsImage:        node.Status.NodeInfo.OSImage,
		KubeletVersion: node.Status.NodeInfo.KubeletVersion,
		Age:            age,
		NodeGroup:      nodeGroup,
	}
}

// Helper: convert StaticInstancePhase enum to string.
func phaseToString(phase pb.StaticInstancePhase) string {
	switch phase {
	case pb.StaticInstancePhase_STATIC_INSTANCE_PHASE_PENDING:
		return "Pending"
	case pb.StaticInstancePhase_STATIC_INSTANCE_PHASE_BOOTSTRAPPING:
		return "Bootstrapping"
	case pb.StaticInstancePhase_STATIC_INSTANCE_PHASE_RUNNING:
		return "Running"
	case pb.StaticInstancePhase_STATIC_INSTANCE_PHASE_CLEANING:
		return "Cleaning"
	case pb.StaticInstancePhase_STATIC_INSTANCE_PHASE_ERROR:
		return "Error"
	default:
		return ""
	}
}

// Unstructured helpers to avoid repeating type assertions.
func unstructuredNestedString(obj map[string]interface{}, fields ...string) (string, bool, error) {
	val, found, err := nestedField(obj, fields...)
	if !found || err != nil {
		return "", found, err
	}
	s, ok := val.(string)
	return s, ok, nil
}

func unstructuredNestedInt64(obj map[string]interface{}, fields ...string) (int64, bool, error) {
	val, found, err := nestedField(obj, fields...)
	if !found || err != nil {
		return 0, found, err
	}
	switch v := val.(type) {
	case int64:
		return v, true, nil
	case float64:
		return int64(v), true, nil
	default:
		return 0, false, nil
	}
}

func unstructuredNestedSlice(obj map[string]interface{}, fields ...string) ([]interface{}, bool, error) {
	val, found, err := nestedField(obj, fields...)
	if !found || err != nil {
		return nil, found, err
	}
	s, ok := val.([]interface{})
	return s, ok, nil
}

func unstructuredNestedStringMap(obj map[string]interface{}, fields ...string) (map[string]string, bool, error) {
	val, found, err := nestedField(obj, fields...)
	if !found || err != nil {
		return nil, found, err
	}
	m, ok := val.(map[string]interface{})
	if !ok {
		return nil, false, nil
	}
	result := make(map[string]string, len(m))
	for k, v := range m {
		if s, ok := v.(string); ok {
			result[k] = s
		}
	}
	return result, true, nil
}

func nestedField(obj map[string]interface{}, fields ...string) (interface{}, bool, error) {
	var current interface{} = obj
	for _, f := range fields {
		m, ok := current.(map[string]interface{})
		if !ok {
			return nil, false, nil
		}
		current, ok = m[f]
		if !ok {
			return nil, false, nil
		}
	}
	return current, true, nil
}

func getString(m map[string]interface{}, key string) string {
	if v, ok := m[key]; ok {
		if s, ok := v.(string); ok {
			return s
		}
	}
	return ""
}

// GetNode returns detailed information about a specific node.
func (h *DiagnosticsHandler) GetNode(ctx context.Context, req *pb.GetNodeRequest) (*pb.GetNodeResponse, error) {
	node, err := h.client.GetNode(ctx, req.Name)
	if err != nil {
		return nil, fmt.Errorf("getting node %s: %w", req.Name, err)
	}

	info := nodeToInfo(node)

	// Map node conditions.
	var conditions []*pb.NodeCondition
	for _, c := range node.Status.Conditions {
		conditions = append(conditions, &pb.NodeCondition{
			Type:    string(c.Type),
			Status:  string(c.Status),
			Message: c.Message,
		})
	}

	// Map allocatable and capacity resources.
	allocatable := make(map[string]string)
	for res, qty := range node.Status.Allocatable {
		allocatable[string(res)] = qty.String()
	}
	capacity := make(map[string]string)
	for res, qty := range node.Status.Capacity {
		capacity[string(res)] = qty.String()
	}

	// Try to get StaticInstance phase for this node (may not exist for cloud nodes).
	var siPhase *string
	si, err := h.client.GetStaticInstance(ctx, req.Name)
	if err == nil && si != nil {
		phase, _, _ := unstructuredNestedString(si.Object, "status", "currentStatus", "phase")
		if phase != "" {
			siPhase = &phase
		}
	}
	// Ignore error — cloud nodes have no StaticInstance.

	// Fetch last 10 events for this node.
	rawEvents, _ := h.client.ListNodeEvents(ctx, req.Name)
	var events []*pb.NodeEvent
	for _, e := range rawEvents {
		events = append(events, &pb.NodeEvent{
			Reason:   e.Reason,
			Message:  e.Message,
			Type:     e.Type,
			LastTime: e.LastTimestamp.UTC().Format(time.RFC3339),
			Count:    e.Count,
		})
	}

	return &pb.GetNodeResponse{
		Node:                info,
		Conditions:          conditions,
		Allocatable:         allocatable,
		Capacity:            capacity,
		StaticInstancePhase: siPhase,
		Events:              events,
	}, nil
}

// GetNodeGroup returns detailed information about a specific NodeGroup including member node names.
func (h *DiagnosticsHandler) GetNodeGroup(ctx context.Context, req *pb.GetNodeGroupRequest) (*pb.GetNodeGroupResponse, error) {
	ng, err := h.client.GetNodeGroup(ctx, req.Name)
	if err != nil {
		return nil, fmt.Errorf("getting node group %s: %w", req.Name, err)
	}

	name, _, _ := unstructuredNestedString(ng.Object, "metadata", "name")
	nodeType, _, _ := unstructuredNestedString(ng.Object, "spec", "nodeType")
	ngReady, _, _ := unstructuredNestedInt64(ng.Object, "status", "ready")
	ngTotal, _, _ := unstructuredNestedInt64(ng.Object, "status", "nodes")
	upToDate, _, _ := unstructuredNestedInt64(ng.Object, "status", "upToDate")
	statusMsg, _, _ := unstructuredNestedString(ng.Object, "status", "error")

	// Collect node names that belong to this group.
	nodes, err := h.client.ListNodes(ctx)
	if err != nil {
		return nil, fmt.Errorf("listing nodes for group %s: %w", req.Name, err)
	}

	var nodeNames []string
	for _, n := range nodes {
		if n.Labels["node.deckhouse.io/group"] == name {
			nodeNames = append(nodeNames, n.Name)
		}
	}

	return &pb.GetNodeGroupResponse{
		Name:          name,
		NodeType:      nodeType,
		Ready:         int32(ngReady),
		Total:         int32(ngTotal),
		UpToDate:      int32(upToDate),
		StatusMessage: statusMsg,
		NodeNames:     nodeNames,
	}, nil
}

// GetDeckhouseLogs retrieves the Deckhouse controller pod logs with optional filtering.
func (h *DiagnosticsHandler) GetDeckhouseLogs(ctx context.Context, req *pb.GetDeckhouseLogsRequest) (*pb.GetDeckhouseLogsResponse, error) {
	pods, err := h.client.ListPods(ctx, "d8-system")
	if err != nil {
		return nil, fmt.Errorf("listing pods in d8-system: %w", err)
	}

	// Find the main Deckhouse controller pod.
	var podName string
	for _, p := range pods {
		if p.Labels["app"] == "deckhouse" {
			podName = p.Name
			break
		}
	}
	if podName == "" {
		return nil, fmt.Errorf("deckhouse pod not found in d8-system namespace")
	}

	// Convert tail and since to client types.
	var tailLines *int64
	if req.Tail != nil {
		lines := int64(*req.Tail)
		tailLines = &lines
	}
	var since *string
	if req.Since != nil {
		since = req.Since
	}

	logs, err := h.client.GetPodLogs(ctx, "d8-system", podName, "deckhouse", tailLines, since)
	if err != nil {
		return nil, fmt.Errorf("getting pod logs for %s: %w", podName, err)
	}

	// Apply client-side grep filter if provided.
	if req.Grep != nil && *req.Grep != "" {
		pattern := *req.Grep
		var filtered []string
		for _, line := range strings.Split(logs, "\n") {
			if strings.Contains(line, pattern) {
				filtered = append(filtered, line)
			}
		}
		// Re-join with "\n" suffix on each line to preserve log format.
		var b strings.Builder
		for _, line := range filtered {
			b.WriteString(line)
			b.WriteString("\n")
		}
		logs = b.String()
	}

	return &pb.GetDeckhouseLogsResponse{Logs: logs}, nil
}
