package handler

import (
	"context"
	"encoding/base64"
	"fmt"
	"testing"

	corev1 "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"
	"k8s.io/apimachinery/pkg/runtime/schema"

	pb "github.com/sipki-tech/deckhouse-mcp/proto/deckhouse/v1"
)

func TestCreateSSHCredentials_OK(t *testing.T) {
	var captured *unstructured.Unstructured
	mc := &mockClient{
		createSSHCredentialsFunc: func(_ context.Context, obj *unstructured.Unstructured) (*unstructured.Unstructured, error) {
			captured = obj
			return obj, nil
		},
	}

	h := NewNodesHandler(mc)
	resp, err := h.CreateSSHCredentials(context.Background(), &pb.CreateSSHCredentialsRequest{
		Name:       "test-creds",
		User:       "ubuntu",
		PrivateKey: "-----BEGIN RSA PRIVATE KEY-----\ntest\n-----END RSA PRIVATE KEY-----",
	})
	if err != nil {
		t.Fatal(err)
	}
	if resp.Name != "test-creds" {
		t.Errorf("expected test-creds, got %s", resp.Name)
	}

	// Verify base64 encoding.
	spec := captured.Object["spec"].(map[string]interface{})
	encoded := spec["privateSSHKey"].(string)
	decoded, err := base64.StdEncoding.DecodeString(encoded)
	if err != nil {
		t.Fatalf("failed to decode base64: %v", err)
	}
	if string(decoded) != "-----BEGIN RSA PRIVATE KEY-----\ntest\n-----END RSA PRIVATE KEY-----" {
		t.Errorf("round-trip failed: got %q", string(decoded))
	}
}

func TestCreateSSHCredentials_AlreadyExists(t *testing.T) {
	mc := &mockClient{
		createSSHCredentialsFunc: func(_ context.Context, _ *unstructured.Unstructured) (*unstructured.Unstructured, error) {
			return nil, errors.NewAlreadyExists(schema.GroupResource{Group: "deckhouse.io", Resource: "sshcredentials"}, "test-creds")
		},
	}

	h := NewNodesHandler(mc)
	_, err := h.CreateSSHCredentials(context.Background(), &pb.CreateSSHCredentialsRequest{
		Name:       "test-creds",
		User:       "ubuntu",
		PrivateKey: "key",
	})
	if err == nil {
		t.Fatal("expected error, got nil")
	}
}

func TestCreateSSHCredentials_Defaults(t *testing.T) {
	var captured *unstructured.Unstructured
	mc := &mockClient{
		createSSHCredentialsFunc: func(_ context.Context, obj *unstructured.Unstructured) (*unstructured.Unstructured, error) {
			captured = obj
			return obj, nil
		},
	}

	h := NewNodesHandler(mc)
	_, err := h.CreateSSHCredentials(context.Background(), &pb.CreateSSHCredentialsRequest{
		Name:       "test-creds",
		User:       "ubuntu",
		PrivateKey: "key",
	})
	if err != nil {
		t.Fatal(err)
	}

	spec := captured.Object["spec"].(map[string]interface{})
	port := spec["sshPort"]
	if port != int64(22) {
		t.Errorf("expected default port 22, got %v", port)
	}
}

func TestCreateStaticInstance_OK(t *testing.T) {
	var captured *unstructured.Unstructured
	mc := &mockClient{
		createStaticInstanceFunc: func(_ context.Context, obj *unstructured.Unstructured) (*unstructured.Unstructured, error) {
			captured = obj
			return obj, nil
		},
	}

	h := NewNodesHandler(mc)
	resp, err := h.CreateStaticInstance(context.Background(), &pb.CreateStaticInstanceRequest{
		Name:           "node-1",
		Address:        "10.0.0.1",
		CredentialsRef: "creds-1",
		Labels: map[string]string{
			"node.deckhouse.io/group": "workers",
		},
	})
	if err != nil {
		t.Fatal(err)
	}
	if resp.Name != "node-1" {
		t.Errorf("expected node-1, got %s", resp.Name)
	}

	// Verify labels propagated.
	labels := captured.GetLabels()
	if labels["node.deckhouse.io/group"] != "workers" {
		t.Errorf("expected label workers, got %q", labels["node.deckhouse.io/group"])
	}
}

