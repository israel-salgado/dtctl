package anomalydetector

import (
	"encoding/json"
	"fmt"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/dynatrace-oss/dtctl/pkg/client"
)

func newTestHandler(t *testing.T, fn http.HandlerFunc) (*Handler, *httptest.Server) {
	t.Helper()
	server := httptest.NewServer(fn)
	c, err := client.NewForTesting(server.URL, "test-token")
	if err != nil {
		server.Close()
		t.Fatalf("client.NewForTesting() error = %v", err)
	}
	c.HTTP().SetRetryCount(0)
	return NewHandler(c), server
}

// settingsConstraintGuard rejects requests that violate the Settings API rule:
// pageSize, schemaIds, and scopes must NOT be combined with nextPageKey.
func settingsConstraintGuard(t *testing.T, w http.ResponseWriter, r *http.Request) bool {
	t.Helper()
	if r.URL.Query().Get("nextPageKey") != "" {
		for _, param := range []string{"pageSize", "schemaIds", "scopes"} {
			if r.URL.Query().Get(param) != "" {
				t.Errorf("%s must not be sent with nextPageKey", param)
				w.WriteHeader(http.StatusBadRequest)
				fmt.Fprintf(w, `{"error":{"code":400,"message":"Constraints violated."}}`)
				return true
			}
		}
	}
	return false
}

// sampleItem returns a settingsItem for testing.
func sampleItem(objectID, title string, enabled bool) settingsItem {
	return settingsItem{
		ObjectID:      objectID,
		SchemaID:      SchemaID,
		SchemaVersion: "1.0.15",
		Scope:         Scope,
		Value: map[string]any{
			"title":       title,
			"enabled":     enabled,
			"description": "Test detector",
			"source":      "dtctl",
			"analyzer": map[string]any{
				"name": "dt.statistics.ui.anomaly_detection.StaticThresholdAnomalyDetectionAnalyzer",
				"input": []any{
					map[string]any{"key": "alertCondition", "value": "ABOVE"},
					map[string]any{"key": "threshold", "value": "90"},
					map[string]any{"key": "query", "value": "timeseries cpu=avg(dt.host.cpu.usage)"},
				},
			},
			"eventTemplate": map[string]any{
				"properties": []any{
					map[string]any{"key": "event.type", "value": "PERFORMANCE_EVENT"},
					map[string]any{"key": "event.name", "value": "High CPU"},
				},
			},
		},
	}
}

// ---------------------------------------------------------------------------
// List tests
// ---------------------------------------------------------------------------

func TestList_Success(t *testing.T) {
	h, server := newTestHandler(t, func(w http.ResponseWriter, r *http.Request) {
		if settingsConstraintGuard(t, w, r) {
			return
		}
		w.Header().Set("Content-Type", "application/json")
		json.NewEncoder(w).Encode(listResponse{
			Items:      []settingsItem{sampleItem("obj-1", "CPU Alert", true)},
			TotalCount: 1,
		})
	})
	defer server.Close()

	detectors, err := h.List(ListOptions{})
	if err != nil {
		t.Fatalf("List() error = %v", err)
	}
	if len(detectors) != 1 {
		t.Fatalf("List() returned %d items, want 1", len(detectors))
	}
	if detectors[0].Title != "CPU Alert" {
		t.Errorf("Title = %q, want %q", detectors[0].Title, "CPU Alert")
	}
	if detectors[0].ObjectID != "obj-1" {
		t.Errorf("ObjectID = %q, want %q", detectors[0].ObjectID, "obj-1")
	}
	if !detectors[0].Enabled {
		t.Error("Enabled = false, want true")
	}
	if detectors[0].AnalyzerShort != "static (>90)" {
		t.Errorf("AnalyzerShort = %q, want %q", detectors[0].AnalyzerShort, "static (>90)")
	}
	if detectors[0].EventType != "PERFORMANCE_EVENT" {
		t.Errorf("EventType = %q, want %q", detectors[0].EventType, "PERFORMANCE_EVENT")
	}
}

