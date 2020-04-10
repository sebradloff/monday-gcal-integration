package cmd

import (
	"errors"
	"fmt"
	"strings"

	"github.com/spf13/cobra"
	"github.com/spf13/viper"
)

var configCmd = &cobra.Command{
	Use:   "config",
	Short: "To set api keys and secrets needed for the cli tool",
}

func init() {
	configCmd.AddCommand(newCmdConfigSet())

	rootCmd.AddCommand(configCmd)
}

const usageString = "mgint config set <key>=<value>"

func newCmdConfigSet() *cobra.Command {
	configSetCmd := &cobra.Command{
		Use:   "set",
		Short: "change variables in the config file",
		Long:  "set one or all the config variables using '" + usageString + "'",
		// allows us to ignore the required config flags set on the root cmd
		PersistentPreRunE: func(cmd *cobra.Command, args []string) error {
			return nil
		},
		RunE: func(cmd *cobra.Command, args []string) error {
			return set(args)
		},
	}

	usage :=
		"Usage:\n" +
			"  " + usageString + "\n"

	flagNames := make([]string, 0, len(configFlags))
	flagUsageArr := make([]string, 0, len(configFlags))
	for _, cF := range configFlags {
		flagNames = append(flagNames, cF.Name)
		flagUsageArr = append(flagNames, fmt.Sprintf("%s=value", cF.Name))
	}
	validKeyNames := strings.Join(flagNames, ", ")
	flagUsageString := strings.Join(flagUsageArr, " ")

	usageExamples :=
		"Usage examples:\n" +
			"  mgint config set " + flagUsageString + "\n" +
			"  mgint config set " + configFlags[0].Name + "=value"
	validKeyNamesString :=
		"Valid key names:\n" +
			"  " + validKeyNames + "\n"

	useageFunc := func(c *cobra.Command) error {
		fmt.Println(usage)
		fmt.Println(validKeyNamesString)
		fmt.Println(usageExamples)
		return errors.New(usageString)
	}
	helpFunc := func(c *cobra.Command, s []string) {
		fmt.Println(c.Long)
		c.Usage()
	}

	configSetCmd.SetUsageFunc(useageFunc)
	configSetCmd.SetHelpFunc(helpFunc)

	return configSetCmd
}

func set(args []string) error {
	if len(args) < 1 {
		return errors.New("no args given")
	}

	for _, arg := range args {
		keys := strings.Split(arg, "=")
		if len(keys) < 2 || keys[0] == "" || keys[1] == "" {
			return errors.New("use '<key>=<value>' format (no spaces)")
		}

		flagExists := false
		for _, cF := range configFlags {
			if cF.Name == keys[0] {
				flagExists = true

				viper.Set(cF.Name, keys[1])
				err := viper.WriteConfig()
				if err != nil {
					panic(err)
				}
			}
		}

		if !flagExists {
			return fmt.Errorf("the key '%s' is not a valid key in the config", keys[0])
		}
	}
	return nil
}
