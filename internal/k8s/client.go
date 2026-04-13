package k8s

import (
	"context"
	"fmt"

	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"
	"k8s.io/apimachinery/pkg/runtime/schema"
	"k8s.io/client-go/dynamic"
	"k8s.io/client-go/kubernetes"
	"k8s.io/client-go/rest"
)

// Client abstracts Kubernetes API operations for both core resources and Deckhouse CRDs.
type Client interface {
	// Core resources (typed).
	ListNodes(ctx context.Context) ([]corev1.Node, error)
	ListPods(ctx context.Context, namespace string) ([]corev1.Pod, error)

	// Deckhouse CRDs (dynamic/unstructured).
	ListNodeGroups(ctx context.Context) ([]unstructured.Unstructured, error)
	ListStaticInstances(ctx context.Context) ([]unstructured.Unstructured, error)
	GetStaticInstance(ctx context.Context, name string) (*unstructured.Unstructured, error)
	CreateStaticInstance(ctx context.Context, obj *unstructured.Unstructured) (*unstructured.Unstructured, error)
	ListModuleConfigs(ctx context.Context) ([]unstructured.Unstructured, error)
	ListDeckhouseReleases(ctx context.Context) ([]unstructured.Unstructured, error)
	CreateSSHCredentials(ctx context.Context, obj *unstructured.Unstructured) (*unstructured.Unstructured, error)
}

// GVR constants for Deckhouse CRDs.
var (
	NodeGroupGVR = schema.GroupVersionResource{
		Group:    "deckhouse.io",
		Version:  "v1",
		Resource: "nodegroups",
	}
	StaticInstanceGVR = schema.GroupVersionResource{
		Group:    "deckhouse.io",
		Version:  "v1alpha2",
		Resource: "staticinstances",
	}
	SSHCredentialsGVR = schema.GroupVersionResource{
		Group:    "deckhouse.io",
		Version:  "v1alpha2",
		Resource: "sshcredentials",
	}
	ModuleConfigGVR = schema.GroupVersionResource{
		Group:    "deckhouse.io",
		Version:  "v1alpha1",
		Resource: "moduleconfigs",
	}
	DeckhouseReleaseGVR = schema.GroupVersionResource{
		Group:    "deckhouse.io",
		Version:  "v1alpha1",
		Resource: "deckhouserelease",
	}
)

type client struct {
	typed   kubernetes.Interface
	dynamic dynamic.Interface
}

// New creates a new Client from the given rest.Config.
func New(cfg *rest.Config) (Client, error) {
	typedClient, err := kubernetes.NewForConfig(cfg)
	if err != nil {
		return nil, fmt.Errorf("creating typed client: %w", err)
	}

	dynamicClient, err := dynamic.NewForConfig(cfg)
	if err != nil {
		return nil, fmt.Errorf("creating dynamic client: %w", err)
	}

	return &client{
		typed:   typedClient,
		dynamic: dynamicClient,
	}, nil
}

// ListNodes returns all cluster nodes.
func (c *client) ListNodes(ctx context.Context) ([]corev1.Node, error) {
	list, err := c.typed.CoreV1().Nodes().List(ctx, metav1.ListOptions{})
	if err != nil {
		return nil, err
	}
	return list.Items, nil
}

// ListPods returns pods in the given namespace (empty string means all namespaces).
func (c *client) ListPods(ctx context.Context, namespace string) ([]corev1.Pod, error) {
	list, err := c.typed.CoreV1().Pods(namespace).List(ctx, metav1.ListOptions{})
	if err != nil {
		return nil, err
	}
	return list.Items, nil
}

// ListNodeGroups returns all NodeGroup resources.
func (c *client) ListNodeGroups(ctx context.Context) ([]unstructured.Unstructured, error) {
	list, err := c.dynamic.Resource(NodeGroupGVR).List(ctx, metav1.ListOptions{})
	if err != nil {
		return nil, err
	}
	return list.Items, nil
}

// ListStaticInstances returns all StaticInstance resources.
func (c *client) ListStaticInstances(ctx context.Context) ([]unstructured.Unstructured, error) {
	list, err := c.dynamic.Resource(StaticInstanceGVR).List(ctx, metav1.ListOptions{})
	if err != nil {
		return nil, err
	}
	return list.Items, nil
}

// GetStaticInstance returns a single StaticInstance by name.
func (c *client) GetStaticInstance(ctx context.Context, name string) (*unstructured.Unstructured, error) {
	return c.dynamic.Resource(StaticInstanceGVR).Get(ctx, name, metav1.GetOptions{})
}

// CreateStaticInstance creates a StaticInstance resource.
func (c *client) CreateStaticInstance(ctx context.Context, obj *unstructured.Unstructured) (*unstructured.Unstructured, error) {
	return c.dynamic.Resource(StaticInstanceGVR).Create(ctx, obj, metav1.CreateOptions{})
}

// ListModuleConfigs returns all ModuleConfig resources.
func (c *client) ListModuleConfigs(ctx context.Context) ([]unstructured.Unstructured, error) {
	list, err := c.dynamic.Resource(ModuleConfigGVR).List(ctx, metav1.ListOptions{})
	if err != nil {
		return nil, err
	}
	return list.Items, nil
}

// ListDeckhouseReleases returns all DeckhouseRelease resources.
func (c *client) ListDeckhouseReleases(ctx context.Context) ([]unstructured.Unstructured, error) {
	list, err := c.dynamic.Resource(DeckhouseReleaseGVR).List(ctx, metav1.ListOptions{})
	if err != nil {
		return nil, err
	}
	return list.Items, nil
}

// CreateSSHCredentials creates an SSHCredentials resource.
func (c *client) CreateSSHCredentials(ctx context.Context, obj *unstructured.Unstructured) (*unstructured.Unstructured, error) {
	return c.dynamic.Resource(SSHCredentialsGVR).Create(ctx, obj, metav1.CreateOptions{})
}
