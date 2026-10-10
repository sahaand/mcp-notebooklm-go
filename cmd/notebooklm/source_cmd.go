package main

import (
	"context"
	"fmt"
	"strings"

	"github.com/saeedata/notebooklm-go/rpc"
	"github.com/spf13/cobra"
)

var sourceCmd = &cobra.Command{
	Use:   "source",
	Short: "Manage notebook sources",
}

func init() {
	var notebookID string
	var mimeType string
	var driveTitle string
	var discoverMode string

	listCmd := &cobra.Command{
		Use:   "list",
		Short: "List sources in a notebook",
		RunE: func(cmd *cobra.Command, args []string) error {
			c, err := client()
			if err != nil {
				return err
			}
			nb := notebookOrCtx(notebookID)
			if err := requireNotebook(nb); err != nil {
				return err
			}
			sources, err := c.Sources.List(context.Background(), nb)
			if err != nil {
				return err
			}
			if outputJSON {
				printJSON(sources)
				return nil
			}
			for _, s := range sources {
				fmt.Printf("%s\t%s\t%s\n", s.ID, string(s.Kind()), s.Title)
			}
			return nil
		},
	}
	listCmd.Flags().StringVarP(&notebookID, "notebook", "n", "", "notebook ID (required)")

	getCmd := &cobra.Command{
		Use:   "get <source-id>",
		Short: "Get source details",
		Args:  cobra.ExactArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			c, err := client()
			if err != nil {
				return err
			}
			nb := notebookOrCtx(notebookID)
			if err := requireNotebook(nb); err != nil {
				return err
			}
			src, err := c.Sources.Get(context.Background(), nb, args[0])
			if err != nil {
				return err
			}
			if outputJSON {
				printJSON(src)
				return nil
			}
			fmt.Printf("ID:     %s\nTitle:  %s\nType:   %s\nStatus: %s\n", src.ID, src.Title, string(src.Kind()), rpc.SourceStatusToStr(src.Status))
			if src.URL != "" {
				fmt.Printf("URL:    %s\n", src.URL)
			}
			return nil
		},
	}
	getCmd.Flags().StringVarP(&notebookID, "notebook", "n", "", "notebook ID (required)")

	addURLCmd := &cobra.Command{
		Use:   "add-url <url>",
		Short: "Add a URL or YouTube video as a source",
		Args:  cobra.ExactArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			c, err := client()
			if err != nil {
				return err
			}
			nb := notebookOrCtx(notebookID)
			if err := requireNotebook(nb); err != nil {
				return err
			}
			src, err := c.Sources.AddURL(context.Background(), nb, args[0])
			if err != nil {
				return err
			}
			fmt.Printf("Added source: %s (%s)\n", src.ID, args[0])
			return nil
		},
	}
	addURLCmd.Flags().StringVarP(&notebookID, "notebook", "n", "", "notebook ID (required)")

	addTextCmd := &cobra.Command{
		Use:   "add-text <title> <content>",
		Short: "Add pasted text as a source",
		Args:  cobra.ExactArgs(2),
		RunE: func(cmd *cobra.Command, args []string) error {
			c, err := client()
			if err != nil {
				return err
			}
			nb := notebookOrCtx(notebookID)
			if err := requireNotebook(nb); err != nil {
				return err
			}
			src, err := c.Sources.AddText(context.Background(), nb, args[0], args[1])
			if err != nil {
				return err
			}
			fmt.Printf("Added text source: %s\n", src.ID)
			return nil
		},
	}
	addTextCmd.Flags().StringVarP(&notebookID, "notebook", "n", "", "notebook ID (required)")

	addFileCmd := &cobra.Command{
		Use:   "add-file <path>",
		Short: "Upload and add a file as a source (PDF, TXT, MD, DOCX)",
		Args:  cobra.ExactArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			c, err := client()
			if err != nil {
				return err
			}
			nb := notebookOrCtx(notebookID)
			if err := requireNotebook(nb); err != nil {
				return err
			}
			src, err := c.Sources.AddFile(context.Background(), nb, args[0])
			if err != nil {
				return err
			}
			fmt.Printf("Added file source: %s\n", src.ID)
			return nil
		},
	}
	addFileCmd.Flags().StringVarP(&notebookID, "notebook", "n", "", "notebook ID (required)")

	addDriveCmd := &cobra.Command{
		Use:   "add-drive <drive-file-id-or-url>",
		Short: "Add a Google Drive document as a source",
		Args:  cobra.ExactArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			c, err := client()
			if err != nil {
				return err
			}
			nb := notebookOrCtx(notebookID)
			if err := requireNotebook(nb); err != nil {
				return err
			}
			src, err := c.Sources.AddDrive(context.Background(), nb, args[0], driveTitle, mimeType)
			if err != nil {
				return err
			}
			fmt.Printf("Added Drive source: %s\n", src.ID)
			return nil
		},
	}
	addDriveCmd.Flags().StringVarP(&notebookID, "notebook", "n", "", "notebook ID (required)")
	addDriveCmd.Flags().StringVar(&mimeType, "mime-type", "", "MIME type (default: application/vnd.google-apps.document)")
	addDriveCmd.Flags().StringVar(&driveTitle, "title", "", "display title (default: the Drive file's title)")

	renameCmd := &cobra.Command{
		Use:   "rename <source-id> <new-title>",
		Short: "Rename a source",
		Args:  cobra.ExactArgs(2),
		RunE: func(cmd *cobra.Command, args []string) error {
			c, err := client()
			if err != nil {
				return err
			}
			nb := notebookOrCtx(notebookID)
			if err := requireNotebook(nb); err != nil {
				return err
			}
			src, err := c.Sources.Rename(context.Background(), nb, args[0], args[1])
			if err != nil {
				return err
			}
			fmt.Printf("Renamed to: %s\n", src.Title)
			return nil
		},
	}
	renameCmd.Flags().StringVarP(&notebookID, "notebook", "n", "", "notebook ID (required)")

	refreshCmd := &cobra.Command{
		Use:   "refresh <source-id>",
		Short: "Re-fetch content for a URL or Drive source",
		Args:  cobra.ExactArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			c, err := client()
			if err != nil {
				return err
			}
			nb := notebookOrCtx(notebookID)
			if err := requireNotebook(nb); err != nil {
				return err
			}
			if err := c.Sources.Refresh(context.Background(), nb, args[0]); err != nil {
				return err
			}
			fmt.Printf("Refreshed source: %s\n", args[0])
			return nil
		},
	}
	refreshCmd.Flags().StringVarP(&notebookID, "notebook", "n", "", "notebook ID (required)")

	deleteCmd := &cobra.Command{
		Use:   "delete <source-id>",
		Short: "Delete a source from a notebook",
		Args:  cobra.ExactArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			c, err := client()
			if err != nil {
				return err
			}
			nb := notebookOrCtx(notebookID)
			if err := requireNotebook(nb); err != nil {
				return err
			}
			if err := c.Sources.Delete(context.Background(), nb, args[0]); err != nil {
				return err
			}
			fmt.Printf("Deleted source: %s\n", args[0])
			return nil
		},
	}
	deleteCmd.Flags().StringVarP(&notebookID, "notebook", "n", "", "notebook ID (required)")

	getGuideCmd := &cobra.Command{
		Use:   "get-guide <source-id>",
		Short: "Get the guide text for a source",
		Args:  cobra.ExactArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			c, err := client()
			if err != nil {
				return err
			}
			nb := notebookOrCtx(notebookID)
			if err := requireNotebook(nb); err != nil {
				return err
			}
			guide, err := c.Sources.GetGuide(context.Background(), nb, args[0])
			if err != nil {
				return err
			}
			if outputJSON {
				printJSON(guide)
				return nil
			}
			fmt.Println(guide.Summary)
			if len(guide.Keywords) > 0 {
				fmt.Printf("\nKeywords: %s\n", strings.Join(guide.Keywords, ", "))
			}
			return nil
		},
	}
	getGuideCmd.Flags().StringVarP(&notebookID, "notebook", "n", "", "notebook ID (required)")

	checkFreshnessCmd := &cobra.Command{
		Use:   "check-freshness <source-id>",
		Short: "Check whether a source is fresh",
		Args:  cobra.ExactArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			c, err := client()
			if err != nil {
				return err
			}
			nb := notebookOrCtx(notebookID)
			if err := requireNotebook(nb); err != nil {
				return err
			}
			fresh, err := c.Sources.CheckFreshness(context.Background(), nb, args[0])
			if err != nil {
				return err
			}
			if outputJSON {
				printJSON(map[string]bool{"fresh": fresh})
				return nil
			}
			fmt.Printf("Fresh: %v\n", fresh)
			return nil
		},
	}
	checkFreshnessCmd.Flags().StringVarP(&notebookID, "notebook", "n", "", "notebook ID (required)")

	getFulltextCmd := &cobra.Command{
		Use:   "get-fulltext <source-id>",
		Short: "Print the full indexed text of a source",
		Args:  cobra.ExactArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			c, err := client()
			if err != nil {
				return err
			}
			nb := notebookOrCtx(notebookID)
			if err := requireNotebook(nb); err != nil {
				return err
			}
			ft, err := c.Sources.GetFulltext(context.Background(), nb, args[0])
			if err != nil {
				return err
			}
			if outputJSON {
				printJSON(ft)
				return nil
			}
			fmt.Println(ft.Content)
			return nil
		},
	}
	getFulltextCmd.Flags().StringVarP(&notebookID, "notebook", "n", "", "notebook ID (required)")

	discoverCmd := &cobra.Command{
		Use:   "discover [query]",
		Short: "Search the web for sources to add to a notebook",
		Args:  cobra.MaximumNArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			c, err := client()
			if err != nil {
				return err
			}
			nb := notebookOrCtx(notebookID)
			if err := requireNotebook(nb); err != nil {
				return err
			}
			query := ""
			if len(args) == 1 {
				query = args[0]
			}
			res, err := c.Sources.DiscoverSources(context.Background(), nb, query, discoverMode)
			if err != nil {
				return err
			}
			if outputJSON {
				printJSON(res)
				return nil
			}
			if res.Summary != "" {
				fmt.Printf("%s\n\n", res.Summary)
			}
			if len(res.Sources) == 0 {
				fmt.Println("No sources discovered.")
				return nil
			}
			for _, s := range res.Sources {
				fmt.Printf("%s\t%s\n", s.URL, s.Title)
			}
			return nil
		},
	}
	discoverCmd.Flags().StringVarP(&notebookID, "notebook", "n", "", "notebook ID (required)")
	discoverCmd.Flags().StringVar(&discoverMode, "mode", "default", "default, raw, curious or curious_raw (curious modes ignore the query)")

	sourceCmd.AddCommand(listCmd, getCmd, addURLCmd, addTextCmd, addFileCmd, addDriveCmd, renameCmd, refreshCmd, deleteCmd, getGuideCmd, getFulltextCmd, checkFreshnessCmd, discoverCmd)
}
