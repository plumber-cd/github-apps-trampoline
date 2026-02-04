package cmd

import (
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"os"
	"regexp"
	"strconv"
	"strings"
	"time"

	"github.com/spf13/cobra"
	"github.com/spf13/viper"

	"github.com/plumber-cd/github-apps-trampoline/helper"
	"github.com/plumber-cd/github-apps-trampoline/logger"
)

var (
	verbose bool

	server         string
	privateKey     string
	appID          int
	filter         string
	currentRepo    bool
	repositories   string
	repositoryIDs  string
	permissions    string
	installation   string
	installationID int

	cliMode bool

	logFile          string
	logTeeStderr     bool
	tokenFingerprint bool

	cfgFile string
	cfg     string
)

var rootCmd = &cobra.Command{
	Use:   "github-apps-trampoline",
	Short: "A GIT_ASKPASS trampoline for GitHub Apps",
	Long: `A cross-platform no-dependency GIT_ASKPASS trampoline for GitHub Apps,
				written in Go`,
	Args: cobra.ArbitraryArgs,
	Run: func(cmd *cobra.Command, args []string) {
		logger.Refresh()
		logger.Get().Println("hi")
		if viper.GetBool("verbose") {
			outData, err := json.MarshalIndent(viper.AllSettings(), "", "    ")
			cobra.CheckErr(err)
			logger.Get().Println(string(outData))
		}

		if cfgFile := viper.GetString("config"); cfgFile != "" {
			logger.Get().Printf("Reading config from file %s", cfgFile)
			dat, err := os.ReadFile(cfgFile)
			cobra.CheckErr(err)
			cfg = string(dat)
		} else if dat, present := os.LookupEnv("GITHUB_APPS_TRAMPOLINE"); present {
			logger.Get().Println("Reading config from environment")
			cfg = dat
		}

		if cfg == "" {
			logger.Get().Println("Config was not set - inferring in-memory from cli args")

			key := viper.GetString("key")
			if key == "" {
				cobra.CheckErr(errors.New("If no config was provided, must specify private key via --key or GITHUB_APPS_TRAMPOLINE_KEY"))
			}

			app := viper.GetInt("app")
			if app <= 0 {
				cobra.CheckErr(errors.New("If no config was provided, must specify app ID via --app or GITHUB_APPS_TRAMPOLINE_APP"))
			}

			filter := viper.GetString("filter")
			if filter == "" {
				logger.Get().Println("Filter was not set - assuming '.*'")
				filter = ".*"
			}

			config := helper.Config{
				PrivateKey: key,
				AppID:      app,
			}

			if server := viper.GetString("server"); server != "" {
				config.GitHubServer = &server
			}

			if api := viper.GetString("api"); api != "" {
				config.GitHubAPI = &api
			}

			if currentRepo := viper.GetBool("current-repo"); currentRepo {
				logger.Get().Println("Enabled: current-repo")
				config.CurrentRepositoryOnly = &currentRepo
			}

			if repositories := viper.GetString("repositories"); repositories != "" {
				logger.Get().Println("Enabled: repositories")
				split := strings.Split(repositories, ",")
				logger.Get().Printf("Repositories: %v", split)
				config.Repositories = &split
			}

			if repositoryIDs := viper.GetString("repository-ids"); repositoryIDs != "" {
				logger.Get().Println("Enabled: repository-ids")
				ids := strings.Split(repositoryIDs, ",")
				int_ids := make([]int, len(ids))
				for i, id := range ids {
					int_id, err := strconv.Atoi(id)
					cobra.CheckErr(err)
					int_ids[i] = int_id
				}
				logger.Get().Printf("Repository IDs: %v", int_ids)
				config.RepositoryIDs = &int_ids
			}

			if permissions := viper.GetString("permissions"); permissions != "" {
				logger.Get().Println("Enabled: permissions")
				raw := json.RawMessage(permissions)
				logger.Get().Printf("Permissions: %s", string(raw))
				config.Permissions = &raw
			}

			if installation := viper.GetString("installation"); installation != "" {
				logger.Get().Printf("Enabled: installation %q", installation)
				config.Installation = &installation
			}

			if installationID := viper.GetInt("installation-id"); installationID > 0 {
				logger.Get().Printf("Enabled: installation-id %q", installation)
				config.InstallationID = &installationID
			}

			obj := map[string]helper.Config{}
			obj[filter] = config

			jsonData, err := json.MarshalIndent(obj, "", "    ")
			cobra.CheckErr(err)
			cfg = string(jsonData)
		}

		logger.Get().Printf("Config: %s", cfg)
		_helper := helper.New(cfg)

		if cliMode = viper.GetBool("cli"); !cliMode {
			logger.Get().Println("Git AskPass Credentials Helper mode enabled")

			if len(args) != 1 || args[0] != "get" {
				logger.Get().Printf("Expecting single arg 'get', got: %v", args)
				logger.Get().Println("Silently exiting - nothing to do")
				os.Exit(0)
			}

			inBytes, err := io.ReadAll(os.Stdin)
			cobra.CheckErr(err)
			in := string(inBytes)
			logger.Get().Printf("Read input from git:\n%s", in)

			var protocol, host, path string

			re := regexp.MustCompile("(protocol|host|path)=(.*)")
			result := re.FindAllStringSubmatchIndex(in, -1)
			for _, match := range result {
				key := in[match[2]:match[3]]
				value := in[match[4]:match[5]]
				switch key {
				case "protocol":
					protocol = value
				case "host":
					host = value
				case "path":
					path = strings.TrimSuffix(value, ".git")
				}
			}

			if protocol != "https" {
				logger.Get().Printf("Expecting protocol 'https', got: %q", protocol)
				logger.Get().Println("Silently exiting - nothing to do")
				os.Exit(0)
			}

			repoPath := fmt.Sprintf("%s/%s", host, path)
			git, err := _helper.GitHelper(repoPath)
			checkSilentErr(err)

			token, err := git.GetToken()
			checkSilentErr(err)

			if viper.GetBool("token-fingerprint") {
				fp := sha256.Sum256([]byte(token))
				fingerprint := hex.EncodeToString(fp[:])
				logger.Get().Printf("Correlation: time=%s repo=%s token_fp=%s", time.Now().UTC().Format(time.RFC3339Nano), repoPath, fingerprint[:12])
			}

			logger.Filef("Returning token in a helper format: %q", token)
			logger.Stderrf("Returning token in a helper format: [redacted]")
			fmt.Printf("username=%s\n", "x-access-token")
			fmt.Printf("password=%s\n", token)
		} else {
			logger.Get().Println("Standalone CLI mode enabled")

			cli, err := _helper.CLIHelper()
			cobra.CheckErr(err)

			token, err := cli.GetToken()
			cobra.CheckErr(err)

			logger.Filef("Returning token in JSON format: %q", token)
			logger.Stderrf("Returning token in JSON format: [redacted]")
			out := map[string]string{
				"username": "x-access-token",
				"password": token,
			}
			outData, err := json.MarshalIndent(out, "", "    ")
			cobra.CheckErr(err)
			fmt.Println(string(outData))
		}
	},
}

