package sqlstore

import (
	"time"

	"xata/services/projects/store"
)

func defaultProjectScaleToZeroConfig() store.ProjectScaleToZero {
	return store.ProjectScaleToZero{
		BaseBranches:  defaultScaleToZeroConfig(),
		ChildBranches: defaultScaleToZeroConfig(),
	}
}

func defaultScaleToZeroConfig() store.ScaleToZero {
	return store.ScaleToZero{
		Enabled:          false,
		InactivityPeriod: store.InactivityPeriod(time.Hour),
	}
}

func createProjectConfig(name string, scaleToZero *store.ProjectScaleToZero) *store.CreateProjectConfiguration {
	cfg := &store.CreateProjectConfiguration{
		Name:        name,
		ScaleToZero: defaultProjectScaleToZeroConfig(),
		UsageTier:   string(store.TierT2),
	}
	if scaleToZero != nil {
		cfg.ScaleToZero = *scaleToZero
	}
	return cfg
}

func updateProjectConfig(name *string, scaleToZero *store.ProjectScaleToZero) *store.UpdateProjectConfiguration {
	return &store.UpdateProjectConfiguration{
		Name:        name,
		ScaleToZero: scaleToZero,
	}
}

func createBranchConfig(name string, parentID, description *string) *store.CreateBranchConfiguration {
	return &store.CreateBranchConfiguration{
		Name:                  name,
		ParentID:              parentID,
		Description:           description,
		BackupRetentionPeriod: 2,
		BackupsEnabled:        true,
		UsageTier:             string(store.TierT2),
		Limits: &store.OrgLimits{
			MaxBranchesPerOrg:     10000,
			MaxBranchesPerProject: store.TierDefaultInt(store.TierT2, store.LimitMaxBranchesPerProject, 0),
			MaxBranchesPerHour:    10000,
		},
	}
}

func updateBranchConfig(name, description *string) *store.UpdateBranchConfiguration {
	return &store.UpdateBranchConfiguration{
		Name:        name,
		Description: description,
	}
}