func TestList_Pagination(t *testing.T) {
	callCount := 0
	h, server := newTestHandler(t, func(w http.ResponseWriter, r *http.Request) {
		if settingsConstraintGuard(t, w, r) {
			return
		}
		w.Header().Set("Content-Type", "application/json")
		callCount++

		if callCount == 1 {
			// First page: verify schemaIds and scopes are sent
			if r.URL.Query().Get("schemaIds") != SchemaID {
				t.Errorf("page 1: schemaIds = %q, want %q", r.URL.Query().Get("schemaIds"), SchemaID)
			}
			if r.URL.Query().Get("scopes") != Scope {
				t.Errorf("page 1: scopes = %q, want %q", r.URL.Query().Get("scopes"), Scope)
			}
			json.NewEncoder(w).Encode(listResponse{
				Items:       []settingsItem{sampleItem("obj-1", "Alpha", true)},
				TotalCount:  2,
				NextPageKey: "page2key",
			})
		} else {
			// Second page: verify ONLY nextPageKey is sent (no schemaIds, scopes, pageSize)
			if r.URL.Query().Get("nextPageKey") != "page2key" {
				t.Errorf("page 2: nextPageKey = %q, want %q", r.URL.Query().Get("nextPageKey"), "page2key")
			}
			json.NewEncoder(w).Encode(listResponse{
				Items:      []settingsItem{sampleItem("obj-2", "Beta", false)},
				TotalCount: 2,
			})
		}
	})
	defer server.Close()

	detectors, err := h.List(ListOptions{})
	if err != nil {
		t.Fatalf("List() error = %v", err)
	}
	if callCount != 2 {
		t.Fatalf("expected 2 API calls, got %d", callCount)
	}
	if len(detectors) != 2 {
		t.Fatalf("List() returned %d items, want 2", len(detectors))
	}
	// Results should be sorted by title
	if detectors[0].Title != "Alpha" || detectors[1].Title != "Beta" {
		t.Errorf("unexpected order: %q, %q", detectors[0].Title, detectors[1].Title)
	}
}

func TestList_EnabledFilter(t *testing.T) {
	h, server := newTestHandler(t, func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		json.NewEncoder(w).Encode(listResponse{
			Items: []settingsItem{
				sampleItem("obj-1", "Enabled", true),
				sampleItem("obj-2", "Disabled", false),
			},
			TotalCount: 2,
		})
	})
	defer server.Close()

	enabledTrue := true
	detectors, err := h.List(ListOptions{Enabled: &enabledTrue})
	if err != nil {
		t.Fatalf("List() error = %v", err)
	}
	if len(detectors) != 1 {
		t.Fatalf("List(enabled=true) returned %d items, want 1", len(detectors))
	}
	if detectors[0].Title != "Enabled" {
		t.Errorf("Title = %q, want %q", detectors[0].Title, "Enabled")
	}

	enabledFalse := false
	detectors, err = h.List(ListOptions{Enabled: &enabledFalse})
	if err != nil {
		t.Fatalf("List() error = %v", err)
	}
	if len(detectors) != 1 {
		t.Fatalf("List(enabled=false) returned %d items, want 1", len(detectors))
	}
	if detectors[0].Title != "Disabled" {
		t.Errorf("Title = %q, want %q", detectors[0].Title, "Disabled")
	}
}

func TestList_ServerError(t *testing.T) {
	h, server := newTestHandler(t, func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusInternalServerError)
		w.Write([]byte("boom"))
	})
	defer server.Close()

	_, err := h.List(ListOptions{})
	if err == nil {
		t.Fatal("List() expected error, got nil")
	}
	if !strings.Contains(err.Error(), "failed to list") {
		t.Errorf("List() error = %q, want to contain %q", err.Error(), "failed to list")
	}
}