func TestCreateStaticInstance_AlreadyExists(t *testing.T) {
	mc := &mockClient{
		createStaticInstanceFunc: func(_ context.Context, _ *unstructured.Unstructured) (*unstructured.Unstructured, error) {
			return nil, errors.NewAlreadyExists(schema.GroupResource{Group: "deckhouse.io", Resource: "staticinstances"}, "node-1")
		},
	}

	h := NewNodesHandler(mc)
	_, err := h.CreateStaticInstance(context.Background(), &pb.CreateStaticInstanceRequest{
		Name:           "node-1",
		Address:        "10.0.0.1",
		CredentialsRef: "creds-1",
	})
	if err == nil {
		t.Fatal("expected error, got nil")
	}
}

func TestAddWorkerNode_HappyPath(t *testing.T) {
	pollCount := 0
	mc := &mockClient{
		createSSHCredentialsFunc: func(_ context.Context, obj *unstructured.Unstructured) (*unstructured.Unstructured, error) {
			return obj, nil
		},
		createStaticInstanceFunc: func(_ context.Context, obj *unstructured.Unstructured) (*unstructured.Unstructured, error) {
			return obj, nil
		},
		getStaticInstanceFunc: func(_ context.Context, name string) (*unstructured.Unstructured, error) {
			pollCount++
			phase := "Pending"
			if pollCount >= 2 {
				phase = "Running"
			}
			return &unstructured.Unstructured{
				Object: map[string]interface{}{
					"status": map[string]interface{}{
						"currentStatus": map[string]interface{}{
							"phase": phase,
						},
					},
				},
			}, nil
		},
	}

	h := NewNodesHandler(mc)
	timeout := int32(5) // short timeout for test
	resp, err := h.AddWorkerNode(context.Background(), &pb.AddWorkerNodeRequest{
		Address:        "10.0.0.1",
		SshUser:        "ubuntu",
		PrivateKey:     "key",
		NodeGroup:      "workers",
		TimeoutSeconds: &timeout,
	})
	if err != nil {
		t.Fatal(err)
	}
	if resp.NodeName != "10-0-0-1" {
		t.Errorf("expected 10-0-0-1, got %s", resp.NodeName)
	}
	if resp.SshCredentialsName != "10-0-0-1-creds" {
		t.Errorf("expected 10-0-0-1-creds, got %s", resp.SshCredentialsName)
	}
	if resp.Phase != "Running" {
		t.Errorf("expected Running, got %s", resp.Phase)
	}
	if resp.TimedOut {
		t.Error("expected timedOut=false")
	}
}

func TestAddWorkerNode_Timeout(t *testing.T) {
	mc := &mockClient{
		createSSHCredentialsFunc: func(_ context.Context, obj *unstructured.Unstructured) (*unstructured.Unstructured, error) {
			return obj, nil
		},
		createStaticInstanceFunc: func(_ context.Context, obj *unstructured.Unstructured) (*unstructured.Unstructured, error) {
			return obj, nil
		},
		getStaticInstanceFunc: func(_ context.Context, _ string) (*unstructured.Unstructured, error) {
			return &unstructured.Unstructured{
				Object: map[string]interface{}{
					"status": map[string]interface{}{
						"currentStatus": map[string]interface{}{
							"phase": "Bootstrapping",
						},
					},
				},
			}, nil
		},
	}

	h := NewNodesHandler(mc)
	timeout := int32(1)
	resp, err := h.AddWorkerNode(context.Background(), &pb.AddWorkerNodeRequest{
		Address:        "10.0.0.1",
		SshUser:        "ubuntu",
		PrivateKey:     "key",
		NodeGroup:      "workers",
		TimeoutSeconds: &timeout,
	})
	if err != nil {
		t.Fatal(err)
	}
	if !resp.TimedOut {
		t.Error("expected timedOut=true")
	}
	if resp.Phase != "Bootstrapping" {
		t.Errorf("expected Bootstrapping, got %s", resp.Phase)
	}
}

