package cmd

import (
	"fmt"
	"os"
	"strings"

	homedir "github.com/mitchellh/go-homedir"
	"github.com/spf13/cobra"
	"github.com/spf13/pflag"
	"github.com/spf13/viper"
)

const (
	defaultCfgFile = ".mgint.yaml"
)

var (
	cfgFile string

	mondayAPIKey   string
	googleClientID string
	googleSecret   string
)

var configFlags = []configFlag{
	configFlag{
		Name:   "mondayAPIKey",
		Value:  "",
		Usage:  "Monday.com api key",
		RefVar: &mondayAPIKey,
	},
	configFlag{
		Name:   "googleClientID",
		Value:  "",
		Usage:  "Google client id for google calendar api access",
		RefVar: &googleClientID,
	},
	configFlag{
		Name:   "googleSecret",
		Value:  "",
		Usage:  "Google secret for google calendar api access",
		RefVar: &googleSecret,
	},
}

const requiredAnnotationString = "requiredByMgint"

type configFlag struct {
	Name   string
	Value  string
	Usage  string
	RefVar *string
}

var rootCmd = &cobra.Command{
	Use:   "mgint",
	Short: "A tool to integrate monday.com boards and google calendar",
	// runs on all subcommands unless they have their own PersistentPreRunE declared
	PersistentPreRunE: func(cmd *cobra.Command, args []string) error {
		bindViperFlags(cmd.Flags())
		err := checkRequiredFlags(cmd.Flags())
		if err != nil {
			return err
		}
		return nil
	},
	Run: func(cmd *cobra.Command, args []string) {
		cmd.Usage()
	},
}

// Execute root command
func Execute() error {
	return rootCmd.Execute()
}

func init() {
	cobra.OnInitialize(initViperConfig)

	rootCmd.PersistentFlags().StringVar(&cfgFile, "config", "", fmt.Sprintf("config file (default is $HOME/%s)", defaultCfgFile))

	for _, cF := range configFlags {
		rootCmd.PersistentFlags().StringVar(cF.RefVar, cF.Name, cF.Value, cF.Usage)
		rootCmd.PersistentFlags().SetAnnotation(cF.Name, requiredAnnotationString, []string{"true"})
	}
}

func initViperConfig() {
	if cfgFile != "" {
		viper.SetConfigFile(cfgFile)
	} else {
		home, err := homedir.Dir()
		if err != nil {
			fmt.Println("Error: ", err)
			os.Exit(1)
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

// doing custom flag requirement checking because we don't want this
// to be have required flag on the subcommand 'config' where we write
// these flags to the config
func checkRequiredFlags(flags *pflag.FlagSet) error {
	cFlagMap := map[string]*configFlag{}
	for _, cF := range configFlags {
		cFlagMap[cF.Name] = &cF
	}

	flags.VisitAll(func(flag *pflag.Flag) {
		requiredAnnotation := flag.Annotations[requiredAnnotationString]
		if len(requiredAnnotation) == 0 {
			return
		}

		flagRequired := requiredAnnotation[0] == "true"
		if flagRequired && flag.Changed {
			delete(cFlagMap, flag.Name)
		}
	})

	if len(cFlagMap) > 0 {
		requiredFlagsNotSetMsg := "The following flag(s) are not set and are required"

		flagNames := make([]string, 0, len(cFlagMap))
		for flagName := range cFlagMap {
			flagNames = append(flagNames, flagName)
		}

		return fmt.Errorf("%s: %s", requiredFlagsNotSetMsg, strings.Join(flagNames, ", "))
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
