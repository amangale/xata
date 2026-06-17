package provisioner

import (
	"context"
	"fmt"
	"testing"

	clustersv1 "xata/gen/proto/clusters/v1"
	"xata/gen/protomocks"
	"xata/services/clusters"
	"xata/services/projects/cells/cellsmock"
	"xata/services/projects/store"
	storemocks "xata/services/projects/store/mocks"

	"github.com/stretchr/testify/mock"
	"github.com/stretchr/testify/require"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/status"
)

const (
	orgID     = "org-1"
	projectID = "project-1"
)

func TestCreateBranch(t *testing.T) {
	project := &store.Project{ID: projectID, Name: "test"}
	projectWithIPFiltering := &store.Project{
		ID:   projectID,
		Name: "test",
		IPFiltering: store.IPFiltering{
			Enabled: true,
			CIDRs:   []store.CIDREntry{{CIDR: "10.0.0.0/8"}},
		},
	}

	branchOnPrimary := store.Branch{ID: "branch-1", CellID: "primary_cell", Region: "us-east-1"}
	parentID := "parent-1"
	childBranch := store.Branch{ID: "branch-2", ParentID: &parentID, CellID: "primary_cell", Region: "us-east-1"}
	branchOnSecondary := store.Branch{ID: "branch-3", CellID: "secondary_cell", Region: "us-east-1"}

	primaryCell := &store.Cell{ID: "primary_cell", RegionID: "us-east-1", Primary: true}

	basePayload := func() *ClusterServicePayload {
		return &ClusterServicePayload{
			Configuration: clustersv1.ClusterConfiguration{NumInstances: 1, ImageName: "postgres:17"},
			CellID:        "primary_cell",
			Region:        "us-east-1",
			BackupConfig:  &clustersv1.BackupConfiguration{BackupsEnabled: true},
		}
	}

	// mockCreateBranch sets up store.CreateBranch to invoke the provision callback and propagate its result.
	mockCreateBranch := func(mockStore *storemocks.ProjectsStore, branch store.Branch) {
		b := branch
		var provisionErr error
		mockStore.On("CreateBranch", mock.Anything, orgID, projectID, mock.Anything, mock.Anything, mock.Anything).
			Run(func(args mock.Arguments) {
				provisionFn, ok := args.Get(5).(func(*store.Branch) error)
				require.True(t, ok, "provisionFn should be func(*store.Branch) error")
				provisionErr = provisionFn(&b)
			}).
			Return(
				func(_ context.Context, _, _, _ string, _ *store.CreateBranchConfiguration, _ func(*store.Branch) error) *store.Branch {
					if provisionErr != nil {
						return nil
					}
					return &b
				},
				func(_ context.Context, _, _, _ string, _ *store.CreateBranchConfiguration, _ func(*store.Branch) error) error {
					return provisionErr
				},
			).Once()
	}

	tests := map[string]struct {
		payload    func() *ClusterServicePayload
		branch     store.Branch
		setupMocks func(mockStore *storemocks.ProjectsStore, mockClusters *protomocks.ClustersServiceClient)
		wantErr    error
	}{
		"main branch on primary cell succeeds": {
			payload: basePayload,
			branch:  branchOnPrimary,
			setupMocks: func(mockStore *storemocks.ProjectsStore, mockClusters *protomocks.ClustersServiceClient) {
				mockStore.EXPECT().GetProject(mock.Anything, orgID, projectID).Return(project, nil).Once()
				mockCreateBranch(mockStore, branchOnPrimary)
				mockClusters.EXPECT().CreatePostgresCluster(mock.Anything, mock.Anything).
					Return(&clustersv1.CreatePostgresClusterResponse{}, nil).Once()
				mockStore.EXPECT().GetPrimaryCell(mock.Anything, orgID, "us-east-1").Return(primaryCell, nil).Once()
			},
		},
		"child branch sets ClusterSnapshot data source": {
			payload: func() *ClusterServicePayload {
				p := basePayload()
				p.ParentID = &parentID
				return p
			},
			branch: childBranch,
			setupMocks: func(mockStore *storemocks.ProjectsStore, mockClusters *protomocks.ClustersServiceClient) {
				mockStore.EXPECT().GetProject(mock.Anything, orgID, projectID).Return(project, nil).Once()
				mockCreateBranch(mockStore, childBranch)
				mockClusters.EXPECT().CreatePostgresCluster(mock.Anything, mock.MatchedBy(func(req *clustersv1.CreatePostgresClusterRequest) bool {
					snap, ok := req.DataSource.(*clustersv1.CreatePostgresClusterRequest_ClusterSnapshot)
					return ok && snap.ClusterSnapshot.ClusterId == parentID
				})).Return(&clustersv1.CreatePostgresClusterResponse{}, nil).Once()
				mockStore.EXPECT().GetPrimaryCell(mock.Anything, orgID, "us-east-1").Return(primaryCell, nil).Once()
			},
		},
		"UsePool flag is forwarded": {
			payload: func() *ClusterServicePayload {
				p := basePayload()
				p.Flags.UsePool = true
				return p
			},
			branch: branchOnPrimary,
			setupMocks: func(mockStore *storemocks.ProjectsStore, mockClusters *protomocks.ClustersServiceClient) {
				mockStore.EXPECT().GetProject(mock.Anything, orgID, projectID).Return(project, nil).Once()
				mockCreateBranch(mockStore, branchOnPrimary)
				mockClusters.EXPECT().CreatePostgresCluster(mock.Anything, mock.MatchedBy(func(req *clustersv1.CreatePostgresClusterRequest) bool {
					return req.UsePool != nil && *req.UsePool
				})).Return(&clustersv1.CreatePostgresClusterResponse{}, nil).Once()
				mockStore.EXPECT().GetPrimaryCell(mock.Anything, orgID, "us-east-1").Return(primaryCell, nil).Once()
			},
		},
		"UseXatastor flag is forwarded for main branch": {
			payload: func() *ClusterServicePayload {
				p := basePayload()
				p.Flags.UseXatastor = true
				return p
			},
			branch: branchOnPrimary,
			setupMocks: func(mockStore *storemocks.ProjectsStore, mockClusters *protomocks.ClustersServiceClient) {
				mockStore.EXPECT().GetProject(mock.Anything, orgID, projectID).Return(project, nil).Once()
				mockCreateBranch(mockStore, branchOnPrimary)
				mockClusters.EXPECT().CreatePostgresCluster(mock.Anything, mock.MatchedBy(func(req *clustersv1.CreatePostgresClusterRequest) bool {
					return req.UseXatastor != nil && *req.UseXatastor
				})).Return(&clustersv1.CreatePostgresClusterResponse{}, nil).Once()
				mockStore.EXPECT().GetPrimaryCell(mock.Anything, orgID, "us-east-1").Return(primaryCell, nil).Once()
			},
		},
		"UseXatastor flag is not set for child branch": {
			payload: func() *ClusterServicePayload {
				p := basePayload()
				p.ParentID = &parentID
				p.Flags.UseXatastor = true
				return p
			},
			branch: childBranch,
			setupMocks: func(mockStore *storemocks.ProjectsStore, mockClusters *protomocks.ClustersServiceClient) {
				mockStore.EXPECT().GetProject(mock.Anything, orgID, projectID).Return(project, nil).Once()
				mockCreateBranch(mockStore, childBranch)
				mockClusters.EXPECT().CreatePostgresCluster(mock.Anything, mock.MatchedBy(func(req *clustersv1.CreatePostgresClusterRequest) bool {
					return req.UseXatastor == nil
				})).Return(&clustersv1.CreatePostgresClusterResponse{}, nil).Once()
				mockStore.EXPECT().GetPrimaryCell(mock.Anything, orgID, "us-east-1").Return(primaryCell, nil).Once()
			},
		},
		"IP filtering is applied when project has it": {
			payload: basePayload,
			branch:  branchOnPrimary,
			setupMocks: func(mockStore *storemocks.ProjectsStore, mockClusters *protomocks.ClustersServiceClient) {
				mockStore.EXPECT().GetProject(mock.Anything, orgID, projectID).Return(projectWithIPFiltering, nil).Once()
				mockCreateBranch(mockStore, branchOnPrimary)
				mockClusters.EXPECT().CreatePostgresCluster(mock.Anything, mock.Anything).
					Return(&clustersv1.CreatePostgresClusterResponse{}, nil).Once()
				mockStore.EXPECT().GetPrimaryCell(mock.Anything, orgID, "us-east-1").Return(primaryCell, nil).Once()
				mockClusters.EXPECT().SetBranchIPFiltering(mock.Anything, &clustersv1.SetBranchIPFilteringRequest{
					BranchId: branchOnPrimary.ID,
					IpFiltering: &clustersv1.IPFilteringConfig{
						Enabled: true,
						Allowed: []string{"10.0.0.0/8"},
					},
				}).Return(nil, nil).Once()
			},
		},
		"secondary cell registers on primary": {
			payload: func() *ClusterServicePayload {
				p := basePayload()
				p.CellID = "secondary_cell"
				return p
			},
			branch: branchOnSecondary,
			setupMocks: func(mockStore *storemocks.ProjectsStore, mockClusters *protomocks.ClustersServiceClient) {
				mockStore.EXPECT().GetProject(mock.Anything, orgID, projectID).Return(project, nil).Once()
				mockCreateBranch(mockStore, branchOnSecondary)
				mockClusters.EXPECT().CreatePostgresCluster(mock.Anything, mock.Anything).
					Return(&clustersv1.CreatePostgresClusterResponse{}, nil).Once()
				mockStore.EXPECT().GetPrimaryCell(mock.Anything, orgID, "us-east-1").Return(primaryCell, nil).Once()
				mockClusters.EXPECT().RegisterPostgresCluster(mock.Anything, &clustersv1.RegisterPostgresClusterRequest{Id: branchOnSecondary.ID}).
					Return(&clustersv1.RegisterPostgresClusterResponse{}, nil).Once()
			},
		},
		"GetProject error is returned": {
			payload: basePayload,
			setupMocks: func(mockStore *storemocks.ProjectsStore, mockClusters *protomocks.ClustersServiceClient) {
				mockStore.EXPECT().GetProject(mock.Anything, orgID, projectID).Return(nil, fmt.Errorf("project not found")).Once()
			},
			wantErr: fmt.Errorf("project not found"),
		},
		"store CreateBranch error is returned": {
			payload: basePayload,
			setupMocks: func(mockStore *storemocks.ProjectsStore, mockClusters *protomocks.ClustersServiceClient) {
				mockStore.EXPECT().GetProject(mock.Anything, orgID, projectID).Return(project, nil).Once()
				mockStore.On("CreateBranch", mock.Anything, orgID, projectID, mock.Anything, mock.Anything, mock.Anything).
					Return(nil, fmt.Errorf("branch already exists")).Once()
			},
			wantErr: fmt.Errorf("branch already exists"),
		},
		"NotFound with parent maps to ErrBranchNotFound": {
			payload: func() *ClusterServicePayload {
				p := basePayload()
				p.ParentID = &parentID
				return p
			},
			branch: childBranch,
			setupMocks: func(mockStore *storemocks.ProjectsStore, mockClusters *protomocks.ClustersServiceClient) {
				mockStore.EXPECT().GetProject(mock.Anything, orgID, projectID).Return(project, nil).Once()
				mockCreateBranch(mockStore, childBranch)
				mockClusters.EXPECT().CreatePostgresCluster(mock.Anything, mock.Anything).
					Return(nil, status.Error(codes.NotFound, "not found")).Once()
			},
			wantErr: ErrBranchNotFound{BranchID: parentID},
		},
		"InvalidArgument maps to ErrInvalidConfiguration": {
			payload: basePayload,
			branch:  branchOnPrimary,
			setupMocks: func(mockStore *storemocks.ProjectsStore, mockClusters *protomocks.ClustersServiceClient) {
				mockStore.EXPECT().GetProject(mock.Anything, orgID, projectID).Return(project, nil).Once()
				mockCreateBranch(mockStore, branchOnPrimary)
				mockClusters.EXPECT().CreatePostgresCluster(mock.Anything, mock.Anything).
					Return(nil, status.Error(codes.InvalidArgument, "bad config")).Once()
			},
			wantErr: ErrInvalidConfiguration{Name: "test-branch", Message: "bad config"},
		},
		"FailedPrecondition with parent maps to ErrParentBranchUnhealthy": {
			payload: func() *ClusterServicePayload {
				p := basePayload()
				p.ParentID = &parentID
				return p
			},
			branch: childBranch,
			setupMocks: func(mockStore *storemocks.ProjectsStore, mockClusters *protomocks.ClustersServiceClient) {
				mockStore.EXPECT().GetProject(mock.Anything, orgID, projectID).Return(project, nil).Once()
				mockCreateBranch(mockStore, childBranch)
				mockClusters.EXPECT().CreatePostgresCluster(mock.Anything, mock.Anything).
					Return(nil, status.Error(codes.FailedPrecondition, "unhealthy")).Once()
			},
			wantErr: ErrParentBranchUnhealthy{ParentID: parentID},
		},
	}

	for name, tt := range tests {
		t.Run(name, func(t *testing.T) {
			mockStore := storemocks.NewProjectsStore(t)
			mockClusters := protomocks.NewClustersServiceClient(t)
			mockCells := cellsmock.NewCellsMock(t, mockClusters)

			tt.setupMocks(mockStore, mockClusters)

			prov := NewBranchProvisioner(mockStore, mockCells)
			payload := tt.payload()
			got, err := prov.CreateBranch(context.Background(), projectID, orgID, "test-branch", payload)

			if tt.wantErr != nil {
				require.Error(t, err)
				require.Equal(t, tt.wantErr, err)
			} else {
				require.NoError(t, err)
				require.NotNil(t, got)
				require.Equal(t, tt.branch.ID, got.ID)
			}
		})
	}
}