// ---------------------------------------------------------------------------
// Get tests
// ---------------------------------------------------------------------------

func TestGet_Success(t *testing.T) {
	h, server := newTestHandler(t, func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		item := sampleItem("obj-1", "CPU Alert", true)
		json.NewEncoder(w).Encode(item)
	})
	defer server.Close()

	ad, err := h.Get("obj-1")
	if err != nil {
		t.Fatalf("Get() error = %v", err)
	}
	if ad.ObjectID != "obj-1" {
		t.Errorf("ObjectID = %q, want %q", ad.ObjectID, "obj-1")
	}
	if ad.Title != "CPU Alert" {
		t.Errorf("Title = %q, want %q", ad.Title, "CPU Alert")
	}
}

func TestGet_StatusMapping(t *testing.T) {
	tests := []struct {
		status  int
		wantErr string
	}{
		{status: 404, wantErr: "not found"},
		{status: 403, wantErr: "access denied"},
		{status: 500, wantErr: "failed to get anomaly detector: status 500"},
	}
	for _, tc := range tests {
		t.Run(fmt.Sprintf("status_%d", tc.status), func(t *testing.T) {
			h, server := newTestHandler(t, func(w http.ResponseWriter, r *http.Request) {
				w.WriteHeader(tc.status)
				w.Write([]byte("error"))
			})
			defer server.Close()

			_, err := h.Get("obj-1")
			if err == nil || !strings.Contains(err.Error(), tc.wantErr) {
				t.Fatalf("Get() error = %v, want to contain %q", err, tc.wantErr)
			}
		})
	}
}

// ---------------------------------------------------------------------------
// FindByName tests
// ---------------------------------------------------------------------------

func TestFindByName_ExactMatch(t *testing.T) {
	h, server := newTestHandler(t, func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		json.NewEncoder(w).Encode(listResponse{
			Items: []settingsItem{
				sampleItem("obj-1", "CPU Alert", true),
				sampleItem("obj-2", "CPU Alert - Production", true),
			},
			TotalCount: 2,
		})
	})
	defer server.Close()

	ad, err := h.FindByName("CPU Alert")
	if err != nil {
		t.Fatalf("FindByName() error = %v", err)
	}
	// Should match exactly, not the prefix match
	if ad.ObjectID != "obj-1" {
		t.Errorf("ObjectID = %q, want %q (exact match)", ad.ObjectID, "obj-1")
	}
}

func TestFindByName_PrefixMatch(t *testing.T) {
	h, server := newTestHandler(t, func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		json.NewEncoder(w).Encode(listResponse{
			Items:      []settingsItem{sampleItem("obj-1", "CPU Alert - Production", true)},
			TotalCount: 1,
		})
	})
	defer server.Close()

	ad, err := h.FindByName("CPU")
	if err != nil {
		t.Fatalf("FindByName() error = %v", err)
	}
	if ad.ObjectID != "obj-1" {
		t.Errorf("ObjectID = %q, want %q", ad.ObjectID, "obj-1")
	}
}

func TestFindByName_CaseInsensitive(t *testing.T) {
	h, server := newTestHandler(t, func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		json.NewEncoder(w).Encode(listResponse{
			Items:      []settingsItem{sampleItem("obj-1", "CPU Alert", true)},
			TotalCount: 1,
		})
	})
	defer server.Close()

	ad, err := h.FindByName("cpu alert")
	if err != nil {
		t.Fatalf("FindByName() error = %v", err)
	}
	if ad.ObjectID != "obj-1" {
		t.Errorf("ObjectID = %q, want %q", ad.ObjectID, "obj-1")
	}
}

