package cmd

import (
	"errors"
	"fmt"
	"os"

	homedir "github.com/mitchellh/go-homedir"
	"github.com/spf13/cobra"
	"github.com/spf13/pflag"
	"github.com/spf13/viper"
)

const defaultCfgFile = ".mgint"

var (
	cfgFile        string
	googleClientID string
	googleSecret   string
	mondayAPIKey   string

	rootCmd = &cobra.Command{
		Use:   "mgint",
		Short: "A tool to integrate monday.com boards and google calendar",
		PersistentPreRun: func(cmd *cobra.Command, args []string) {
			bindViperFlags(cmd.Flags())
			err := checkRequiredFlags(cmd.Flags())
			if err != nil {
				panic(fmt.Sprintf("issue checkRequiredFlags: %v", err))
			}
		},
	}
)

// Execute root command
func Execute() error {
	return rootCmd.Execute()
}

func init() {
	cobra.OnInitialize(initConfig)

	rootCmd.PersistentFlags().StringVar(&cfgFile, "config", "", fmt.Sprintf("config file (default is $HOME/%s)", defaultCfgFile))

	const mondayAPIKeyString = "mondayAPIKey"
	rootCmd.PersistentFlags().StringVar(&mondayAPIKey, mondayAPIKeyString, "", "Monday.com api key")
	rootCmd.MarkPersistentFlagRequired(mondayAPIKeyString)

	const googleClientIDString = "googleClientID"
	rootCmd.PersistentFlags().StringVar(&googleClientID, googleClientIDString, "", "Google client id for google calendar api access")
	rootCmd.MarkPersistentFlagRequired(googleClientIDString)

	const googleSecretString = "googleSecret"
	rootCmd.PersistentFlags().StringVar(&googleSecret, googleSecretString, "", "Google secret for google calendar api access")
	rootCmd.MarkPersistentFlagRequired(googleSecretString)
}

func er(msg interface{}) {
	fmt.Println("Error:", msg)
	os.Exit(1)
}

func initConfig() {
	if cfgFile != "" {
		viper.SetConfigFile(cfgFile)
	} else {
		home, err := homedir.Dir()
		if err != nil {
			er(err)
		}

		// if config file does not exist create it
		cfgFilePath := fmt.Sprintf("%s/%s", home, defaultCfgFile)
		if _, err := os.Stat(cfgFilePath); os.IsNotExist(err) {
			_, err := os.Create(cfgFilePath)
			if err != nil {
				panic(fmt.Sprintf("error creating config file: %v", err))
			}
		}

		viper.AddConfigPath(home)
		viper.SetConfigName(defaultCfgFile)
		viper.SetConfigType("yaml")
	}

	viper.AutomaticEnv()

	if err := viper.ReadInConfig(); err == nil {
		fmt.Println("Using config file:", viper.ConfigFileUsed())
	} else {
		panic("err: " + err.Error())
	}
}

func checkRequiredFlags(flags *pflag.FlagSet) error {
	requiredError := false
	flagName := ""

	flags.VisitAll(func(flag *pflag.Flag) {
		requiredAnnotation := flag.Annotations[cobra.BashCompOneRequiredFlag]
		if len(requiredAnnotation) == 0 {
			return
		}

		flagRequired := requiredAnnotation[0] == "true"

		if flagRequired && !flag.Changed {
			requiredError = true
			flagName = flag.Name
		}
	})

	if requiredError {
		return errors.New("Required flag `" + flagName + "` has not been set")
	}

	return nil
}

func bindViperFlags(flags *pflag.FlagSet) {
	viper.BindPFlags(flags)

	flags.VisitAll(func(f *pflag.Flag) {
		if viper.IsSet(f.Name) && viper.GetString(f.Name) != "" {
			flags.Set(f.Name, viper.GetString(f.Name))
		}
	})
}
