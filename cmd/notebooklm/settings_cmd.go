package main

import (
	"context"
	"fmt"

	"github.com/spf13/cobra"
)

var settingsCmd = &cobra.Command{
	Use:   "settings",
	Short: "Manage user settings",
}

func init() {
	getCmd := &cobra.Command{
		Use:   "get",
		Short: "Get current user settings",
		RunE: func(cmd *cobra.Command, args []string) error {
			c, err := client()
			if err != nil {
				return err
			}
			settings, err := c.Settings.Get(context.Background())
			if err != nil {
				return err
			}
			if outputJSON {
				printJSON(settings)
				return nil
			}
			lang := settings.OutputLanguage
			if lang == "" {
				lang = "(account default)"
			}
			fmt.Printf("Output language: %s\n", lang)
			return nil
		},
	}

	setLangCmd := &cobra.Command{
		Use:   "set-language <language-code>",
		Short: "Set output language (e.g., en, es, fr, de, ja)",
		Args:  cobra.ExactArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			c, err := client()
			if err != nil {
				return err
			}
			settings, err := c.Settings.SetOutputLanguage(context.Background(), args[0])
			if err != nil {
				return err
			}
			fmt.Printf("Output language set to: %s\n", settings.OutputLanguage)
			return nil
		},
	}

	settingsCmd.AddCommand(getCmd, setLangCmd)
}
