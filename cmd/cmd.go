package cmd

import (
	"encoding/json"
	"errors"
	"fmt"
	"io/ioutil"
	"os"
	"regexp"
	"strconv"
	"strings"

	"github.com/spf13/cobra"
	"github.com/spf13/viper"

	"github.com/plumber-cd/github-apps-trampoline/helper"
)

var (
	verbose bool

	privateKey     string
	appID          string
	filter         string
	currentRepo    bool
	repositories   string
	repositoryIDs  string
	permissions    string
	installation   string
	installationID int

	cliMode bool

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
		if cfgFile != "" {
			dat, err := os.ReadFile(cfgFile)
			cobra.CheckErr(err)
			cfg = string(dat)
		} else if dat, present := os.LookupEnv("GITHUB_APPS_TRAMPOLINE"); present {
			cfg = dat
		} else if file, present := os.LookupEnv("GITHUB_APPS_TRAMPOLINE_CONFIG"); present {
			dat, err := os.ReadFile(file)
			cobra.CheckErr(err)
			cfg = string(dat)
		}

		if cfg == "" {
			key := viper.GetString("key")
			if key == "" {
				fmt.Fprintln(os.Stderr, errors.New("If no config was provided, must specify private key via --key or GITHUB_APPS_TRAMPOLINE_KEY"))
				os.Exit(1)
			}

			app := viper.GetString("app")
			if app == "" {
				fmt.Fprintln(os.Stderr, errors.New("If no config was provided, must specify app ID via --app or GITHUB_APPS_TRAMPOLINE_APP"))
				os.Exit(1)
			}

			filter := viper.GetString("filter")
			if filter == "" {
				filter = ".*"
			}

			config := helper.Config{
				PrivateKey: key,
				AppID:      app,
			}

			if currentRepo := viper.GetBool("current-repo"); currentRepo {
				config.CurrentRepositoryOnly = &currentRepo
			}

			if repositories := viper.GetString("repositories"); repositories != "" {
				split := strings.Split(repositories, ",")
				config.Repositories = &split
			}

			if repositoryIDs := viper.GetString("repository-ids"); repositoryIDs != "" {
				ids := strings.Split(repositoryIDs, ",")
				int_ids := make([]int, len(ids))
				for i, id := range ids {
					int_id, err := strconv.Atoi(id)
					cobra.CheckErr(err)
					int_ids[i] = int_id
				}
				config.RepositoryIDs = &int_ids
			}

			if permissions := viper.GetString("permissions"); permissions != "" {
				raw := json.RawMessage(permissions)
				config.Permissions = &raw
			}

			if installation := viper.GetString("installation"); installation != "" {
				config.Installation = &installation
			}

			if installationID := viper.GetInt("installation-id"); installationID > 0 {
				config.InstallationID = &installationID
			}

			obj := map[string]helper.Config{}
			obj[filter] = config

			jsonData, err := json.MarshalIndent(obj, "", "    ")
			cobra.CheckErr(err)
			cfg = string(jsonData)
		}

		if cliMode = viper.GetBool("cli"); !cliMode {
			if args[0] != "get" {
				os.Exit(0)
			}

			inBytes, err := ioutil.ReadAll(os.Stdin)
			cobra.CheckErr(err)
			in := string(inBytes)

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
				os.Exit(0)
			}

			helper.Run(cfg, fmt.Sprintf("%s/%s", host, path))
		} else {
			helper.Run(cfg, "")
		}
	},
}

func Execute() {
	if err := rootCmd.Execute(); err != nil {
		fmt.Fprintln(os.Stderr, err)
		os.Exit(1)
	}
}

func init() {
	cobra.OnInitialize(initConfig)

	rootCmd.PersistentFlags().StringVarP(&cfgFile, "config", "c", "", "config file")

	rootCmd.PersistentFlags().BoolVarP(&verbose, "verbose", "v", false, "verbose output")
	if err := viper.BindPFlag("verbose", rootCmd.PersistentFlags().Lookup("verbose")); err != nil {
		fmt.Fprintln(os.Stderr, err)
		os.Exit(1)
	}

	rootCmd.PersistentFlags().BoolVar(&cliMode, "cli", false, "cli mode")
	if err := viper.BindPFlag("cli", rootCmd.PersistentFlags().Lookup("cli")); err != nil {
		fmt.Fprintln(os.Stderr, err)
		os.Exit(1)
	}

	rootCmd.PersistentFlags().StringVarP(&privateKey, "key", "k", "", "path to the private key")
	if err := viper.BindPFlag("key", rootCmd.PersistentFlags().Lookup("key")); err != nil {
		fmt.Fprintln(os.Stderr, err)
		os.Exit(1)
	}

	rootCmd.PersistentFlags().StringVarP(&appID, "app", "a", "", "app ID")
	if err := viper.BindPFlag("app", rootCmd.PersistentFlags().Lookup("app")); err != nil {
		fmt.Fprintln(os.Stderr, err)
		os.Exit(1)
	}

	rootCmd.PersistentFlags().StringVarP(&filter, "filter", "f", "", "filter")
	if err := viper.BindPFlag("filter", rootCmd.PersistentFlags().Lookup("filter")); err != nil {
		fmt.Fprintln(os.Stderr, err)
		os.Exit(1)
	}

	rootCmd.PersistentFlags().BoolVar(&currentRepo, "current-repo", false, "if set to true and no repos provided - request token to the current repo")
	if err := viper.BindPFlag("current-repo", rootCmd.PersistentFlags().Lookup("current-repo")); err != nil {
		fmt.Fprintln(os.Stderr, err)
		os.Exit(1)
	}

	rootCmd.PersistentFlags().StringVarP(&repositories, "repositories", "r", "", "repositories")
	if err := viper.BindPFlag("repositories", rootCmd.PersistentFlags().Lookup("repositories")); err != nil {
		fmt.Fprintln(os.Stderr, err)
		os.Exit(1)
	}

	rootCmd.PersistentFlags().StringVar(&repositoryIDs, "repository-ids", "", "repository IDs")
	if err := viper.BindPFlag("repository-ids", rootCmd.PersistentFlags().Lookup("repository-ids")); err != nil {
		fmt.Fprintln(os.Stderr, err)
		os.Exit(1)
	}

	rootCmd.PersistentFlags().StringVarP(&permissions, "permissions", "p", "", "permissions")
	if err := viper.BindPFlag("permissions", rootCmd.PersistentFlags().Lookup("permissions")); err != nil {
		fmt.Fprintln(os.Stderr, err)
		os.Exit(1)
	}

	rootCmd.PersistentFlags().StringVarP(&installation, "installation", "i", "", "installation")
	if err := viper.BindPFlag("installation", rootCmd.PersistentFlags().Lookup("installation")); err != nil {
		fmt.Fprintln(os.Stderr, err)
		os.Exit(1)
	}

	rootCmd.PersistentFlags().IntVar(&installationID, "installation-id", -1, "installation ID")
	if err := viper.BindPFlag("installation-id", rootCmd.PersistentFlags().Lookup("installation-id")); err != nil {
		fmt.Fprintln(os.Stderr, err)
		os.Exit(1)
	}
}

func initConfig() {
	viper.SetEnvPrefix("GITHUB_APPS_TRAMPOLINE")
	viper.AutomaticEnv()

	if err := viper.ReadInConfig(); err == nil {
		fmt.Fprintln(os.Stderr, "Using config file:", viper.ConfigFileUsed())
	}
}
