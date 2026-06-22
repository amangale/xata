package rpc

import (
	"context"
	"encoding/json"
	"testing"

	projectsv1 "xata/gen/proto/projects/v1"
	"xata/services/projects/store"
	"xata/services/projects/store/mocks"

	"github.com/stretchr/testify/require"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/status"
	"k8s.io/utils/ptr"
)

func TestSetOrganizationLimits(t *testing.T) {
	const orgID = "org-1"

	tests := map[string]struct {
		request   *projectsv1.SetOrganizationLimitsRequest
		setupMock func(*mocks.ProjectsStore)
		wantErr   codes.Code
		want      *projectsv1.Limits
	}{
		"set org-level limits": {
			request: &projectsv1.SetOrganizationLimitsRequest{
				OrganizationId: orgID,
				Limits: &projectsv1.Limits{
					MaxBranchesPerProject: ptr.To[int64](50),
					MaxProjects:           ptr.To[int64](20),
				},
			},
			setupMock: func(m *mocks.ProjectsStore) {
				m.EXPECT().SetOrgLimit(context.Background(), orgID, "", store.LimitMaxBranchesPerProject, int64(50)).Return(nil)
				m.EXPECT().SetOrgLimit(context.Background(), orgID, "", store.LimitMaxProjects, int64(20)).Return(nil)
				m.EXPECT().GetOrgLimits(context.Background(), orgID, "").Return(map[store.LimitKey]any{
					store.LimitMaxBranchesPerProject: json.Number("50"),
					store.LimitMaxProjects:           json.Number("20"),
				}, nil)
			},
			want: &projectsv1.Limits{
				MaxBranchesPerProject: ptr.To[int64](50),
				MaxProjects:           ptr.To[int64](20),
			},
		},
		"set project-level limit": {
			request: &projectsv1.SetOrganizationLimitsRequest{
				OrganizationId: orgID,
				ProjectId:      "proj-1",
				Limits: &projectsv1.Limits{
					MaxBranchesPerProject: ptr.To[int64](500),
				},
			},
			setupMock: func(m *mocks.ProjectsStore) {
				m.EXPECT().GetProject(context.Background(), orgID, "proj-1").Return(&store.Project{ID: "proj-1"}, nil)
				m.EXPECT().SetOrgLimit(context.Background(), orgID, "proj-1", store.LimitMaxBranchesPerProject, int64(500)).Return(nil)
				m.EXPECT().GetOrgLimits(context.Background(), orgID, "proj-1").Return(map[store.LimitKey]any{
					store.LimitMaxBranchesPerProject: json.Number("500"),
				}, nil)
			},
			want: &projectsv1.Limits{MaxBranchesPerProject: ptr.To[int64](500)},
		},
		"project does not exist": {
			request: &projectsv1.SetOrganizationLimitsRequest{
				OrganizationId: orgID,
				ProjectId:      "missing",
				Limits:         &projectsv1.Limits{MaxBranchesPerProject: ptr.To[int64](500)},
			},
			setupMock: func(m *mocks.ProjectsStore) {
				m.EXPECT().GetProject(context.Background(), orgID, "missing").Return(nil, store.ErrProjectNotFound{ID: "missing"})
			},
			wantErr: codes.NotFound,
		},
		"reset a limit": {
			request: &projectsv1.SetOrganizationLimitsRequest{
				OrganizationId: orgID,
				Reset_:         []string{string(store.LimitMaxProjects)},
			},
			setupMock: func(m *mocks.ProjectsStore) {
				m.EXPECT().DeleteOrgLimit(context.Background(), orgID, "", store.LimitMaxProjects).Return(nil)
				m.EXPECT().GetOrgLimits(context.Background(), orgID, "").Return(map[store.LimitKey]any{}, nil)
			},
			want: &projectsv1.Limits{},
		},
		"empty org id": {
			request:   &projectsv1.SetOrganizationLimitsRequest{},
			setupMock: func(*mocks.ProjectsStore) {},
			wantErr:   codes.InvalidArgument,
		},
		"unknown reset key": {
			request: &projectsv1.SetOrganizationLimitsRequest{
				OrganizationId: orgID,
				Reset_:         []string{"not_a_real_limit"},
			},
			setupMock: func(*mocks.ProjectsStore) {},
			wantErr:   codes.InvalidArgument,
		},
		"set and reset same key": {
			request: &projectsv1.SetOrganizationLimitsRequest{
				OrganizationId: orgID,
				Limits:         &projectsv1.Limits{MaxProjects: ptr.To[int64](20)},
				Reset_:         []string{string(store.LimitMaxProjects)},
			},
			setupMock: func(*mocks.ProjectsStore) {},
			wantErr:   codes.InvalidArgument,
		},
		// A conflict on a later field must reject the whole request without
		// persisting an earlier, valid field. The mock asserts no SetOrgLimit
		// call happens (NewProjectsStore fails on unexpected calls).
		"conflict on later field persists nothing": {
			request: &projectsv1.SetOrganizationLimitsRequest{
				OrganizationId: orgID,
				Limits: &projectsv1.Limits{
					MaxDescriptionLength: ptr.To[int64](100),
					MaxProjects:          ptr.To[int64](20),
				},
				Reset_: []string{string(store.LimitMaxProjects)},
			},
			setupMock: func(*mocks.ProjectsStore) {},
			wantErr:   codes.InvalidArgument,
		},
	}

	for name, tt := range tests {
		t.Run(name, func(t *testing.T) {
			mockStore := mocks.NewProjectsStore(t)
			tt.setupMock(mockStore)
			service := NewProjectsService(mockStore, nil)

			resp, err := service.SetOrganizationLimits(context.Background(), tt.request)

			if tt.wantErr != codes.OK {
				require.Equal(t, tt.wantErr, status.Code(err))
				return
			}
			require.NoError(t, err)
			require.Equal(t, tt.want, resp.GetLimits())
		})
	}
}

