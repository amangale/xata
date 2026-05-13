package sqlstore

import (
	"context"
	"errors"
	"fmt"
	"log"
	"testing"
	"time"

	"github.com/stretchr/testify/require"
	"github.com/testcontainers/testcontainers-go"
	"github.com/testcontainers/testcontainers-go/modules/postgres"
	"github.com/testcontainers/testcontainers-go/wait"

	"xata/internal/api/key"
	"xata/services/auth/store"
)

func TestSQLAuthStore(t *testing.T) {
	ctx := context.Background()
	sqlStore := setupSQLStore(ctx, t)

	t.Run("api_keys", func(t *testing.T) {
		// Test creating API keys
		t.Run("create_api_key", func(t *testing.T) {
			// Use UTC time to avoid timezone issues in tests
			expiry := time.Now().UTC().Add(24 * time.Hour)

			tests := []struct {
				name         string
				targetType   store.KeyTargetType
				targetID     string
				keyCreate    *store.APIKeyCreate
				wantError    bool
				errorChecker func(error) bool
			}{
				{
					name:       "create organization API key",
					targetType: store.KeyTargetOrganization,
					targetID:   "org123",
					keyCreate: &store.APIKeyCreate{
						Name: "test-org-key",
					},
					wantError: false,
				},
				{
					name:       "create user API key",
					targetType: store.KeyTargetUser,
					targetID:   "user456",
					keyCreate: &store.APIKeyCreate{
						Name: "test-user-key",
					},
					wantError: false,
				},
				{
					name:       "create API key with expiry",
					targetType: store.KeyTargetOrganization,
					targetID:   "org123",
					keyCreate: &store.APIKeyCreate{
						Name:   "test-expiry-key",
						Expiry: &expiry,
					},
					wantError: false,
				},
				{
					name:       "create duplicate key name",
					targetType: store.KeyTargetOrganization,
					targetID:   "org123",
					keyCreate: &store.APIKeyCreate{
						Name: "test-org-key", // Same name as first test
					},
					wantError: true,
					errorChecker: func(err error) bool {
						var apiKeyErr *store.ErrAPIKeyAlreadyExists
						return errors.As(err, &apiKeyErr)
					},
				},
				{
					name:       "create key fails when limit reached",
					targetType: store.KeyTargetOrganization,
					targetID:   "org-limit",
					keyCreate: &store.APIKeyCreate{
						Name: "limit-key",
					},
					wantError: true,
					errorChecker: func(err error) bool {
						var limitErr *store.ErrAPIKeyLimitReached
						return errors.As(err, &limitErr)
					},
				},
				{
					name:       "create key with invalid target type",
					targetType: store.KeyTargetType("invalid-target"),
					targetID:   "123",
					keyCreate: &store.APIKeyCreate{
						Name: "test-invalid-key",
					},
					wantError: true,
					errorChecker: func(err error) bool {
						var targetTypeErr *store.ErrUnsupportedTargetType
						return errors.As(err, &targetTypeErr)
					},
				},
				{
					name:       "create user API key with scopes",
					targetType: store.KeyTargetUser,
					targetID:   "user-scopes",
					keyCreate: &store.APIKeyCreate{
						Name:   "user-key-with-scopes",
						Scopes: []string{"org:read", "project:write"},
					},
					wantError: false,
				},
				{
					name:       "create organization API key with scopes and restrictions",
					targetType: store.KeyTargetOrganization,
					targetID:   "org-restricted",
					keyCreate: &store.APIKeyCreate{
						Name:     "org-key-restricted",
						Scopes:   []string{"project:read", "branch:read"},
						Projects: []string{"proj1", "proj2"},
						Branches: []string{"main", "dev"},
					},
					wantError: false,
				},
			}

			for _, tt := range tests {
				t.Run(tt.name, func(t *testing.T) {
					if tt.name == "create key fails when limit reached" {
						for i := range store.MaxAPIKeysPerTarget {
							_, _, err := sqlStore.CreateAPIKey(ctx, tt.targetType, tt.targetID, &store.APIKeyCreate{Name: fmt.Sprintf("pre-%d", i)})
							require.NoError(t, err)
						}
					}

					apiKeyStr, apiKey, err := sqlStore.CreateAPIKey(ctx, tt.targetType, tt.targetID, tt.keyCreate)

					if tt.wantError {
						require.Error(t, err)
						if tt.errorChecker != nil {
							require.True(t, tt.errorChecker(err), "expected specific error type")
						}
					} else {
						require.NoError(t, err)
						require.NotEmpty(t, apiKeyStr)
						require.NotEmpty(t, apiKey.ID)
						require.Equal(t, tt.keyCreate.Name, apiKey.Name)
						require.Equal(t, tt.targetType, apiKey.TargetType)
						require.Equal(t, tt.targetID, apiKey.TargetID)
						require.NotEmpty(t, apiKey.KeyHash)
						require.Equal(t, apiKeyStr.Obfuscate(key.DefaultObfuscateCharsCount), apiKey.KeyPreview)

						if tt.keyCreate.Expiry != nil {
							require.NotNil(t, apiKey.Expiry)
							require.WithinDuration(t, *tt.keyCreate.Expiry, *apiKey.Expiry, time.Second)
						}

						// Validate scopes and restrictions
						require.Equal(t, tt.keyCreate.Scopes, apiKey.Scopes)
						require.Equal(t, tt.keyCreate.Projects, apiKey.Projects)
						require.Equal(t, tt.keyCreate.Branches, apiKey.Branches)
					}
				})
			}
		})

		// Test listing API keys
		t.Run("list_api_keys", func(t *testing.T) {
			tests := []struct {
				name         string
				targetType   store.KeyTargetType
				targetID     string
				expectEmpty  bool
				expectedType store.KeyTargetType
				expectedID   string
			}{
				{
					name:         "list organization API keys",
					targetType:   store.KeyTargetOrganization,
					targetID:     "org123",
					expectEmpty:  false,
					expectedType: store.KeyTargetOrganization,
					expectedID:   "org123",
				},
				{
					name:         "list user API keys",
					targetType:   store.KeyTargetUser,
					targetID:     "user456",
					expectEmpty:  false,
					expectedType: store.KeyTargetUser,
					expectedID:   "user456",
				},
				{
					name:        "list non-existent target",
					targetType:  store.KeyTargetOrganization,
					targetID:    "non-existent",
					expectEmpty: true,
				},
			}

			for _, tt := range tests {
				t.Run(tt.name, func(t *testing.T) {
					keys, err := sqlStore.ListAPIKeys(ctx, tt.targetType, tt.targetID)
					require.NoError(t, err)

					if tt.expectEmpty {
						require.Empty(t, keys)
					} else {
						require.NotEmpty(t, keys)
						for _, key := range keys {
							require.Equal(t, tt.expectedType, key.TargetType)
							require.Equal(t, tt.expectedID, key.TargetID)
						}
					}
				})
			}
		})

		// Test deleting API keys
		t.Run("delete_api_keys", func(t *testing.T) {
			// Create keys that we'll use in our tests
			_, deleteTestKey, err := sqlStore.CreateAPIKey(ctx, store.KeyTargetOrganization, "org-to-delete", &store.APIKeyCreate{
				Name: "key-to-delete-test",
			})
			require.NoError(t, err)

			tests := []struct {
				name       string
				targetType store.KeyTargetType
				targetID   string
				keyIDs     []string
				verify     bool // Whether to verify deletion by listing
			}{
				{
					name:       "delete existing key",
					targetType: store.KeyTargetOrganization,
					targetID:   "org-to-delete",
					keyIDs:     []string{deleteTestKey.ID},
					verify:     true,
				},
				{
					name:       "delete non-existent key",
					targetType: store.KeyTargetOrganization,
					targetID:   "org-to-delete",
					keyIDs:     []string{"non-existent-id"},
					verify:     false,
				},
				{
					name:       "delete with empty slice",
					targetType: store.KeyTargetOrganization,
					targetID:   "org-to-delete",
					keyIDs:     []string{},
					verify:     false,
				},
			}

			for _, tt := range tests {
				t.Run(tt.name, func(t *testing.T) {
					err := sqlStore.DeleteAPIKeys(ctx, tt.targetType, tt.targetID, tt.keyIDs)
					require.NoError(t, err)

					if tt.verify {
						keys, err := sqlStore.ListAPIKeys(ctx, tt.targetType, tt.targetID)
						require.NoError(t, err)

						// Verify the key was deleted
						for _, keyID := range tt.keyIDs {
							for _, key := range keys {
								require.NotEqual(t, keyID, key.ID, "Key should have been deleted but is still present")
							}
						}
					}
				})
			}
		})
	})

	t.Run("org_limits", func(t *testing.T) {
		const orgID = "test-org"

		t.Run("empty returns no overrides", func(t *testing.T) {
			limits, err := sqlStore.GetOrgLimits(ctx, orgID)
			require.NoError(t, err)
			require.Empty(t, limits)
		})

		t.Run("set and get", func(t *testing.T) {
			tests := map[string]struct {
				key   store.OrgLimitKey
				value any
				want  any
			}{
				"integer": {key: store.OrgLimitMaxMembers, value: int64(50), want: int64(50)},
			}
			for name, tc := range tests {
				t.Run(name, func(t *testing.T) {
					require.NoError(t, sqlStore.SetOrgLimit(ctx, orgID, tc.key, tc.value))
					limits, err := sqlStore.GetOrgLimits(ctx, orgID)
					require.NoError(t, err)
					got, ok := limits[tc.key]
					require.True(t, ok)
					require.Equal(t, tc.want, jsonNumberToInt(got))
				})
			}
		})

		t.Run("overwrite updates value", func(t *testing.T) {
			require.NoError(t, sqlStore.SetOrgLimit(ctx, orgID, store.OrgLimitMaxMembers, int64(10)))
			require.NoError(t, sqlStore.SetOrgLimit(ctx, orgID, store.OrgLimitMaxMembers, int64(99)))
			limits, err := sqlStore.GetOrgLimits(ctx, orgID)
			require.NoError(t, err)
			require.Equal(t, int64(99), jsonNumberToInt(limits[store.OrgLimitMaxMembers]))
		})

		t.Run("delete removes key", func(t *testing.T) {
			require.NoError(t, sqlStore.SetOrgLimit(ctx, orgID, store.OrgLimitMaxMembers, int64(5)))
			require.NoError(t, sqlStore.DeleteOrgLimit(ctx, orgID, store.OrgLimitMaxMembers))
			limits, err := sqlStore.GetOrgLimits(ctx, orgID)
			require.NoError(t, err)
			_, ok := limits[store.OrgLimitMaxMembers]
			require.False(t, ok)
		})

		t.Run("invalid key is rejected", func(t *testing.T) {
			err := sqlStore.SetOrgLimit(ctx, orgID, store.OrgLimitKey("max_branches_per_project"), int64(1))
			require.Error(t, err)
		})
	})
}

