package handler

import (
	"context"
	"encoding/base64"
	"errors"
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

var errPrivateKeyRequired = errors.New("privateKey is required")

// NodesHandler implements pb.NodesAPIToolHandler.
type NodesHandler struct {
	client k8s.Client
}

// NewNodesHandler creates a new NodesHandler.
func NewNodesHandler(client k8s.Client) *NodesHandler {
	return &NodesHandler{client: client}
}

func (h *NodesHandler) CreateSSHCredentials(
	ctx context.Context,
	req *pb.CreateSSHCredentialsRequest,
) (*pb.CreateSSHCredentialsResponse, error) {
	if req.GetPrivateKey() == "" {
		return nil, errPrivateKeyRequired
	}

	port := int64(defaultSSHPort)
	if req.Port != nil {
		port = int64(req.GetPort())
	}

	encodedKey := base64.StdEncoding.EncodeToString([]byte(req.GetPrivateKey()))

	obj := &unstructured.Unstructured{
		Object: map[string]any{
			"apiVersion": "deckhouse.io/v1alpha2",
			"kind":       "SSHCredentials",
			"metadata": map[string]any{
				"name": req.GetName(),
			},
			"spec": map[string]any{
				"user":          req.GetUser(),
				"privateSSHKey": encodedKey,
				"sshPort":       port,
			},
		},
	}

	if req.SshExtraArgs != nil {
		spec, ok := obj.Object["spec"].(map[string]any)
		if !ok {
			spec = make(map[string]any)
			obj.Object["spec"] = spec
		}

		spec["sshExtraArgs"] = req.GetSshExtraArgs()
	}

	if req.SudoPassword != nil {
		spec, ok := obj.Object["spec"].(map[string]any)
		if !ok {
			spec = make(map[string]any)
			obj.Object["spec"] = spec
		}

		spec["sudoPasswordEncoded"] = base64.StdEncoding.EncodeToString([]byte(req.GetSudoPassword()))
	}

	_, err := h.client.CreateSSHCredentials(ctx, obj)
	if err != nil {
		return nil, fmt.Errorf("creating SSHCredentials: %w", err)
	}

	return &pb.CreateSSHCredentialsResponse{Name: req.GetName()}, nil
}

func (h *NodesHandler) CreateStaticInstance(
	ctx context.Context,
	req *pb.CreateStaticInstanceRequest,
) (*pb.CreateStaticInstanceResponse, error) {
	labels := make(map[string]any)
	for k, v := range req.GetLabels() {
		labels[k] = v
	}

	obj := &unstructured.Unstructured{
		Object: map[string]any{
			"apiVersion": "deckhouse.io/v1alpha2",
			"kind":       "StaticInstance",
			"metadata": map[string]any{
				"name":   req.GetName(),
				"labels": labels,
			},
			"spec": map[string]any{
				"address": req.GetAddress(),
				"credentialsRef": map[string]any{
					"kind": "SSHCredentials",
					"name": req.GetCredentialsRef(),
				},
			},
		},
	}

	created, err := h.client.CreateStaticInstance(ctx, obj)
	if err != nil {
		return nil, fmt.Errorf("creating StaticInstance: %w", err)
	}

	var resultLabels map[string]string

	createdLabels := created.GetLabels()
	if createdLabels != nil {
		resultLabels = createdLabels
	} else {
		resultLabels = req.GetLabels()
	}

	return &pb.CreateStaticInstanceResponse{
		Name:           req.GetName(),
		Address:        req.GetAddress(),
		CredentialsRef: req.GetCredentialsRef(),
		Labels:         resultLabels,
	}, nil
}

func (h *NodesHandler) AddWorkerNode(
	ctx context.Context,
	req *pb.AddWorkerNodeRequest,
) (*pb.AddWorkerNodeResponse, error) {
	var nodeName string

	if req.NodeName != nil {
		nodeName = req.GetNodeName()
	} else {
		nodeName = strings.ReplaceAll(req.GetAddress(), ".", "-")
	}

	credsName := nodeName + "-creds"

	sshPort := int32(defaultSSHPort)
	if req.SshPort != nil {
		sshPort = req.GetSshPort()
	}

	err := h.provisionNodeResources(ctx, req, nodeName, credsName, sshPort)
	if err != nil {
		return nil, err
	}

	waitReady := true
	if req.WaitReady != nil {
		waitReady = req.GetWaitReady()
	}

	timeoutSec := int32(defaultTimeoutSeconds)
	if req.TimeoutSeconds != nil {
		timeoutSec = req.GetTimeoutSeconds()
	}

	phase, elapsed, timedOut, err := h.waitForNode(ctx, waitReady, nodeName, timeoutSec)
	if err != nil {
		return nil, err
	}

	return &pb.AddWorkerNodeResponse{
		NodeName:           nodeName,
		SshCredentialsName: credsName,
		StaticInstanceName: nodeName,
		Phase:              phase,
		Elapsed:            elapsed,
		TimedOut:           timedOut,
	}, nil
}

// DeleteStaticInstance deletes a StaticInstance resource by name.
func (h *NodesHandler) DeleteStaticInstance(
	ctx context.Context,
	req *pb.DeleteStaticInstanceRequest,
) (*pb.DeleteStaticInstanceResponse, error) {
	err := h.client.DeleteStaticInstance(ctx, req.GetName())
	if err != nil {
		return nil, fmt.Errorf("deleting StaticInstance %s: %w", req.GetName(), err)
	}

	return &pb.DeleteStaticInstanceResponse{Success: true}, nil
}

// RemoveNode cordons the node, deletes non-DaemonSet pods, then removes its StaticInstance.
func (h *NodesHandler) RemoveNode(
	ctx context.Context,
	req *pb.RemoveNodeRequest,
) (*pb.RemoveNodeResponse, error) {
	// Verify StaticInstance exists (static nodes only).
	_, err := h.client.GetStaticInstance(ctx, req.GetName())
	if err != nil {
		return nil, fmt.Errorf("static instance for node %q not found: %w", req.GetName(), err)
	}

	drain := true
	if req.Drain != nil {
		drain = req.GetDrain()
	}

	drained := false

	if drain {
		err = h.drainNode(ctx, req.GetName())
		if err != nil {
			return nil, err
		}

		drained = true
	}

	// Delete the StaticInstance — Deckhouse will clean up.
	err = h.client.DeleteStaticInstance(ctx, req.GetName())
	if err != nil {
		return &pb.RemoveNodeResponse{Drained: drained, Deleted: false},
			fmt.Errorf("deleting StaticInstance %s: %w", req.GetName(), err)
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
func (h *NodesHandler) CreateNodeGroup(
	ctx context.Context,
	req *pb.CreateNodeGroupRequest,
) (*pb.CreateNodeGroupResponse, error) {
	spec := map[string]any{
		"nodeType": req.GetNodeType(),
	}

	if req.Count != nil {
		spec["staticInstances"] = map[string]any{
			"count": int64(req.GetCount()),
		}
	}

	if req.Disruptions != nil {
		spec["disruptions"] = map[string]any{
			"approvalMode": req.GetDisruptions(),
		}
	}

	if req.MaxPodsPerNode != nil {
		spec["kubelet"] = map[string]any{
			"maxPods": int64(req.GetMaxPodsPerNode()),
		}
	}

	labels := make(map[string]any)
	for k, v := range req.GetLabels() {
		labels[k] = v
	}

	obj := &unstructured.Unstructured{
		Object: map[string]any{
			"apiVersion": "deckhouse.io/v1",
			"kind":       "NodeGroup",
			"metadata": map[string]any{
				"name":   req.GetName(),
				"labels": labels,
			},
			"spec": spec,
		},
	}

	created, err := h.client.CreateNodeGroup(ctx, obj)
	if err != nil {
		return nil, fmt.Errorf("creating NodeGroup %s: %w", req.GetName(), err)
	}

	resp := &pb.CreateNodeGroupResponse{
		Name:     req.GetName(),
		NodeType: req.GetNodeType(),
	}
	if req.Count != nil {
		count := req.Count
		resp.Count = count
	}

	_ = created

	return resp, nil
}

// WaitNodeReady polls a StaticInstance until it reaches Running phase or timeout expires.
func (h *NodesHandler) WaitNodeReady(
	ctx context.Context,
	req *pb.WaitNodeReadyRequest,
) (*pb.WaitNodeReadyResponse, error) {
	timeoutSec := int32(defaultTimeoutSeconds)
	if req.TimeoutSeconds != nil {
		timeoutSec = req.GetTimeoutSeconds()
	}

	phase, elapsed, timedOut, err := h.pollStaticInstance(ctx, req.GetName(), timeoutSec)
	if err != nil {
		return nil, err
	}

	return &pb.WaitNodeReadyResponse{
		Phase:    phase,
		Elapsed:  elapsed,
		TimedOut: timedOut,
	}, nil
}

func (h *NodesHandler) waitForNode(
	ctx context.Context,
	waitReady bool,
	nodeName string,
	timeoutSec int32,
) (string, string, bool, error) {
	start := time.Now()

	if !waitReady {
		return phasePending, "0s", false, nil
	}

	phase, elapsed, timedOut, err := h.pollStaticInstance(ctx, nodeName, timeoutSec)
	if err != nil {
		return "", time.Since(start).Truncate(time.Second).String(), false, err
	}

	return phase, elapsed, timedOut, nil
}

func (h *NodesHandler) pollStaticInstance(
	ctx context.Context,
	name string,
	timeoutSec int32,
) (string, string, bool, error) {
	start := time.Now()
	timeout := time.Duration(timeoutSec) * time.Second
	deadline := start.Add(timeout)
	phase := phasePending
	timedOut := false

	for {
		staticInstance, siErr := h.client.GetStaticInstance(ctx, name)
		if siErr != nil {
			return phase, time.Since(start).Truncate(time.Second).String(), false,
				fmt.Errorf("polling StaticInstance %s: %w", name, siErr)
		}

		currentPhase := unstructuredNestedString(staticInstance.Object, "status", "currentStatus", "phase")
		if currentPhase != "" {
			phase = currentPhase
		}

		if phase == phaseRunning {
			break
		}

		if time.Now().After(deadline) {
			timedOut = true

			break
		}

		select {
		case <-ctx.Done():
			return phase, time.Since(start).Truncate(time.Second).String(), false,
				fmt.Errorf("polling StaticInstance %s: %w", name, ctx.Err())
		case <-time.After(pollInterval):
		}
	}

	return phase, time.Since(start).Truncate(time.Second).String(), timedOut, nil
}

func (h *NodesHandler) drainNode(ctx context.Context, nodeName string) error {
	err := h.client.CordonNode(ctx, nodeName)
	if err != nil {
		return fmt.Errorf("cordoning node %s: %w", nodeName, err)
	}

	pods, err := h.client.ListPods(ctx, "")
	if err != nil {
		return fmt.Errorf("listing pods for node %s: %w", nodeName, err)
	}

	for _, pod := range pods {
		if pod.Spec.NodeName != nodeName {
			continue
		}

		if isDaemonSetPod(&pod) {
			continue
		}

		err = h.client.DeletePod(ctx, pod.Namespace, pod.Name)
		if err != nil {
			return fmt.Errorf("deleting pod %s/%s: %w", pod.Namespace, pod.Name, err)
		}
	}

	return nil
}

func (h *NodesHandler) provisionNodeResources(
	ctx context.Context,
	req *pb.AddWorkerNodeRequest,
	nodeName, credsName string,
	sshPort int32,
) error {
	_, err := h.CreateSSHCredentials(ctx, &pb.CreateSSHCredentialsRequest{
		Name:       credsName,
		User:       req.GetSshUser(),
		PrivateKey: req.GetPrivateKey(),
		Port:       &sshPort,
	})
	if err != nil {
		return fmt.Errorf("creating SSHCredentials for node %s: %w", nodeName, err)
	}

	_, err = h.CreateStaticInstance(ctx, &pb.CreateStaticInstanceRequest{
		Name:           nodeName,
		Address:        req.GetAddress(),
		CredentialsRef: credsName,
		Labels: map[string]string{
			"node.deckhouse.io/group": req.GetNodeGroup(),
		},
	})
	if err != nil {
		return fmt.Errorf(
			"creating StaticInstance for node %s (SSHCredentials %q already created): %w",
			nodeName, credsName, err,
		)
	}

	return nil
}
