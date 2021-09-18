package helper

import (
	"encoding/json"
	"fmt"
	"os"
	"regexp"
	"strings"

	"github.com/spf13/cobra"

	"github.com/plumber-cd/github-apps-trampoline/github"
)

type Config struct {
	// PrivateKey is a path to the key file.
	PrivateKey string `json:"key"`

	// AppID is a GitHub App ID.
	AppID string `json:"app"`

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

func Run(cfg string, currentRepo string) {
	configs := map[string]Config{}
	if err := json.Unmarshal([]byte(cfg), &configs); err != nil {
		cobra.CheckErr(err)
	}

	var config Config
	if currentRepo == "" {
		if len(configs) != 1 {
			fmt.Fprintln(os.Stderr, fmt.Errorf(
				"In CLI mode expected exactly 1 matcher, got: %d", len(configs)))
			os.Exit(1)
		}

		config = func(c map[string]Config) Config {
			for _, v := range c {
				return v
			}
			panic("Can't get first entry in a map of exactly one element")
		}(configs)

		if config.CurrentRepositoryOnly != nil && *config.CurrentRepositoryOnly {
			fmt.Fprintln(os.Stderr, fmt.Errorf(
				"Can't infer current repository in CLI mode"))
			os.Exit(1)
		}

		if config.Installation == nil && config.InstallationID == nil {
			fmt.Fprintln(os.Stderr, fmt.Errorf(
				"Either installation or installation ID must be specified in CLI mode"))
			os.Exit(1)
		}
	} else {
		config = func(c map[string]Config) Config {
			currentRepoBytes := []byte(currentRepo)
			for filter, config := range c {
				matched, err := regexp.Match(filter, currentRepoBytes)
				cobra.CheckErr(err)
				if matched {
					return config
				}
			}

			// Nothing to do - request was probably not for this helper
			os.Exit(0)
			panic("Emulating return on exit")
		}(configs)

		if config.CurrentRepositoryOnly != nil && *config.CurrentRepositoryOnly {
			config.RepositoryIDs = nil
			split := strings.Split(currentRepo, "/")
			repos := []string{split[len(split)-1]}
			config.Repositories = &repos
		}

		if config.InstallationID == nil {
			if config.Installation != nil {
				// TBD: convert installation into ID
				fmt.Println()
			} else {
				// TBD: infer installation ID from current repo
				fmt.Println()
			}
		}
	}

	jsonData, err := json.MarshalIndent(config, "", "    ")
	cobra.CheckErr(err)
	fmt.Println(string(jsonData))

	if config.PrivateKey == "" {
		fmt.Fprintln(os.Stderr, fmt.Errorf("Private Key was not set"))
		os.Exit(1)
	}

	if config.AppID == "" {
		fmt.Fprintln(os.Stderr, fmt.Errorf("GitHub App ID was not set"))
		os.Exit(1)
	}

	jwt, err := github.CreateJWT(config.PrivateKey, config.AppID)
	cobra.CheckErr(err)
	fmt.Println(jwt)
}