func Execute() {
	if err := rootCmd.Execute(); err != nil {
		cobra.CheckErr(err)
	}
}

func init() {
	cobra.OnInitialize(initConfig)

	rootCmd.PersistentFlags().StringVarP(&cfgFile, "config", "c", "", "config file")
	if err := viper.BindPFlag("config", rootCmd.PersistentFlags().Lookup("config")); err != nil {
		cobra.CheckErr(err)
	}

	rootCmd.PersistentFlags().BoolVarP(&verbose, "verbose", "v", false, "verbose output")
	if err := viper.BindPFlag("verbose", rootCmd.PersistentFlags().Lookup("verbose")); err != nil {
		cobra.CheckErr(err)
	}

	rootCmd.PersistentFlags().BoolVar(&cliMode, "cli", false, "cli mode")
	if err := viper.BindPFlag("cli", rootCmd.PersistentFlags().Lookup("cli")); err != nil {
		cobra.CheckErr(err)
	}

	rootCmd.PersistentFlags().StringVarP(&server, "server", "s", "", "GitHub Server")
	if err := viper.BindPFlag("server", rootCmd.PersistentFlags().Lookup("server")); err != nil {
		cobra.CheckErr(err)
	}

	rootCmd.PersistentFlags().StringVar(&server, "api", "", "GitHub API url")
	if err := viper.BindPFlag("api", rootCmd.PersistentFlags().Lookup("api")); err != nil {
		cobra.CheckErr(err)
	}

	rootCmd.PersistentFlags().StringVarP(&privateKey, "key", "k", "", "path to the private key")
	if err := viper.BindPFlag("key", rootCmd.PersistentFlags().Lookup("key")); err != nil {
		cobra.CheckErr(err)
	}

	rootCmd.PersistentFlags().IntVarP(&appID, "app", "a", 0, "app ID")
	if err := viper.BindPFlag("app", rootCmd.PersistentFlags().Lookup("app")); err != nil {
		cobra.CheckErr(err)
	}

	rootCmd.PersistentFlags().StringVarP(&filter, "filter", "f", "", "filter")
	if err := viper.BindPFlag("filter", rootCmd.PersistentFlags().Lookup("filter")); err != nil {
		cobra.CheckErr(err)
	}

	rootCmd.PersistentFlags().BoolVar(&currentRepo, "current-repo", false, "if set to true and no repos provided - request token to the current repo")
	if err := viper.BindPFlag("current-repo", rootCmd.PersistentFlags().Lookup("current-repo")); err != nil {
		cobra.CheckErr(err)
	}

	rootCmd.PersistentFlags().StringVarP(&repositories, "repositories", "r", "", "repositories")
	if err := viper.BindPFlag("repositories", rootCmd.PersistentFlags().Lookup("repositories")); err != nil {
		cobra.CheckErr(err)
	}

	rootCmd.PersistentFlags().StringVar(&repositoryIDs, "repository-ids", "", "repository IDs")
	if err := viper.BindPFlag("repository-ids", rootCmd.PersistentFlags().Lookup("repository-ids")); err != nil {
		cobra.CheckErr(err)
	}

	rootCmd.PersistentFlags().StringVarP(&permissions, "permissions", "p", "", "permissions")
	if err := viper.BindPFlag("permissions", rootCmd.PersistentFlags().Lookup("permissions")); err != nil {
		cobra.CheckErr(err)
	}

	rootCmd.PersistentFlags().StringVarP(&installation, "installation", "i", "", "installation")
	if err := viper.BindPFlag("installation", rootCmd.PersistentFlags().Lookup("installation")); err != nil {
		cobra.CheckErr(err)
	}

	rootCmd.PersistentFlags().IntVar(&installationID, "installation-id", -1, "installation ID")
	if err := viper.BindPFlag("installation-id", rootCmd.PersistentFlags().Lookup("installation-id")); err != nil {
		cobra.CheckErr(err)
	}

	rootCmd.PersistentFlags().StringVar(&logFile, "log-file", "", "log file path")
	if err := viper.BindPFlag("log-file", rootCmd.PersistentFlags().Lookup("log-file")); err != nil {
		cobra.CheckErr(err)
	}

	rootCmd.PersistentFlags().BoolVar(&logTeeStderr, "log-tee-stderr", false, "tee logs to stderr even when log-file is set")
	if err := viper.BindPFlag("log-tee-stderr", rootCmd.PersistentFlags().Lookup("log-tee-stderr")); err != nil {
		cobra.CheckErr(err)
	}

	rootCmd.PersistentFlags().BoolVar(&tokenFingerprint, "token-fingerprint", false, "log token fingerprint and correlation line")
	if err := viper.BindPFlag("token-fingerprint", rootCmd.PersistentFlags().Lookup("token-fingerprint")); err != nil {
		cobra.CheckErr(err)
	}
}

func initConfig() {
	viper.SetEnvPrefix("GITHUB_APPS_TRAMPOLINE")
	viper.AutomaticEnv()

	if err := viper.ReadInConfig(); err == nil {
		if v := viper.GetBool("verbose"); v {
			fmt.Fprintln(os.Stderr, "Using config file:", viper.ConfigFileUsed())
		}
	}
}

func checkSilentErr(err error) {
	if err != nil {
		var s *helper.SilentExitError
		if errors.As(err, &s) {
			logger.Get().Printf("Silently exiting: %s", err)
			os.Exit(0)
		}
		cobra.CheckErr(err)
	}
}
