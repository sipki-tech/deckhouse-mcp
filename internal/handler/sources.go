// Package handler — SourcesAPI handlers for Deckhouse ModuleSource and
// ModuleUpdatePolicy CRDs.
package handler

import (
	"context"
	"fmt"

	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"

	"github.com/sipki-tech/deckhouse-mcp/internal/k8s"
	pb "github.com/sipki-tech/deckhouse-mcp/proto/deckhouse/v1"
)

const deckhouseAPIVersionV1Alpha1 = "deckhouse.io/v1alpha1"

// SourcesHandler implements pb.SourcesAPIToolHandler.
type SourcesHandler struct {
	client k8s.Client
}

// NewSourcesHandler creates a new SourcesHandler.
func NewSourcesHandler(client k8s.Client) *SourcesHandler {
	return &SourcesHandler{client: client}
}

// ListModuleSources returns all ModuleSource resources, projecting each into
// pb.ModuleSourceInfo (name + spec.registry.repo + status.message).
func (h *SourcesHandler) ListModuleSources(
	ctx context.Context,
	_ *pb.ListModuleSourcesRequest,
) (*pb.ListModuleSourcesResponse, error) {
	items, err := h.client.ListModuleSources(ctx)
	if err != nil {
		return nil, fmt.Errorf("listing module sources: %w", err)
	}

	sources := make([]*pb.ModuleSourceInfo, 0, len(items))
	for i := range items {
		obj := &items[i]
		sources = append(sources, &pb.ModuleSourceInfo{
			Name:     obj.GetName(),
			Registry: extractModuleSourceRegistry(obj),
			Status:   extractModuleSourceStatus(obj),
		})
	}

	return &pb.ListModuleSourcesResponse{Sources: sources}, nil
}

// CreateModuleSource creates a new ModuleSource resource. The request name and
// registry are mapped to metadata.name and spec.registry.repo respectively.
func (h *SourcesHandler) CreateModuleSource(
	ctx context.Context,
	req *pb.CreateModuleSourceRequest,
) (*pb.CreateModuleSourceResponse, error) {
	obj := &unstructured.Unstructured{Object: map[string]any{
		"apiVersion": deckhouseAPIVersionV1Alpha1,
		"kind":       "ModuleSource",
		"metadata": map[string]any{
			"name": req.GetName(),
		},
		"spec": map[string]any{
			"registry": map[string]any{
				"repo": req.GetRegistry(),
			},
		},
	}}

	_, err := h.client.CreateModuleSource(ctx, obj)
	if err != nil {
		return nil, fmt.Errorf("creating module source %s: %w", req.GetName(), err)
	}

	return &pb.CreateModuleSourceResponse{
		Created: true,
		Name:    req.GetName(),
	}, nil
}

// ListModuleUpdatePolicies returns all ModuleUpdatePolicy resources, projecting
// each into pb.ModuleUpdatePolicyInfo (name + spec.update.mode).
func (h *SourcesHandler) ListModuleUpdatePolicies(
	ctx context.Context,
	_ *pb.ListModuleUpdatePoliciesRequest,
) (*pb.ListModuleUpdatePoliciesResponse, error) {
	items, err := h.client.ListModuleUpdatePolicies(ctx)
	if err != nil {
		return nil, fmt.Errorf("listing module update policies: %w", err)
	}

	policies := make([]*pb.ModuleUpdatePolicyInfo, 0, len(items))
	for i := range items {
		obj := &items[i]
		policies = append(policies, &pb.ModuleUpdatePolicyInfo{
			Name:       obj.GetName(),
			UpdateMode: extractUpdatePolicyMode(obj),
		})
	}

	return &pb.ListModuleUpdatePoliciesResponse{Policies: policies}, nil
}

// CreateModuleUpdatePolicy creates a new ModuleUpdatePolicy resource. The
// request name and updateMode are mapped to metadata.name and spec.update.mode.
func (h *SourcesHandler) CreateModuleUpdatePolicy(
	ctx context.Context,
	req *pb.CreateModuleUpdatePolicyRequest,
) (*pb.CreateModuleUpdatePolicyResponse, error) {
	obj := &unstructured.Unstructured{Object: map[string]any{
		"apiVersion": deckhouseAPIVersionV1Alpha1,
		"kind":       "ModuleUpdatePolicy",
		"metadata": map[string]any{
			"name": req.GetName(),
		},
		"spec": map[string]any{
			"update": map[string]any{
				"mode": req.GetUpdateMode(),
			},
		},
	}}

	_, err := h.client.CreateModuleUpdatePolicy(ctx, obj)
	if err != nil {
		return nil, fmt.Errorf("creating module update policy %s: %w", req.GetName(), err)
	}

	return &pb.CreateModuleUpdatePolicyResponse{
		Created: true,
		Name:    req.GetName(),
	}, nil
}

// extractModuleSourceRegistry reads spec.registry.repo as a string. Returns
// empty string if the path is absent or not a string.
func extractModuleSourceRegistry(obj *unstructured.Unstructured) string {
	spec, ok := obj.Object["spec"].(map[string]any)
	if !ok {
		return ""
	}

	registry, ok := spec["registry"].(map[string]any)
	if !ok {
		return ""
	}

	repo, _ := registry["repo"].(string)

	return repo
}

// extractModuleSourceStatus reads status.message as a string. Returns empty
// string if the path is absent or not a string.
func extractModuleSourceStatus(obj *unstructured.Unstructured) string {
	status, ok := obj.Object["status"].(map[string]any)
	if !ok {
		return ""
	}

	msg, _ := status["message"].(string)

	return msg
}

// extractUpdatePolicyMode reads spec.update.mode as a string. Returns empty
// string if the path is absent or not a string.
func extractUpdatePolicyMode(obj *unstructured.Unstructured) string {
	spec, ok := obj.Object["spec"].(map[string]any)
	if !ok {
		return ""
	}

	update, ok := spec["update"].(map[string]any)
	if !ok {
		return ""
	}

	mode, _ := update["mode"].(string)

	return mode
}
