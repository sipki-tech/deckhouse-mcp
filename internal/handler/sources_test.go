package handler

import (
	"context"
	"testing"

	"k8s.io/apimachinery/pkg/api/errors"
	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"
	"k8s.io/apimachinery/pkg/runtime/schema"

	pb "github.com/sipki-tech/deckhouse-mcp/proto/deckhouse/v1"
)

// makeModuleSource builds a synthetic ModuleSource fixture.
func makeModuleSource(name, registry, statusMessage string) unstructured.Unstructured {
	obj := unstructured.Unstructured{Object: map[string]any{
		"apiVersion": "deckhouse.io/v1alpha1",
		"kind":       "ModuleSource",
		"metadata":   map[string]any{"name": name},
		"spec": map[string]any{
			"registry": map[string]any{"repo": registry},
		},
		"status": map[string]any{"message": statusMessage},
	}}

	return obj
}

// makeModuleUpdatePolicy builds a synthetic ModuleUpdatePolicy fixture.
func makeModuleUpdatePolicy(name, mode string) unstructured.Unstructured {
	return unstructured.Unstructured{Object: map[string]any{
		"apiVersion": "deckhouse.io/v1alpha1",
		"kind":       "ModuleUpdatePolicy",
		"metadata":   map[string]any{"name": name},
		"spec": map[string]any{
			"update": map[string]any{"mode": mode},
		},
	}}
}

func TestListModuleSources_Empty(t *testing.T) {
	mc := &mockClient{
		listModuleSourcesFunc: func(_ context.Context) ([]unstructured.Unstructured, error) {
			return nil, nil
		},
	}

	h := NewSourcesHandler(mc)
	resp, err := h.ListModuleSources(context.Background(), &pb.ListModuleSourcesRequest{})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if resp.GetSources() == nil {
		t.Error("expected non-nil empty slice, got nil")
	}
	if len(resp.GetSources()) != 0 {
		t.Errorf("expected empty list, got %d sources", len(resp.GetSources()))
	}
}

func TestListModuleSources_Happy(t *testing.T) {
	mc := &mockClient{
		listModuleSourcesFunc: func(_ context.Context) ([]unstructured.Unstructured, error) {
			return []unstructured.Unstructured{
				makeModuleSource("deckhouse", "registry.deckhouse.io/deckhouse/ce/modules", "Synced"),
				makeModuleSource("custom", "registry.example.com/modules", ""),
			}, nil
		},
	}

	h := NewSourcesHandler(mc)
	resp, err := h.ListModuleSources(context.Background(), &pb.ListModuleSourcesRequest{})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if len(resp.GetSources()) != 2 {
		t.Fatalf("expected 2 sources, got %d", len(resp.GetSources()))
	}

	first := resp.GetSources()[0]
	if first.GetName() != "deckhouse" {
		t.Errorf("expected name=deckhouse, got %q", first.GetName())
	}
	if first.GetRegistry() != "registry.deckhouse.io/deckhouse/ce/modules" {
		t.Errorf("expected registry from spec.registry.repo, got %q", first.GetRegistry())
	}
	if first.GetStatus() != "Synced" {
		t.Errorf("expected status=Synced, got %q", first.GetStatus())
	}
}

func TestCreateModuleSource_Happy(t *testing.T) {
	var captured *unstructured.Unstructured
	mc := &mockClient{
		createModuleSourceFunc: func(_ context.Context, obj *unstructured.Unstructured) (*unstructured.Unstructured, error) {
			captured = obj

			return obj, nil
		},
	}

	h := NewSourcesHandler(mc)
	resp, err := h.CreateModuleSource(context.Background(), &pb.CreateModuleSourceRequest{
		Name:     "custom-modules",
		Registry: "registry.example.com/modules",
	})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if !resp.GetCreated() {
		t.Error("expected created=true")
	}
	if resp.GetName() != "custom-modules" {
		t.Errorf("expected echoed name=custom-modules, got %q", resp.GetName())
	}

	if captured == nil {
		t.Fatal("expected CreateModuleSource to be called")
	}
	if captured.GetKind() != "ModuleSource" {
		t.Errorf("expected kind=ModuleSource, got %q", captured.GetKind())
	}
	if captured.GetAPIVersion() != "deckhouse.io/v1alpha1" {
		t.Errorf("expected apiVersion=deckhouse.io/v1alpha1, got %q", captured.GetAPIVersion())
	}
	if captured.GetName() != "custom-modules" {
		t.Errorf("expected metadata.name=custom-modules, got %q", captured.GetName())
	}

	spec, ok := captured.Object["spec"].(map[string]any)
	if !ok {
		t.Fatalf("expected spec object, got %#v", captured.Object["spec"])
	}
	registry, ok := spec["registry"].(map[string]any)
	if !ok {
		t.Fatalf("expected spec.registry object, got %#v", spec["registry"])
	}
	if got := registry["repo"]; got != "registry.example.com/modules" {
		t.Errorf("expected spec.registry.repo=registry.example.com/modules, got %v", got)
	}
}

