package handler

import (
	"context"
	"testing"

	emptypb "google.golang.org/protobuf/types/known/emptypb"
	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"

	pb "github.com/sipki-tech/deckhouse-mcp/proto/deckhouse/v1"
)

func TestGetClusterStatus_Empty(t *testing.T) {
	h := NewDiagnosticsHandler(&mockClient{})
	resp, err := h.GetClusterStatus(context.Background(), &emptypb.Empty{})
	if err != nil {
		t.Fatal(err)
	}
	if resp.Nodes.Total != 0 {
		t.Errorf("expected total=0, got %d", resp.Nodes.Total)
	}
	if resp.Nodes.Ready != 0 {
		t.Errorf("expected ready=0, got %d", resp.Nodes.Ready)
	}
	if len(resp.NodeGroups) != 0 {
		t.Errorf("expected 0 node groups, got %d", len(resp.NodeGroups))
	}
	if len(resp.ErroredModules) != 0 {
		t.Errorf("expected 0 errored modules, got %d", len(resp.ErroredModules))
	}
	if len(resp.PendingReleases) != 0 {
		t.Errorf("expected 0 pending releases, got %d", len(resp.PendingReleases))
	}
}

func TestGetClusterStatus_Mixed(t *testing.T) {
	mc := &mockClient{
		listNodesFunc: func(_ context.Context) ([]corev1.Node, error) {
			return []corev1.Node{
				makeNode("node-1", true),
				makeNode("node-2", true),
				makeNode("node-3", false),
			}, nil
		},
		listNodeGroupsFunc: func(_ context.Context) ([]unstructured.Unstructured, error) {
			return []unstructured.Unstructured{
				makeNodeGroup("workers", 2, 3),
			}, nil
		},
		listModuleConfigsFunc: func(_ context.Context) ([]unstructured.Unstructured, error) {
			return []unstructured.Unstructured{
				makeModuleConfig("mod-ok", true, ""),
				makeModuleConfig("mod-err", true, "something failed"),
			}, nil
		},
		listDeckhouseReleasesFunc: func(_ context.Context) ([]unstructured.Unstructured, error) {
			return []unstructured.Unstructured{
				makeRelease("v1.60.0", "Deployed", "1.60.0"),
				makeRelease("v1.61.0", "Pending", "1.61.0"),
			}, nil
		},
		listPodsFunc: func(_ context.Context, _ string) ([]corev1.Pod, error) {
			return []corev1.Pod{
				{Status: corev1.PodStatus{Phase: corev1.PodRunning}},
				{Status: corev1.PodStatus{Phase: corev1.PodFailed}},
			}, nil
		},
	}

	h := NewDiagnosticsHandler(mc)
	resp, err := h.GetClusterStatus(context.Background(), &emptypb.Empty{})
	if err != nil {
		t.Fatal(err)
	}
	if resp.Nodes.Total != 3 {
		t.Errorf("expected total=3, got %d", resp.Nodes.Total)
	}
	if resp.Nodes.Ready != 2 {
		t.Errorf("expected ready=2, got %d", resp.Nodes.Ready)
	}
	if resp.Nodes.NotReady != 1 {
		t.Errorf("expected notReady=1, got %d", resp.Nodes.NotReady)
	}
	if len(resp.NodeGroups) != 1 {
		t.Fatalf("expected 1 node group, got %d", len(resp.NodeGroups))
	}
	if resp.NodeGroups[0].Name != "workers" {
		t.Errorf("expected ng name=workers, got %s", resp.NodeGroups[0].Name)
	}
	if len(resp.ErroredModules) != 1 || resp.ErroredModules[0] != "mod-err" {
		t.Errorf("expected [mod-err], got %v", resp.ErroredModules)
	}
	if len(resp.PendingReleases) != 1 {
		t.Fatalf("expected 1 pending release, got %d", len(resp.PendingReleases))
	}
	if resp.DeckhouseVersion != "1.60.0" {
		t.Errorf("expected version 1.60.0, got %s", resp.DeckhouseVersion)
	}
	if resp.UnhealthyPodsCount != 1 {
		t.Errorf("expected unhealthy=1, got %d", resp.UnhealthyPodsCount)
	}
}

