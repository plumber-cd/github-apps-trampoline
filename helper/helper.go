package helper

import (
	"encoding/json"
	"errors"
	"fmt"
	"regexp"
	"sort"
	"strings"

	"github.com/plumber-cd/github-apps-trampoline/cache"
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

	// CurrentOwnerOnly if set to true - will request access for all repositories in the installation owner.
	// Conflicts with CurrentRepositoryOnly.
	CurrentOwnerOnly *bool `json:"current_owner,omitempty"`

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

	// ResolvedOwner is derived at runtime for cache keys; it is not part of config JSON.
	ResolvedOwner string `json:"-"`
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

	if config.CurrentOwnerOnly != nil && *config.CurrentOwnerOnly {
		logger.Get().Println("Enabled: CurrentOwnerOnly")
		config.RepositoryIDs = nil
		config.Repositories = nil
	} else if config.CurrentRepositoryOnly != nil && *config.CurrentRepositoryOnly {
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

	return getTokenWithRetry(&h.config, jwt, h.currentRepo)
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

	return getTokenWithRetry(&h.config, jwt, "")
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

	if config.CurrentOwnerOnly != nil && *config.CurrentOwnerOnly && config.CurrentRepositoryOnly != nil && *config.CurrentRepositoryOnly {
		return fmt.Errorf("current_owner conflicts with current_repo")
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
		config.ResolvedOwner = owner

		if cache.Enabled() {
			if cachedID, ok, err := getCachedInstallationID(config, owner); err != nil {
				return err
			} else if ok {
				config.InstallationID = &cachedID
				return nil
			}
		}

		logger.Get().Printf("Getting installation IDs")
		installations, _, err := getInstallationsWithCache(*config.GitHubAPI, jwt, config.AppID)
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
			if cache.Enabled() {
				refreshInstallationsCache(config)
				installations, _, err = getInstallationsWithCache(*config.GitHubAPI, jwt, config.AppID)
				if err != nil {
					return err
				}
				installationPtr = func(installations []github.AppInstallation, owner string) *github.AppInstallation {
					for i := range installations {
						if installations[i].Account.Login == owner {
							logger.Get().Printf("Matched owner %q with ID %d", owner, installations[i].ID)
							return &installations[i]
						}
					}
					return nil
				}(installations, owner)
			}
			if installationPtr == nil {
				return &SilentExitError{Err: fmt.Errorf("Can't find an installation ID for owner %s", owner)}
			}
		}
		installation := *installationPtr

		config.InstallationID = &installation.ID
		if cache.Enabled() {
			setCachedInstallationID(config, owner, installation.ID)
		}
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

	if cache.Enabled() {
		return getTokenWithCache(config, jwt, requestData)
	}

	token, err := github.GetToken(*config.GitHubAPI, jwt, *config.InstallationID, requestData)
	if err != nil {
		return "", err
	}

	return token.Token, nil
}

func getTokenWithRetry(config *Config, jwt, currentRepo string) (string, error) {
	token, err := getToken(*config, jwt)
	if err == nil {
		return token, nil
	}
	if !cache.Enabled() {
		return "", err
	}
	var apiErr *github.APIError
	if !errors.As(err, &apiErr) {
		return "", err
	}
	if apiErr.Status != 401 && apiErr.Status != 404 {
		return "", err
	}

	logger.Get().Printf("Token request failed with status=%d, invalidating installation caches and retrying", apiErr.Status)
	invalidateInstallationCaches(config)
	config.InstallationID = nil
	if err := validateInstallationID(config, jwt, currentRepo); err != nil {
		return "", err
	}

	return getToken(*config, jwt)
}

func getTokenWithCache(config Config, jwt string, requestData []byte) (string, error) {
	tokenKey := tokenCacheKey(config, requestData)
	var cachedToken string
	if hit, err := cache.Get(tokenKey, &cachedToken); err != nil {
		return "", err
	} else if hit && cachedToken != "" {
		return cachedToken, nil
	}

	var token *github.AppInstallationAccessToken
	err := cache.WithLock(tokenKey, func() error {
		if hit, err := cache.Get(tokenKey, &cachedToken); err != nil {
			return err
		} else if hit && cachedToken != "" {
			return nil
		}
		fetched, err := github.GetToken(*config.GitHubAPI, jwt, *config.InstallationID, requestData)
		if err != nil {
			return err
		}
		token = fetched
		return cache.Set(tokenKey, token.Token, cache.TTLToken())
	})
	if err != nil {
		return "", err
	}
	if cachedToken != "" {
		return cachedToken, nil
	}
	if token == nil {
		if hit, err := cache.Get(tokenKey, &cachedToken); err != nil {
			return "", err
		} else if hit && cachedToken != "" {
			return cachedToken, nil
		}
		return "", fmt.Errorf("token was not cached")
	}
	return token.Token, nil
}

func getInstallationsWithCache(api, jwt string, appID int) ([]github.AppInstallation, bool, error) {
	if !cache.Enabled() {
		installations, err := github.GetInstallations(api, jwt)
		return installations, false, err
	}

	key := installationsCacheKey(appID, api)
	installations := []github.AppInstallation{}
	if hit, err := cache.Get(key, &installations); err != nil {
		return nil, false, err
	} else if hit {
		return installations, true, nil
	}
	var fetched []github.AppInstallation
	err := cache.WithLock(key, func() error {
		if hit, err := cache.Get(key, &installations); err != nil {
			return err
		} else if hit {
			return nil
		}
		resp, err := github.GetInstallations(api, jwt)
		if err != nil {
			return err
		}
		fetched = resp
		return cache.Set(key, fetched, cache.TTLInstallations())
	})
	if err != nil {
		return nil, false, err
	}
	if len(installations) > 0 {
		return installations, true, nil
	}
	if fetched != nil {
		return fetched, false, nil
	}
	if hit, err := cache.Get(key, &installations); err != nil {
		return nil, false, err
	} else if hit {
		return installations, true, nil
	}
	return []github.AppInstallation{}, false, nil
}

func getCachedInstallationID(config *Config, owner string) (int, bool, error) {
	key := ownerCacheKey(config.AppID, *config.GitHubAPI, owner)
	var cachedID int
	hit, err := cache.Get(key, &cachedID)
	if err != nil {
		return 0, false, err
	}
	if !hit || cachedID == 0 {
		return 0, false, nil
	}
	installationsKey := installationsCacheKey(config.AppID, *config.GitHubAPI)
	installations := []github.AppInstallation{}
	if listHit, err := cache.Get(installationsKey, &installations); err == nil && listHit {
		if !installationIDMatchesOwner(installations, owner, cachedID) {
			cache.Delete(key)
			refreshInstallationsCache(config)
			return 0, false, nil
		}
	}
	return cachedID, true, nil
}

func setCachedInstallationID(config *Config, owner string, id int) {
	key := ownerCacheKey(config.AppID, *config.GitHubAPI, owner)
	_ = cache.Set(key, id, cache.TTLOwnerMapping())
}

func installationIDMatchesOwner(installations []github.AppInstallation, owner string, id int) bool {
	for i := range installations {
		if installations[i].Account.Login == owner {
			return installations[i].ID == id
		}
	}
	return false
}

func refreshInstallationsCache(config *Config) {
	if config == nil || config.GitHubAPI == nil {
		return
	}
	cache.Delete(installationsCacheKey(config.AppID, *config.GitHubAPI))
	if config.ResolvedOwner != "" {
		cache.Delete(ownerCacheKey(config.AppID, *config.GitHubAPI, config.ResolvedOwner))
	}
}

func invalidateInstallationCaches(config *Config) {
	refreshInstallationsCache(config)
}

func installationsCacheKey(appID int, api string) string {
	return fmt.Sprintf("installations:app=%d api=%s", appID, api)
}

func ownerCacheKey(appID int, api, owner string) string {
	return fmt.Sprintf("owner_map:app=%d api=%s owner=%s", appID, api, owner)
}

func tokenCacheKey(config Config, requestData []byte) string {
	repoPart := "repos=all"
	idPart := "repo_ids=all"
	if config.Repositories != nil {
		repos := append([]string{}, *config.Repositories...)
		sort.Strings(repos)
		repoPart = fmt.Sprintf("repos=%s", strings.Join(repos, ","))
	}
	if config.RepositoryIDs != nil {
		ids := append([]int{}, *config.RepositoryIDs...)
		sort.Ints(ids)
		parts := make([]string, 0, len(ids))
		for _, id := range ids {
			parts = append(parts, fmt.Sprintf("%d", id))
		}
		idPart = fmt.Sprintf("repo_ids=%s", strings.Join(parts, ","))
	}
	permissionsPart := "permissions="
	if config.Permissions != nil {
		permissionsPart = fmt.Sprintf("permissions=%s", canonicalPermissions(*config.Permissions))
	}
	ownerPart := "owner="
	if config.ResolvedOwner != "" {
		ownerPart = fmt.Sprintf("owner=%s", config.ResolvedOwner)
	}
	return fmt.Sprintf(
		"token:app=%d api=%s installation=%d %s %s %s %s request=%s",
		config.AppID,
		*config.GitHubAPI,
		*config.InstallationID,
		ownerPart,
		repoPart,
		idPart,
		permissionsPart,
		string(requestData),
	)
}

func canonicalPermissions(raw json.RawMessage) string {
	decoded := map[string]interface{}{}
	if err := json.Unmarshal(raw, &decoded); err != nil {
		return string(raw)
	}
	out, err := json.Marshal(decoded)
	if err != nil {
		return string(raw)
	}
	return string(out)
}
