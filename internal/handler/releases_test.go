package handler

import (
	"context"
	"testing"

	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"

	pb "github.com/sipki-tech/deckhouse-mcp/proto/deckhouse/v1"
)

func TestListDeckhouseReleases_All(t *testing.T) {
	mc := &mockClient{
		listDeckhouseReleasesFunc: func(_ context.Context) ([]unstructured.Unstructured, error) {
			return []unstructured.Unstructured{
				makeRelease("v1.60.0", "Deployed", "1.60.0"),
				makeRelease("v1.61.0", "Pending", "1.61.0"),
			}, nil
		},
	}

	h := NewReleasesHandler(mc)
	resp, err := h.ListDeckhouseReleases(context.Background(), &pb.ListDeckhouseReleasesRequest{})
	if err != nil {
		t.Fatal(err)
	}
	if len(resp.Releases) != 2 {
		t.Errorf("expected 2, got %d", len(resp.Releases))
	}
}

func TestListDeckhouseReleases_ByPhase(t *testing.T) {
	mc := &mockClient{
		listDeckhouseReleasesFunc: func(_ context.Context) ([]unstructured.Unstructured, error) {
			return []unstructured.Unstructured{
				makeRelease("v1.60.0", "Deployed", "1.60.0"),
				makeRelease("v1.61.0", "Pending", "1.61.0"),
				makeRelease("v1.59.0", "Superseded", "1.59.0"),
			}, nil
		},
	}

	h := NewReleasesHandler(mc)
	phase := pb.DeckhouseReleasePhase_DECKHOUSE_RELEASE_PHASE_PENDING
	resp, err := h.ListDeckhouseReleases(context.Background(), &pb.ListDeckhouseReleasesRequest{Phase: &phase})
	if err != nil {
		t.Fatal(err)
	}
	if len(resp.Releases) != 1 {
		t.Errorf("expected 1, got %d", len(resp.Releases))
	}
	if resp.Releases[0].Name != "v1.61.0" {
		t.Errorf("expected v1.61.0, got %s", resp.Releases[0].Name)
	}
}
