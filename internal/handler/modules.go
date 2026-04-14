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

// GetModuleConfig returns detailed configuration for a specific module.
func (h *ModulesHandler) GetModuleConfig(ctx context.Context, req *pb.GetModuleConfigRequest) (*pb.GetModuleConfigResponse, error) {
	mc, err := h.client.GetModuleConfig(ctx, req.Name)
	if err != nil {
		return nil, fmt.Errorf("getting module config %s: %w", req.Name, err)
	}

	name, _, _ := unstructuredNestedString(mc.Object, "metadata", "name")
	enabledVal, _, _ := nestedField(mc.Object, "spec", "enabled")
	statusMsg, _, _ := unstructuredNestedString(mc.Object, "status", "message")

	isEnabled := false
	if b, ok := enabledVal.(bool); ok {
		isEnabled = b
	}

	// Read version as int.
	var version *int32
	v, _, _ := unstructuredNestedInt64(mc.Object, "spec", "version")
	if v != 0 {
		v32 := int32(v)
		version = &v32
	}

	// Read settings as map[string]string (best-effort — values may be of any type).
	settings := make(map[string]string)
	settingsVal, _, _ := nestedField(mc.Object, "spec", "settings")
	if m, ok := settingsVal.(map[string]interface{}); ok {
		for k, v := range m {
			settings[k] = fmt.Sprintf("%v", v)
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
func (h *ModulesHandler) EnableModule(ctx context.Context, req *pb.EnableModuleRequest) (*pb.EnableModuleResponse, error) {
	return h.setModuleEnabled(ctx, req.Name, true)
}

// DisableModule sets spec.enabled=false on a ModuleConfig resource.
func (h *ModulesHandler) DisableModule(ctx context.Context, req *pb.DisableModuleRequest) (*pb.DisableModuleResponse, error) {
	resp, err := h.setModuleEnabled(ctx, req.Name, false)
	if err != nil {
		return nil, err
	}
	return &pb.DisableModuleResponse{
		Success:       resp.Success,
		PreviousState: resp.PreviousState,
	}, nil
}

// setModuleEnabled is the shared implementation for Enable/Disable.
func (h *ModulesHandler) setModuleEnabled(ctx context.Context, name string, enable bool) (*pb.EnableModuleResponse, error) {
	mc, err := h.client.GetModuleConfig(ctx, name)
	if err != nil {
		return nil, fmt.Errorf("getting module config %s: %w", name, err)
	}

	// Read previous state.
	prevVal, _, _ := nestedField(mc.Object, "spec", "enabled")
	prevEnabled := false
	if b, ok := prevVal.(bool); ok {
		prevEnabled = b
	}

	// Ensure spec map exists.
	spec, ok := mc.Object["spec"].(map[string]interface{})
	if !ok {
		spec = make(map[string]interface{})
		mc.Object["spec"] = spec
	}
	spec["enabled"] = enable

	updated, err := h.client.UpdateModuleConfig(ctx, mc)
	if err != nil {
		return nil, fmt.Errorf("updating module config %s: %w", name, err)
	}
	_ = updated

	return &pb.EnableModuleResponse{
		Success:       true,
		PreviousState: prevEnabled,
	}, nil
}