func TestFindByName_NotFound(t *testing.T) {
	h, server := newTestHandler(t, func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		json.NewEncoder(w).Encode(listResponse{
			Items:      []settingsItem{sampleItem("obj-1", "CPU Alert", true)},
			TotalCount: 1,
		})
	})
	defer server.Close()

	_, err := h.FindByName("Memory Alert")
	if err == nil {
		t.Fatal("FindByName() expected error, got nil")
	}
	if !strings.Contains(err.Error(), "not found") {
		t.Errorf("FindByName() error = %q, want to contain %q", err.Error(), "not found")
	}
}

// ---------------------------------------------------------------------------
// Create tests
// ---------------------------------------------------------------------------

func TestCreate_FlattenedFormat(t *testing.T) {
	h, server := newTestHandler(t, func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		switch r.Method {
		case http.MethodPost:
			// Verify the POST body is an array
			var body []map[string]any
			if err := json.NewDecoder(r.Body).Decode(&body); err != nil {
				t.Fatalf("failed to decode POST body: %v", err)
			}
			if len(body) != 1 {
				t.Fatalf("POST body has %d items, want 1", len(body))
			}
			if body[0]["schemaId"] != SchemaID {
				t.Errorf("schemaId = %v, want %q", body[0]["schemaId"], SchemaID)
			}
			if body[0]["scope"] != Scope {
				t.Errorf("scope = %v, want %q", body[0]["scope"], Scope)
			}
			// Verify source defaults to "dtctl"
			value, ok := body[0]["value"].(map[string]any)
			if !ok {
				t.Fatal("POST body missing value field")
			}
			if value["source"] != "dtctl" {
				t.Errorf("source = %v, want %q", value["source"], "dtctl")
			}

			w.WriteHeader(http.StatusOK)
			json.NewEncoder(w).Encode([]createResponse{{ObjectID: "new-obj-1"}})
		case http.MethodGet:
			// Return the created object for the follow-up Get
			item := sampleItem("new-obj-1", "New Detector", true)
			json.NewEncoder(w).Encode(item)
		}
	})
	defer server.Close()

	data := []byte(`{
		"title": "New Detector",
		"enabled": true,
		"analyzer": {
			"name": "dt.statistics.ui.anomaly_detection.StaticThresholdAnomalyDetectionAnalyzer",
			"input": {"threshold": "90", "alertCondition": "ABOVE"}
		},
		"eventTemplate": {"event.type": "PERFORMANCE_EVENT", "event.name": "High CPU"}
	}`)

	result, err := h.Create(data)
	if err != nil {
		t.Fatalf("Create() error = %v", err)
	}
	if result.ObjectID != "new-obj-1" {
		t.Errorf("ObjectID = %q, want %q", result.ObjectID, "new-obj-1")
	}
}

func TestCreate_RawSettingsFormat(t *testing.T) {
	h, server := newTestHandler(t, func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		switch r.Method {
		case http.MethodPost:
			var body []map[string]any
			if err := json.NewDecoder(r.Body).Decode(&body); err != nil {
				t.Fatalf("failed to decode POST body: %v", err)
			}
			if body[0]["schemaId"] != SchemaID {
				t.Errorf("schemaId = %v, want %q", body[0]["schemaId"], SchemaID)
			}
			w.WriteHeader(http.StatusOK)
			json.NewEncoder(w).Encode([]createResponse{{ObjectID: "new-obj-2"}})
		case http.MethodGet:
			item := sampleItem("new-obj-2", "Raw Detector", true)
			json.NewEncoder(w).Encode(item)
		}
	})
	defer server.Close()

	data := []byte(fmt.Sprintf(`{
		"schemaId": "%s",
		"scope": "%s",
		"value": {
			"title": "Raw Detector",
			"enabled": true,
			"source": "Clouds",
			"analyzer": {"name": "dt.statistics.ui.anomaly_detection.StaticThresholdAnomalyDetectionAnalyzer"},
			"eventTemplate": {"properties": [{"key": "event.type", "value": "PERFORMANCE_EVENT"}]}
		}
	}`, SchemaID, Scope))

	result, err := h.Create(data)
	if err != nil {
		t.Fatalf("Create() error = %v", err)
	}
	if result.ObjectID != "new-obj-2" {
		t.Errorf("ObjectID = %q, want %q", result.ObjectID, "new-obj-2")
	}
}

