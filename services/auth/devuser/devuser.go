package devuser

import (
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"net/url"

	"github.com/Nerzal/gocloak/v13"
	"github.com/labstack/echo/v4"
	"github.com/spf13/cobra"

	"xata/internal/envcfg"
	"xata/services/auth/config"
)

const (
	DevUsername     = "dev@xata.tech"
	DevPassword     = "Xata1234!"
	DevOrganization = "123xyz"
)

func CreateDevUserCmd() *cobra.Command {
	return &cobra.Command{
		Use:   "create_dev_user",
		Short: "Create a dev user",
		RunE: func(cmd *cobra.Command, args []string) error {
			var cfg struct {
				config.AuthConfig
			}
			if err := envcfg.Read(&cfg); err != nil {
				return err
			}

			client := gocloak.NewClient(cfg.KeycloakURL)
			jwt, err := client.LoginAdmin(cmd.Context(), "temp-admin", cfg.KeycloakAdminPassword, "master")
			if err != nil {
				return fmt.Errorf("failed to login as admin: %w", err)
			}

			userID, err := createUser(cmd.Context(), client, jwt.AccessToken, cfg.Realm)
			if err != nil {
				return fmt.Errorf("failed to create user: %w", err)
			}

			err = client.SetPassword(cmd.Context(), jwt.AccessToken, userID, cfg.Realm, DevPassword, false)
			if err != nil {
				return fmt.Errorf("failed to set password: %w", err)
			}

			err = createOrganization(cmd.Context(), client, jwt.AccessToken, cfg.KeycloakURL, cfg.Realm, DevOrganization, userID)
			if err != nil {
				return fmt.Errorf("failed to create organization: %w", err)
			}

			//nolint:forbidigo
			fmt.Println("Setup done, user: dev@xata.tech, password: xata created")
			return nil
		},
	}
}

func createUser(ctx context.Context, client *gocloak.GoCloak, token, realm string) (string, error) {
	users, err := client.GetUsers(ctx, token, realm, gocloak.GetUsersParams{
		Username: new(DevUsername),
	})
	if err != nil {
		return "", fmt.Errorf("failed to get users: %w", err)
	}
	if len(users) > 0 {
		//nolint:forbidigo
		fmt.Printf("User %s already exists\n", DevUsername)
		return *users[0].ID, nil
	}

	userID, err := client.CreateUser(ctx, token, realm, gocloak.User{
		Username:      new(DevUsername),
		Email:         new(DevUsername),
		Enabled:       new(true),
		EmailVerified: new(true),
		FirstName:     new("xata"),
		LastName:      new("Dev"),
	})
	if err != nil {
		return "", fmt.Errorf("failed to create user: %w", err)
	}

	return userID, nil
}

func createOrganization(ctx context.Context, client *gocloak.GoCloak, token, keycloakURL, realm, orgName, userID string) error {
	orgsURL, err := url.JoinPath(keycloakURL, "admin/realms", realm, "organizations")
	if err != nil {
		return fmt.Errorf("failed to join URL: %w", err)
	}

	resp, err := client.GetRequestWithBearerAuth(ctx, token).
		SetBody(organization{
			Name:    orgName,
			Enabled: true,
			Attributes: map[string][]string{
				"usageTier": {"t2"},
			},
		}).
		Post(orgsURL)
	if err != nil {
		return fmt.Errorf("failed to create organization: %w", err)
	}

	if resp.StatusCode() != http.StatusCreated && resp.StatusCode() != http.StatusConflict {
		return fmt.Errorf("failed to create organization: %s", resp.String())
	}

	// search for the organization to get the ID
	resp, err = client.GetRequestWithBearerAuth(ctx, token).
		SetBody(echo.Map{
			"search": orgName,
			"exact":  true,
		}).Get(orgsURL)
	if err != nil {
		return fmt.Errorf("failed to get organization: %w", err)
	}

	var orgs []organization
	err = json.Unmarshal(resp.Body(), &orgs)
	if err != nil {
		return fmt.Errorf("failed to unmarshal organization: %w", err)
	}

	if len(orgs) == 0 {
		return fmt.Errorf("organization not found")
	}

	//nolint:forbidigo
	fmt.Printf("Organization %s created, id: %s\n", orgName, orgs[0].ID)

	// Add user to organization
	addMemberURL, err := url.JoinPath(keycloakURL, "admin/realms", realm, "organizations", orgs[0].ID, "members")
	if err != nil {
		return fmt.Errorf("failed to join URL: %w", err)
	}
	resp, err = client.GetRequestWithBearerAuth(ctx, token).
		SetBody(userID).
		Post(addMemberURL)
	if err != nil {
		return fmt.Errorf("failed to add user to organization: %w", err)
	}

	if resp.StatusCode() != http.StatusCreated && resp.StatusCode() != http.StatusConflict {
		return fmt.Errorf("failed to add user to organization: %s", resp.String())
	}

	//nolint:forbidigo
	fmt.Printf("User %s is member of organization %s\n", userID, orgName)

	return nil
}

type organization struct {
	ID         string              `json:"id"`
	Name       string              `json:"name"`
	Enabled    bool                `json:"enabled"`
	Attributes map[string][]string `json:"attributes,omitempty"`
}
