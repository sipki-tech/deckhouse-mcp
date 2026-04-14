package handler

import (
	"context"

	corev1 "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"

	"github.com/sipki-tech/deckhouse-mcp/internal/k8s"
)

// Verify mockClient implements k8s.Client.
var _ k8s.Client = (*mockClient)(nil)

// mockClient is a test double for k8s.Client.
type mockClient struct {
	listNodesFunc             func(ctx context.Context) ([]corev1.Node, error)
	getNodeFunc               func(ctx context.Context, name string) (*corev1.Node, error)
	cordonNodeFunc            func(ctx context.Context, name string) error
	listPodsFunc              func(ctx context.Context, namespace string) ([]corev1.Pod, error)
	deletePodFunc             func(ctx context.Context, namespace, name string) error
	listNodeEventsFunc        func(ctx context.Context, nodeName string) ([]corev1.Event, error)
	getPodLogsFunc            func(ctx context.Context, namespace, pod, container string, tail *int64, since *string) (string, error)
	getSecretFunc             func(ctx context.Context, namespace, name string) (*corev1.Secret, error)
	listNodeGroupsFunc        func(ctx context.Context) ([]unstructured.Unstructured, error)
	getNodeGroupFunc          func(ctx context.Context, name string) (*unstructured.Unstructured, error)
	createNodeGroupFunc       func(ctx context.Context, obj *unstructured.Unstructured) (*unstructured.Unstructured, error)
	listStaticInstancesFunc   func(ctx context.Context) ([]unstructured.Unstructured, error)
	getStaticInstanceFunc     func(ctx context.Context, name string) (*unstructured.Unstructured, error)
	createStaticInstanceFunc  func(ctx context.Context, obj *unstructured.Unstructured) (*unstructured.Unstructured, error)
	deleteStaticInstanceFunc  func(ctx context.Context, name string) error
	listModuleConfigsFunc     func(ctx context.Context) ([]unstructured.Unstructured, error)
	getModuleConfigFunc       func(ctx context.Context, name string) (*unstructured.Unstructured, error)
	updateModuleConfigFunc    func(ctx context.Context, obj *unstructured.Unstructured) (*unstructured.Unstructured, error)
	listDeckhouseReleasesFunc func(ctx context.Context) ([]unstructured.Unstructured, error)
	getDeckhouseReleaseFunc   func(ctx context.Context, name string) (*unstructured.Unstructured, error)
	patchDeckhouseReleaseFunc func(ctx context.Context, name string, patch []byte) (*unstructured.Unstructured, error)
	createSSHCredentialsFunc  func(ctx context.Context, obj *unstructured.Unstructured) (*unstructured.Unstructured, error)
}

func (m *mockClient) ListNodes(ctx context.Context) ([]corev1.Node, error) {
	if m.listNodesFunc != nil {
		return m.listNodesFunc(ctx)
	}
	return nil, nil
}

func (m *mockClient) GetNode(ctx context.Context, name string) (*corev1.Node, error) {
	if m.getNodeFunc != nil {
		return m.getNodeFunc(ctx, name)
	}
	return nil, nil
}

func (m *mockClient) CordonNode(ctx context.Context, name string) error {
	if m.cordonNodeFunc != nil {
		return m.cordonNodeFunc(ctx, name)
	}
	return nil
}

func (m *mockClient) ListPods(ctx context.Context, namespace string) ([]corev1.Pod, error) {
	if m.listPodsFunc != nil {
		return m.listPodsFunc(ctx, namespace)
	}
	return nil, nil
}

func (m *mockClient) DeletePod(ctx context.Context, namespace, name string) error {
	if m.deletePodFunc != nil {
		return m.deletePodFunc(ctx, namespace, name)
	}
	return nil
}

func (m *mockClient) ListNodeEvents(ctx context.Context, nodeName string) ([]corev1.Event, error) {
	if m.listNodeEventsFunc != nil {
		return m.listNodeEventsFunc(ctx, nodeName)
	}
	return nil, nil
}

func (m *mockClient) GetPodLogs(ctx context.Context, namespace, pod, container string, tail *int64, since *string) (string, error) {
	if m.getPodLogsFunc != nil {
		return m.getPodLogsFunc(ctx, namespace, pod, container, tail, since)
	}
	return "", nil
}