func TestListNodes_NoFilter(t *testing.T) {
	mc := &mockClient{
		listNodesFunc: func(_ context.Context) ([]corev1.Node, error) {
			return []corev1.Node{
				makeNode("node-1", true),
				makeNode("node-2", false),
			}, nil
		},
	}

	h := NewDiagnosticsHandler(mc)
	resp, err := h.ListNodes(context.Background(), &pb.ListNodesRequest{})
	if err != nil {
		t.Fatal(err)
	}
	if len(resp.Nodes) != 2 {
		t.Errorf("expected 2 nodes, got %d", len(resp.Nodes))
	}
}

func TestListNodes_ByNodeGroup(t *testing.T) {
	mc := &mockClient{
		listNodesFunc: func(_ context.Context) ([]corev1.Node, error) {
			return []corev1.Node{
				makeNodeWithGroup("node-1", true, "workers"),
				makeNodeWithGroup("node-2", true, "masters"),
			}, nil
		},
	}

	h := NewDiagnosticsHandler(mc)
	ng := "workers"
	resp, err := h.ListNodes(context.Background(), &pb.ListNodesRequest{NodeGroup: &ng})
	if err != nil {
		t.Fatal(err)
	}
	if len(resp.Nodes) != 1 {
		t.Errorf("expected 1 node, got %d", len(resp.Nodes))
	}
	if resp.Nodes[0].Name != "node-1" {
		t.Errorf("expected node-1, got %s", resp.Nodes[0].Name)
	}
}

func TestListNodes_ByStatus(t *testing.T) {
	mc := &mockClient{
		listNodesFunc: func(_ context.Context) ([]corev1.Node, error) {
			return []corev1.Node{
				makeNode("node-1", true),
				makeNode("node-2", false),
				makeNode("node-3", true),
			}, nil
		},
	}

	h := NewDiagnosticsHandler(mc)
	status := pb.NodeStatusFilter_NODE_STATUS_FILTER_READY
	resp, err := h.ListNodes(context.Background(), &pb.ListNodesRequest{Status: &status})
	if err != nil {
		t.Fatal(err)
	}
	if len(resp.Nodes) != 2 {
		t.Errorf("expected 2 ready nodes, got %d", len(resp.Nodes))
	}
}

func TestListNodes_ByRole(t *testing.T) {
	mc := &mockClient{
		listNodesFunc: func(_ context.Context) ([]corev1.Node, error) {
			return []corev1.Node{
				makeNodeWithRole("node-1", true, "master"),
				makeNodeWithRole("node-2", true, "worker"),
			}, nil
		},
	}

	h := NewDiagnosticsHandler(mc)
	role := "worker"
	resp, err := h.ListNodes(context.Background(), &pb.ListNodesRequest{Role: &role})
	if err != nil {
		t.Fatal(err)
	}
	if len(resp.Nodes) != 1 {
		t.Errorf("expected 1 node, got %d", len(resp.Nodes))
	}
	if resp.Nodes[0].Name != "node-2" {
		t.Errorf("expected node-2, got %s", resp.Nodes[0].Name)
	}
}

func TestListNodes_Combined(t *testing.T) {
	mc := &mockClient{
		listNodesFunc: func(_ context.Context) ([]corev1.Node, error) {
			n1 := makeNodeFull("node-1", true, "workers", "worker")
			n2 := makeNodeFull("node-2", false, "workers", "worker")
			n3 := makeNodeFull("node-3", true, "masters", "master")
			return []corev1.Node{n1, n2, n3}, nil
		},
	}

	h := NewDiagnosticsHandler(mc)
	ng := "workers"
	status := pb.NodeStatusFilter_NODE_STATUS_FILTER_READY
	role := "worker"
	resp, err := h.ListNodes(context.Background(), &pb.ListNodesRequest{
		NodeGroup: &ng,
		Status:    &status,
		Role:      &role,
	})
	if err != nil {
		t.Fatal(err)
	}
	if len(resp.Nodes) != 1 {
		t.Errorf("expected 1 node, got %d", len(resp.Nodes))
	}
	if resp.Nodes[0].Name != "node-1" {
		t.Errorf("expected node-1, got %s", resp.Nodes[0].Name)
	}
}

func TestListNodes_Empty(t *testing.T) {
	mc := &mockClient{
		listNodesFunc: func(_ context.Context) ([]corev1.Node, error) {
			return []corev1.Node{makeNode("node-1", true)}, nil
		},
	}

	h := NewDiagnosticsHandler(mc)
	ng := "nonexistent"
	resp, err := h.ListNodes(context.Background(), &pb.ListNodesRequest{NodeGroup: &ng})
	if err != nil {
		t.Fatal(err)
	}
	if len(resp.Nodes) != 0 {
		t.Errorf("expected 0 nodes, got %d", len(resp.Nodes))
	}
}

