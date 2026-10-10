package main

import (
	"context"
	"fmt"

	notebooklm "github.com/saeedata/notebooklm-go"
	"github.com/spf13/cobra"
)

var notesCmd = &cobra.Command{
	Use:   "notes",
	Short: "Manage notebook notes and mind maps",
}

func init() {
	var (
		notebookID string
		sourceIDs  []string
		noteTitle  string
	)

	listCmd := &cobra.Command{
		Use:   "list",
		Short: "List all notes in a notebook",
		RunE: func(cmd *cobra.Command, args []string) error {
			c, err := client()
			if err != nil {
				return err
			}
			nb := notebookOrCtx(notebookID)
			if err := requireNotebook(nb); err != nil {
				return err
			}
			notes, err := c.Notes.List(context.Background(), nb)
			if err != nil {
				return err
			}
			if outputJSON {
				printJSON(notes)
				return nil
			}
			if len(notes) == 0 {
				fmt.Println("No notes found.")
				return nil
			}
			for _, n := range notes {
				fmt.Printf("%s\t%s\n", n.ID, n.Title)
			}
			return nil
		},
	}
	listCmd.Flags().StringVarP(&notebookID, "notebook", "n", "", "notebook ID (required)")

	createCmd := &cobra.Command{
		Use:   "create <title> <content>",
		Short: "Create a new note",
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
			note, err := c.Notes.Create(context.Background(), nb, args[0], args[1])
			if err != nil {
				return err
			}
			if outputJSON {
				printJSON(note)
				return nil
			}
			fmt.Printf("Created note: %s (%s)\n", note.Title, note.ID)
			return nil
		},
	}
	createCmd.Flags().StringVarP(&notebookID, "notebook", "n", "", "notebook ID (required)")

	updateCmd := &cobra.Command{
		Use:   "update <note-id> <content>",
		Short: "Update note content",
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
			if _, err := c.Notes.Update(context.Background(), nb, args[0], noteTitle, args[1]); err != nil {
				return err
			}
			fmt.Printf("Updated note: %s\n", args[0])
			return nil
		},
	}
	updateCmd.Flags().StringVarP(&notebookID, "notebook", "n", "", "notebook ID (required)")
	updateCmd.Flags().StringVar(&noteTitle, "title", "", "new title (default: keep the current title)")

	deleteCmd := &cobra.Command{
		Use:   "delete <note-id>",
		Short: "Delete a note",
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
			if err := c.Notes.Delete(context.Background(), nb, args[0]); err != nil {
				return err
			}
			fmt.Printf("Deleted note: %s\n", args[0])
			return nil
		},
	}
	deleteCmd.Flags().StringVarP(&notebookID, "notebook", "n", "", "notebook ID (required)")

	var mindMapInstructions, mindMapLanguage string
	mindMapCmd := &cobra.Command{
		Use:   "mind-map",
		Short: "Generate a mind map from notebook sources and save it as a note",
		RunE: func(cmd *cobra.Command, args []string) error {
			c, err := client()
			if err != nil {
				return err
			}
			nb := notebookOrCtx(notebookID)
			if err := requireNotebook(nb); err != nil {
				return err
			}
			note, err := c.Notes.GenerateMindMap(context.Background(), nb, notebooklm.GenerateMindMapOptions{
				SourceIDs:    sourceIDs,
				Language:     mindMapLanguage,
				Instructions: mindMapInstructions,
			})
			if err != nil {
				return err
			}
			if outputJSON {
				printJSON(note)
				return nil
			}
			fmt.Printf("Saved mind map as note: %s (%s)\n", note.Title, note.ID)
			return nil
		},
	}
	mindMapCmd.Flags().StringVarP(&notebookID, "notebook", "n", "", "notebook ID (required)")
	mindMapCmd.Flags().StringArrayVar(&sourceIDs, "source", nil, "source IDs to include (default: all)")
	mindMapCmd.Flags().StringVar(&mindMapInstructions, "instructions", "", "what the mind map should focus on")
	mindMapCmd.Flags().StringVarP(&mindMapLanguage, "language", "l", "en", "output language (BCP 47)")

	notesCmd.AddCommand(listCmd, createCmd, updateCmd, deleteCmd, mindMapCmd)
}
