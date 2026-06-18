package provisioner

import (
	"context"
	"fmt"

	clustersv1 "xata/gen/proto/clusters/v1"
	"xata/services/projects/cells"
	"xata/services/projects/store"

	"github.com/rs/zerolog/log"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/status"
)

type FlagsConfig struct {
	UsePool     bool
	UseXatastor bool
}

type OrgLimits struct {
	MaxProjects            int
	MaxProjectsPerHour     int
	MaxBranchesPerProject  int
	MaxBranchesPerOrg      int
	MaxBranchesPerHour     int
	MaxInstancesPerBranch  int
	MinInstancesPerBranch  int
	MaxDescriptionLength   int
	MaxAllowedInstanceType int
}

// StoreLimits returns the subset of limits needed by store.CreateBranch.
func (l OrgLimits) StoreLimits() *store.OrgLimits {
	return &store.OrgLimits{
		MaxBranchesPerOrg:     l.MaxBranchesPerOrg,
		MaxBranchesPerProject: l.MaxBranchesPerProject,
		MaxBranchesPerHour:    l.MaxBranchesPerHour,
	}
}

// ResolveOrgLimits returns the effective limits for an organization by combining
// tier defaults, flag-based adjustments, and per-org DB overrides.
// T1 orgs skip the DB lookup.
func ResolveOrgLimits(ctx context.Context, s store.ProjectsStore, usageTier, organizationID, projectID string, flags FlagsConfig) (OrgLimits, error) {
	tier := parseTier(ctx, usageTier)
	def := func(key store.LimitKey) int { return store.TierDefaultInt(tier, key, 0) }

	var overrides map[store.LimitKey]any
	if tier != store.TierT1 {
		var err error
		overrides, err = s.GetOrgLimits(ctx, organizationID, projectID)
		if err != nil {
			return OrgLimits{}, fmt.Errorf("get org limits: %w", err)
		}
	}

	maxBranchesPerOrg := def(store.LimitMaxBranchesPerOrg)
	maxBranchesPerProject := def(store.LimitMaxBranchesPerProject)
	maxBranchesPerHour := def(store.LimitMaxBranchesPerHour)
	if flags.UseXatastor && tier != store.TierT1 {
		maxBranchesPerOrg = store.XatastorMaxBranchesPerOrg
		maxBranchesPerProject = store.XatastorMaxBranchesPerProject
		maxBranchesPerHour = store.XatastorMaxBranchesPerHour
	}

	resolve := func(key store.LimitKey, fallback int) int {
		return store.ResolveIntLimit(overrides, key, fallback)
	}

	return OrgLimits{
		MaxProjects:            resolve(store.LimitMaxProjects, def(store.LimitMaxProjects)),
		MaxProjectsPerHour:     resolve(store.LimitMaxProjectsPerHour, def(store.LimitMaxProjectsPerHour)),
		MaxBranchesPerProject:  resolve(store.LimitMaxBranchesPerProject, maxBranchesPerProject),
		MaxBranchesPerOrg:      resolve(store.LimitMaxBranchesPerOrg, maxBranchesPerOrg),
		MaxBranchesPerHour:     resolve(store.LimitMaxBranchesPerHour, maxBranchesPerHour),
		MaxInstancesPerBranch:  resolve(store.LimitMaxInstancesPerBranch, def(store.LimitMaxInstancesPerBranch)),
		MinInstancesPerBranch:  resolve(store.LimitMinInstancesPerBranch, def(store.LimitMinInstancesPerBranch)),
		MaxDescriptionLength:   resolve(store.LimitMaxDescriptionLength, def(store.LimitMaxDescriptionLength)),
		MaxAllowedInstanceType: resolve(store.LimitMaxAllowedInstanceType, def(store.LimitMaxAllowedInstanceType)),
	}, nil
}

func parseTier(ctx context.Context, usageTier string) store.UsageTier {
	tier, ok := store.ParseUsageTier(usageTier)
	if !ok {
		log.Ctx(ctx).Warn().Str("usage_tier", usageTier).Msg("unknown usage tier, defaulting to t1")
	}
	return tier
}

type ClusterServicePayload struct {
	ParentID              *string
	Configuration         clustersv1.ClusterConfiguration
	Description           *string
	CellID                string
	Region                string
	BackupsEnabled        bool
	BackupRetentionPeriod int
	BackupConfig          *clustersv1.BackupConfiguration
	Flags                 FlagsConfig
	UsageTier             string
	Limits                *store.OrgLimits
}

//go:generate go run github.com/vektra/mockery/v3 --output mocks --outpkg mocks --with-expecter --name Provisioner
type Provisioner interface {
	CreateBranch(ctx context.Context, projectID, organizationID, name string, payload *ClusterServicePayload) (*store.Branch, error)
	DeleteBranch(ctx context.Context, organizationID, projectID, branchID string) error
}

type BranchProvisioner struct {
	store store.ProjectsStore
	cells cells.Cells
}