func TestListNodeGroups_All(t *testing.T) {
	mc := &mockClient{
		listNodeGroupsFunc: func(_ context.Context) ([]unstructured.Unstructured, error) {
			return []unstructured.Unstructured{
				makeNodeGroup("workers", 3, 3),
				makeNodeGroup("masters", 1, 1),
			}, nil
		},
	}

	h := NewDiagnosticsHandler(mc)
	resp, err := h.ListNodeGroups(context.Background(), &emptypb.Empty{})
	if err != nil {
		t.Fatal(err)
	}
	if len(resp.NodeGroups) != 2 {
		t.Errorf("expected 2 node groups, got %d", len(resp.NodeGroups))
	}
}

func TestListNodeGroups_WithError(t *testing.T) {
	mc := &mockClient{
		listNodeGroupsFunc: func(_ context.Context) ([]unstructured.Unstructured, error) {
			ng := makeNodeGroup("broken", 0, 1)
			ng.Object["status"].(map[string]interface{})["error"] = "some error"
			return []unstructured.Unstructured{ng}, nil
		},
	}

	h := NewDiagnosticsHandler(mc)
	resp, err := h.ListNodeGroups(context.Background(), &emptypb.Empty{})
	if err != nil {
		t.Fatal(err)
	}
	if resp.NodeGroups[0].Error != "some error" {
		t.Errorf("expected error 'some error', got %q", resp.NodeGroups[0].Error)
	}
}

func TestListNodeGroups_Empty(t *testing.T) {
	mc := &mockClient{
		listNodeGroupsFunc: func(_ context.Context) ([]unstructured.Unstructured, error) {
			return nil, nil
		},
	}

	h := NewDiagnosticsHandler(mc)
	resp, err := h.ListNodeGroups(context.Background(), &emptypb.Empty{})
	if err != nil {
		t.Fatal(err)
	}
	if len(resp.NodeGroups) != 0 {
		t.Errorf("expected 0, got %d", len(resp.NodeGroups))
	}
}

func TestListStaticInstances_NoFilter(t *testing.T) {
	mc := &mockClient{
		listStaticInstancesFunc: func(_ context.Context) ([]unstructured.Unstructured, error) {
			return []unstructured.Unstructured{
				makeStaticInstance("si-1", "10.0.0.1", "Running", "workers"),
				makeStaticInstance("si-2", "10.0.0.2", "Pending", "workers"),
			}, nil
		},
	}

	h := NewDiagnosticsHandler(mc)
	resp, err := h.ListStaticInstances(context.Background(), &pb.ListStaticInstancesRequest{})
	if err != nil {
		t.Fatal(err)
	}
	if len(resp.Instances) != 2 {
		t.Errorf("expected 2, got %d", len(resp.Instances))
	}
}

func TestListStaticInstances_ByPhase(t *testing.T) {
	mc := &mockClient{
		listStaticInstancesFunc: func(_ context.Context) ([]unstructured.Unstructured, error) {
			return []unstructured.Unstructured{
				makeStaticInstance("si-1", "10.0.0.1", "Running", "workers"),
				makeStaticInstance("si-2", "10.0.0.2", "Pending", "workers"),
			}, nil
		},
	}

	h := NewDiagnosticsHandler(mc)
	phase := pb.StaticInstancePhase_STATIC_INSTANCE_PHASE_RUNNING
	resp, err := h.ListStaticInstances(context.Background(), &pb.ListStaticInstancesRequest{Phase: &phase})
	if err != nil {
		t.Fatal(err)
	}
	if len(resp.Instances) != 1 {
		t.Errorf("expected 1, got %d", len(resp.Instances))
	}
	if resp.Instances[0].Name != "si-1" {
		t.Errorf("expected si-1, got %s", resp.Instances[0].Name)
	}
}

