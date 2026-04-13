package handler

import (
	"context"
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
