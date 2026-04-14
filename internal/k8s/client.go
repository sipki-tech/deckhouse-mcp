package k8s

import (
	"bytes"
	"context"
	"fmt"
	"io"
	"time"

	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"
	"k8s.io/apimachinery/pkg/runtime/schema"
	"k8s.io/apimachinery/pkg/types"
	"k8s.io/client-go/dynamic"
	"k8s.io/client-go/kubernetes"
	"k8s.io/client-go/rest"
)

// Client abstracts Kubernetes API operations for both core resources and Deckhouse CRDs.
type Client interface {
	// Core resources (typed).
	ListNodes(ctx context.Context) ([]corev1.Node, error)
	GetNode(ctx context.Context, name string) (*corev1.Node, error)
	CordonNode(ctx context.Context, name string) error
	ListPods(ctx context.Context, namespace string) ([]corev1.Pod, error)
	DeletePod(ctx context.Context, namespace, name string) error
	ListNodeEvents(ctx context.Context, nodeName string) ([]corev1.Event, error)
	GetPodLogs(ctx context.Context, namespace, pod, container string, tail *int64, since *string) (string, error)
	GetSecret(ctx context.Context, namespace, name string) (*corev1.Secret, error)

	// Deckhouse CRDs (dynamic/unstructured).
	ListNodeGroups(ctx context.Context) ([]unstructured.Unstructured, error)
	GetNodeGroup(ctx context.Context, name string) (*unstructured.Unstructured, error)
	CreateNodeGroup(ctx context.Context, obj *unstructured.Unstructured) (*unstructured.Unstructured, error)
	ListStaticInstances(ctx context.Context) ([]unstructured.Unstructured, error)
	GetStaticInstance(ctx context.Context, name string) (*unstructured.Unstructured, error)
	CreateStaticInstance(ctx context.Context, obj *unstructured.Unstructured) (*unstructured.Unstructured, error)
	DeleteStaticInstance(ctx context.Context, name string) error
	ListModuleConfigs(ctx context.Context) ([]unstructured.Unstructured, error)
	GetModuleConfig(ctx context.Context, name string) (*unstructured.Unstructured, error)
	UpdateModuleConfig(ctx context.Context, obj *unstructured.Unstructured) (*unstructured.Unstructured, error)
	ListDeckhouseReleases(ctx context.Context) ([]unstructured.Unstructured, error)
	GetDeckhouseRelease(ctx context.Context, name string) (*unstructured.Unstructured, error)
	PatchDeckhouseRelease(ctx context.Context, name string, patch []byte) (*unstructured.Unstructured, error)
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

// GetNode returns a single Node by name.
func (c *client) GetNode(ctx context.Context, name string) (*corev1.Node, error) {
	return c.typed.CoreV1().Nodes().Get(ctx, name, metav1.GetOptions{})
}

// CordonNode marks a node as unschedulable.
func (c *client) CordonNode(ctx context.Context, name string) error {
	node, err := c.typed.CoreV1().Nodes().Get(ctx, name, metav1.GetOptions{})
	if err != nil {
		return err
	}
	node.Spec.Unschedulable = true
	_, err = c.typed.CoreV1().Nodes().Update(ctx, node, metav1.UpdateOptions{})
	return err
}

// GetPodLogs returns logs for a specific pod. container selects the container (empty = default). tail limits lines; since limits by duration (e.g. "30m").
func (c *client) GetPodLogs(ctx context.Context, namespace, pod, container string, tail *int64, since *string) (string, error) {
	opts := &corev1.PodLogOptions{Container: container}
	if tail != nil {
		opts.TailLines = tail
	}
	if since != nil {
		d, err := time.ParseDuration(*since)
		if err != nil {
			return "", fmt.Errorf("parsing since duration %q: %w", *since, err)
		}
		secs := int64(d.Seconds())
		opts.SinceSeconds = &secs
	}
	req := c.typed.CoreV1().Pods(namespace).GetLogs(pod, opts)
	rc, err := req.Stream(ctx)
	if err != nil {
		return "", err
	}
	defer rc.Close()
	var buf bytes.Buffer
	if _, err := io.Copy(&buf, rc); err != nil {
		return "", err
	}
	return buf.String(), nil
}

// DeletePod deletes a single pod by namespace and name.
func (c *client) DeletePod(ctx context.Context, namespace, name string) error {
	return c.typed.CoreV1().Pods(namespace).Delete(ctx, name, metav1.DeleteOptions{})
}

// ListNodeEvents returns the last 10 events for a node (involvedObject.name=<nodeName>).
func (c *client) ListNodeEvents(ctx context.Context, nodeName string) ([]corev1.Event, error) {
	list, err := c.typed.CoreV1().Events("").List(ctx, metav1.ListOptions{
		FieldSelector: "involvedObject.name=" + nodeName,
		Limit:         10,
	})
	if err != nil {
		return nil, err
	}
	return list.Items, nil
}

// GetSecret returns a single Secret by namespace and name.
func (c *client) GetSecret(ctx context.Context, namespace, name string) (*corev1.Secret, error) {
	return c.typed.CoreV1().Secrets(namespace).Get(ctx, name, metav1.GetOptions{})
}

// GetNodeGroup returns a single NodeGroup by name.
func (c *client) GetNodeGroup(ctx context.Context, name string) (*unstructured.Unstructured, error) {
	return c.dynamic.Resource(NodeGroupGVR).Get(ctx, name, metav1.GetOptions{})
}

// CreateNodeGroup creates a NodeGroup resource.
func (c *client) CreateNodeGroup(ctx context.Context, obj *unstructured.Unstructured) (*unstructured.Unstructured, error) {
	return c.dynamic.Resource(NodeGroupGVR).Create(ctx, obj, metav1.CreateOptions{})
}

// DeleteStaticInstance deletes a StaticInstance by name.
func (c *client) DeleteStaticInstance(ctx context.Context, name string) error {
	return c.dynamic.Resource(StaticInstanceGVR).Delete(ctx, name, metav1.DeleteOptions{})
}

// GetModuleConfig returns a single ModuleConfig by name.
func (c *client) GetModuleConfig(ctx context.Context, name string) (*unstructured.Unstructured, error) {
	return c.dynamic.Resource(ModuleConfigGVR).Get(ctx, name, metav1.GetOptions{})
}

// UpdateModuleConfig replaces a ModuleConfig resource.
func (c *client) UpdateModuleConfig(ctx context.Context, obj *unstructured.Unstructured) (*unstructured.Unstructured, error) {
	return c.dynamic.Resource(ModuleConfigGVR).Update(ctx, obj, metav1.UpdateOptions{})
}

// GetDeckhouseRelease returns a single DeckhouseRelease by name.
func (c *client) GetDeckhouseRelease(ctx context.Context, name string) (*unstructured.Unstructured, error) {
	return c.dynamic.Resource(DeckhouseReleaseGVR).Get(ctx, name, metav1.GetOptions{})
}

// PatchDeckhouseRelease applies a merge patch to a DeckhouseRelease.
func (c *client) PatchDeckhouseRelease(ctx context.Context, name string, patch []byte) (*unstructured.Unstructured, error) {
	return c.dynamic.Resource(DeckhouseReleaseGVR).Patch(ctx, name, types.MergePatchType, patch, metav1.PatchOptions{})
}