func NewBranchProvisioner(store store.ProjectsStore, cells cells.Cells) *BranchProvisioner {
	return &BranchProvisioner{
		store: store,
		cells: cells,
	}
}

func (p *BranchProvisioner) CreateBranch(ctx context.Context, projectID, organizationID, name string, payload *ClusterServicePayload) (*store.Branch, error) {
	project, err := p.store.GetProject(ctx, organizationID, projectID)
	if err != nil {
		return nil, err
	}

	// Default scale-to-zero from project settings when not explicitly provided
	if payload.Configuration.ScaleToZero == nil {
		s2z := project.ScaleToZero.BaseBranches
		if payload.ParentID != nil {
			s2z = project.ScaleToZero.ChildBranches
		}
		payload.Configuration.ScaleToZero = &clustersv1.ScaleToZero{
			Enabled:                 s2z.Enabled,
			InactivityPeriodMinutes: int64(s2z.InactivityPeriod.Duration().Minutes()),
		}
	}

	branch, err := p.store.CreateBranch(ctx, organizationID, projectID, payload.CellID, &store.CreateBranchConfiguration{
		Name:                  name,
		ParentID:              payload.ParentID,
		Description:           payload.Description,
		BackupRetentionPeriod: payload.BackupRetentionPeriod,
		BackupsEnabled:        payload.BackupsEnabled,
		UsageTier:             payload.UsageTier,
		Limits:                payload.Limits,
	}, func(branch *store.Branch) error {
		request := clustersv1.CreatePostgresClusterRequest{
			Id:                  branch.ID,
			ParentId:            branch.ParentID,
			OrganizationId:      organizationID,
			ProjectId:           projectID,
			Configuration:       &payload.Configuration,
			BackupConfiguration: payload.BackupConfig,
		}
		if branch.ParentID != nil {
			request.DataSource = &clustersv1.CreatePostgresClusterRequest_ClusterSnapshot{
				ClusterSnapshot: &clustersv1.ClusterSnapshot{
					ClusterId: *branch.ParentID,
				},
			}
		}

		if payload.Flags.UsePool {
			request.UsePool = new(true)
		}

		if branch.ParentID == nil {
			if payload.Flags.UseXatastor {
				request.UseXatastor = new(true)
			}
		}

		client, err := p.cells.GetCellConnection(ctx, organizationID, payload.CellID)
		if err != nil {
			return err
		}
		defer client.Close()

		_, err = client.CreatePostgresCluster(ctx, &request)
		if err != nil {
			return err
		}

		return p.setupBranchOnPrimaryCell(ctx, organizationID, payload.Region, payload.CellID, branch.ID, project)
	})
	if err != nil {
		st, _ := status.FromError(err)
		if st.Code() == codes.NotFound && payload.ParentID != nil {
			return nil, ErrBranchNotFound{BranchID: *payload.ParentID}
		}
		if st.Code() == codes.InvalidArgument {
			return nil, ErrInvalidConfiguration{Name: name, Message: st.Message()}
		}
		if st.Code() == codes.FailedPrecondition && payload.ParentID != nil {
			return nil, ErrParentBranchUnhealthy{ParentID: *payload.ParentID}
		}
		return nil, err
	}
	return branch, nil
}

func (p *BranchProvisioner) DeleteBranch(ctx context.Context, organizationID, projectID, branchID string) error {
	return p.store.DeleteBranch(ctx, organizationID, projectID, branchID, func(branch *store.Branch) error {
		return cells.DeprovisionBranch(ctx, organizationID, p.store, p.cells, branch)
	})
}

// setupBranchOnPrimaryCell registers a cluster with the primary cell if it was
// created on a secondary cell, and applies IP filtering settings from the project.
func (p *BranchProvisioner) setupBranchOnPrimaryCell(ctx context.Context, organizationID, region, cellID, branchID string, project *store.Project) error {
	primaryCell, err := p.store.GetPrimaryCell(ctx, organizationID, region)
	if err != nil {
		return err
	}

	hasIPFiltering := project.IPFiltering.Enabled || len(project.IPFiltering.CIDRs) > 0
	needsRegistration := primaryCell.ID != cellID

	if !hasIPFiltering && !needsRegistration {
		return nil
	}

	client, err := p.cells.GetCellConnection(ctx, organizationID, primaryCell.ID)
	if err != nil {
		return err
	}
	defer client.Close()

	if hasIPFiltering {
		_, err = client.SetBranchIPFiltering(ctx, &clustersv1.SetBranchIPFilteringRequest{
			BranchId: branchID,
			IpFiltering: &clustersv1.IPFilteringConfig{
				Enabled: project.IPFiltering.Enabled,
				Allowed: project.IPFiltering.CIDRStrings(),
			},
		})
		if err != nil {
			return fmt.Errorf("setting IP filtering for branch: %w", err)
		}
	}

	if needsRegistration {
		_, err = client.RegisterPostgresCluster(ctx, &clustersv1.RegisterPostgresClusterRequest{Id: branchID})
		if err != nil {
			return err
		}
	}

	return nil
}
