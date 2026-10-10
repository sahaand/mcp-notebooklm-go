package main

import (
	"context"
	"fmt"
	"time"

	"github.com/spf13/cobra"
)

var notesCmd = &cobra.Command{
	Use:   "notes",
	Short: "Manage notebook notes and mind maps",
}

func init() {
	var (
		notebookID string
		waitFlag   bool
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

	mindMapCmd := &cobra.Command{
		Use:   "mind-map",
		Short: "Generate a mind map from notebook sources",
		RunE: func(cmd *cobra.Command, args []string) error {
			c, err := client()
			if err != nil {
				return err
			}
			nb := notebookOrCtx(notebookID)
			if err := requireNotebook(nb); err != nil {
				return err
			}
			status, err := c.Notes.GenerateMindMap(context.Background(), nb, sourceIDs)
			if err != nil {
				return err
			}
			if !waitFlag || status.TaskID == "" {
				fmt.Printf("Started mind map generation: task=%s\n", status.TaskID)
				return nil
			}
			fmt.Println("Waiting for completion...")
			completed, err := c.Artifacts.WaitForCompletion(context.Background(), nb, status.TaskID, 10*time.Minute)
			if err != nil {
				return err
			}
			if outputJSON {
				printJSON(completed)
				return nil
			}
			fmt.Printf("Status: %s\n", completed.Status)
			return nil
		},
	}
	mindMapCmd.Flags().StringVarP(&notebookID, "notebook", "n", "", "notebook ID (required)")
	mindMapCmd.Flags().BoolVarP(&waitFlag, "wait", "w", false, "wait for completion")
	mindMapCmd.Flags().StringArrayVar(&sourceIDs, "source", nil, "source IDs to include")

	notesCmd.AddCommand(listCmd, createCmd, updateCmd, deleteCmd, mindMapCmd)
}
