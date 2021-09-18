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
	// GitHubServer is a GitHub server, default is github.com.
	GitHubServer string `json:"server"`

	// GitHubAPI is address for GitHub API - by default it's automatically inferred from GitHubServer.
	GitHubAPI string `json:"api"`

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

func GetToken(cfg string, currentRepo string) (string, error) {
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
	}

	if config.GitHubServer == "" {
		config.GitHubServer = "github.com"
	}

	if config.GitHubAPI == "" {
		if config.GitHubServer == "github.com" {
			config.GitHubAPI = fmt.Sprintf("https://api.%s", config.GitHubServer)
		} else {
			config.GitHubAPI = fmt.Sprintf("https://%s/api/v3", config.GitHubServer)
		}
	}

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

	installations, err := github.GetInstallations(config.GitHubAPI, jwt)
	cobra.CheckErr(err)

	if currentRepo != "" && config.InstallationID == nil {
		var owner string
		if config.Installation != nil {
			split := strings.Split(*config.Installation, "/")
			owner = split[len(split)-2]
		} else {
			split := strings.Split(currentRepo, "/")
			owner = split[len(split)-2]
		}

		installation := func(installations []github.AppInstallation, owner string) github.AppInstallation {
			for i := range installations {
				if installations[i].Account.Login == owner {
					return installations[i]
				}
			}

			// Nothing to do - request was probably not for this helper
			os.Exit(0)
			panic("Emulating return on exit")
		}(installations, owner)

		config.InstallationID = &installation.ID
	}

	request := map[string]interface{}{}
	if config.Repositories != nil {
		request["repositories"] = *config.Repositories
	}
	if config.RepositoryIDs != nil {
		request["repository_ids"] = *config.RepositoryIDs
	}
	if config.Permissions != nil {
		permissions := map[string]interface{}{}
		err := json.Unmarshal(*config.Permissions, &permissions)
		cobra.CheckErr(err)
		request["permissions"] = permissions
	}
	requestData, err := json.MarshalIndent(request, "", "    ")
	cobra.CheckErr(err)
	token, err := github.GetToken(config.GitHubAPI, jwt, *config.InstallationID, requestData)
	cobra.CheckErr(err)

	return token.Token, nil
}