func (m *mockClient) GetSecret(ctx context.Context, namespace, name string) (*corev1.Secret, error) {
	if m.getSecretFunc != nil {
		return m.getSecretFunc(ctx, namespace, name)
	}
	return nil, nil
}

func (m *mockClient) ListNodeGroups(ctx context.Context) ([]unstructured.Unstructured, error) {
	if m.listNodeGroupsFunc != nil {
		return m.listNodeGroupsFunc(ctx)
	}
	return nil, nil
}

func (m *mockClient) GetNodeGroup(ctx context.Context, name string) (*unstructured.Unstructured, error) {
	if m.getNodeGroupFunc != nil {
		return m.getNodeGroupFunc(ctx, name)
	}
	return nil, nil
}

func (m *mockClient) CreateNodeGroup(ctx context.Context, obj *unstructured.Unstructured) (*unstructured.Unstructured, error) {
	if m.createNodeGroupFunc != nil {
		return m.createNodeGroupFunc(ctx, obj)
	}
	return obj, nil
}

func (m *mockClient) ListStaticInstances(ctx context.Context) ([]unstructured.Unstructured, error) {
	if m.listStaticInstancesFunc != nil {
		return m.listStaticInstancesFunc(ctx)
	}
	return nil, nil
}

func (m *mockClient) GetStaticInstance(ctx context.Context, name string) (*unstructured.Unstructured, error) {
	if m.getStaticInstanceFunc != nil {
		return m.getStaticInstanceFunc(ctx, name)
	}
	return nil, nil
}

func (m *mockClient) CreateStaticInstance(ctx context.Context, obj *unstructured.Unstructured) (*unstructured.Unstructured, error) {
	if m.createStaticInstanceFunc != nil {
		return m.createStaticInstanceFunc(ctx, obj)
	}
	return obj, nil
}

func (m *mockClient) DeleteStaticInstance(ctx context.Context, name string) error {
	if m.deleteStaticInstanceFunc != nil {
		return m.deleteStaticInstanceFunc(ctx, name)
	}
	return nil
}

func (m *mockClient) ListModuleConfigs(ctx context.Context) ([]unstructured.Unstructured, error) {
	if m.listModuleConfigsFunc != nil {
		return m.listModuleConfigsFunc(ctx)
	}
	return nil, nil
}

func (m *mockClient) GetModuleConfig(ctx context.Context, name string) (*unstructured.Unstructured, error) {
	if m.getModuleConfigFunc != nil {
		return m.getModuleConfigFunc(ctx, name)
	}
	return nil, nil
}

func (m *mockClient) UpdateModuleConfig(ctx context.Context, obj *unstructured.Unstructured) (*unstructured.Unstructured, error) {
	if m.updateModuleConfigFunc != nil {
		return m.updateModuleConfigFunc(ctx, obj)
	}
	return obj, nil
}

func (m *mockClient) ListDeckhouseReleases(ctx context.Context) ([]unstructured.Unstructured, error) {
	if m.listDeckhouseReleasesFunc != nil {
		return m.listDeckhouseReleasesFunc(ctx)
	}
	return nil, nil
}

func (m *mockClient) GetDeckhouseRelease(ctx context.Context, name string) (*unstructured.Unstructured, error) {
	if m.getDeckhouseReleaseFunc != nil {
		return m.getDeckhouseReleaseFunc(ctx, name)
	}
	return nil, nil
}

func (m *mockClient) PatchDeckhouseRelease(ctx context.Context, name string, patch []byte) (*unstructured.Unstructured, error) {
	if m.patchDeckhouseReleaseFunc != nil {
		return m.patchDeckhouseReleaseFunc(ctx, name, patch)
	}
	return nil, nil
}

func (m *mockClient) CreateSSHCredentials(ctx context.Context, obj *unstructured.Unstructured) (*unstructured.Unstructured, error) {
	if m.createSSHCredentialsFunc != nil {
		return m.createSSHCredentialsFunc(ctx, obj)
	}
	return obj, nil
}
