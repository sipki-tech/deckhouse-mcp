package handler

import (
	"context"
	"fmt"
	"testing"

	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"

	pb "github.com/sipki-tech/deckhouse-mcp/proto/deckhouse/v1"
)

func TestListModuleConfigs_All(t *testing.T) {
	mc := &mockClient{
		listModuleConfigsFunc: func(_ context.Context) ([]unstructured.Unstructured, error) {
			return []unstructured.Unstructured{
				makeModuleConfig("mod-a", true, ""),
				makeModuleConfig("mod-b", false, ""),
			}, nil
		},
	}

	h := NewModulesHandler(mc)
	resp, err := h.ListModuleConfigs(context.Background(), &pb.ListModuleConfigsRequest{})
	if err != nil {
		t.Fatal(err)
	}
	if len(resp.Modules) != 2 {
		t.Errorf("expected 2, got %d", len(resp.Modules))
	}
}

func TestListModuleConfigs_Enabled(t *testing.T) {
	mc := &mockClient{
		listModuleConfigsFunc: func(_ context.Context) ([]unstructured.Unstructured, error) {
			return []unstructured.Unstructured{
				makeModuleConfig("mod-a", true, ""),
				makeModuleConfig("mod-b", false, ""),
				makeModuleConfig("mod-c", true, ""),
			}, nil
		},
	}

	h := NewModulesHandler(mc)
	enabled := true
	resp, err := h.ListModuleConfigs(context.Background(), &pb.ListModuleConfigsRequest{Enabled: &enabled})
	if err != nil {
		t.Fatal(err)
	}
	if len(resp.Modules) != 2 {
		t.Errorf("expected 2, got %d", len(resp.Modules))
	}
	for _, m := range resp.Modules {
		if !m.Enabled {
			t.Errorf("expected enabled=true for %s", m.Name)
		}
	}
}

func TestListModuleConfigs_Disabled(t *testing.T) {
	mc := &mockClient{
		listModuleConfigsFunc: func(_ context.Context) ([]unstructured.Unstructured, error) {
			return []unstructured.Unstructured{
				makeModuleConfig("mod-a", true, ""),
				makeModuleConfig("mod-b", false, ""),
			}, nil
		},
	}

	h := NewModulesHandler(mc)
	enabled := false
	resp, err := h.ListModuleConfigs(context.Background(), &pb.ListModuleConfigsRequest{Enabled: &enabled})
	if err != nil {
		t.Fatal(err)
	}
	if len(resp.Modules) != 1 {
		t.Errorf("expected 1, got %d", len(resp.Modules))
	}
	if resp.Modules[0].Name != "mod-b" {
		t.Errorf("expected mod-b, got %s", resp.Modules[0].Name)
	}
}

func TestGetModuleConfig_Found(t *testing.T) {
	mc := &mockClient{
		getModuleConfigFunc: func(_ context.Context, name string) (*unstructured.Unstructured, error) {
			obj := makeModuleConfig(name, true, "all good")
			return &obj, nil
		},
	}

	h := NewModulesHandler(mc)
	resp, err := h.GetModuleConfig(context.Background(), &pb.GetModuleConfigRequest{Name: "cert-manager"})
	if err != nil {
		t.Fatal(err)
	}
	if resp.Name != "cert-manager" {
		t.Errorf("expected cert-manager, got %s", resp.Name)
	}
	if !resp.Enabled {
		t.Error("expected enabled=true")
	}
}

func TestGetModuleConfig_NotFound(t *testing.T) {
	mc := &mockClient{
		getModuleConfigFunc: func(_ context.Context, name string) (*unstructured.Unstructured, error) {
			return nil, fmt.Errorf("module config %q not found", name)
		},
	}

	h := NewModulesHandler(mc)
	_, err := h.GetModuleConfig(context.Background(), &pb.GetModuleConfigRequest{Name: "missing"})
	if err == nil {
		t.Fatal("expected error, got nil")
	}
}

func TestEnableModule_WasDisabled(t *testing.T) {
	existing := makeModuleConfig("mod-a", false, "")
	updated := makeModuleConfig("mod-a", true, "")
	mc := &mockClient{
		getModuleConfigFunc: func(_ context.Context, _ string) (*unstructured.Unstructured, error) {
			return &existing, nil
		},
		updateModuleConfigFunc: func(_ context.Context, _ *unstructured.Unstructured) (*unstructured.Unstructured, error) {
			return &updated, nil
		},
	}

	h := NewModulesHandler(mc)
	resp, err := h.EnableModule(context.Background(), &pb.EnableModuleRequest{Name: "mod-a"})
	if err != nil {
		t.Fatal(err)
	}
	if !resp.Success {
		t.Error("expected success=true")
	}
	if resp.PreviousState {
		t.Error("expected previousState=false")
	}
}

func TestEnableModule_AlreadyEnabled(t *testing.T) {
	existing := makeModuleConfig("mod-a", true, "")
	mc := &mockClient{
		getModuleConfigFunc: func(_ context.Context, _ string) (*unstructured.Unstructured, error) {
			return &existing, nil
		},
		updateModuleConfigFunc: func(_ context.Context, obj *unstructured.Unstructured) (*unstructured.Unstructured, error) {
			return obj, nil
		},
	}

	h := NewModulesHandler(mc)
	resp, err := h.EnableModule(context.Background(), &pb.EnableModuleRequest{Name: "mod-a"})
	if err != nil {
		t.Fatal(err)
	}
	if !resp.Success {
		t.Error("expected success=true")
	}
	if !resp.PreviousState {
		t.Error("expected previousState=true")
	}
}

func TestEnableModule_NotFound(t *testing.T) {
	mc := &mockClient{
		getModuleConfigFunc: func(_ context.Context, name string) (*unstructured.Unstructured, error) {
			return nil, fmt.Errorf("module config %q not found", name)
		},
	}

	h := NewModulesHandler(mc)
	_, err := h.EnableModule(context.Background(), &pb.EnableModuleRequest{Name: "missing"})
	if err == nil {
		t.Fatal("expected error, got nil")
	}
}

func TestDisableModule_WasEnabled(t *testing.T) {
	existing := makeModuleConfig("mod-b", true, "")
	updated := makeModuleConfig("mod-b", false, "")
	mc := &mockClient{
		getModuleConfigFunc: func(_ context.Context, _ string) (*unstructured.Unstructured, error) {
			return &existing, nil
		},
		updateModuleConfigFunc: func(_ context.Context, _ *unstructured.Unstructured) (*unstructured.Unstructured, error) {
			return &updated, nil
		},
	}

	h := NewModulesHandler(mc)
	resp, err := h.DisableModule(context.Background(), &pb.DisableModuleRequest{Name: "mod-b"})
	if err != nil {
		t.Fatal(err)
	}
	if !resp.Success {
		t.Error("expected success=true")
	}
	if !resp.PreviousState {
		t.Error("expected previousState=true (was enabled)")
	}
}

func TestDisableModule_NotFound(t *testing.T) {
	mc := &mockClient{
		getModuleConfigFunc: func(_ context.Context, name string) (*unstructured.Unstructured, error) {
			return nil, fmt.Errorf("module config %q not found", name)
		},
	}

	h := NewModulesHandler(mc)
	_, err := h.DisableModule(context.Background(), &pb.DisableModuleRequest{Name: "missing"})
	if err == nil {
		t.Fatal("expected error, got nil")
	}
}
