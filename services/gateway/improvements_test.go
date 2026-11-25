package main

import (
	"testing"

	meshimprovementsv1 "github.com/civil-labs/civil-os/internal/gen/go/civil/mesh/improvements/v1"
)

// TestMapImprovementResults: improvement read results are copied through
// field by field (docs/improvement-read-api/plan.md).
func TestMapImprovementResults(t *testing.T) {
	checkFullMapping(t, &meshimprovementsv1.ImprovementQuality{}, mapImprovementQuality)
}
