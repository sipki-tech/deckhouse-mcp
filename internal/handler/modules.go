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

func (h *ModulesHandler) ListModuleConfigs(
	ctx context.Context,
	req *pb.ListModuleConfigsRequest,
) (*pb.ListModuleConfigsResponse, error) {
	configs, err := h.client.ListModuleConfigs(ctx)
	if err != nil {
		return nil, fmt.Errorf("listing module configs: %w", err)
	}

	var result []*pb.ModuleConfigInfo

	for _, moduleConfig := range configs {
		name := unstructuredNestedString(moduleConfig.Object, "metadata", "name")
		enabled, _ := nestedField(moduleConfig.Object, "spec", "enabled")
		version := unstructuredNestedString(moduleConfig.Object, "spec", "version")
		source := unstructuredNestedString(moduleConfig.Object, "status", "source")
		updatePolicy := unstructuredNestedString(moduleConfig.Object, "status", "updatePolicy")
		statusMsg := unstructuredNestedString(moduleConfig.Object, "status", "message")

		isEnabled := false
		if b, ok := enabled.(bool); ok {
			isEnabled = b
		}

		if req.Enabled != nil && isEnabled != req.GetEnabled() {
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

// GetModuleConfig returns detailed configuration for a specific module.
func (h *ModulesHandler) GetModuleConfig(
	ctx context.Context,
	req *pb.GetModuleConfigRequest,
) (*pb.GetModuleConfigResponse, error) {
	moduleConfig, err := h.client.GetModuleConfig(ctx, req.GetName())
	if err != nil {
		return nil, fmt.Errorf("getting module config %s: %w", req.GetName(), err)
	}

	name := unstructuredNestedString(moduleConfig.Object, "metadata", "name")
	enabledVal, _ := nestedField(moduleConfig.Object, "spec", "enabled")
	statusMsg := unstructuredNestedString(moduleConfig.Object, "status", "message")

	isEnabled := false
	if b, ok := enabledVal.(bool); ok {
		isEnabled = b
	}

	// Read version as int.
	var version *int32

	v := unstructuredNestedInt64(moduleConfig.Object, "spec", "version")
	if v != 0 {
		v32 := int32(v) //nolint:gosec // version numbers fit in int32
		version = &v32
	}

	// Read settings as map[string]string (best-effort — values may be of any type).
	settings := make(map[string]string)

	settingsVal, _ := nestedField(moduleConfig.Object, "spec", "settings")
	if m, ok := settingsVal.(map[string]any); ok {
		for k, val := range m {
			settings[k] = fmt.Sprintf("%v", val)
		}
	}

	return &pb.GetModuleConfigResponse{
		Name:          name,
		Enabled:       isEnabled,
		Version:       version,
		Settings:      settings,
		StatusMessage: statusMsg,
	}, nil
}

// EnableModule sets spec.enabled=true on a ModuleConfig resource.
func (h *ModulesHandler) EnableModule(
	ctx context.Context,
	req *pb.EnableModuleRequest,
) (*pb.EnableModuleResponse, error) {
	return h.setModuleEnabled(ctx, req.GetName(), true)
}

// DisableModule sets spec.enabled=false on a ModuleConfig resource.
func (h *ModulesHandler) DisableModule(
	ctx context.Context,
	req *pb.DisableModuleRequest,
) (*pb.DisableModuleResponse, error) {
	resp, err := h.setModuleEnabled(ctx, req.GetName(), false)
	if err != nil {
		return nil, err
	}

	return &pb.DisableModuleResponse{
		Success:       resp.GetSuccess(),
		PreviousState: resp.GetPreviousState(),
	}, nil
}

// setModuleEnabled is the shared implementation for Enable/Disable.
func (h *ModulesHandler) setModuleEnabled(
	ctx context.Context,
	name string,
	enable bool,
) (*pb.EnableModuleResponse, error) {
	moduleConfig, err := h.client.GetModuleConfig(ctx, name)
	if err != nil {
		return nil, fmt.Errorf("getting module config %s: %w", name, err)
	}

	// Read previous state.
	prevVal, _ := nestedField(moduleConfig.Object, "spec", "enabled")

	prevEnabled := false
	if b, ok := prevVal.(bool); ok {
		prevEnabled = b
	}

	// Ensure spec map exists.
	spec, ok := moduleConfig.Object["spec"].(map[string]any)
	if !ok {
		spec = make(map[string]any)
		moduleConfig.Object["spec"] = spec
	}

	spec["enabled"] = enable

	updated, err := h.client.UpdateModuleConfig(ctx, moduleConfig)
	if err != nil {
		return nil, fmt.Errorf("updating module config %s: %w", name, err)
	}

	_ = updated

	return &pb.EnableModuleResponse{
		Success:       true,
		PreviousState: prevEnabled,
	}, nil
}
