package rpc

import (
	"context"
	"errors"
	"fmt"

	projectsv1 "xata/gen/proto/projects/v1"
	"xata/services/projects/store"

	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/status"
)

// limitField links a projects-store limit key to the matching optional field on
// the proto Limits message. The single table keeps Set/Get in sync so adding a
// limit only touches one place (plus proto).
type limitField struct {
	key store.LimitKey
	// get reads the override from a request Limits message; nil means absent.
	get func(*projectsv1.Limits) *int64
	// set writes a value onto a response Limits message.
	set func(*projectsv1.Limits, int64)
}

var limitFields = []limitField{
	{
		store.LimitMaxDescriptionLength,
		func(l *projectsv1.Limits) *int64 { return l.MaxDescriptionLength },
		func(l *projectsv1.Limits, v int64) { l.MaxDescriptionLength = &v },
	},
	{
		store.LimitMaxBranchesPerProject,
		func(l *projectsv1.Limits) *int64 { return l.MaxBranchesPerProject },
		func(l *projectsv1.Limits, v int64) { l.MaxBranchesPerProject = &v },
	},
	{
		store.LimitMaxInstancesPerBranch,
		func(l *projectsv1.Limits) *int64 { return l.MaxInstancesPerBranch },
		func(l *projectsv1.Limits, v int64) { l.MaxInstancesPerBranch = &v },
	},
	{
		store.LimitMinInstancesPerBranch,
		func(l *projectsv1.Limits) *int64 { return l.MinInstancesPerBranch },
		func(l *projectsv1.Limits, v int64) { l.MinInstancesPerBranch = &v },
	},
	{
		store.LimitMaxStorageGBPerBranch,
		func(l *projectsv1.Limits) *int64 { return l.MaxStorageGbPerBranch },
		func(l *projectsv1.Limits, v int64) { l.MaxStorageGbPerBranch = &v },
	},
	{
		store.LimitMaxAllowedInstanceType,
		func(l *projectsv1.Limits) *int64 { return l.MaxAllowedInstanceType },
		func(l *projectsv1.Limits, v int64) { l.MaxAllowedInstanceType = &v },
	},
	{
		store.LimitMaxBranchesPerHour,
		func(l *projectsv1.Limits) *int64 { return l.MaxBranchesPerHour },
		func(l *projectsv1.Limits, v int64) { l.MaxBranchesPerHour = &v },
	},
	{
		store.LimitMaxBranchesPerOrg,
		func(l *projectsv1.Limits) *int64 { return l.MaxBranchesPerOrg },
		func(l *projectsv1.Limits, v int64) { l.MaxBranchesPerOrg = &v },
	},
	{
		store.LimitMaxProjects,
		func(l *projectsv1.Limits) *int64 { return l.MaxProjects },
		func(l *projectsv1.Limits, v int64) { l.MaxProjects = &v },
	},
	{
		store.LimitMaxProjectsPerHour,
		func(l *projectsv1.Limits) *int64 { return l.MaxProjectsPerHour },
		func(l *projectsv1.Limits, v int64) { l.MaxProjectsPerHour = &v },
	},
}