func TestDeleteBranch(t *testing.T) {
	primaryBranch := store.Branch{
		ID:     "branch-1",
		CellID: "primary_cell",
		Region: "us-east-1",
	}

	secondaryBranch := store.Branch{
		ID:     "branch-2",
		CellID: "secondary_cell",
		Region: "us-east-1",
	}

	tests := map[string]struct {
		branch     store.Branch
		setupMocks func(mockStore *storemocks.ProjectsStore, mockClusters *protomocks.ClustersServiceClient)
		wantErr    string
	}{
		"delete on primary cell succeeds": {
			branch: primaryBranch,
			setupMocks: func(mockStore *storemocks.ProjectsStore, mockClusters *protomocks.ClustersServiceClient) {
				mockClusters.EXPECT().DeletePostgresCluster(mock.Anything, &clustersv1.DeletePostgresClusterRequest{Id: primaryBranch.ID}).
					Return(&clustersv1.DeletePostgresClusterResponse{}, nil).Once()
				mockStore.EXPECT().GetPrimaryCell(mock.Anything, orgID, "us-east-1").
					Return(&store.Cell{ID: "primary_cell", RegionID: "us-east-1", Primary: true}, nil).Once()
				mockClusters.EXPECT().DeleteBranchIPFiltering(mock.Anything, &clustersv1.DeleteBranchIPFilteringRequest{BranchId: primaryBranch.ID}).
					Return(nil, nil).Once()
			},
		},
		"delete on secondary cell deregisters from primary": {
			branch: secondaryBranch,
			setupMocks: func(mockStore *storemocks.ProjectsStore, mockClusters *protomocks.ClustersServiceClient) {
				mockClusters.EXPECT().DeletePostgresCluster(mock.Anything, &clustersv1.DeletePostgresClusterRequest{Id: secondaryBranch.ID}).
					Return(&clustersv1.DeletePostgresClusterResponse{}, nil).Once()
				mockStore.EXPECT().GetPrimaryCell(mock.Anything, orgID, "us-east-1").
					Return(&store.Cell{ID: "primary_cell", RegionID: "us-east-1", Primary: true}, nil).Once()
				mockClusters.EXPECT().DeleteBranchIPFiltering(mock.Anything, &clustersv1.DeleteBranchIPFilteringRequest{BranchId: secondaryBranch.ID}).
					Return(nil, nil).Once()
				mockClusters.EXPECT().DeregisterPostgresCluster(mock.Anything, &clustersv1.DeregisterPostgresClusterRequest{Id: secondaryBranch.ID}).
					Return(&clustersv1.DeregisterPostgresClusterResponse{}, nil).Once()
			},
		},
		"cluster not found in kubernetes proceeds with deletion": {
			branch: primaryBranch,
			setupMocks: func(mockStore *storemocks.ProjectsStore, mockClusters *protomocks.ClustersServiceClient) {
				mockClusters.EXPECT().DeletePostgresCluster(mock.Anything, &clustersv1.DeletePostgresClusterRequest{Id: primaryBranch.ID}).
					Return(nil, clusters.ClusterNotFoundError(primaryBranch.ID)).Once()
				mockStore.EXPECT().GetPrimaryCell(mock.Anything, orgID, "us-east-1").
					Return(&store.Cell{ID: "primary_cell", RegionID: "us-east-1", Primary: true}, nil).Once()
				mockClusters.EXPECT().DeleteBranchIPFiltering(mock.Anything, &clustersv1.DeleteBranchIPFilteringRequest{BranchId: primaryBranch.ID}).
					Return(nil, nil).Once()
			},
		},
		"cluster delete error is returned": {
			branch: primaryBranch,
			setupMocks: func(mockStore *storemocks.ProjectsStore, mockClusters *protomocks.ClustersServiceClient) {
				mockClusters.EXPECT().DeletePostgresCluster(mock.Anything, &clustersv1.DeletePostgresClusterRequest{Id: primaryBranch.ID}).
					Return(nil, fmt.Errorf("infra error")).Once()
			},
			wantErr: "infra error",
		},
		"store delete error is returned": {
			branch:     primaryBranch,
			setupMocks: func(mockStore *storemocks.ProjectsStore, mockClusters *protomocks.ClustersServiceClient) {},
			wantErr:    "store error",
		},
		"IP filtering cleanup failure does not fail deletion": {
			branch: primaryBranch,
			setupMocks: func(mockStore *storemocks.ProjectsStore, mockClusters *protomocks.ClustersServiceClient) {
				mockClusters.EXPECT().DeletePostgresCluster(mock.Anything, &clustersv1.DeletePostgresClusterRequest{Id: primaryBranch.ID}).
					Return(&clustersv1.DeletePostgresClusterResponse{}, nil).Once()
				mockStore.EXPECT().GetPrimaryCell(mock.Anything, orgID, "us-east-1").
					Return(&store.Cell{ID: "primary_cell", RegionID: "us-east-1", Primary: true}, nil).Once()
				mockClusters.EXPECT().DeleteBranchIPFiltering(mock.Anything, &clustersv1.DeleteBranchIPFilteringRequest{BranchId: primaryBranch.ID}).
					Return(nil, fmt.Errorf("ip filtering error")).Once()
			},
		},
		"GetPrimaryCell failure is returned": {
			branch: primaryBranch,
			setupMocks: func(mockStore *storemocks.ProjectsStore, mockClusters *protomocks.ClustersServiceClient) {
				mockClusters.EXPECT().DeletePostgresCluster(mock.Anything, &clustersv1.DeletePostgresClusterRequest{Id: primaryBranch.ID}).
					Return(&clustersv1.DeletePostgresClusterResponse{}, nil).Once()
				mockStore.EXPECT().GetPrimaryCell(mock.Anything, orgID, "us-east-1").
					Return(nil, fmt.Errorf("cell not found")).Once()
			},
			wantErr: "get primary cell",
		},
		"deregister failure is returned": {
			branch: secondaryBranch,
			setupMocks: func(mockStore *storemocks.ProjectsStore, mockClusters *protomocks.ClustersServiceClient) {
				mockClusters.EXPECT().DeletePostgresCluster(mock.Anything, &clustersv1.DeletePostgresClusterRequest{Id: secondaryBranch.ID}).
					Return(&clustersv1.DeletePostgresClusterResponse{}, nil).Once()
				mockStore.EXPECT().GetPrimaryCell(mock.Anything, orgID, "us-east-1").
					Return(&store.Cell{ID: "primary_cell", RegionID: "us-east-1", Primary: true}, nil).Once()
				mockClusters.EXPECT().DeleteBranchIPFiltering(mock.Anything, &clustersv1.DeleteBranchIPFilteringRequest{BranchId: secondaryBranch.ID}).
					Return(nil, nil).Once()
				mockClusters.EXPECT().DeregisterPostgresCluster(mock.Anything, &clustersv1.DeregisterPostgresClusterRequest{Id: secondaryBranch.ID}).
					Return(nil, fmt.Errorf("deregister failed")).Once()
			},
			wantErr: "deregister from primary cell",
		},
	}

	for name, tt := range tests {
		t.Run(name, func(t *testing.T) {
			mockStore := storemocks.NewProjectsStore(t)
			mockClusters := protomocks.NewClustersServiceClient(t)
			mockCells := cellsmock.NewCellsMock(t, mockClusters)

			tt.setupMocks(mockStore, mockClusters)

			// Mock store.DeleteBranch to invoke the deprovision callback and propagate its error
			if tt.wantErr == "store error" {
				mockStore.EXPECT().DeleteBranch(mock.Anything, orgID, projectID, tt.branch.ID, mock.Anything).
					Return(fmt.Errorf("store error")).Once()
			} else {
				b := tt.branch
				mockStore.On("DeleteBranch", mock.Anything, orgID, projectID, tt.branch.ID, mock.Anything).
					Return(func(_ context.Context, _, _, _ string, deprovisionFn func(*store.Branch) error) error {
						return deprovisionFn(&b)
					}).Once()
			}

			prov := NewBranchProvisioner(mockStore, mockCells)
			err := prov.DeleteBranch(context.Background(), orgID, projectID, tt.branch.ID)

			if tt.wantErr != "" {
				require.ErrorContains(t, err, tt.wantErr)
			} else {
				require.NoError(t, err)
			}
		})
	}
}
