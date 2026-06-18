package api

import (
	"xata/internal/apitest"
	"xata/services/projects/store"
)

func createProjectConfig(name string, scaleToZero *store.ProjectScaleToZero) *store.CreateProjectConfiguration {
	cfg := &store.CreateProjectConfiguration{
		Name:        name,
		ScaleToZero: defaultProjectScaleToZeroConfig(),
		IPFiltering: store.IPFiltering{
			Enabled: false,
			CIDRs:   []store.CIDREntry{},
		},
		UsageTier: string(apitest.TestClaims.Organizations[apitest.TestOrganization].UsageTier),
	}
	if scaleToZero != nil {
		cfg.ScaleToZero = *scaleToZero
	}
	return cfg
}

func updateProjectConfig(name *string, scaleToZero *store.ProjectScaleToZero, ipFiltering *store.IPFiltering) *store.UpdateProjectConfiguration {
	return &store.UpdateProjectConfiguration{
		Name:        name,
		ScaleToZero: scaleToZero,
		IPFiltering: ipFiltering,
	}
}

func createBranchConfig(name string, parentID, description *string) *store.CreateBranchConfiguration {
	tier := store.TierT2
	return &store.CreateBranchConfiguration{
		Name:                  name,
		ParentID:              parentID,
		Description:           description,
		BackupRetentionPeriod: DefaultBackupRetentionPeriod,
		BackupsEnabled:        true,
		UsageTier:             string(apitest.TestClaims.Organizations[apitest.TestOrganization].UsageTier),
		Limits: &store.OrgLimits{
			MaxBranchesPerOrg:     store.TierDefaultInt(tier, store.LimitMaxBranchesPerOrg, 0),
			MaxBranchesPerProject: store.TierDefaultInt(tier, store.LimitMaxBranchesPerProject, 0),
			MaxBranchesPerHour:    store.TierDefaultInt(tier, store.LimitMaxBranchesPerHour, 0),
		},
	}
}

func updateBranchConfig(name, description *string) *store.UpdateBranchConfiguration {
	return &store.UpdateBranchConfiguration{
		Name:        name,
		Description: description,
	}
}