func TestCreate_StatusMapping(t *testing.T) {
	tests := []struct {
		status  int
		wantErr string
	}{
		{status: 400, wantErr: "invalid anomaly detector"},
		{status: 403, wantErr: "access denied"},
		{status: 404, wantErr: fmt.Sprintf("schema %q not found", SchemaID)},
		{status: 500, wantErr: "failed to create anomaly detector: status 500"},
	}
	for _, tc := range tests {
		t.Run(fmt.Sprintf("status_%d", tc.status), func(t *testing.T) {
			h, server := newTestHandler(t, func(w http.ResponseWriter, r *http.Request) {
				w.Header().Set("Content-Type", "application/json")
				w.WriteHeader(tc.status)
				w.Write([]byte("boom"))
			})
			defer server.Close()

			data := []byte(`{"title":"x","analyzer":{"name":"dt.statistics.ui.anomaly_detection.StaticThresholdAnomalyDetectionAnalyzer"},"eventTemplate":{"event.type":"PERFORMANCE_EVENT"}}`)
			_, err := h.Create(data)
			if err == nil || !strings.Contains(err.Error(), tc.wantErr) {
				t.Fatalf("Create() error = %v, want to contain %q", err, tc.wantErr)
			}
		})
	}
}

// ---------------------------------------------------------------------------
// Update tests
// ---------------------------------------------------------------------------

func TestUpdate_Success(t *testing.T) {
	h, server := newTestHandler(t, func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		switch r.Method {
		case http.MethodGet:
			item := sampleItem("obj-1", "Updated Detector", true)
			json.NewEncoder(w).Encode(item)
		case http.MethodPut:
			// Verify If-Match header
			ifMatch := r.Header.Get("If-Match")
			if ifMatch != "1.0.15" {
				t.Errorf("If-Match = %q, want %q", ifMatch, "1.0.15")
			}
			w.WriteHeader(http.StatusOK)
		}
	})
	defer server.Close()

	data := []byte(`{"title":"Updated Detector","enabled":true,"analyzer":{"name":"dt.statistics.ui.anomaly_detection.StaticThresholdAnomalyDetectionAnalyzer"},"eventTemplate":{"event.type":"PERFORMANCE_EVENT"}}`)
	result, err := h.Update("obj-1", data)
	if err != nil {
		t.Fatalf("Update() error = %v", err)
	}
	if result.Title != "Updated Detector" {
		t.Errorf("Title = %q, want %q", result.Title, "Updated Detector")
	}
}

func TestUpdate_StatusMapping(t *testing.T) {
	tests := []struct {
		status  int
		wantErr string
	}{
		{status: 400, wantErr: "invalid anomaly detector"},
		{status: 403, wantErr: "access denied"},
		{status: 404, wantErr: "not found"},
		{status: 409, wantErr: "version conflict"},
		{status: 412, wantErr: "version conflict"},
		{status: 500, wantErr: "failed to update anomaly detector: status 500"},
	}
	for _, tc := range tests {
		t.Run(fmt.Sprintf("status_%d", tc.status), func(t *testing.T) {
			h, server := newTestHandler(t, func(w http.ResponseWriter, r *http.Request) {
				w.Header().Set("Content-Type", "application/json")
				switch r.Method {
				case http.MethodGet:
					item := sampleItem("obj-1", "Existing", true)
					json.NewEncoder(w).Encode(item)
				case http.MethodPut:
					w.WriteHeader(tc.status)
					w.Write([]byte("boom"))
				}
			})
			defer server.Close()

			data := []byte(`{"title":"x","analyzer":{"name":"dt.statistics.ui.anomaly_detection.StaticThresholdAnomalyDetectionAnalyzer"},"eventTemplate":{"event.type":"PERFORMANCE_EVENT"}}`)
			_, err := h.Update("obj-1", data)
			if err == nil || !strings.Contains(err.Error(), tc.wantErr) {
				t.Fatalf("Update() error = %v, want to contain %q", err, tc.wantErr)
			}
		})
	}
}

