package handler

import (
	"context"
	"fmt"
	"testing"

	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"

	pb "github.com/sipki-tech/deckhouse-mcp/proto/deckhouse/v1"
)

func TestGetClusterConfiguration_Success(t *testing.T) {
	configYAML := "apiVersion: deckhouse.io/v1\nkind: ClusterConfiguration\n"
	mc := &mockClient{
		getSecretFunc: func(_ context.Context, namespace, name string) (*corev1.Secret, error) {
			if namespace != "kube-system" || name != "d8-cluster-configuration" {
				return nil, fmt.Errorf("unexpected secret: %s/%s", namespace, name)
			}
			return &corev1.Secret{
				ObjectMeta: metav1.ObjectMeta{
					Name:      name,
					Namespace: namespace,
				},
				Data: map[string][]byte{
					"cluster-configuration.yaml": []byte(configYAML),
				},
			}, nil
		},
	}

	h := NewConfigHandler(mc)
	resp, err := h.GetClusterConfiguration(context.Background(), &pb.GetClusterConfigurationRequest{})
	if err != nil {
		t.Fatal(err)
	}
	if resp.Configuration != configYAML {
		t.Errorf("expected configuration YAML, got %q", resp.Configuration)
	}
}

func TestGetClusterConfiguration_SecretNotFound(t *testing.T) {
	mc := &mockClient{
		getSecretFunc: func(_ context.Context, namespace, name string) (*corev1.Secret, error) {
			return nil, fmt.Errorf("secret %s/%s not found", namespace, name)
		},
	}

	h := NewConfigHandler(mc)
	_, err := h.GetClusterConfiguration(context.Background(), &pb.GetClusterConfigurationRequest{})
	if err == nil {
		t.Fatal("expected error, got nil")
	}
}
