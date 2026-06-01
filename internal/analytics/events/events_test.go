package events

import (
	"testing"

	"github.com/stretchr/testify/assert"
)

func TestNewOrganizationCreatedEvent(t *testing.T) {
	tests := []struct {
		name           string
		organizationID string
		expectedEvent  Event
	}{
		{
			name:           "basic organization creation",
			organizationID: "org-12345",
			expectedEvent: Event{
				Name:  "organization created",
				OrgID: "org-12345",
				Properties: map[string]any{
					"organization": "org-12345",
				},
			},
		},
		{
			name:           "empty organization ID",
			organizationID: "",
			expectedEvent: Event{
				Name:  "organization created",
				OrgID: "",
				Properties: map[string]any{
					"organization": "",
				},
			},
		},
		{
			name:           "organization with special characters",
			organizationID: "org-test_123-αβγ",
			expectedEvent: Event{
				Name:  "organization created",
				OrgID: "org-test_123-αβγ",
				Properties: map[string]any{
					"organization": "org-test_123-αβγ",
				},
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			event := NewOrganizationCreatedEvent(tt.organizationID)

			assert.Equal(t, tt.expectedEvent.Name, event.Name)
			assert.Equal(t, tt.expectedEvent.OrgID, event.OrgID)
			assert.Equal(t, tt.expectedEvent.Properties, event.Properties)
		})
	}
}

func TestNewProjectCreatedEvent(t *testing.T) {
	tests := []struct {
		name           string
		organizationID string
		projectID      string
		expectedEvent  Event
	}{
		{
			name:           "basic project creation",
			organizationID: "org-123",
			projectID:      "proj-456",
			expectedEvent: Event{
				Name:  "project created",
				OrgID: "org-123",
				Properties: map[string]any{
					"organization": "org-123",
					"project":      "proj-456",
				},
			},
		},
		{
			name:           "empty IDs",
			organizationID: "",
			projectID:      "",
			expectedEvent: Event{
				Name:  "project created",
				OrgID: "",
				Properties: map[string]any{
					"organization": "",
					"project":      "",
				},
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			event := NewProjectCreatedEvent(tt.organizationID, tt.projectID)

			assert.Equal(t, tt.expectedEvent.Name, event.Name)
			assert.Equal(t, tt.expectedEvent.OrgID, event.OrgID)
			assert.Equal(t, tt.expectedEvent.Properties, event.Properties)
		})
	}
}

func TestNewProjectDeletedEvent(t *testing.T) {
	tests := []struct {
		name           string
		organizationID string
		projectID      string
		expectedEvent  Event
	}{
		{
			name:           "basic project deletion",
			organizationID: "org-123",
			projectID:      "proj-456",
			expectedEvent: Event{
				Name:  "project deleted",
				OrgID: "org-123",
				Properties: map[string]any{
					"organization": "org-123",
					"project":      "proj-456",
				},
			},
		},
		{
			name:           "project deletion with special chars",
			organizationID: "org-test_123",
			projectID:      "proj-test_456",
			expectedEvent: Event{
				Name:  "project deleted",
				OrgID: "org-test_123",
				Properties: map[string]any{
					"organization": "org-test_123",
					"project":      "proj-test_456",
				},
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			event := NewProjectDeletedEvent(tt.organizationID, tt.projectID)

			assert.Equal(t, tt.expectedEvent.Name, event.Name)
			assert.Equal(t, tt.expectedEvent.OrgID, event.OrgID)
			assert.Equal(t, tt.expectedEvent.Properties, event.Properties)
		})
	}
}