func TestListStaticInstances_ByNodeGroup(t *testing.T) {
	mc := &mockClient{
		listStaticInstancesFunc: func(_ context.Context) ([]unstructured.Unstructured, error) {
			return []unstructured.Unstructured{
				makeStaticInstance("si-1", "10.0.0.1", "Running", "workers"),
				makeStaticInstance("si-2", "10.0.0.2", "Running", "masters"),
			}, nil
		},
	}

	h := NewDiagnosticsHandler(mc)
	ng := "workers"
	resp, err := h.ListStaticInstances(context.Background(), &pb.ListStaticInstancesRequest{NodeGroup: &ng})
	if err != nil {
		t.Fatal(err)
	}
	if len(resp.Instances) != 1 {
		t.Errorf("expected 1, got %d", len(resp.Instances))
	}
}

func TestListStaticInstances_Combined(t *testing.T) {
	mc := &mockClient{
		listStaticInstancesFunc: func(_ context.Context) ([]unstructured.Unstructured, error) {
			return []unstructured.Unstructured{
				makeStaticInstance("si-1", "10.0.0.1", "Running", "workers"),
				makeStaticInstance("si-2", "10.0.0.2", "Pending", "workers"),
				makeStaticInstance("si-3", "10.0.0.3", "Running", "masters"),
			}, nil
		},
	}

	h := NewDiagnosticsHandler(mc)
	ng := "workers"
	phase := pb.StaticInstancePhase_STATIC_INSTANCE_PHASE_RUNNING
	resp, err := h.ListStaticInstances(context.Background(), &pb.ListStaticInstancesRequest{
		NodeGroup: &ng,
		Phase:     &phase,
	})
	if err != nil {
		t.Fatal(err)
	}
	if len(resp.Instances) != 1 {
		t.Errorf("expected 1, got %d", len(resp.Instances))
	}
	if resp.Instances[0].Name != "si-1" {
		t.Errorf("expected si-1, got %s", resp.Instances[0].Name)
	}
}

func TestListUnhealthyPods_All(t *testing.T) {
	mc := &mockClient{
		listPodsFunc: func(_ context.Context, _ string) ([]corev1.Pod, error) {
			return []corev1.Pod{
				makePod("pod-1", "ns-1", corev1.PodRunning),
				makePod("pod-2", "ns-1", corev1.PodFailed),
				makePod("pod-3", "ns-2", corev1.PodPending),
			}, nil
		},
	}

	h := NewDiagnosticsHandler(mc)
	resp, err := h.ListUnhealthyPods(context.Background(), &pb.ListUnhealthyPodsRequest{})
	if err != nil {
		t.Fatal(err)
	}
	if len(resp.Pods) != 2 {
		t.Errorf("expected 2, got %d", len(resp.Pods))
	}
}

func TestListUnhealthyPods_Namespace(t *testing.T) {
	called := ""
	mc := &mockClient{
		listPodsFunc: func(_ context.Context, namespace string) ([]corev1.Pod, error) {
			called = namespace
			return []corev1.Pod{
				makePod("pod-1", "ns-1", corev1.PodFailed),
			}, nil
		},
	}

	h := NewDiagnosticsHandler(mc)
	ns := "ns-1"
	_, err := h.ListUnhealthyPods(context.Background(), &pb.ListUnhealthyPodsRequest{Namespace: &ns})
	if err != nil {
		t.Fatal(err)
	}
	if called != "ns-1" {
		t.Errorf("expected namespace ns-1, got %q", called)
	}
}

func TestListUnhealthyPods_ExcludeCompleted(t *testing.T) {
	mc := &mockClient{
		listPodsFunc: func(_ context.Context, _ string) ([]corev1.Pod, error) {
			return []corev1.Pod{
				makePod("pod-1", "ns-1", corev1.PodFailed),
				makePod("pod-2", "ns-1", corev1.PodSucceeded),
				makePod("pod-3", "ns-1", corev1.PodPending),
			}, nil
		},
	}

	h := NewDiagnosticsHandler(mc)
	exclude := true
	resp, err := h.ListUnhealthyPods(context.Background(), &pb.ListUnhealthyPodsRequest{ExcludeCompleted: &exclude})
	if err != nil {
		t.Fatal(err)
	}
	// PodFailed and PodPending are unhealthy. PodSucceeded is filtered by the base filter (not in Running/Succeeded).
	// Wait — Succeeded is filtered by base filter already. Let me check: Running/Succeeded are filtered out.
	// So PodFailed and PodPending remain = 2.
	if len(resp.Pods) != 2 {
		t.Errorf("expected 2, got %d", len(resp.Pods))
	}
}

