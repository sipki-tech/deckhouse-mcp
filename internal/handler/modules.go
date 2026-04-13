package handler

import (
	"context"
	"fmt"

	"github.com/sipki-tech/deckhouse-mcp/internal/k8s"
	pb "github.com/sipki-tech/deckhouse-mcp/proto/deckhouse/v1"
)

// ModulesHandler implements pb.ModulesAPIToolHandler.
type ModulesHandler struct {
	client k8s.Client
}

// NewModulesHandler creates a new ModulesHandler.
func NewModulesHandler(client k8s.Client) *ModulesHandler {
	return &ModulesHandler{client: client}
}

func (h *ModulesHandler) ListModuleConfigs(ctx context.Context, req *pb.ListModuleConfigsRequest) (*pb.ListModuleConfigsResponse, error) {
	configs, err := h.client.ListModuleConfigs(ctx)
	if err != nil {
		return nil, fmt.Errorf("listing module configs: %w", err)
	}

	var result []*pb.ModuleConfigInfo
	for _, mc := range configs {
		name, _, _ := unstructuredNestedString(mc.Object, "metadata", "name")
		enabled, _, _ := nestedField(mc.Object, "spec", "enabled")
		version, _, _ := unstructuredNestedString(mc.Object, "spec", "version")
		source, _, _ := unstructuredNestedString(mc.Object, "status", "source")
		updatePolicy, _, _ := unstructuredNestedString(mc.Object, "status", "updatePolicy")
		statusMsg, _, _ := unstructuredNestedString(mc.Object, "status", "message")

		isEnabled := false
		if b, ok := enabled.(bool); ok {
			isEnabled = b
		}

		if req.Enabled != nil && isEnabled != *req.Enabled {
			continue
		}

		result = append(result, &pb.ModuleConfigInfo{
			Name:          name,
			Enabled:       isEnabled,
			Version:       version,
			Source:        source,
			UpdatePolicy:  updatePolicy,
			StatusMessage: statusMsg,
		})
	}

	return &pb.ListModuleConfigsResponse{Modules: result}, nil
}
