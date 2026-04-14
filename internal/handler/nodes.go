package handler

import (
	"context"
	"encoding/base64"
	"fmt"
	"strings"
	"time"

	corev1 "k8s.io/api/core/v1"
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

// pollStaticInstance polls until the StaticInstance reaches "Running" phase or timeout.
// Returns the last observed phase, elapsed time, and whether timeout occurred.
func (h *NodesHandler) pollStaticInstance(ctx context.Context, name string, timeoutSec int32) (phase string, elapsed string, timedOut bool, err error) {
	start := time.Now()
	timeout := time.Duration(timeoutSec) * time.Second
	deadline := start.Add(timeout)
	phase = "Pending"

	for {
		si, siErr := h.client.GetStaticInstance(ctx, name)
		if siErr != nil {
			return phase, time.Since(start).Truncate(time.Second).String(), false, fmt.Errorf("polling StaticInstance %s: %w", name, siErr)
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
			return phase, time.Since(start).Truncate(time.Second).String(), false, ctx.Err()
		case <-time.After(pollInterval):
		}
	}

	return phase, time.Since(start).Truncate(time.Second).String(), timedOut, nil
}

// DeleteStaticInstance deletes a StaticInstance resource by name.
func (h *NodesHandler) DeleteStaticInstance(ctx context.Context, req *pb.DeleteStaticInstanceRequest) (*pb.DeleteStaticInstanceResponse, error) {
	if err := h.client.DeleteStaticInstance(ctx, req.Name); err != nil {
		return nil, fmt.Errorf("deleting StaticInstance %s: %w", req.Name, err)
	}
	return &pb.DeleteStaticInstanceResponse{Success: true}, nil
}

// RemoveNode cordons the node, deletes non-DaemonSet pods, then removes its StaticInstance.
func (h *NodesHandler) RemoveNode(ctx context.Context, req *pb.RemoveNodeRequest) (*pb.RemoveNodeResponse, error) {
	// Verify StaticInstance exists (static nodes only).
	_, err := h.client.GetStaticInstance(ctx, req.Name)
	if err != nil {
		return nil, fmt.Errorf("static instance for node %q not found: %w", req.Name, err)
	}

	drain := true
	if req.Drain != nil {
		drain = *req.Drain
	}

	drained := false
	if drain {
		// Cordon the node to prevent new pods scheduling.
		if err := h.client.CordonNode(ctx, req.Name); err != nil {
			return nil, fmt.Errorf("cordoning node %s: %w", req.Name, err)
		}

		// Delete all non-DaemonSet pods on this node.
		pods, err := h.client.ListPods(ctx, "")
		if err != nil {
			return nil, fmt.Errorf("listing pods for node %s: %w", req.Name, err)
		}
		for _, pod := range pods {
			if pod.Spec.NodeName != req.Name {
				continue
			}
			if isDaemonSetPod(&pod) {
				continue
			}
			if err := h.client.DeletePod(ctx, pod.Namespace, pod.Name); err != nil {
				return nil, fmt.Errorf("deleting pod %s/%s: %w", pod.Namespace, pod.Name, err)
			}
		}
		drained = true
	}

	// Delete the StaticInstance — Deckhouse will clean up.
	if err := h.client.DeleteStaticInstance(ctx, req.Name); err != nil {
		return &pb.RemoveNodeResponse{Drained: drained, Deleted: false}, fmt.Errorf("deleting StaticInstance %s: %w", req.Name, err)
	}

	return &pb.RemoveNodeResponse{
		Drained: drained,
		Deleted: true,
	}, nil
}

// isDaemonSetPod reports whether the pod is owned by a DaemonSet.
func isDaemonSetPod(pod *corev1.Pod) bool {
	for _, ref := range pod.OwnerReferences {
		if ref.Kind == "DaemonSet" {
			return true
		}
	}
	return false
}

// CreateNodeGroup creates a new NodeGroup resource.
func (h *NodesHandler) CreateNodeGroup(ctx context.Context, req *pb.CreateNodeGroupRequest) (*pb.CreateNodeGroupResponse, error) {
	spec := map[string]interface{}{
		"nodeType": req.NodeType,
	}

	if req.Count != nil {
		spec["staticInstances"] = map[string]interface{}{
			"count": int64(*req.Count),
		}
	}

	if req.Disruptions != nil {
		spec["disruptions"] = map[string]interface{}{
			"approvalMode": *req.Disruptions,
		}
	}

	if req.MaxPodsPerNode != nil {
		spec["kubelet"] = map[string]interface{}{
			"maxPods": int64(*req.MaxPodsPerNode),
		}
	}

	labels := make(map[string]interface{})
	for k, v := range req.Labels {
		labels[k] = v
	}

	obj := &unstructured.Unstructured{
		Object: map[string]interface{}{
			"apiVersion": "deckhouse.io/v1",
			"kind":       "NodeGroup",
			"metadata": map[string]interface{}{
				"name":   req.Name,
				"labels": labels,
			},
			"spec": spec,
		},
	}

	created, err := h.client.CreateNodeGroup(ctx, obj)
	if err != nil {
		return nil, fmt.Errorf("creating NodeGroup %s: %w", req.Name, err)
	}

	resp := &pb.CreateNodeGroupResponse{
		Name:     req.Name,
		NodeType: req.NodeType,
	}
	if req.Count != nil {
		count := req.Count
		resp.Count = count
	}
	_ = created
	return resp, nil
}

// WaitNodeReady polls a StaticInstance until it reaches Running phase or timeout expires.
func (h *NodesHandler) WaitNodeReady(ctx context.Context, req *pb.WaitNodeReadyRequest) (*pb.WaitNodeReadyResponse, error) {
	timeoutSec := int32(defaultTimeoutSeconds)
	if req.TimeoutSeconds != nil {
		timeoutSec = *req.TimeoutSeconds
	}

	phase, elapsed, timedOut, err := h.pollStaticInstance(ctx, req.Name, timeoutSec)
	if err != nil {
		return nil, err
	}

	return &pb.WaitNodeReadyResponse{
		Phase:    phase,
		Elapsed:  elapsed,
		TimedOut: timedOut,
	}, nil
}