func TestListUnhealthyPods_AllHealthy(t *testing.T) {
	mc := &mockClient{
		listPodsFunc: func(_ context.Context, _ string) ([]corev1.Pod, error) {
			return []corev1.Pod{
				makePod("pod-1", "ns-1", corev1.PodRunning),
				makePod("pod-2", "ns-1", corev1.PodSucceeded),
			}, nil
		},
	}

	h := NewDiagnosticsHandler(mc)
	resp, err := h.ListUnhealthyPods(context.Background(), &pb.ListUnhealthyPodsRequest{})
	if err != nil {
		t.Fatal(err)
	}
	if len(resp.Pods) != 0 {
		t.Errorf("expected 0, got %d", len(resp.Pods))
	}
}

// Test helpers

func makeNode(name string, ready bool) corev1.Node {
	condStatus := corev1.ConditionFalse
	if ready {
		condStatus = corev1.ConditionTrue
	}
	return corev1.Node{
		ObjectMeta: metav1.ObjectMeta{Name: name, Labels: map[string]string{}, CreationTimestamp: metav1.Now()},
		Status: corev1.NodeStatus{
			Conditions: []corev1.NodeCondition{
				{Type: corev1.NodeReady, Status: condStatus},
			},
		},
	}
}

func makeNodeWithGroup(name string, ready bool, group string) corev1.Node {
	n := makeNode(name, ready)
	n.Labels["node.deckhouse.io/group"] = group
	return n
}

func makeNodeWithRole(name string, ready bool, role string) corev1.Node {
	n := makeNode(name, ready)
	n.Labels["node-role.kubernetes.io/"+role] = ""
	return n
}

func makeNodeFull(name string, ready bool, group, role string) corev1.Node {
	n := makeNode(name, ready)
	n.Labels["node.deckhouse.io/group"] = group
	n.Labels["node-role.kubernetes.io/"+role] = ""
	return n
}

func makeNodeGroup(name string, ready, total int64) unstructured.Unstructured {
	return unstructured.Unstructured{
		Object: map[string]interface{}{
			"apiVersion": "deckhouse.io/v1",
			"kind":       "NodeGroup",
			"metadata": map[string]interface{}{
				"name": name,
			},
			"spec": map[string]interface{}{
				"nodeType": "Static",
			},
			"status": map[string]interface{}{
				"ready": ready,
				"nodes": total,
			},
		},
	}
}

func makeStaticInstance(name, address, phase, nodeGroup string) unstructured.Unstructured {
	return unstructured.Unstructured{
		Object: map[string]interface{}{
			"apiVersion": "deckhouse.io/v1alpha2",
			"kind":       "StaticInstance",
			"metadata": map[string]interface{}{
				"name": name,
				"labels": map[string]interface{}{
					"node.deckhouse.io/group": nodeGroup,
				},
			},
			"spec": map[string]interface{}{
				"address": address,
			},
			"status": map[string]interface{}{
				"currentStatus": map[string]interface{}{
					"phase": phase,
				},
			},
		},
	}
}

func makeModuleConfig(name string, enabled bool, statusMsg string) unstructured.Unstructured {
	obj := map[string]interface{}{
		"apiVersion": "deckhouse.io/v1alpha1",
		"kind":       "ModuleConfig",
		"metadata": map[string]interface{}{
			"name": name,
		},
		"spec": map[string]interface{}{
			"enabled": enabled,
		},
		"status": map[string]interface{}{},
	}
	if statusMsg != "" {
		obj["status"].(map[string]interface{})["message"] = statusMsg
	}
	return unstructured.Unstructured{Object: obj}
}

func makeRelease(name, phase, version string) unstructured.Unstructured {
	return unstructured.Unstructured{
		Object: map[string]interface{}{
			"apiVersion": "deckhouse.io/v1alpha1",
			"kind":       "DeckhouseRelease",
			"metadata": map[string]interface{}{
				"name": name,
			},
			"spec": map[string]interface{}{
				"version": version,
			},
			"status": map[string]interface{}{
				"phase": phase,
			},
		},
	}
}

func makePod(name, namespace string, phase corev1.PodPhase) corev1.Pod {
	return corev1.Pod{
		ObjectMeta: metav1.ObjectMeta{Name: name, Namespace: namespace, CreationTimestamp: metav1.Now()},
		Status:     corev1.PodStatus{Phase: phase},
	}
}
