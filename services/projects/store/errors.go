package store

import (
	"fmt"
	"net/http"
)

// ErrRegionAlreadyExists is returned when trying to create a region that already exists
type ErrRegionAlreadyExists struct {
	ID string
}

func (e ErrRegionAlreadyExists) Error() string {
	return fmt.Sprintf("region [%s] already exists", e.ID)
}

func (e ErrRegionAlreadyExists) StatusCode() int {
	return http.StatusConflict
}

type ErrRegionNotFound struct {
	ID string
}

func (e ErrRegionNotFound) Error() string {
	return fmt.Sprintf("region [%s] not found", e.ID)
}

func (e ErrRegionNotFound) StatusCode() int {
	return http.StatusNotFound
}

type ErrCellAlreadyExists struct {
	ID string
}

func (e ErrCellAlreadyExists) Error() string {
	return fmt.Sprintf("cell [%s] already exists", e.ID)
}

func (e ErrCellAlreadyExists) StatusCode() int {
	return http.StatusConflict
}

// ErrCellNotFound is returned when a cell is not found
type ErrCellNotFound struct {
	ID string
}

func (e ErrCellNotFound) Error() string {
	return fmt.Sprintf("cell [%s] not found", e.ID)
}

func (e ErrCellNotFound) StatusCode() int {
	return http.StatusNotFound
}

// ErrProjectNotFound is returned when a project is not found
type ErrProjectNotFound struct {
	ID string
}

func (e ErrProjectNotFound) Error() string {
	return fmt.Sprintf("project [%s] not found", e.ID)
}

func (e ErrProjectNotFound) StatusCode() int {
	return http.StatusNotFound
}

// ErrProjectAlreadyExists is returned when trying to create a project that already exists
type ErrProjectAlreadyExists struct {
	Name string
}

func (e ErrProjectAlreadyExists) Error() string {
	return fmt.Sprintf("project [%s] already exists", e.Name)
}

func (e ErrProjectAlreadyExists) StatusCode() int {
	return http.StatusConflict
}

// ErrInvalidProjectName is returned when a project name is invalid
type ErrInvalidProjectName struct {
	Name string
}

func (e ErrInvalidProjectName) Error() string {
	return fmt.Sprintf("invalid project name [%s]", e.Name)
}

func (e ErrInvalidProjectName) StatusCode() int {
	return http.StatusBadRequest
}

// ErrBranchAlreadyExists is returned when trying to create a branch with a name that already exists
type ErrBranchAlreadyExists struct {
	Name string
}

func (e ErrBranchAlreadyExists) Error() string {
	return fmt.Sprintf("branch [%s] already exists", e.Name)
}

func (e ErrBranchAlreadyExists) StatusCode() int {
	return http.StatusConflict
}

// ErrBranchNotFound is returned when trying to delete/describe a branch that does not exist
type ErrBranchNotFound struct {
	ID string
}

func (e ErrBranchNotFound) Error() string {
	return fmt.Sprintf("branch [%s] not found", e.ID)
}

func (e ErrBranchNotFound) StatusCode() int {
	return http.StatusNotFound
}

// ErrTooManyRecords is returned when trying to delete/describe one record and deleting more than one
type ErrTooManyRecords struct {
	NumRecords int64
}

func (e ErrTooManyRecords) Error() string {
	return fmt.Sprintf("too many records affected %d", e.NumRecords)
}

func (e ErrTooManyRecords) StatusCode() int {
	return http.StatusInternalServerError
}

type ErrProjectNotEmpty struct {
	ID string
}

func (e ErrProjectNotEmpty) Error() string {
	return fmt.Sprintf("project %s is not empty. Please delete its branches first.", e.ID)
}

func (e ErrProjectNotEmpty) StatusCode() int {
	return http.StatusBadRequest
}

// ErrTooManyBranches is returned when trying to create a branch for a project that reaches its limit
type ErrTooManyBranches struct {
	ID    string
	Limit int
}

func (e ErrTooManyBranches) Error() string {
	return fmt.Sprintf("project [%s] has reached the limit of %d branches", e.ID, e.Limit)
}

func (e ErrTooManyBranches) StatusCode() int {
	return http.StatusBadRequest
}

type ErrOrgBranchLimitExceeded struct {
	OrganizationID string
	Limit          int
}

func (e ErrOrgBranchLimitExceeded) Error() string {
	return fmt.Sprintf("organization [%s] has reached the limit of %d branches", e.OrganizationID, e.Limit)
}

func (e ErrOrgBranchLimitExceeded) StatusCode() int {
	return http.StatusForbidden
}

type ErrOrgProjectLimitExceeded struct {
	OrganizationID string
	Limit          int
}