func TestAddWorkerNode_SSHCredsError(t *testing.T) {
	siCreated := false
	mc := &mockClient{
		createSSHCredentialsFunc: func(_ context.Context, _ *unstructured.Unstructured) (*unstructured.Unstructured, error) {
			return nil, fmt.Errorf("ssh error")
		},
		createStaticInstanceFunc: func(_ context.Context, obj *unstructured.Unstructured) (*unstructured.Unstructured, error) {
			siCreated = true
			return obj, nil
		},
	}

	h := NewNodesHandler(mc)
	_, err := h.AddWorkerNode(context.Background(), &pb.AddWorkerNodeRequest{
		Address:    "10.0.0.1",
		SshUser:    "ubuntu",
		PrivateKey: "key",
		NodeGroup:  "workers",
	})
	if err == nil {
		t.Fatal("expected error")
	}
	if siCreated {
		t.Error("StaticInstance should NOT be created when SSHCredentials fails")
	}
}

func TestAddWorkerNode_StaticInstanceError(t *testing.T) {
	mc := &mockClient{
		createSSHCredentialsFunc: func(_ context.Context, obj *unstructured.Unstructured) (*unstructured.Unstructured, error) {
			return obj, nil
		},
		createStaticInstanceFunc: func(_ context.Context, _ *unstructured.Unstructured) (*unstructured.Unstructured, error) {
			return nil, fmt.Errorf("si error")
		},
	}

	h := NewNodesHandler(mc)
	_, err := h.AddWorkerNode(context.Background(), &pb.AddWorkerNodeRequest{
		Address:    "10.0.0.1",
		SshUser:    "ubuntu",
		PrivateKey: "key",
		NodeGroup:  "workers",
	})
	if err == nil {
		t.Fatal("expected error")
	}
	// Error should mention SSHCredentials was already created.
	if !containsSubstring(err.Error(), "already created") {
		t.Errorf("error should mention SSHCredentials was already created: %v", err)
	}
}

func TestAddWorkerNode_NoWait(t *testing.T) {
	mc := &mockClient{
		createSSHCredentialsFunc: func(_ context.Context, obj *unstructured.Unstructured) (*unstructured.Unstructured, error) {
			return obj, nil
		},
		createStaticInstanceFunc: func(_ context.Context, obj *unstructured.Unstructured) (*unstructured.Unstructured, error) {
			return obj, nil
		},
	}

	h := NewNodesHandler(mc)
	waitReady := false
	resp, err := h.AddWorkerNode(context.Background(), &pb.AddWorkerNodeRequest{
		Address:    "10.0.0.1",
		SshUser:    "ubuntu",
		PrivateKey: "key",
		NodeGroup:  "workers",
		WaitReady:  &waitReady,
	})
	if err != nil {
		t.Fatal(err)
	}
	if resp.Phase != "Pending" {
		t.Errorf("expected Pending (no polling), got %s", resp.Phase)
	}
	if resp.TimedOut {
		t.Error("expected timedOut=false")
	}
}

func TestAddWorkerNode_GeneratedName(t *testing.T) {
	mc := &mockClient{
		createSSHCredentialsFunc: func(_ context.Context, obj *unstructured.Unstructured) (*unstructured.Unstructured, error) {
			return obj, nil
		},
		createStaticInstanceFunc: func(_ context.Context, obj *unstructured.Unstructured) (*unstructured.Unstructured, error) {
			return obj, nil
		},
	}

	h := NewNodesHandler(mc)
	waitReady := false
	resp, err := h.AddWorkerNode(context.Background(), &pb.AddWorkerNodeRequest{
		Address:    "192.168.1.100",
		SshUser:    "ubuntu",
		PrivateKey: "key",
		NodeGroup:  "workers",
		WaitReady:  &waitReady,
	})
	if err != nil {
		t.Fatal(err)
	}
	if resp.NodeName != "192-168-1-100" {
		t.Errorf("expected 192-168-1-100, got %s", resp.NodeName)
	}
}

