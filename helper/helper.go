package helper

import (
	"encoding/json"
	"fmt"
	"regexp"
	"sort"
	"strings"

	"github.com/plumber-cd/github-apps-trampoline/github"
	"github.com/plumber-cd/github-apps-trampoline/logger"
)

type SilentExitError struct {
	Err error
}

func (m *SilentExitError) Error() string {
	return m.Err.Error()
}

func (e *SilentExitError) Unwrap() error { return e.Err }

type Config struct {
	// GitHubServer is a GitHub server, default is github.com.
	GitHubServer *string `json:"server,omitempty"`

	// GitHubAPI is address for GitHub API - by default it's automatically inferred from GitHubServer.
	GitHubAPI *string `json:"api,omitempty"`

	// PrivateKey is a path to the key file.
	PrivateKey string `json:"key"`

	// AppID is a GitHub App ID.
	AppID int `json:"app"`

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

type Helper struct {
	configs map[string]Config
}

type IHelper interface {
	GetToken() (string, error)
}

type GitHelper struct {
	currentRepo string
	config      Config
}

type CLIHelper struct {
	config Config
}

func New(cfg string) *Helper {
	configs := map[string]Config{}
	if err := json.Unmarshal([]byte(cfg), &configs); err != nil {
		panic(err)
	}
	return &Helper{configs: configs}
}

func (h Helper) GitHelper(currentRepo string) (IHelper, error) {
	configPtr := func(c map[string]Config) *Config {
		currentRepoBytes := []byte(currentRepo)
		filters := make([]string, 0, len(c))
		for filter := range c {
			filters = append(filters, filter)
		}
		sort.Slice(filters, func(i, j int) bool {
			if len(filters[i]) != len(filters[j]) {
				return len(filters[i]) > len(filters[j])
			}
			return filters[i] < filters[j]
		})
		for _, filter := range filters {
			config := c[filter]
			matched, err := regexp.Match(filter, currentRepoBytes)
			if err != nil {
				panic(err)
			}
			if matched {
				logger.Get().Printf("Matched %q with %q", currentRepo, filter)
				return &config
			}
		}

		logger.Get().Printf("Can't match %s with anything", currentRepo)
		return nil
	}(h.configs)

	if configPtr == nil {
		return nil, &SilentExitError{Err: fmt.Errorf("Can't match %s with anything", currentRepo)}
	}
	config := *configPtr

	if config.CurrentRepositoryOnly != nil && *config.CurrentRepositoryOnly {
		logger.Get().Println("Enabled: CurrentRepositoryOnly")
		config.RepositoryIDs = nil
		split := strings.Split(currentRepo, "/")
		repos := []string{split[len(split)-1]}
		logger.Get().Printf("CurrentRepositoryOnly overrides Repositories=%v", repos)
		config.Repositories = &repos
	}

	return GitHelper{currentRepo: currentRepo, config: config}, nil
}

func (h Helper) CLIHelper() (IHelper, error) {
	if len(h.configs) != 1 {
		return nil, fmt.Errorf("In CLI mode expected exactly 1 matcher, got: %d", len(h.configs))
	}

	config := func(c map[string]Config) Config {
		for _, v := range c {
			return v
		}
		panic("Can't get first entry in a map of exactly one element - can't ever happen")
	}(h.configs)

	if config.CurrentRepositoryOnly != nil && *config.CurrentRepositoryOnly {
		return nil, fmt.Errorf("Can't infer current repository in CLI mode")
	}

	if config.Installation == nil && config.InstallationID == nil {
		return nil, fmt.Errorf("Either installation or installation ID must be specified in CLI mode")
	}

	return CLIHelper{config: config}, nil
}

func (h GitHelper) GetToken() (string, error) {
	if err := validateConfig(&h.config); err != nil {
		return "", err
	}

	jwt, err := github.CreateJWT(h.config.PrivateKey, h.config.AppID)
	if err != nil {
		return "", err
	}

	if err := validateInstallationID(&h.config, jwt, h.currentRepo); err != nil {
		return "", err
	}

	return getToken(h.config, jwt)
}

func (h CLIHelper) GetToken() (string, error) {
	if err := validateConfig(&h.config); err != nil {
		return "", err
	}

	jwt, err := github.CreateJWT(h.config.PrivateKey, h.config.AppID)
	if err != nil {
		return "", err
	}

	if err := validateInstallationID(&h.config, jwt, ""); err != nil {
		return "", err
	}

	return getToken(h.config, jwt)
}

func validateConfig(config *Config) error {
	if config.GitHubServer == nil {
		defaultServer := "github.com"
		logger.Get().Printf("Server was not set - assuming %s", defaultServer)
		config.GitHubServer = &defaultServer
	}

	if config.GitHubAPI == nil {
		logger.Get().Printf("API URL was not set - calculating automatically")
		var api string
		if *config.GitHubServer == "github.com" {
			api = fmt.Sprintf("https://api.%s", *config.GitHubServer)
		} else {
			api = fmt.Sprintf("https://%s/api/v3", *config.GitHubServer)
		}
		logger.Get().Printf("API URL was calculated automatically to %q", api)
		config.GitHubAPI = &api
	}

	if config.PrivateKey == "" {
		return fmt.Errorf("Private Key was not set")
	}

	if config.AppID <= 0 {
		return fmt.Errorf("GitHub App ID was not set")
	}

	return nil
}

func validateInstallationID(config *Config, jwt, currentRepo string) error {
	if config.InstallationID == nil {
		logger.Get().Printf("Installation ID was not provided, calculating automatically...")

		var owner string
		if config.Installation != nil {
			logger.Get().Printf("Looking up installation ID for %s", *config.Installation)
			split := strings.Split(*config.Installation, "/")
			if len(split) > 2 {
				owner = split[len(split)-2]
			} else {
				owner = split[1]
			}
		} else if currentRepo != "" {
			logger.Get().Printf("Looking up installation for current repo %s", currentRepo)
			split := strings.Split(currentRepo, "/")
			owner = split[len(split)-2]
		} else {
			return &SilentExitError{Err: fmt.Errorf("Can't find an owner for automatic installation ID lookup")}
		}
		logger.Get().Printf("Owner determined %q", owner)

		logger.Get().Printf("Getting installation IDs")
		installations, err := github.GetInstallations(*config.GitHubAPI, jwt)
		if err != nil {
			return err
		}

		logger.Get().Printf("Matching installation ID for owner=%q", owner)
		installationPtr := func(installations []github.AppInstallation, owner string) *github.AppInstallation {
			for i := range installations {
				if installations[i].Account.Login == owner {
					logger.Get().Printf("Matched owner %q with ID %d", owner, installations[i].ID)
					return &installations[i]
				}
			}

			return nil
		}(installations, owner)

		if installationPtr == nil {
			return &SilentExitError{Err: fmt.Errorf("Can't find an installation ID for owner %s", owner)}
		}
		installation := *installationPtr

		config.InstallationID = &installation.ID
	}

	return nil
}

func getToken(config Config, jwt string) (string, error) {
	logger.Get().Printf("Building token request")

	request := map[string]interface{}{}

	if config.Repositories != nil {
		logger.Get().Printf("Enabled: repositories")
		request["repositories"] = *config.Repositories
	}

	if config.RepositoryIDs != nil {
		logger.Get().Printf("Enabled: repository_ids")
		request["repository_ids"] = *config.RepositoryIDs
	}

	if config.Permissions != nil {
		logger.Get().Printf("Enabled: permissions")
		permissions := map[string]interface{}{}
		if err := json.Unmarshal(*config.Permissions, &permissions); err != nil {
			return "", err
		}
		request["permissions"] = permissions
	}

	requestData, err := json.MarshalIndent(request, "", "    ")
	if err != nil {
		return "", err
	}

	token, err := github.GetToken(*config.GitHubAPI, jwt, *config.InstallationID, requestData)
	if err != nil {
		return "", err
	}

	return token.Token, nil
}
