package segment

import (
	"encoding/json"
	"errors"
	"fmt"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/dynatrace-oss/dtctl/pkg/client"
)

func TestNewHandler(t *testing.T) {
	c, err := client.New("https://test.dynatrace.com", "test-token")
	if err != nil {
		t.Fatalf("failed to create client: %v", err)
	}
	h := NewHandler(c)

	if h == nil {
		t.Fatal("NewHandler() returned nil")
	}
	if h.client == nil {
		t.Error("Handler.client is nil")
	}
}

func TestList(t *testing.T) {
	tests := []struct {
		name          string
		statusCode    int
		responseBody  interface{}
		expectError   bool
		errorContains string
		validate      func(*testing.T, *FilterSegmentList)
	}{
		{
			name:       "successful list",
			statusCode: 200,
			responseBody: FilterSegmentList{
				FilterSegments: []FilterSegment{
					{
						UID:      "seg-uid-001",
						Name:     "k8s-alpha",
						IsPublic: true,
						Owner:    "user@example.invalid",
					},
					{
						UID:      "seg-uid-002",
						Name:     "prod-logs",
						IsPublic: false,
						Owner:    "admin@example.invalid",
					},
				},
				TotalCount: 2,
			},
			expectError: false,
			validate: func(t *testing.T, result *FilterSegmentList) {
				if len(result.FilterSegments) != 2 {
					t.Errorf("expected 2 segments, got %d", len(result.FilterSegments))
				}
				if result.FilterSegments[0].UID != "seg-uid-001" {
					t.Errorf("expected first segment UID 'seg-uid-001', got %q", result.FilterSegments[0].UID)
				}
				if result.FilterSegments[1].Name != "prod-logs" {
					t.Errorf("expected second segment name 'prod-logs', got %q", result.FilterSegments[1].Name)
				}
			},
		},
		{
			name:       "empty list",
			statusCode: 200,
			responseBody: FilterSegmentList{
				FilterSegments: []FilterSegment{},
				TotalCount:     0,
			},
			expectError: false,
			validate: func(t *testing.T, result *FilterSegmentList) {
				if len(result.FilterSegments) != 0 {
					t.Errorf("expected 0 segments, got %d", len(result.FilterSegments))
				}
			},
		},
		{
			name:          "server error",
			statusCode:    500,
			responseBody:  "internal server error",
			expectError:   true,
			errorContains: "status 500",
		},
		{
			name:          "forbidden",
			statusCode:    403,
			responseBody:  "access denied",
			expectError:   true,
			errorContains: "status 403",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
				if r.URL.Path != "/platform/storage/filter-segments/v1/filter-segments" {
					t.Errorf("expected path '/platform/storage/filter-segments/v1/filter-segments', got %q", r.URL.Path)
				}
				// The list endpoint does not support pagination params
				if r.URL.Query().Get("page-size") != "" {
					t.Error("list endpoint should not send page-size (API has no pagination)")
				}
				w.WriteHeader(tt.statusCode)
				if str, ok := tt.responseBody.(string); ok {
					w.Write([]byte(str))
				} else {
					json.NewEncoder(w).Encode(tt.responseBody)
				}
			}))
			defer server.Close()

			c, err := client.NewForTesting(server.URL, "test-token")
			if err != nil {
				t.Fatalf("failed to create client: %v", err)
			}
			h := NewHandler(c)

			result, err := h.List()

			if tt.expectError {
				if err == nil {
					t.Error("expected error, got nil")
				} else if tt.errorContains != "" && !strings.Contains(err.Error(), tt.errorContains) {
					t.Errorf("expected error containing %q, got %q", tt.errorContains, err.Error())
				}
			} else {
				if err != nil {
					t.Errorf("unexpected error: %v", err)
				}
				if tt.validate != nil {
					tt.validate(t, result)
				}
			}
		})
	}
}