func TestGetOrganizationLimits(t *testing.T) {
	const orgID = "org-1"

	tests := map[string]struct {
		request   *projectsv1.GetOrganizationLimitsRequest
		setupMock func(*mocks.ProjectsStore)
		wantErr   codes.Code
		want      *projectsv1.Limits
	}{
		"returns only stored overrides": {
			request: &projectsv1.GetOrganizationLimitsRequest{
				OrganizationId: orgID,
				ProjectId:      "proj-1",
			},
			setupMock: func(m *mocks.ProjectsStore) {
				m.EXPECT().GetProject(context.Background(), orgID, "proj-1").Return(&store.Project{ID: "proj-1"}, nil)
				m.EXPECT().GetOrgLimits(context.Background(), orgID, "proj-1").Return(map[store.LimitKey]any{
					store.LimitMaxBranchesPerProject:  json.Number("100"),
					store.LimitMaxAllowedInstanceType: json.Number("32000"),
				}, nil)
			},
			want: &projectsv1.Limits{
				MaxBranchesPerProject:  ptr.To[int64](100),
				MaxAllowedInstanceType: ptr.To[int64](32000),
			},
		},
		"project does not exist": {
			request: &projectsv1.GetOrganizationLimitsRequest{
				OrganizationId: orgID,
				ProjectId:      "missing",
			},
			setupMock: func(m *mocks.ProjectsStore) {
				m.EXPECT().GetProject(context.Background(), orgID, "missing").Return(nil, store.ErrProjectNotFound{ID: "missing"})
			},
			wantErr: codes.NotFound,
		},
		"corrupt stored override (non-numeric string)": {
			request: &projectsv1.GetOrganizationLimitsRequest{OrganizationId: orgID},
			setupMock: func(m *mocks.ProjectsStore) {
				m.EXPECT().GetOrgLimits(context.Background(), orgID, "").Return(map[store.LimitKey]any{
					store.LimitMaxProjects: "not-a-number",
				}, nil)
			},
			wantErr: codes.Internal,
		},
		"corrupt stored override (float value)": {
			request: &projectsv1.GetOrganizationLimitsRequest{OrganizationId: orgID},
			setupMock: func(m *mocks.ProjectsStore) {
				m.EXPECT().GetOrgLimits(context.Background(), orgID, "").Return(map[store.LimitKey]any{
					store.LimitMaxProjects: 20.9,
				}, nil)
			},
			wantErr: codes.Internal,
		},
		"corrupt stored override (fractional number)": {
			request: &projectsv1.GetOrganizationLimitsRequest{OrganizationId: orgID},
			setupMock: func(m *mocks.ProjectsStore) {
				m.EXPECT().GetOrgLimits(context.Background(), orgID, "").Return(map[store.LimitKey]any{
					store.LimitMaxProjects: json.Number("1.9"),
				}, nil)
			},
			wantErr: codes.Internal,
		},
		"empty org id": {
			request:   &projectsv1.GetOrganizationLimitsRequest{},
			setupMock: func(*mocks.ProjectsStore) {},
			wantErr:   codes.InvalidArgument,
		},
	}

	for name, tt := range tests {
		t.Run(name, func(t *testing.T) {
			mockStore := mocks.NewProjectsStore(t)
			tt.setupMock(mockStore)
			service := NewProjectsService(mockStore, nil)

			resp, err := service.GetOrganizationLimits(context.Background(), tt.request)

			if tt.wantErr != codes.OK {
				require.Equal(t, tt.wantErr, status.Code(err))
				return
			}
			require.NoError(t, err)
			require.Equal(t, tt.want, resp.GetLimits())
		})
	}
}
