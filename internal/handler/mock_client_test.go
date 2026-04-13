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
	listPodsFunc              func(ctx context.Context, namespace string) ([]corev1.Pod, error)
	listNodeGroupsFunc        func(ctx context.Context) ([]unstructured.Unstructured, error)
	listStaticInstancesFunc   func(ctx context.Context) ([]unstructured.Unstructured, error)
	getStaticInstanceFunc     func(ctx context.Context, name string) (*unstructured.Unstructured, error)
	createStaticInstanceFunc  func(ctx context.Context, obj *unstructured.Unstructured) (*unstructured.Unstructured, error)
	listModuleConfigsFunc     func(ctx context.Context) ([]unstructured.Unstructured, error)
	listDeckhouseReleasesFunc func(ctx context.Context) ([]unstructured.Unstructured, error)
	createSSHCredentialsFunc  func(ctx context.Context, obj *unstructured.Unstructured) (*unstructured.Unstructured, error)
}

func (m *mockClient) ListNodes(ctx context.Context) ([]corev1.Node, error) {
	if m.listNodesFunc != nil {
		return m.listNodesFunc(ctx)
	}
	return nil, nil
}

func (m *mockClient) ListPods(ctx context.Context, namespace string) ([]corev1.Pod, error) {
	if m.listPodsFunc != nil {
		return m.listPodsFunc(ctx, namespace)
	}
	return nil, nil
}

func (m *mockClient) ListNodeGroups(ctx context.Context) ([]unstructured.Unstructured, error) {
	if m.listNodeGroupsFunc != nil {
		return m.listNodeGroupsFunc(ctx)
	}
	return nil, nil
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

func (m *mockClient) ListModuleConfigs(ctx context.Context) ([]unstructured.Unstructured, error) {
	if m.listModuleConfigsFunc != nil {
		return m.listModuleConfigsFunc(ctx)
	}
	return nil, nil
}

func (m *mockClient) ListDeckhouseReleases(ctx context.Context) ([]unstructured.Unstructured, error) {
	if m.listDeckhouseReleasesFunc != nil {
		return m.listDeckhouseReleasesFunc(ctx)
	}
	return nil, nil
}

func (m *mockClient) CreateSSHCredentials(ctx context.Context, obj *unstructured.Unstructured) (*unstructured.Unstructured, error) {
	if m.createSSHCredentialsFunc != nil {
		return m.createSSHCredentialsFunc(ctx, obj)
	}
	return obj, nil
}
