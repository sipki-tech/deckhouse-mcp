package handler

import (
	"context"
	"encoding/json"
	"fmt"

	"github.com/sipki-tech/deckhouse-mcp/internal/k8s"
	pb "github.com/sipki-tech/deckhouse-mcp/proto/deckhouse/v1"
)

// ReleasesHandler implements pb.ReleasesAPIToolHandler.
type ReleasesHandler struct {
	client k8s.Client
}

// NewReleasesHandler creates a new ReleasesHandler.
func NewReleasesHandler(client k8s.Client) *ReleasesHandler {
	return &ReleasesHandler{client: client}
}

func (h *ReleasesHandler) ListDeckhouseReleases(ctx context.Context, req *pb.ListDeckhouseReleasesRequest) (*pb.ListDeckhouseReleasesResponse, error) {
	releases, err := h.client.ListDeckhouseReleases(ctx)
	if err != nil {
		return nil, fmt.Errorf("listing deckhouse releases: %w", err)
	}

	var result []*pb.DeckhouseReleaseInfo
	for _, r := range releases {
		name, _, _ := unstructuredNestedString(r.Object, "metadata", "name")
		phase, _, _ := unstructuredNestedString(r.Object, "status", "phase")
		version, _, _ := unstructuredNestedString(r.Object, "spec", "version")
		transitionTime, _, _ := unstructuredNestedString(r.Object, "status", "transitionTime")
		changelogLink, _, _ := unstructuredNestedString(r.Object, "spec", "changelogLink")
		approvedVal, _, _ := nestedField(r.Object, "status", "approved")
		approved := false
		if b, ok := approvedVal.(bool); ok {
			approved = b
		}

		if req.Phase != nil && *req.Phase != pb.DeckhouseReleasePhase_DECKHOUSE_RELEASE_PHASE_UNSPECIFIED {
			wantPhase := phaseEnumToString(*req.Phase)
			if phase != wantPhase {
				continue
			}
		}

		result = append(result, &pb.DeckhouseReleaseInfo{
			Name:           name,
			Phase:          phase,
			Version:        version,
			TransitionTime: transitionTime,
			ChangelogLink:  changelogLink,
			Approved:       approved,
		})
	}

	return &pb.ListDeckhouseReleasesResponse{Releases: result}, nil
}

// phaseEnumToString converts a DeckhouseReleasePhase enum to the string used in the CRD status.
func phaseEnumToString(p pb.DeckhouseReleasePhase) string {
	switch p {
	case pb.DeckhouseReleasePhase_DECKHOUSE_RELEASE_PHASE_PENDING:
		return "Pending"
	case pb.DeckhouseReleasePhase_DECKHOUSE_RELEASE_PHASE_DEPLOYED:
		return "Deployed"
	case pb.DeckhouseReleasePhase_DECKHOUSE_RELEASE_PHASE_SUPERSEDED:
		return "Superseded"
	case pb.DeckhouseReleasePhase_DECKHOUSE_RELEASE_PHASE_SKIPPED:
		return "Skipped"
	default:
		return ""
	}
}

// GetDeckhouseRelease returns full details about a specific DeckhouseRelease by version.
// In Deckhouse the resource name matches the version (e.g. "v1.74.0").
func (h *ReleasesHandler) GetDeckhouseRelease(ctx context.Context, req *pb.GetDeckhouseReleaseRequest) (*pb.GetDeckhouseReleaseResponse, error) {
	r, err := h.client.GetDeckhouseRelease(ctx, req.Version)
	if err != nil {
		return nil, fmt.Errorf("getting deckhouse release %s: %w", req.Version, err)
	}

	name, _, _ := unstructuredNestedString(r.Object, "metadata", "name")
	phase, _, _ := unstructuredNestedString(r.Object, "status", "phase")
	version, _, _ := unstructuredNestedString(r.Object, "spec", "version")
	transitionTime, _, _ := unstructuredNestedString(r.Object, "status", "transitionTime")
	changelogLink, _, _ := unstructuredNestedString(r.Object, "spec", "changelogLink")
	approvedVal, _, _ := nestedField(r.Object, "status", "approved")
	approved := false
	if b, ok := approvedVal.(bool); ok {
		approved = b
	}

	// Also check annotation-based approval.
	annotations := r.GetAnnotations()
	if annotations["release.deckhouse.io/approved"] == "true" {
		approved = true
	}

	// Read requirements map.
	requirements := make(map[string]string)
	reqVal, _, _ := nestedField(r.Object, "spec", "requirements")
	if m, ok := reqVal.(map[string]interface{}); ok {
		for k, v := range m {
			requirements[k] = fmt.Sprintf("%v", v)
		}
	}

	return &pb.GetDeckhouseReleaseResponse{
		Name:           name,
		Phase:          phase,
		Version:        version,
		TransitionTime: transitionTime,
		Approved:       approved,
		ChangelogLink:  changelogLink,
		Requirements:   requirements,
	}, nil
}

// ApproveRelease approves a pending Deckhouse release via merge patch on the annotation.
func (h *ReleasesHandler) ApproveRelease(ctx context.Context, req *pb.ApproveReleaseRequest) (*pb.ApproveReleaseResponse, error) {
	// First check if it exists and read current approval state.
	r, err := h.client.GetDeckhouseRelease(ctx, req.Version)
	if err != nil {
		return nil, fmt.Errorf("getting deckhouse release %s: %w", req.Version, err)
	}

	// Check if already approved.
	annotations := r.GetAnnotations()
	prevApproved := annotations["release.deckhouse.io/approved"] == "true"

	// Apply approval via merge patch.
	patch := map[string]interface{}{
		"metadata": map[string]interface{}{
			"annotations": map[string]interface{}{
				"release.deckhouse.io/approved": "true",
			},
		},
	}
	patchBytes, err := json.Marshal(patch)
	if err != nil {
		return nil, fmt.Errorf("marshaling approval patch: %w", err)
	}

	_, err = h.client.PatchDeckhouseRelease(ctx, req.Version, patchBytes)
	if err != nil {
		return nil, fmt.Errorf("patching deckhouse release %s: %w", req.Version, err)
	}

	return &pb.ApproveReleaseResponse{
		Success:          true,
		PreviousApproved: prevApproved,
	}, nil
}
