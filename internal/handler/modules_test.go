package handler

import (
	"context"
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