// ---------------------------------------------------------------------------
// Delete tests
// ---------------------------------------------------------------------------

func TestDelete_Success(t *testing.T) {
	h, server := newTestHandler(t, func(w http.ResponseWriter, r *http.Request) {
		if r.Method == http.MethodDelete {
			w.WriteHeader(http.StatusNoContent)
			return
		}
		w.WriteHeader(http.StatusNotFound)
	})
	defer server.Close()

	if err := h.Delete("obj-1"); err != nil {
		t.Fatalf("Delete() error = %v", err)
	}
}

func TestDelete_StatusMapping(t *testing.T) {
	tests := []struct {
		status  int
		wantErr string
	}{
		{status: 403, wantErr: "access denied"},
		{status: 404, wantErr: "not found"},
		{status: 500, wantErr: "failed to delete anomaly detector: status 500"},
	}
	for _, tc := range tests {
		t.Run(fmt.Sprintf("status_%d", tc.status), func(t *testing.T) {
			h, server := newTestHandler(t, func(w http.ResponseWriter, r *http.Request) {
				w.WriteHeader(tc.status)
				w.Write([]byte("boom"))
			})
			defer server.Close()

			err := h.Delete("obj-1")
			if err == nil || !strings.Contains(err.Error(), tc.wantErr) {
				t.Fatalf("Delete() error = %v, want to contain %q", err, tc.wantErr)
			}
		})
	}
}

// ---------------------------------------------------------------------------
// Format conversion tests
// ---------------------------------------------------------------------------

func TestDeriveAnalyzerShort(t *testing.T) {
	tests := []struct {
		name  string
		value map[string]any
		want  string
	}{
		{
			name: "static threshold with values",
			value: map[string]any{
				"analyzer": map[string]any{
					"name": "dt.statistics.ui.anomaly_detection.StaticThresholdAnomalyDetectionAnalyzer",
					"input": []any{
						map[string]any{"key": "alertCondition", "value": "ABOVE"},
						map[string]any{"key": "threshold", "value": "90"},
					},
				},
			},
			want: "static (>90)",
		},
		{
			name: "static threshold BELOW",
			value: map[string]any{
				"analyzer": map[string]any{
					"name": "dt.statistics.ui.anomaly_detection.StaticThresholdAnomalyDetectionAnalyzer",
					"input": []any{
						map[string]any{"key": "alertCondition", "value": "BELOW"},
						map[string]any{"key": "threshold", "value": "10"},
					},
				},
			},
			want: "static (<10)",
		},
		{
			name: "static threshold no threshold value",
			value: map[string]any{
				"analyzer": map[string]any{
					"name":  "dt.statistics.ui.anomaly_detection.StaticThresholdAnomalyDetectionAnalyzer",
					"input": []any{},
				},
			},
			want: "static",
		},
		{
			name: "auto-adaptive",
			value: map[string]any{
				"analyzer": map[string]any{
					"name": "dt.statistics.ui.anomaly_detection.AutoAdaptiveAnomalyDetectionAnalyzer",
				},
			},
			want: "auto-adaptive",
		},
		{
			name:  "missing analyzer",
			value: map[string]any{},
			want:  "",
		},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			got := deriveAnalyzerShort(tc.value)
			if got != tc.want {
				t.Errorf("deriveAnalyzerShort() = %q, want %q", got, tc.want)
			}
		})
	}
}

