package cmd

import (
	"errors"
	"fmt"
	"os"

	"github.com/spf13/cobra"
	"github.com/spf13/viper"
)

var (
	cfgFile    string
	cfg        string
	privateKey string
	filter     string
	verbose    bool
)

var rootCmd = &cobra.Command{
	Use:   "github-apps-trampoline",
	Short: "A GIT_ASKPASS trampoline for GitHub Apps",
	Long: `A cross-platform no-dependency GIT_ASKPASS trampoline for GitHub Apps,
				written in Go`,
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
			key := viper.Get("key")
			filter := viper.Get("filter")
			if key == "" {
				fmt.Fprintln(os.Stderr, errors.New("If no config was provided, must at least specify private key via --key or GITHUB_APPS_TRAMPOLINE_KEY"))
				os.Exit(1)
			}

			if filter != "" {
				cfg = fmt.Sprintf(`{"%s": "%s"}`, filter, key)
			} else {
				cfg = fmt.Sprintf(`{"*": "%s"}`, key)
			}
		}

		fmt.Println(cfg)
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

	rootCmd.PersistentFlags().StringVarP(&privateKey, "key", "k", "", "path to the private key")
	if err := viper.BindPFlag("key", rootCmd.PersistentFlags().Lookup("key")); err != nil {
		fmt.Fprintln(os.Stderr, err)
		os.Exit(1)
	}

	rootCmd.PersistentFlags().StringVarP(&filter, "filter", "f", "", "filter")
	if err := viper.BindPFlag("filter", rootCmd.PersistentFlags().Lookup("filter")); err != nil {
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