func TestNewBranchFromConfigurationEvent(t *testing.T) {
	tests := []struct {
		name           string
		organizationID string
		projectID      string
		branchID       string
		region         string
		image          string
		instanceType   string
		replicas       int
		storageSize    *int32
		expectedEvent  Event
	}{
		{
			name:           "basic branch from configuration",
			organizationID: "org-123",
			projectID:      "proj-456",
			branchID:       "branch-789",
			region:         "us-east-1",
			image:          "postgres:15",
			instanceType:   "t3.micro",
			replicas:       2,
			storageSize:    nil,
			expectedEvent: Event{
				Name:  "branch created",
				OrgID: "org-123",
				Properties: map[string]any{
					"organization":  "org-123",
					"project":       "proj-456",
					"branch":        "branch-789",
					"region":        "us-east-1",
					"child_branch":  false,
					"image":         "postgres:15",
					"instance_type": "t3.micro",
					"replicas":      2,
				},
			},
		},
		{
			name:           "branch with zero replicas",
			organizationID: "org-test",
			projectID:      "proj-test",
			branchID:       "branch-zero",
			region:         "eu-west-1",
			image:          "postgres:14",
			instanceType:   "t3.nano",
			replicas:       0,
			storageSize:    nil,
			expectedEvent: Event{
				Name:  "branch created",
				OrgID: "org-test",
				Properties: map[string]any{
					"organization":  "org-test",
					"project":       "proj-test",
					"branch":        "branch-zero",
					"region":        "eu-west-1",
					"child_branch":  false,
					"image":         "postgres:14",
					"instance_type": "t3.nano",
					"replicas":      0,
				},
			},
		},
		{
			name:           "branch with storage size",
			organizationID: "org-storage",
			projectID:      "proj-storage",
			branchID:       "branch-storage",
			region:         "us-west-1",
			image:          "postgres:16",
			instanceType:   "t3.small",
			replicas:       1,
			storageSize:    new(int32(250)),
			expectedEvent: Event{
				Name:  "branch created",
				OrgID: "org-storage",
				Properties: map[string]any{
					"organization":  "org-storage",
					"project":       "proj-storage",
					"branch":        "branch-storage",
					"region":        "us-west-1",
					"child_branch":  false,
					"image":         "postgres:16",
					"instance_type": "t3.small",
					"replicas":      1,
					"storage_size":  250,
				},
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			event := NewBranchFromConfigurationEvent(
				tt.organizationID,
				tt.projectID,
				tt.branchID,
				tt.region,
				tt.image,
				tt.instanceType,
				tt.replicas,
				tt.storageSize,
			)

			assert.Equal(t, tt.expectedEvent.Name, event.Name)
			assert.Equal(t, tt.expectedEvent.OrgID, event.OrgID)
			assert.Equal(t, tt.expectedEvent.Properties, event.Properties)
		})
	}
}

func TestNewBranchFromParentEvent(t *testing.T) {
	tests := []struct {
		name           string
		organizationID string
		projectID      string
		parentID       string
		branchID       string
		region         string
		expectedEvent  Event
	}{
		{
			name:           "basic branch from parent",
			organizationID: "org-123",
			projectID:      "proj-456",
			parentID:       "branch-parent",
			branchID:       "branch-child",
			region:         "us-west-2",
			expectedEvent: Event{
				Name:  "branch created",
				OrgID: "org-123",
				Properties: map[string]any{
					"organization": "org-123",
					"project":      "proj-456",
					"branch":       "branch-child",
					"region":       "us-west-2",
					"parent_id":    "branch-parent",
					"child_branch": true,
				},
			},
		},
		{
			name:           "child branch with special characters",
			organizationID: "org-test_123",
			projectID:      "proj-test_456",
			parentID:       "branch-parent_test",
			branchID:       "branch-child_test",
			region:         "ap-south-1",
			expectedEvent: Event{
				Name:  "branch created",
				OrgID: "org-test_123",
				Properties: map[string]any{
					"organization": "org-test_123",
					"project":      "proj-test_456",
					"branch":       "branch-child_test",
					"region":       "ap-south-1",
					"parent_id":    "branch-parent_test",
					"child_branch": true,
				},
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			event := NewBranchFromParentEvent(
				tt.organizationID,
				tt.projectID,
				tt.parentID,
				tt.branchID,
				tt.region,
			)

			assert.Equal(t, tt.expectedEvent.Name, event.Name)
			assert.Equal(t, tt.expectedEvent.OrgID, event.OrgID)
			assert.Equal(t, tt.expectedEvent.Properties, event.Properties)
		})
	}
}

