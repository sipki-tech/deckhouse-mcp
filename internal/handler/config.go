package handler

import (
	"context"
	"fmt"

	"github.com/sipki-tech/deckhouse-mcp/internal/k8s"
	pb "github.com/sipki-tech/deckhouse-mcp/proto/deckhouse/v1"
)

// ConfigHandler implements pb.ConfigAPIToolHandler.
type ConfigHandler struct {
	client k8s.Client
}

// NewConfigHandler creates a new ConfigHandler.
func NewConfigHandler(client k8s.Client) *ConfigHandler {
	return &ConfigHandler{client: client}
}

// GetClusterConfiguration reads the ClusterConfiguration YAML from the d8-cluster-configuration Secret.
func (h *ConfigHandler) GetClusterConfiguration(ctx context.Context, _ *pb.GetClusterConfigurationRequest) (*pb.GetClusterConfigurationResponse, error) {
	secret, err := h.client.GetSecret(ctx, "kube-system", "d8-cluster-configuration")
	if err != nil {
		return nil, fmt.Errorf("cluster configuration secret not found: %w", err)
	}

	config, ok := secret.Data["cluster-configuration.yaml"]
	if !ok {
		return nil, fmt.Errorf("key cluster-configuration.yaml not found in d8-cluster-configuration secret")
	}

	return &pb.GetClusterConfigurationResponse{
		Configuration: string(config),
	}, nil
}