func TestDeriveEventType(t *testing.T) {
	value := map[string]any{
		"eventTemplate": map[string]any{
			"properties": []any{
				map[string]any{"key": "event.type", "value": "AVAILABILITY_EVENT"},
				map[string]any{"key": "event.name", "value": "Test"},
			},
		},
	}
	got := deriveEventType(value)
	if got != "AVAILABILITY_EVENT" {
		t.Errorf("deriveEventType() = %q, want %q", got, "AVAILABILITY_EVENT")
	}
}

func TestExtractEventName(t *testing.T) {
	value := map[string]any{
		"eventTemplate": map[string]any{
			"properties": []any{
				map[string]any{"key": "event.type", "value": "PERFORMANCE_EVENT"},
				map[string]any{"key": "event.name", "value": "High CPU"},
			},
		},
	}
	got := ExtractEventName(value)
	if got != "High CPU" {
		t.Errorf("ExtractEventName() = %q, want %q", got, "High CPU")
	}
}

func TestExtractKVMap(t *testing.T) {
	t.Run("array format", func(t *testing.T) {
		parent := map[string]any{
			"input": []any{
				map[string]any{"key": "threshold", "value": "90"},
				map[string]any{"key": "condition", "value": "ABOVE"},
			},
		}
		result := ExtractKVMap(parent, "input")
		if result["threshold"] != "90" || result["condition"] != "ABOVE" {
			t.Errorf("ExtractKVMap() = %v, unexpected", result)
		}
	})

	t.Run("map format", func(t *testing.T) {
		parent := map[string]any{
			"input": map[string]any{
				"threshold": "90",
				"condition": "ABOVE",
			},
		}
		result := ExtractKVMap(parent, "input")
		if result["threshold"] != "90" || result["condition"] != "ABOVE" {
			t.Errorf("ExtractKVMap() = %v, unexpected", result)
		}
	})

	t.Run("missing field", func(t *testing.T) {
		parent := map[string]any{}
		result := ExtractKVMap(parent, "input")
		if len(result) != 0 {
			t.Errorf("ExtractKVMap() = %v, want empty", result)
		}
	})
}

func TestToFlattenedYAML(t *testing.T) {
	value := map[string]any{
		"title":       "Test Detector",
		"enabled":     true,
		"description": "A test",
		"source":      "dtctl",
		"analyzer": map[string]any{
			"name": "dt.statistics.ui.anomaly_detection.StaticThresholdAnomalyDetectionAnalyzer",
			"input": []any{
				map[string]any{"key": "threshold", "value": "90"},
			},
		},
		"eventTemplate": map[string]any{
			"properties": []any{
				map[string]any{"key": "event.type", "value": "PERFORMANCE_EVENT"},
			},
		},
	}

	flat := ToFlattenedYAML(value)

	if flat["title"] != "Test Detector" {
		t.Errorf("title = %v, want %q", flat["title"], "Test Detector")
	}
	if flat["source"] != "dtctl" {
		t.Errorf("source = %v, want %q", flat["source"], "dtctl")
	}

	// Analyzer input should be flattened to a map
	analyzer, ok := flat["analyzer"].(map[string]any)
	if !ok {
		t.Fatal("analyzer is not a map")
	}
	input, ok := analyzer["input"].(map[string]string)
	if !ok {
		t.Fatal("analyzer.input is not a map[string]string")
	}
	if input["threshold"] != "90" {
		t.Errorf("analyzer.input.threshold = %q, want %q", input["threshold"], "90")
	}

	// Event template should be flattened to a map
	et, ok := flat["eventTemplate"].(map[string]string)
	if !ok {
		t.Fatal("eventTemplate is not a map[string]string")
	}
	if et["event.type"] != "PERFORMANCE_EVENT" {
		t.Errorf("eventTemplate[event.type] = %q, want %q", et["event.type"], "PERFORMANCE_EVENT")
	}
}