func TestNewBranchDeletedEvent(t *testing.T) {
	tests := []struct {
		name           string
		organizationID string
		projectID      string
		branchID       string
		expectedEvent  Event
	}{
		{
			name:           "basic branch deletion",
			organizationID: "org-123",
			projectID:      "proj-456",
			branchID:       "branch-789",
			expectedEvent: Event{
				Name:  "branch deleted",
				OrgID: "org-123",
				Properties: map[string]any{
					"organization": "org-123",
					"project":      "proj-456",
					"branch":       "branch-789",
				},
			},
		},
		{
			name:           "branch deletion with special characters",
			organizationID: "org-test_123",
			projectID:      "proj-test_456",
			branchID:       "branch-test_789",
			expectedEvent: Event{
				Name:  "branch deleted",
				OrgID: "org-test_123",
				Properties: map[string]any{
					"organization": "org-test_123",
					"project":      "proj-test_456",
					"branch":       "branch-test_789",
				},
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			event := NewBranchDeletedEvent(tt.organizationID, tt.projectID, tt.branchID)

			assert.Equal(t, tt.expectedEvent.Name, event.Name)
			assert.Equal(t, tt.expectedEvent.OrgID, event.OrgID)
			assert.Equal(t, tt.expectedEvent.Properties, event.Properties)
		})
	}
}

func TestNewBranchDescribedEvent(t *testing.T) {
	tests := map[string]struct {
		organizationID string
		projectID      string
		branchID       string
		want           Event
	}{
		"basic": {
			organizationID: "org-123",
			projectID:      "proj-456",
			branchID:       "branch-789",
			want: Event{
				Name:  "branch described",
				OrgID: "org-123",
				Properties: map[string]any{
					"organization": "org-123",
					"project":      "proj-456",
					"branch":       "branch-789",
				},
			},
		},
	}

	for name, tt := range tests {
		t.Run(name, func(t *testing.T) {
			got := NewBranchDescribedEvent(tt.organizationID, tt.projectID, tt.branchID)

			assert.Equal(t, tt.want.Name, got.Name)
			assert.Equal(t, tt.want.OrgID, got.OrgID)
			assert.Equal(t, tt.want.Properties, got.Properties)
		})
	}
}

func TestNewProjectUpdatedEvent(t *testing.T) {
	tests := map[string]struct {
		organizationID string
		projectID      string
		changedFields  []string
		newValues      map[string]any
		want           Event
	}{
		"name changed": {
			organizationID: "org-123",
			projectID:      "proj-456",
			changedFields:  []string{"name"},
			newValues:      map[string]any{"name": "new-name"},
			want: Event{
				Name:  "project updated",
				OrgID: "org-123",
				Properties: map[string]any{
					"organization":   "org-123",
					"project":        "proj-456",
					"changed_fields": []string{"name"},
					"new_values":     map[string]any{"name": "new-name"},
				},
			},
		},
		"ip filtering changed": {
			organizationID: "org-123",
			projectID:      "proj-456",
			changedFields:  []string{"ip_filtering"},
			newValues:      map[string]any{"ip_filtering_enabled": true},
			want: Event{
				Name:  "project updated",
				OrgID: "org-123",
				Properties: map[string]any{
					"organization":   "org-123",
					"project":        "proj-456",
					"changed_fields": []string{"ip_filtering"},
					"new_values":     map[string]any{"ip_filtering_enabled": true},
				},
			},
		},
		"multiple fields changed": {
			organizationID: "org-123",
			projectID:      "proj-456",
			changedFields:  []string{"name", "ip_filtering"},
			newValues:      map[string]any{"name": "prod-db", "ip_filtering_enabled": true},
			want: Event{
				Name:  "project updated",
				OrgID: "org-123",
				Properties: map[string]any{
					"organization":   "org-123",
					"project":        "proj-456",
					"changed_fields": []string{"name", "ip_filtering"},
					"new_values":     map[string]any{"name": "prod-db", "ip_filtering_enabled": true},
				},
			},
		},
	}

	for name, tt := range tests {
		t.Run(name, func(t *testing.T) {
			got := NewProjectUpdatedEvent(tt.organizationID, tt.projectID, tt.changedFields, tt.newValues)
			assert.Equal(t, tt.want, got)
		})
	}
}