func jsonNumberToInt(v any) any {
	if n, ok := v.(interface{ Int64() (int64, error) }); ok {
		if i, err := n.Int64(); err == nil {
			return i
		}
	}
	return v
}

func setupSQLStore(ctx context.Context, t *testing.T) *sqlAuthStore {
	// launch postgres container with testcontainers (TODO abstract this with a helper)
	postgresContainer, err := postgres.Run(ctx,
		"postgres:16-alpine", // TODO parametrize version
		testcontainers.WithWaitStrategy(
			wait.ForLog("database system is ready to accept connections").
				WithOccurrence(2).
				WithStartupTimeout(30*time.Second)),
	)
	if err != nil {
		t.Fatalf("failed to start container: %s", err)
	}

	t.Cleanup(func() {
		if err := testcontainers.TerminateContainer(postgresContainer); err != nil {
			log.Printf("failed to terminate container: %s", err)
		}
	})

	// create a new SQL sqlStore
	config, err := ConfigFromConnectionString(postgresContainer.MustConnectionString(ctx, "sslmode=disable"))
	require.NoError(t, err)
	sqlStore, err := NewSQLAuthStore(ctx, config)
	if err != nil {
		t.Fatalf("failed to create store: %s", err)
	}
	t.Cleanup(func() {
		if err := sqlStore.Close(ctx); err != nil {
			log.Printf("failed to close store: %s", err)
		}
	})

	// run migrations
	err = sqlStore.Setup(ctx)
	require.NoError(t, err)

	return sqlStore
}