func TestIsRawSettingsFormat(t *testing.T) {
	t.Run("raw format", func(t *testing.T) {
		data := []byte(fmt.Sprintf(`{"schemaId":"%s","scope":"environment","value":{}}`, SchemaID))
		if !IsRawSettingsFormat(data) {
			t.Error("IsRawSettingsFormat() = false, want true")
		}
	})

	t.Run("flattened format", func(t *testing.T) {
		data := []byte(`{"title":"x","analyzer":{"name":"y"},"eventTemplate":{}}`)
		if IsRawSettingsFormat(data) {
			t.Error("IsRawSettingsFormat() = true, want false")
		}
	})

	t.Run("wrong schema", func(t *testing.T) {
		data := []byte(`{"schemaId":"builtin:other","scope":"environment","value":{}}`)
		if IsRawSettingsFormat(data) {
			t.Error("IsRawSettingsFormat() = true for wrong schema, want false")
		}
	})
}

func TestIsFlattenedFormat(t *testing.T) {
	t.Run("flattened format", func(t *testing.T) {
		data := []byte(`{"title":"x","analyzer":{"name":"y"},"eventTemplate":{"event.type":"z"}}`)
		if !IsFlattenedFormat(data) {
			t.Error("IsFlattenedFormat() = false, want true")
		}
	})

	t.Run("raw format", func(t *testing.T) {
		data := []byte(fmt.Sprintf(`{"schemaId":"%s","value":{}}`, SchemaID))
		if IsFlattenedFormat(data) {
			t.Error("IsFlattenedFormat() = true for raw format, want false")
		}
	})
}

func TestFlattenedToAPIValue_Defaults(t *testing.T) {
	// Verify source defaults to "dtctl" when omitted
	raw := map[string]any{
		"title": "Test",
		"analyzer": map[string]any{
			"name": "dt.statistics.ui.anomaly_detection.StaticThresholdAnomalyDetectionAnalyzer",
		},
	}
	value, err := flattenedToAPIValue(raw)
	if err != nil {
		t.Fatalf("flattenedToAPIValue() error = %v", err)
	}
	if value["source"] != "dtctl" {
		t.Errorf("source = %v, want %q", value["source"], "dtctl")
	}
	if value["enabled"] != true {
		t.Errorf("enabled = %v, want true", value["enabled"])
	}
}

func TestFlattenedToAPIValue_RequiredFields(t *testing.T) {
	t.Run("missing title", func(t *testing.T) {
		raw := map[string]any{
			"analyzer": map[string]any{"name": "x"},
		}
		_, err := flattenedToAPIValue(raw)
		if err == nil || !strings.Contains(err.Error(), "title") {
			t.Fatalf("flattenedToAPIValue() error = %v, want error about title", err)
		}
	})

	t.Run("missing analyzer", func(t *testing.T) {
		raw := map[string]any{
			"title": "Test",
		}
		_, err := flattenedToAPIValue(raw)
		if err == nil || !strings.Contains(err.Error(), "analyzer") {
			t.Fatalf("flattenedToAPIValue() error = %v, want error about analyzer", err)
		}
	})

	t.Run("missing analyzer.name", func(t *testing.T) {
		raw := map[string]any{
			"title":    "Test",
			"analyzer": map[string]any{},
		}
		_, err := flattenedToAPIValue(raw)
		if err == nil || !strings.Contains(err.Error(), "analyzer.name") {
			t.Fatalf("flattenedToAPIValue() error = %v, want error about analyzer.name", err)
		}
	})
}

func TestMapToKVArray_Deterministic(t *testing.T) {
	m := map[string]any{
		"c": "3",
		"a": "1",
		"b": "2",
	}
	result := mapToKVArray(m)
	if len(result) != 3 {
		t.Fatalf("mapToKVArray() returned %d items, want 3", len(result))
	}
	// Should be sorted by key
	if result[0]["key"] != "a" || result[1]["key"] != "b" || result[2]["key"] != "c" {
		t.Errorf("mapToKVArray() not sorted: %v", result)
	}
}