func containsSubstring(s, substr string) bool {
	return len(s) >= len(substr) && (s == substr || len(s) > 0 && containsStr(s, substr))
}

func containsStr(s, substr string) bool {
	for i := 0; i <= len(s)-len(substr); i++ {
		if s[i:i+len(substr)] == substr {
			return true
		}
	}
	return false
}

// Ensure metav1 import is used.
var _ = metav1.Now()

func TestDeleteStaticInstance_Success(t *testing.T) {
	deleted := false
	mc := &mockClient{
		deleteStaticInstanceFunc: func(_ context.Context, name string) error {
			deleted = true
			return nil
		},
	}

	h := NewNodesHandler(mc)
	resp, err := h.DeleteStaticInstance(context.Background(), &pb.DeleteStaticInstanceRequest{Name: "node-1"})
	if err != nil {
		t.Fatal(err)
	}
	if !resp.Success {
		t.Error("expected success=true")
	}
	if !deleted {
		t.Error("expected deleteStaticInstance to be called")
	}
}

func TestDeleteStaticInstance_NotFound(t *testing.T) {
	mc := &mockClient{
		deleteStaticInstanceFunc: func(_ context.Context, name string) error {
			return fmt.Errorf("static instance %q not found", name)
		},
	}

	h := NewNodesHandler(mc)
	_, err := h.DeleteStaticInstance(context.Background(), &pb.DeleteStaticInstanceRequest{Name: "missing"})
	if err == nil {
		t.Fatal("expected error, got nil")
	}
}

func TestRemoveNode_DrainAndDelete(t *testing.T) {
	cordoned := false
	deleted := false
	podDeleted := false
	si := makeStaticInstance("node-1", "10.0.0.1", "Running", "workers")
	mc := &mockClient{
		getStaticInstanceFunc: func(_ context.Context, _ string) (*unstructured.Unstructured, error) {
			return &si, nil
		},
		getNodeFunc: func(_ context.Context, _ string) (*corev1.Node, error) {
			n := makeNode("node-1", true)
			return &n, nil
		},
		cordonNodeFunc: func(_ context.Context, _ string) error {
			cordoned = true
			return nil
		},
		listPodsFunc: func(_ context.Context, _ string) ([]corev1.Pod, error) {
			pod := corev1.Pod{
				ObjectMeta: metav1.ObjectMeta{Name: "app-pod", Namespace: "default"},
				Spec:       corev1.PodSpec{NodeName: "node-1"},
			}
			return []corev1.Pod{pod}, nil
		},
		deletePodFunc: func(_ context.Context, _, _ string) error {
			podDeleted = true
			return nil
		},
		deleteStaticInstanceFunc: func(_ context.Context, _ string) error {
			deleted = true
			return nil
		},
	}

	h := NewNodesHandler(mc)
	drain := true
	resp, err := h.RemoveNode(context.Background(), &pb.RemoveNodeRequest{Name: "node-1", Drain: &drain})
	if err != nil {
		t.Fatal(err)
	}
	if !resp.Deleted {
		t.Error("expected deleted=true")
	}
	if !cordoned {
		t.Error("expected cordon to be called when drain=true")
	}
	if !podDeleted {
		t.Error("expected pod deletion when drain=true")
	}
	if !deleted {
		t.Error("expected deleteStaticInstance to be called")
	}
}

func TestRemoveNode_NoDrain(t *testing.T) {
	cordoned := false
	si := makeStaticInstance("node-1", "10.0.0.1", "Running", "workers")
	mc := &mockClient{
		getStaticInstanceFunc: func(_ context.Context, _ string) (*unstructured.Unstructured, error) {
			return &si, nil
		},
		getNodeFunc: func(_ context.Context, _ string) (*corev1.Node, error) {
			n := makeNode("node-1", true)
			return &n, nil
		},
		cordonNodeFunc: func(_ context.Context, _ string) error {
			cordoned = true
			return nil
		},
		deleteStaticInstanceFunc: func(_ context.Context, _ string) error {
			return nil
		},
	}

	h := NewNodesHandler(mc)
	drain := false
	_, err := h.RemoveNode(context.Background(), &pb.RemoveNodeRequest{Name: "node-1", Drain: &drain})
	if err != nil {
		t.Fatal(err)
	}
	if cordoned {
		t.Error("expected cordon NOT called when drain=false")
	}
}