// SetOrganizationLimits implements projectsv1.ProjectsServiceServer.
func (p *ProjectsService) SetOrganizationLimits(ctx context.Context, req *projectsv1.SetOrganizationLimitsRequest) (*projectsv1.SetOrganizationLimitsResponse, error) {
	if req.GetOrganizationId() == "" {
		return nil, status.Error(codes.InvalidArgument, "organization_id is required")
	}

	resetKeys := make(map[store.LimitKey]struct{}, len(req.GetReset_()))
	for _, k := range req.GetReset_() {
		key := store.LimitKey(k)
		if !key.IsValid() {
			return nil, status.Errorf(codes.InvalidArgument, "invalid reset key %q", k)
		}
		resetKeys[key] = struct{}{}
	}

	// A project-scoped override must target a real project; otherwise a typo
	// would silently create an orphaned override that never applies to anything.
	if err := p.validateProject(ctx, req.GetOrganizationId(), req.GetProjectId()); err != nil {
		return nil, err
	}

	reqLimits := req.GetLimits()
	if reqLimits == nil {
		reqLimits = &projectsv1.Limits{}
	}
	// Validate the whole request before writing anything: collect the sets and
	// reject set/reset conflicts up front so a partially-applied request can't
	// leave persisted state behind when a later field fails validation.
	type limitUpdate struct {
		key   store.LimitKey
		value int64
	}
	updates := make([]limitUpdate, 0, len(limitFields))
	for _, f := range limitFields {
		v := f.get(reqLimits)
		if v == nil {
			continue
		}
		if _, clearing := resetKeys[f.key]; clearing {
			return nil, status.Errorf(codes.InvalidArgument, "limit %q is both set and reset", f.key)
		}
		updates = append(updates, limitUpdate{key: f.key, value: *v})
	}

	for _, u := range updates {
		if err := p.store.SetOrgLimit(ctx, req.GetOrganizationId(), req.GetProjectId(), u.key, u.value); err != nil {
			return nil, fmt.Errorf("set limit %q: %w", u.key, err)
		}
	}

	for key := range resetKeys {
		if err := p.store.DeleteOrgLimit(ctx, req.GetOrganizationId(), req.GetProjectId(), key); err != nil {
			return nil, fmt.Errorf("reset limit %q: %w", key, err)
		}
	}

	limits, err := p.storedLimits(ctx, req.GetOrganizationId(), req.GetProjectId())
	if err != nil {
		return nil, err
	}
	return &projectsv1.SetOrganizationLimitsResponse{Limits: limits}, nil
}

// validateProject ensures a project-scoped request targets an existing project.
// An empty projectID is an organization-level override and needs no check.
func (p *ProjectsService) validateProject(ctx context.Context, orgID, projectID string) error {
	if projectID == "" {
		return nil
	}
	if _, err := p.store.GetProject(ctx, orgID, projectID); err != nil {
		if errors.As(err, &store.ErrProjectNotFound{}) {
			return status.Errorf(codes.NotFound, "project %q not found", projectID)
		}
		return fmt.Errorf("get project: %w", err)
	}
	return nil
}

// GetOrganizationLimits implements projectsv1.ProjectsServiceServer.
func (p *ProjectsService) GetOrganizationLimits(ctx context.Context, req *projectsv1.GetOrganizationLimitsRequest) (*projectsv1.GetOrganizationLimitsResponse, error) {
	if req.GetOrganizationId() == "" {
		return nil, status.Error(codes.InvalidArgument, "organization_id is required")
	}
	if err := p.validateProject(ctx, req.GetOrganizationId(), req.GetProjectId()); err != nil {
		return nil, err
	}
	limits, err := p.storedLimits(ctx, req.GetOrganizationId(), req.GetProjectId())
	if err != nil {
		return nil, err
	}
	return &projectsv1.GetOrganizationLimitsResponse{Limits: limits}, nil
}

// storedLimits reads the stored overrides and converts them into a proto Limits
// message in which only overridden fields are present.
func (p *ProjectsService) storedLimits(ctx context.Context, orgID, projectID string) (*projectsv1.Limits, error) {
	overrides, err := p.store.GetOrgLimits(ctx, orgID, projectID)
	if err != nil {
		return nil, fmt.Errorf("get limits: %w", err)
	}
	limits := &projectsv1.Limits{}
	for _, f := range limitFields {
		raw, ok := overrides[f.key]
		if !ok {
			continue
		}
		v, ok := limitToInt64(raw)
		if !ok {
			// A stored override that can't be coerced is corrupt data; surface
			// it rather than silently dropping the field so it can be fixed.
			return nil, status.Errorf(codes.Internal, "stored override %q has invalid value %v (%T)", f.key, raw, raw)
		}
		f.set(limits, v)
	}
	return limits, nil
}

// limitToInt64 coerces a stored override value into an int64. Values decoded
// from JSONB arrive as json.Number (the store decodes with UseNumber), but we
// also accept the native integer types tests commonly use. float64 is
// deliberately rejected: limits are integers, so a float would either be a
// fractional value or an unexpected decode path, and truncating it (1.9 -> 1)
// would silently change the stored limit. json.Number with a fractional value
// is likewise rejected because its Int64() returns an error.
func limitToInt64(v any) (int64, bool) {
	switch n := v.(type) {
	case interface{ Int64() (int64, error) }:
		if i, err := n.Int64(); err == nil {
			return i, true
		}
	case int64:
		return n, true
	case int:
		return int64(n), true
	}
	return 0, false
}