func TestNewBranchUpdatedEvent(t *testing.T) {
	tests := map[string]struct {
		organizationID string
		projectID      string
		branchID       string
		changedFields  []string
		newValues      map[string]any
		want           Event
	}{
		"scale up replicas": {
			organizationID: "org-123",
			projectID:      "proj-456",
			branchID:       "br-main",
			changedFields:  []string{"replicas"},
			newValues:      map[string]any{"replicas": int32(3)},
			want: Event{
				Name:  "branch updated",
				OrgID: "org-123",
				Properties: map[string]any{
					"organization":   "org-123",
					"project":        "proj-456",
					"branch":         "br-main",
					"changed_fields": []string{"replicas"},
					"new_values":     map[string]any{"replicas": int32(3)},
				},
			},
		},
		"change instance type and storage": {
			organizationID: "org-123",
			projectID:      "proj-456",
			branchID:       "br-main",
			changedFields:  []string{"instance_type", "storage"},
			newValues:      map[string]any{"instance_type": "xata.large", "storage_gi": int32(50)},
			want: Event{
				Name:  "branch updated",
				OrgID: "org-123",
				Properties: map[string]any{
					"organization":   "org-123",
					"project":        "proj-456",
					"branch":         "br-main",
					"changed_fields": []string{"instance_type", "storage"},
					"new_values":     map[string]any{"instance_type": "xata.large", "storage_gi": int32(50)},
				},
			},
		},
		"enable scale to zero": {
			organizationID: "org-123",
			projectID:      "proj-456",
			branchID:       "br-staging",
			changedFields:  []string{"scale_to_zero"},
			newValues:      map[string]any{"scale_to_zero": map[string]any{"enabled": true}},
			want: Event{
				Name:  "branch updated",
				OrgID: "org-123",
				Properties: map[string]any{
					"organization":   "org-123",
					"project":        "proj-456",
					"branch":         "br-staging",
					"changed_fields": []string{"scale_to_zero"},
					"new_values":     map[string]any{"scale_to_zero": map[string]any{"enabled": true}},
				},
			},
		},
		"update postgres config": {
			organizationID: "org-123",
			projectID:      "proj-456",
			branchID:       "br-main",
			changedFields:  []string{"postgres_config"},
			newValues:      map[string]any{"postgres_config": map[string]string{"max_connections": "200"}},
			want: Event{
				Name:  "branch updated",
				OrgID: "org-123",
				Properties: map[string]any{
					"organization":   "org-123",
					"project":        "proj-456",
					"branch":         "br-main",
					"changed_fields": []string{"postgres_config"},
					"new_values":     map[string]any{"postgres_config": map[string]string{"max_connections": "200"}},
				},
			},
		},
	}

	for name, tt := range tests {
		t.Run(name, func(t *testing.T) {
			got := NewBranchUpdatedEvent(tt.organizationID, tt.projectID, tt.branchID, tt.changedFields, tt.newValues)
			assert.Equal(t, tt.want, got)
		})
	}
}

func TestNewBranchRestoredFromBackupEvent(t *testing.T) {
	tests := map[string]struct {
		organizationID string
		projectID      string
		sourceBranchID string
		newBranchID    string
		want           Event
	}{
		"basic restore": {
			organizationID: "org-123",
			projectID:      "proj-456",
			sourceBranchID: "br-main",
			newBranchID:    "br-restored",
			want: Event{
				Name:  "branch restored from backup",
				OrgID: "org-123",
				Properties: map[string]any{
					"organization":    "org-123",
					"project":         "proj-456",
					"source_branch":   "br-main",
					"restored_branch": "br-restored",
				},
			},
		},
	}

	for name, tt := range tests {
		t.Run(name, func(t *testing.T) {
			got := NewBranchRestoredFromBackupEvent(tt.organizationID, tt.projectID, tt.sourceBranchID, tt.newBranchID)
			assert.Equal(t, tt.want, got)
		})
	}
}