func TestRemoveNode_NoStaticInstance(t *testing.T) {
	mc := &mockClient{
		getStaticInstanceFunc: func(_ context.Context, name string) (*unstructured.Unstructured, error) {
			return nil, fmt.Errorf("not found: %s", name)
		},
	}

	h := NewNodesHandler(mc)
	_, err := h.RemoveNode(context.Background(), &pb.RemoveNodeRequest{Name: "missing"})
	if err == nil {
		t.Fatal("expected error, got nil")
	}
}

func TestCreateNodeGroup_Success(t *testing.T) {
	mc := &mockClient{
		createNodeGroupFunc: func(_ context.Context, obj *unstructured.Unstructured) (*unstructured.Unstructured, error) {
			return obj, nil
		},
	}

	h := NewNodesHandler(mc)
	resp, err := h.CreateNodeGroup(context.Background(), &pb.CreateNodeGroupRequest{
		Name:     "workers",
		NodeType: "Static",
	})
	if err != nil {
		t.Fatal(err)
	}
	if resp.Name != "workers" {
		t.Errorf("expected workers, got %s", resp.Name)
	}
}

func TestCreateNodeGroup_AlreadyExists(t *testing.T) {
	mc := &mockClient{
		createNodeGroupFunc: func(_ context.Context, _ *unstructured.Unstructured) (*unstructured.Unstructured, error) {
			return nil, errors.NewAlreadyExists(schema.GroupResource{Group: "deckhouse.io", Resource: "nodegroups"}, "workers")
		},
	}

	h := NewNodesHandler(mc)
	_, err := h.CreateNodeGroup(context.Background(), &pb.CreateNodeGroupRequest{
		Name:     "workers",
		NodeType: "Static",
	})
	if err == nil {
		t.Fatal("expected error, got nil")
	}
}

func TestWaitNodeReady_Success(t *testing.T) {
	pollCount := 0
	mc := &mockClient{
		getStaticInstanceFunc: func(_ context.Context, _ string) (*unstructured.Unstructured, error) {
			pollCount++
			phase := "Bootstrapping"
			if pollCount >= 2 {
				phase = "Running"
			}
			obj := &unstructured.Unstructured{
				Object: map[string]interface{}{
					"status": map[string]interface{}{
						"currentStatus": map[string]interface{}{
							"phase": phase,
						},
					},
				},
			}
			return obj, nil
		},
	}

	h := NewNodesHandler(mc)
	timeout := int32(5)
	resp, err := h.WaitNodeReady(context.Background(), &pb.WaitNodeReadyRequest{Name: "node-1", TimeoutSeconds: &timeout})
	if err != nil {
		t.Fatal(err)
	}
	if resp.Phase != "Running" {
		t.Errorf("expected Running, got %s", resp.Phase)
	}
	if resp.TimedOut {
		t.Error("expected timedOut=false")
	}
}

func TestWaitNodeReady_Timeout(t *testing.T) {
	mc := &mockClient{
		getStaticInstanceFunc: func(_ context.Context, _ string) (*unstructured.Unstructured, error) {
			return &unstructured.Unstructured{
				Object: map[string]interface{}{
					"status": map[string]interface{}{
						"currentStatus": map[string]interface{}{
							"phase": "Bootstrapping",
						},
					},
				},
			}, nil
		},
	}

	h := NewNodesHandler(mc)
	timeout := int32(1)
	resp, err := h.WaitNodeReady(context.Background(), &pb.WaitNodeReadyRequest{Name: "node-1", TimeoutSeconds: &timeout})
	if err != nil {
		t.Fatal(err)
	}
	if !resp.TimedOut {
		t.Error("expected timedOut=true")
	}
	if resp.Phase != "Bootstrapping" {
		t.Errorf("expected Bootstrapping, got %s", resp.Phase)
	}
}