func TestGet(t *testing.T) {
	tests := []struct {
		name          string
		uid           string
		statusCode    int
		responseBody  interface{}
		expectError   bool
		errorContains string
		validate      func(*testing.T, *FilterSegment)
	}{
		{
			name:       "successful get",
			uid:        "seg-uid-001",
			statusCode: 200,
			responseBody: FilterSegment{
				UID:         "seg-uid-001",
				Name:        "k8s-alpha",
				Description: "Kubernetes cluster alpha",
				IsPublic:    true,
				Owner:       "user@example.invalid",
				Version:     3,
				Includes: []Include{
					{DataObject: "_all_data_object", Filter: `k8s.cluster.name = "alpha"`},
					{DataObject: "logs", Filter: `dt.system.bucket = "custom-logs"`},
				},
				Variables: &Variables{
					Type:  "query",
					Value: `fetch logs | limit 1`,
				},
				AllowedOperations: []string{"READ", "WRITE", "DELETE"},
			},
			expectError: false,
			validate: func(t *testing.T, seg *FilterSegment) {
				if seg.UID != "seg-uid-001" {
					t.Errorf("expected UID 'seg-uid-001', got %q", seg.UID)
				}
				if seg.Name != "k8s-alpha" {
					t.Errorf("expected name 'k8s-alpha', got %q", seg.Name)
				}
				if len(seg.Includes) != 2 {
					t.Errorf("expected 2 includes, got %d", len(seg.Includes))
				}
				if seg.Includes[0].DataObject != "_all_data_object" {
					t.Errorf("expected first include dataObject '_all_data_object', got %q", seg.Includes[0].DataObject)
				}
				if seg.Variables == nil {
					t.Error("expected variables to be non-nil")
				} else {
					if seg.Variables.Type != "query" {
						t.Errorf("expected variables type 'query', got %q", seg.Variables.Type)
					}
					if seg.Variables.Value != "fetch logs | limit 1" {
						t.Errorf("expected variables value 'fetch logs | limit 1', got %q", seg.Variables.Value)
					}
				}
			},
		},
		{
			name:       "successful get with add-fields params",
			uid:        "seg-uid-002",
			statusCode: 200,
			responseBody: FilterSegment{
				UID:      "seg-uid-002",
				Name:     "test",
				IsPublic: false,
				Version:  1,
			},
			expectError: false,
			validate: func(t *testing.T, seg *FilterSegment) {
				// Just validates that the request succeeds
				if seg.UID != "seg-uid-002" {
					t.Errorf("expected UID 'seg-uid-002', got %q", seg.UID)
				}
			},
		},
		{
			name:          "segment not found",
			uid:           "non-existent",
			statusCode:    404,
			responseBody:  "not found",
			expectError:   true,
			errorContains: "not found",
		},
		{
			name:          "server error",
			uid:           "seg-uid-001",
			statusCode:    500,
			responseBody:  "internal error",
			expectError:   true,
			errorContains: "status 500",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
				expectedPath := fmt.Sprintf("/platform/storage/filter-segments/v1/filter-segments/%s", tt.uid)
				if r.URL.Path != expectedPath {
					t.Errorf("expected path %q, got %q", expectedPath, r.URL.Path)
				}
				// Verify add-fields params are valid enum values
				addFields := r.URL.Query()["add-fields"]
				for _, f := range addFields {
					switch f {
					case "INCLUDES", "VARIABLES", "EXTERNALID", "RESOURCECONTEXT":
						// valid
					default:
						t.Errorf("invalid add-fields value: %q", f)
					}
				}
				w.WriteHeader(tt.statusCode)
				if str, ok := tt.responseBody.(string); ok {
					w.Write([]byte(str))
				} else {
					json.NewEncoder(w).Encode(tt.responseBody)
				}
			}))
			defer server.Close()

			c, err := client.NewForTesting(server.URL, "test-token")
			if err != nil {
				t.Fatalf("failed to create client: %v", err)
			}
			h := NewHandler(c)

			result, err := h.Get(tt.uid)

			if tt.expectError {
				if err == nil {
					t.Error("expected error, got nil")
				} else if tt.errorContains != "" && !strings.Contains(err.Error(), tt.errorContains) {
					t.Errorf("expected error containing %q, got %q", tt.errorContains, err.Error())
				}
			} else {
				if err != nil {
					t.Errorf("unexpected error: %v", err)
				}
				if tt.validate != nil {
					tt.validate(t, result)
				}
			}
		})
	}
}

