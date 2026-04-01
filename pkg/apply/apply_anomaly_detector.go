package apply

import (
	"encoding/json"
	"fmt"

	"github.com/dynatrace-oss/dtctl/pkg/resources/anomalydetector"
	"github.com/dynatrace-oss/dtctl/pkg/safety"
)

// applyAnomalyDetector applies a custom anomaly detector resource.
// Supports both flattened YAML format and raw Settings API format.
func (a *Applier) applyAnomalyDetector(data []byte) (ApplyResult, error) {
	var raw map[string]interface{}
	if err := json.Unmarshal(data, &raw); err != nil {
		return nil, fmt.Errorf("failed to parse anomaly detector JSON: %w", err)
	}

	handler := anomalydetector.NewHandler(a.client)

	// Extract object ID (present in raw Settings format or if user includes it)
	objectID, _ := raw["objectId"].(string)
	if objectID == "" {
		objectID, _ = raw["objectid"].(string)
	}

	// If no objectId, try to find existing detector by title for idempotent apply
	if objectID == "" {
		title := anomalydetector.ExtractTitle(data)
		if title != "" {
			existing, err := handler.FindByExactTitle(title)
			if err != nil {
				// Log warning but proceed to try create
				stderrWarn(nil, "Failed to lookup existing anomaly detector by title: %v", err)
			} else if existing != nil {
				objectID = existing.ObjectID
				stderrWarn(nil, "Found existing anomaly detector %q (ID: %s), switching to update mode", title, objectID)
			}
		}
	}

	if objectID == "" {
		// No objectId and no existing match — create new anomaly detector
		if err := a.checkSafety(safety.OperationCreate, safety.OwnershipUnknown); err != nil {
			return nil, err
		}

		result, err := handler.Create(data)
		if err != nil {
			return nil, fmt.Errorf("failed to create anomaly detector: %w", err)
		}
		return &AnomalyDetectorApplyResult{
			ApplyResultBase: ApplyResultBase{
				Action:       ActionCreated,
				ResourceType: "anomaly_detector",
				ID:           result.ObjectID,
				Name:         result.Title,
			},
		}, nil
	}

	// objectId present — update existing anomaly detector
	if err := a.checkSafety(safety.OperationUpdate, safety.OwnershipUnknown); err != nil {
		return nil, err
	}

	result, err := handler.Update(objectID, data)
	if err != nil {
		return nil, fmt.Errorf("failed to update anomaly detector: %w", err)
	}

	return &AnomalyDetectorApplyResult{
		ApplyResultBase: ApplyResultBase{
			Action:       ActionUpdated,
			ResourceType: "anomaly_detector",
			ID:           result.ObjectID,
			Name:         result.Title,
		},
	}, nil
}