func TestCreateModuleSource_AlreadyExists(t *testing.T) {
	mc := &mockClient{
		createModuleSourceFunc: func(_ context.Context, obj *unstructured.Unstructured) (*unstructured.Unstructured, error) {
			return nil, errors.NewAlreadyExists(
				schema.GroupResource{Group: "deckhouse.io", Resource: "modulesources"},
				obj.GetName(),
			)
		},
	}

	h := NewSourcesHandler(mc)
	_, err := h.CreateModuleSource(context.Background(), &pb.CreateModuleSourceRequest{
		Name:     "duplicate",
		Registry: "registry.example.com/modules",
	})
	if err == nil {
		t.Fatal("expected already-exists error, got nil")
	}
}

func TestListModuleUpdatePolicies_Empty(t *testing.T) {
	mc := &mockClient{
		listModuleUpdatePoliciesFunc: func(_ context.Context) ([]unstructured.Unstructured, error) {
			return nil, nil
		},
	}

	h := NewSourcesHandler(mc)
	resp, err := h.ListModuleUpdatePolicies(context.Background(), &pb.ListModuleUpdatePoliciesRequest{})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if resp.GetPolicies() == nil {
		t.Error("expected non-nil empty slice, got nil")
	}
	if len(resp.GetPolicies()) != 0 {
		t.Errorf("expected empty list, got %d policies", len(resp.GetPolicies()))
	}
}

func TestListModuleUpdatePolicies_Happy(t *testing.T) {
	mc := &mockClient{
		listModuleUpdatePoliciesFunc: func(_ context.Context) ([]unstructured.Unstructured, error) {
			return []unstructured.Unstructured{
				makeModuleUpdatePolicy("auto", "Auto"),
				makeModuleUpdatePolicy("manual", "Manual"),
			}, nil
		},
	}

	h := NewSourcesHandler(mc)
	resp, err := h.ListModuleUpdatePolicies(context.Background(), &pb.ListModuleUpdatePoliciesRequest{})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if len(resp.GetPolicies()) != 2 {
		t.Fatalf("expected 2 policies, got %d", len(resp.GetPolicies()))
	}

	first := resp.GetPolicies()[0]
	if first.GetName() != "auto" {
		t.Errorf("expected name=auto, got %q", first.GetName())
	}
	if first.GetUpdateMode() != "Auto" {
		t.Errorf("expected updateMode=Auto, got %q", first.GetUpdateMode())
	}
}

func TestCreateModuleUpdatePolicy_Happy(t *testing.T) {
	var captured *unstructured.Unstructured
	mc := &mockClient{
		createModuleUpdatePolicyFunc: func(_ context.Context, obj *unstructured.Unstructured) (*unstructured.Unstructured, error) {
			captured = obj

			return obj, nil
		},
	}

	h := NewSourcesHandler(mc)
	resp, err := h.CreateModuleUpdatePolicy(context.Background(), &pb.CreateModuleUpdatePolicyRequest{
		Name:       "auto",
		UpdateMode: "Auto",
	})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if !resp.GetCreated() {
		t.Error("expected created=true")
	}
	if resp.GetName() != "auto" {
		t.Errorf("expected echoed name=auto, got %q", resp.GetName())
	}

	if captured == nil {
		t.Fatal("expected CreateModuleUpdatePolicy to be called")
	}
	if captured.GetKind() != "ModuleUpdatePolicy" {
		t.Errorf("expected kind=ModuleUpdatePolicy, got %q", captured.GetKind())
	}
	if captured.GetAPIVersion() != "deckhouse.io/v1alpha1" {
		t.Errorf("expected apiVersion=deckhouse.io/v1alpha1, got %q", captured.GetAPIVersion())
	}
	if captured.GetName() != "auto" {
		t.Errorf("expected metadata.name=auto, got %q", captured.GetName())
	}

	spec, ok := captured.Object["spec"].(map[string]any)
	if !ok {
		t.Fatalf("expected spec object, got %#v", captured.Object["spec"])
	}
	update, ok := spec["update"].(map[string]any)
	if !ok {
		t.Fatalf("expected spec.update object, got %#v", spec["update"])
	}
	if got := update["mode"]; got != "Auto" {
		t.Errorf("expected spec.update.mode=Auto, got %v", got)
	}
}

func TestCreateModuleUpdatePolicy_AlreadyExists(t *testing.T) {
	mc := &mockClient{
		createModuleUpdatePolicyFunc: func(_ context.Context, obj *unstructured.Unstructured) (*unstructured.Unstructured, error) {
			return nil, errors.NewAlreadyExists(
				schema.GroupResource{Group: "deckhouse.io", Resource: "moduleupdatepolicies"},
				obj.GetName(),
			)
		},
	}

	h := NewSourcesHandler(mc)
	_, err := h.CreateModuleUpdatePolicy(context.Background(), &pb.CreateModuleUpdatePolicyRequest{
		Name:       "duplicate",
		UpdateMode: "Auto",
	})
	if err == nil {
		t.Fatal("expected already-exists error, got nil")
	}
}