func (e ErrOrgProjectLimitExceeded) Error() string {
	return fmt.Sprintf("organization [%s] has reached the limit of %d projects", e.OrganizationID, e.Limit)
}

func (e ErrOrgProjectLimitExceeded) StatusCode() int {
	return http.StatusForbidden
}

type ErrBranchRateLimitExceeded struct {
	OrganizationID string
	Limit          int
}

func (e ErrBranchRateLimitExceeded) Error() string {
	return fmt.Sprintf("organization [%s] has reached the limit of %d branch creations per hour", e.OrganizationID, e.Limit)
}

func (e ErrBranchRateLimitExceeded) StatusCode() int {
	return http.StatusTooManyRequests
}

type ErrProjectRateLimitExceeded struct {
	OrganizationID string
	Limit          int
}

func (e ErrProjectRateLimitExceeded) Error() string {
	return fmt.Sprintf("organization [%s] has reached the limit of %d project creations per hour", e.OrganizationID, e.Limit)
}

func (e ErrProjectRateLimitExceeded) StatusCode() int {
	return http.StatusTooManyRequests
}

type ErrMaxDepthExceeded struct {
	BranchID string
	MaxDepth int32
}

func (e ErrMaxDepthExceeded) Error() string {
	return fmt.Sprintf("BranchID [%s]: a child branch would exceed the maximum depth of the branch tree (%d). Please, remove a depth level or use a different parent", e.BranchID, e.MaxDepth)
}

func (e ErrMaxDepthExceeded) StatusCode() int {
	return http.StatusBadRequest
}

type ErrMaxChildrenExceeded struct {
	BranchID    string
	MaxChildren int32
}

func (e ErrMaxChildrenExceeded) Error() string {
	return fmt.Sprintf("BranchID [%s]: a child branch would exceed the maximum children allowed (%d). Please, remove a branch or use a different parent", e.BranchID, e.MaxChildren)
}

func (e ErrMaxChildrenExceeded) StatusCode() int {
	return http.StatusBadRequest
}

type ErrInvalidHierarchy struct {
	Type string
	ID   string
}

func (e ErrInvalidHierarchy) Error() string {
	if e.Type == "" && e.ID == "" {
		return "invalid hierarchy"
	}

	if e.ID == "" {
		return fmt.Sprintf("invalid hierarchy: %s", e.Type)
	}

	return fmt.Sprintf("invalid hierarchy: %s with ID %s", e.Type, e.ID)
}

func (e ErrInvalidHierarchy) StatusCode() int {
	return http.StatusBadRequest
}

type ErrGithubInstallationAlreadyExists struct {
	Organization   string
	InstallationID int64
}

func (e ErrGithubInstallationAlreadyExists) Error() string {
	return fmt.Sprintf("github installation [%d] already exists for organization [%s]", e.InstallationID, e.Organization)
}

func (e ErrGithubInstallationAlreadyExists) StatusCode() int {
	return http.StatusConflict
}

type ErrGithubInstallationNotFound struct {
	Organization string
	ID           string
}

func (e ErrGithubInstallationNotFound) Error() string {
	return fmt.Sprintf("github installation [%s] not found for organization [%s]", e.ID, e.Organization)
}

func (e ErrGithubInstallationNotFound) StatusCode() int {
	return http.StatusNotFound
}

type ErrGithubRepoMappingAlreadyExists struct {
	Organization string
	Project      string
}

func (e ErrGithubRepoMappingAlreadyExists) Error() string {
	return fmt.Sprintf("github repo mapping already exists for project [%s] in organization [%s]", e.Project, e.Organization)
}

func (e ErrGithubRepoMappingAlreadyExists) StatusCode() int {
	return http.StatusConflict
}

type ErrGithubRepositoryAlreadyMapped struct {
	RepositoryID int64
}

func (e ErrGithubRepositoryAlreadyMapped) Error() string {
	return fmt.Sprintf("github repository [%d] is already mapped to another project", e.RepositoryID)
}

func (e ErrGithubRepositoryAlreadyMapped) StatusCode() int {
	return http.StatusConflict
}

type ErrGithubRepoMappingNotFound struct {
	Organization string
	Project      string
	RepoID       int64
}

func (e ErrGithubRepoMappingNotFound) Error() string {
	if e.RepoID != 0 {
		return fmt.Sprintf("github repo mapping not found for repository [%d]", e.RepoID)
	}
	return fmt.Sprintf("github repo mapping not found for project [%s] in organization [%s]", e.Project, e.Organization)
}

func (e ErrGithubRepoMappingNotFound) StatusCode() int {
	return http.StatusNotFound
}
