package helper

import (
	"encoding/json"
	"fmt"
)

type Config struct {
	// PrivateKey is a path to the key file.
	PrivateKey string `json:"key"`

	// CurrentRepositoryOnly if set to true - will request access for the current repository.
	// Ignores Repositories and RepositoryIDs.
	CurrentRepositoryOnly *bool `json:"current_repo,omitempty"`

	// Repositories list of repositories to request access to.
	// If neither Repositories nor RepositoryIDs is provided - will default to all repositories in this installation.
	Repositories *[]string `json:"repositories,omitempty"`

	// RepositoryIDs list of repository IDs to request access to.
	// If neither Repositories nor RepositoryIDs is provided - will default to all repositories in this installation.
	RepositoryIDs *[]int `json:"repository_ids,omitempty"`

	// Permissions is a JSON object representing what access the token must have.
	Permissions *json.RawMessage `json:"permissions,omitempty"`

	// Installation is a path to the installation owner that should be used to request token.
	Installation *string `json:"installation,omitempty"`

	// InstallationID is ID of the installation that should be used to request token.
	InstallationID *int `json:"installation_id,omitempty"`
}

func Run(cfg string) {
	fmt.Println(cfg)
}
