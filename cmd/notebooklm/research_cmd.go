package main

import (
	"context"
	"fmt"
	"strings"
	"time"

	"github.com/spf13/cobra"
)

var researchCmd = &cobra.Command{
	Use:   "research",
	Short: "Web research operations",
}

func init() {
	var (
		notebookID string
		waitFlag   bool
		importFlag bool
	)

	fastCmd := &cobra.Command{
		Use:   "fast <query>",
		Short: "Start a fast web research query",
		Args:  cobra.MinimumNArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			c, err := client()
			if err != nil {
				return err
			}
			nb := notebookOrCtx(notebookID)
			if err := requireNotebook(nb); err != nil {
				return err
			}
			query := strings.Join(args, " ")
			res, err := c.Research.StartFast(context.Background(), nb, query)
			if err != nil {
				return err
			}
			if outputJSON {
				printJSON(res)
				return nil
			}
			fmt.Printf("Started fast research: task=%s status=%s\n", res.TaskID, res.Status)
			return nil
		},
	}
	fastCmd.Flags().StringVarP(&notebookID, "notebook", "n", "", "notebook ID (required)")

	deepCmd := &cobra.Command{
		Use:   "deep <query>",
		Short: "Start a deep web research query",
		Args:  cobra.MinimumNArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			c, err := client()
			if err != nil {
				return err
			}
			nb := notebookOrCtx(notebookID)
			if err := requireNotebook(nb); err != nil {
				return err
			}
			query := strings.Join(args, " ")
			res, err := c.Research.StartDeep(context.Background(), nb, query)
			if err != nil {
				return err
			}
			if !waitFlag || res.TaskID == "" {
				if outputJSON {
					printJSON(res)
					return nil
				}
				fmt.Printf("Started deep research: task=%s status=%s\n", res.TaskID, res.Status)
				return nil
			}
			fmt.Println("Waiting for research completion...")
			completed, err := c.Research.WaitForCompletion(context.Background(), nb, res.TaskID, 15*time.Minute)
			if err != nil {
				return err
			}
			if importFlag && len(completed.Sources) > 0 {
				fmt.Printf("Importing %d sources...\n", len(completed.Sources))
				imported, err := c.Research.Import(context.Background(), nb, completed.TaskID, completed.Sources)
				if err != nil {
					return fmt.Errorf("import: %w", err)
				}
				fmt.Printf("Imported %d sources.\n", len(imported))
			}
			if outputJSON {
				printJSON(completed)
				return nil
			}
			fmt.Printf("Status: %s\n", completed.Status)
			for _, src := range completed.Sources {
				if src.URL != "" {
					fmt.Printf("  %s\t%s\n", src.URL, src.Title)
				} else {
					fmt.Printf("  [report] %s\n", src.Title)
				}
			}
			return nil
		},
	}
	deepCmd.Flags().StringVarP(&notebookID, "notebook", "n", "", "notebook ID (required)")
	deepCmd.Flags().BoolVarP(&waitFlag, "wait", "w", false, "wait for completion")
	deepCmd.Flags().BoolVar(&importFlag, "import", false, "auto-import discovered sources after completion (requires --wait)")

	researchCmd.AddCommand(fastCmd, deepCmd)
}