func TestCreate(t *testing.T) {
	tests := []struct {
		name          string
		input         FilterSegment
		statusCode    int
		responseBody  interface{}
		expectError   bool
		errorContains string
		validate      func(*testing.T, *FilterSegment)
	}{
		{
			name: "successful create",
			input: FilterSegment{
				Name:     "new-segment",
				IsPublic: true,
				Includes: []Include{{DataObject: "logs", Filter: `status = "ERROR"`}},
			},
			statusCode: 201,
			responseBody: FilterSegment{
				UID:      "seg-new-001",
				Name:     "new-segment",
				IsPublic: true,
				Owner:    "user@example.invalid",
				Version:  1,
				Includes: []Include{{DataObject: "logs", Filter: `status = "ERROR"`}},
			},
			expectError: false,
			validate: func(t *testing.T, seg *FilterSegment) {
				if seg.UID != "seg-new-001" {
					t.Errorf("expected UID 'seg-new-001', got %q", seg.UID)
				}
				if seg.Name != "new-segment" {
					t.Errorf("expected name 'new-segment', got %q", seg.Name)
				}
			},
		},
		{
			name: "invalid definition",
			input: FilterSegment{
				Name: "",
			},
			statusCode:    400,
			responseBody:  "invalid segment definition",
			expectError:   true,
			errorContains: "invalid segment definition",
		},
		{
			name: "access denied",
			input: FilterSegment{
				Name: "denied-segment",
			},
			statusCode:    403,
			responseBody:  "access denied",
			expectError:   true,
			errorContains: "access denied",
		},
		{
			name: "conflict - segment already exists",
			input: FilterSegment{
				Name: "duplicate-segment",
			},
			statusCode:    409,
			responseBody:  `{"error":"segment already exists"}`,
			expectError:   true,
			errorContains: "segment already exists",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
				if r.Method != "POST" {
					t.Errorf("expected POST method, got %s", r.Method)
				}
				if r.URL.Path != "/platform/storage/filter-segments/v1/filter-segments" {
					t.Errorf("expected path '/platform/storage/filter-segments/v1/filter-segments', got %q", r.URL.Path)
				}
				w.WriteHeader(tt.statusCode)
				if str, ok := tt.responseBody.(string); ok {
					w.Write([]byte(str))
				} else {
					json.NewEncoder(w).Encode(tt.responseBody)
				}
			}))
			defer server.Close()

			c, err := client.NewForTesting(server.URL, "test-token")
			if err != nil {
				t.Fatalf("failed to create client: %v", err)
			}
			h := NewHandler(c)

			inputJSON, _ := json.Marshal(tt.input)
			result, err := h.Create(inputJSON)

			if tt.expectError {
				if err == nil {
					t.Error("expected error, got nil")
				} else if tt.errorContains != "" && !strings.Contains(err.Error(), tt.errorContains) {
					t.Errorf("expected error containing %q, got %q", tt.errorContains, err.Error())
				}
			} else {
				if err != nil {
					t.Errorf("unexpected error: %v", err)
				}
				if tt.validate != nil {
					tt.validate(t, result)
				}
			}
		})
	}
}

func TestUpdate(t *testing.T) {
	tests := []struct {
		name          string
		uid           string
		version       int
		statusCode    int
		responseBody  string
		expectError   bool
		errorContains string
	}{
		{
			name:        "successful update",
			uid:         "seg-uid-001",
			version:     3,
			statusCode:  200,
			expectError: false,
		},
		{
			name:          "segment not found",
			uid:           "non-existent",
			version:       1,
			statusCode:    404,
			responseBody:  "not found",
			expectError:   true,
			errorContains: "not found",
		},
		{
			name:          "version conflict",
			uid:           "seg-uid-001",
			version:       2,
			statusCode:    409,
			responseBody:  "version conflict",
			expectError:   true,
			errorContains: "version conflict",
		},
		{
			name:          "invalid definition",
			uid:           "seg-uid-001",
			version:       1,
			statusCode:    400,
			responseBody:  "invalid",
			expectError:   true,
			errorContains: "invalid segment definition",
		},
		{
			name:          "access denied",
			uid:           "seg-uid-001",
			version:       1,
			statusCode:    403,
			responseBody:  "access denied",
			expectError:   true,
			errorContains: "access denied",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
				if r.Method != "PUT" {
					t.Errorf("expected PUT method, got %s", r.Method)
				}
				expectedPath := fmt.Sprintf("/platform/storage/filter-segments/v1/filter-segments/%s", tt.uid)
				if r.URL.Path != expectedPath {
					t.Errorf("expected path %q, got %q", expectedPath, r.URL.Path)
				}
				// Verify optimistic-locking-version is sent
				lockVer := r.URL.Query().Get("optimistic-locking-version")
				if lockVer == "" {
					t.Error("expected optimistic-locking-version query param")
				}
				expectedVer := fmt.Sprintf("%d", tt.version)
				if lockVer != expectedVer {
					t.Errorf("expected optimistic-locking-version %q, got %q", expectedVer, lockVer)
				}
				w.WriteHeader(tt.statusCode)
				w.Write([]byte(tt.responseBody))
			}))
			defer server.Close()

			c, err := client.NewForTesting(server.URL, "test-token")
			if err != nil {
				t.Fatalf("failed to create client: %v", err)
			}
			h := NewHandler(c)

			updateData := []byte(`{"name":"updated-segment","isPublic":true}`)
			err = h.Update(tt.uid, tt.version, updateData)

			if tt.expectError {
				if err == nil {
					t.Error("expected error, got nil")
				} else if tt.errorContains != "" && !strings.Contains(err.Error(), tt.errorContains) {
					t.Errorf("expected error containing %q, got %q", tt.errorContains, err.Error())
				}
			} else {
				if err != nil {
					t.Errorf("unexpected error: %v", err)
				}
			}
		})
	}
}

