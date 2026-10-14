package reviews

import (
	"testing"

	"review-api/internal/modules/reviews/contract"
)

func TestStatusesAreFixed(t *testing.T) {
	t.Parallel()
	publication := PublicationStatuses()
	release := ReleaseStatuses()
	if len(publication) != 2 || publication[0].Code != "draft" || publication[1].Code != "published" {
		t.Fatalf("publication = %+v", publication)
	}
	if len(release) != 4 || release[0].Code != "announced" || release[2].Code != "completed" {
		t.Fatalf("release = %+v", release)
	}
	publication[0].Code = "mutated"
	if PublicationStatuses()[0].Code != "draft" {
		t.Fatal("status config was mutated")
	}
	for _, status := range release {
		if !contract.IsReleaseStatus(status.Code) {
			t.Fatalf("release status %s is not in the contract", status.Code)
		}
	}
}
