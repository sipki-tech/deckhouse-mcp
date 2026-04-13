package handler

import (
	"context"
	"encoding/base64"
	"fmt"
	"strings"
	"time"

	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"

	"github.com/sipki-tech/deckhouse-mcp/internal/k8s"
	pb "github.com/sipki-tech/deckhouse-mcp/proto/deckhouse/v1"
)

const (
	defaultSSHPort        = 22
	defaultTimeoutSeconds = 900
	pollInterval          = 30 * time.Second
)

// NodesHandler implements pb.NodesAPIToolHandler.
type NodesHandler struct {
	client k8s.Client
}

// NewNodesHandler creates a new NodesHandler.
func NewNodesHandler(client k8s.Client) *NodesHandler {
	return &NodesHandler{client: client}
}

func (h *NodesHandler) CreateSSHCredentials(ctx context.Context, req *pb.CreateSSHCredentialsRequest) (*pb.CreateSSHCredentialsResponse, error) {
	if req.PrivateKey == "" {
		return nil, fmt.Errorf("privateKey is required")
	}

	port := int64(defaultSSHPort)
	if req.Port != nil {
		port = int64(*req.Port)
	}

	encodedKey := base64.StdEncoding.EncodeToString([]byte(req.PrivateKey))

	obj := &unstructured.Unstructured{
		Object: map[string]interface{}{
			"apiVersion": "deckhouse.io/v1alpha2",
			"kind":       "SSHCredentials",
			"metadata": map[string]interface{}{
				"name": req.Name,
			},
			"spec": map[string]interface{}{
				"user":          req.User,
				"privateSSHKey": encodedKey,
				"sshPort":       port,
			},
		},
	}

	if req.SshExtraArgs != nil {
		spec := obj.Object["spec"].(map[string]interface{})
		spec["sshExtraArgs"] = *req.SshExtraArgs
	}

	if req.SudoPassword != nil {
		spec := obj.Object["spec"].(map[string]interface{})
		spec["sudoPasswordEncoded"] = base64.StdEncoding.EncodeToString([]byte(*req.SudoPassword))
	}

	_, err := h.client.CreateSSHCredentials(ctx, obj)
	if err != nil {
		return nil, fmt.Errorf("creating SSHCredentials: %w", err)
	}

	return &pb.CreateSSHCredentialsResponse{Name: req.Name}, nil
}

func (h *NodesHandler) CreateStaticInstance(ctx context.Context, req *pb.CreateStaticInstanceRequest) (*pb.CreateStaticInstanceResponse, error) {
	labels := make(map[string]interface{})
	for k, v := range req.Labels {
		labels[k] = v
	}

	obj := &unstructured.Unstructured{
		Object: map[string]interface{}{
			"apiVersion": "deckhouse.io/v1alpha2",
			"kind":       "StaticInstance",
			"metadata": map[string]interface{}{
				"name":   req.Name,
				"labels": labels,
			},
			"spec": map[string]interface{}{
				"address": req.Address,
				"credentialsRef": map[string]interface{}{
					"kind": "SSHCredentials",
					"name": req.CredentialsRef,
				},
			},
		},
	}

	created, err := h.client.CreateStaticInstance(ctx, obj)
	if err != nil {
		return nil, fmt.Errorf("creating StaticInstance: %w", err)
	}

	resultLabels := make(map[string]string)
	createdLabels := created.GetLabels()
	if createdLabels != nil {
		resultLabels = createdLabels
	} else {
		resultLabels = req.Labels
	}

	return &pb.CreateStaticInstanceResponse{
		Name:           req.Name,
		Address:        req.Address,
		CredentialsRef: req.CredentialsRef,
		Labels:         resultLabels,
	}, nil
}

func (h *NodesHandler) AddWorkerNode(ctx context.Context, req *pb.AddWorkerNodeRequest) (*pb.AddWorkerNodeResponse, error) {
	nodeName := req.Address
	if req.NodeName != nil {
		nodeName = *req.NodeName
	} else {
		nodeName = strings.ReplaceAll(req.Address, ".", "-")
	}

	credsName := nodeName + "-creds"

	sshPort := int32(defaultSSHPort)
	if req.SshPort != nil {
		sshPort = *req.SshPort
	}

	// Step 1: Create SSHCredentials.
	_, err := h.CreateSSHCredentials(ctx, &pb.CreateSSHCredentialsRequest{
		Name:       credsName,
		User:       req.SshUser,
		PrivateKey: req.PrivateKey,
		Port:       &sshPort,
	})
	if err != nil {
		return nil, fmt.Errorf("creating SSHCredentials for node %s: %w", nodeName, err)
	}

	// Step 2: Create StaticInstance.
	_, err = h.CreateStaticInstance(ctx, &pb.CreateStaticInstanceRequest{
		Name:           nodeName,
		Address:        req.Address,
		CredentialsRef: credsName,
		Labels: map[string]string{
			"node.deckhouse.io/group": req.NodeGroup,
		},
	})
	if err != nil {
		return nil, fmt.Errorf("creating StaticInstance for node %s (SSHCredentials %q already created): %w", nodeName, credsName, err)
	}

	waitReady := true
	if req.WaitReady != nil {
		waitReady = *req.WaitReady
	}

	timeoutSec := int32(defaultTimeoutSeconds)
	if req.TimeoutSeconds != nil {
		timeoutSec = *req.TimeoutSeconds
	}

	start := time.Now()
	phase := "Pending"
	timedOut := false

	if waitReady {
		timeout := time.Duration(timeoutSec) * time.Second
		deadline := start.Add(timeout)

		for {
			si, err := h.client.GetStaticInstance(ctx, nodeName)
			if err != nil {
				return nil, fmt.Errorf("polling StaticInstance %s: %w", nodeName, err)
			}

			currentPhase, _, _ := unstructuredNestedString(si.Object, "status", "currentStatus", "phase")
			if currentPhase != "" {
				phase = currentPhase
			}

			if phase == "Running" {
				break
			}

			if time.Now().After(deadline) {
				timedOut = true
				break
			}

			select {
			case <-ctx.Done():
				return nil, ctx.Err()
			case <-time.After(pollInterval):
			}
		}
	}

	elapsed := time.Since(start).Truncate(time.Second).String()

	return &pb.AddWorkerNodeResponse{
		NodeName:           nodeName,
		SshCredentialsName: credsName,
		StaticInstanceName: nodeName,
		Phase:              phase,
		Elapsed:            elapsed,
		TimedOut:           timedOut,
	}, nil
}