func TestDelete(t *testing.T) {
	tests := []struct {
		name          string
		uid           string
		statusCode    int
		responseBody  string
		expectError   bool
		errorContains string
	}{
		{
			name:        "successful delete",
			uid:         "seg-uid-001",
			statusCode:  204,
			expectError: false,
		},
		{
			name:          "segment not found",
			uid:           "non-existent",
			statusCode:    404,
			responseBody:  "not found",
			expectError:   true,
			errorContains: "not found",
		},
		{
			name:          "access denied",
			uid:           "seg-uid-001",
			statusCode:    403,
			responseBody:  "access denied",
			expectError:   true,
			errorContains: "access denied",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
				if r.Method != "DELETE" {
					t.Errorf("expected DELETE method, got %s", r.Method)
				}
				expectedPath := fmt.Sprintf("/platform/storage/filter-segments/v1/filter-segments/%s", tt.uid)
				if r.URL.Path != expectedPath {
					t.Errorf("expected path %q, got %q", expectedPath, r.URL.Path)
				}
				w.WriteHeader(tt.statusCode)
				w.Write([]byte(tt.responseBody))
			}))
			defer server.Close()

			c, err := client.NewForTesting(server.URL, "test-token")
			if err != nil {
				t.Fatalf("failed to create client: %v", err)
			}
			h := NewHandler(c)

			err = h.Delete(tt.uid)

			if tt.expectError {
				if err == nil {
					t.Error("expected error, got nil")
				} else if tt.errorContains != "" && !strings.Contains(err.Error(), tt.errorContains) {
					t.Errorf("expected error containing %q, got %q", tt.errorContains, err.Error())
				}
			} else {
				if err != nil {
					t.Errorf("unexpected error: %v", err)
				}
			}
		})
	}
}

func TestGetRaw(t *testing.T) {
	t.Run("successful get raw", func(t *testing.T) {
		expectedSegment := FilterSegment{
			UID:      "seg-uid-001",
			Name:     "test-segment",
			IsPublic: true,
			Owner:    "user@example.invalid",
			Version:  1,
			Includes: []Include{
				{DataObject: "logs", Filter: `status = "ERROR"`},
			},
		}

		server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			w.WriteHeader(200)
			json.NewEncoder(w).Encode(expectedSegment)
		}))
		defer server.Close()

		c, err := client.NewForTesting(server.URL, "test-token")
		if err != nil {
			t.Fatalf("failed to create client: %v", err)
		}
		h := NewHandler(c)

		raw, err := h.GetRaw("seg-uid-001")
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}

		// Verify it's valid JSON
		var seg FilterSegment
		if err := json.Unmarshal(raw, &seg); err != nil {
			t.Fatalf("failed to unmarshal raw JSON: %v", err)
		}

		if seg.UID != expectedSegment.UID {
			t.Errorf("expected UID %q, got %q", expectedSegment.UID, seg.UID)
		}
		if seg.Name != expectedSegment.Name {
			t.Errorf("expected name %q, got %q", expectedSegment.Name, seg.Name)
		}
	})

	t.Run("get raw with non-existent segment", func(t *testing.T) {
		server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			w.WriteHeader(404)
			w.Write([]byte("not found"))
		}))
		defer server.Close()

		c, err := client.NewForTesting(server.URL, "test-token")
		if err != nil {
			t.Fatalf("failed to create client: %v", err)
		}
		h := NewHandler(c)

		_, err = h.GetRaw("non-existent")
		if err == nil {
			t.Error("expected error, got nil")
		}
	})
}

func TestIsNotFound(t *testing.T) {
	tests := []struct {
		name     string
		err      error
		expected bool
	}{
		{
			name:     "nil error",
			err:      nil,
			expected: false,
		},
		{
			name:     "direct ErrNotFound",
			err:      ErrNotFound,
			expected: true,
		},
		{
			name:     "wrapped ErrNotFound from Get",
			err:      fmt.Errorf("segment %q: %w", "seg-uid-001", ErrNotFound),
			expected: true,
		},
		{
			name:     "double-wrapped ErrNotFound",
			err:      fmt.Errorf("failed: %w", fmt.Errorf("segment %q: %w", "x", ErrNotFound)),
			expected: true,
		},
		{
			name:     "generic error",
			err:      fmt.Errorf("failed to get segment: status 500: internal error"),
			expected: false,
		},
		{
			name:     "access denied error",
			err:      fmt.Errorf("access denied to get segment"),
			expected: false,
		},
		{
			name:     "different sentinel error",
			err:      errors.New("something else not found"),
			expected: false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := IsNotFound(tt.err)
			if got != tt.expected {
				t.Errorf("IsNotFound(%v) = %v, want %v", tt.err, got, tt.expected)
			}
		})
	}
}
