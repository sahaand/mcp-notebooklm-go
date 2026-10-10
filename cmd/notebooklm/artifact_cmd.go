package main

import (
	"context"
	"fmt"
	"os"
	"strconv"
	"time"

	notebooklm "github.com/saeedata/notebooklm-go"
	"github.com/saeedata/notebooklm-go/rpc"
	"github.com/spf13/cobra"
)

const artifactTimeout = 10 * time.Minute

var artifactCmd = &cobra.Command{
	Use:   "artifact",
	Short: "Generate and manage studio artifacts",
}

func init() {
	var (
		notebookID   string
		language     string
		instructions string
		waitFlag     bool
		destPath     string
		sourceIDs    []string
		formatStr    string
		styleStr     string
		lengthStr    string
		detailStr    string
		orientStr    string
		quantity     string
		difficulty   string
		customPrompt string
		stylePrompt  string
		extraInstr   string
	)

	listCmd := &cobra.Command{
		Use:   "list",
		Short: "List artifacts in a notebook",
		RunE: func(cmd *cobra.Command, args []string) error {
			c, err := client()
			if err != nil {
				return err
			}
			nb := notebookOrCtx(notebookID)
			if err := requireNotebook(nb); err != nil {
				return err
			}
			artifacts, err := c.Artifacts.List(context.Background(), nb)
			if err != nil {
				return err
			}
			if outputJSON {
				printJSON(artifacts)
				return nil
			}
			for _, a := range artifacts {
				fmt.Printf("%s\t%s\t%s\n", a.ID, string(a.Kind()), a.Title)
			}
			return nil
		},
	}
	listCmd.Flags().StringVarP(&notebookID, "notebook", "n", "", "notebook ID (required)")

	getCmd := &cobra.Command{
		Use:   "get <artifact-id>",
		Short: "Get artifact details",
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
			art, err := c.Artifacts.Get(context.Background(), nb, args[0])
			if err != nil {
				return err
			}
			if outputJSON {
				printJSON(art)
				return nil
			}
			fmt.Printf("ID:     %s\nTitle:  %s\nType:   %s\nStatus: %s\n", art.ID, art.Title, string(art.Kind()), rpc.ArtifactStatusToStr(art.Status))
			if art.URL != "" {
				fmt.Printf("URL:    %s\n", art.URL)
			}
			return nil
		},
	}
	getCmd.Flags().StringVarP(&notebookID, "notebook", "n", "", "notebook ID (required)")

	renameCmd := &cobra.Command{
		Use:   "rename <artifact-id> <new-title>",
		Short: "Rename an artifact",
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
			if err := c.Artifacts.Rename(context.Background(), nb, args[0], args[1]); err != nil {
				return err
			}
			fmt.Printf("Renamed artifact: %s\n", args[0])
			return nil
		},
	}
	renameCmd.Flags().StringVarP(&notebookID, "notebook", "n", "", "notebook ID (required)")

	// waitAndDownload is a helper for generation commands that support --wait and --output.
	waitAndDownload := func(c *notebooklm.Client, notebookID string, status *notebooklm.GenerationStatus, destPath string) error {
		if !waitFlag || status.TaskID == "" {
			fmt.Printf("Started generation: task=%s\n", status.TaskID)
			return nil
		}
		fmt.Println("Waiting for completion...")
		completed, err := c.Artifacts.WaitForCompletion(context.Background(), notebookID, status.TaskID, artifactTimeout)
		if err != nil {
			return err
		}
		fmt.Printf("Status: %s\n", completed.Status)
		if destPath != "" {
			if err := c.Artifacts.Download(context.Background(), notebookID, status.TaskID, destPath); err != nil {
				return fmt.Errorf("download: %w", err)
			}
			fmt.Printf("Downloaded to: %s\n", destPath)
		}
		return nil
	}

	audioCmd := &cobra.Command{
		Use:   "audio",
		Short: "Generate an audio overview",
		RunE: func(cmd *cobra.Command, args []string) error {
			c, err := client()
			if err != nil {
				return err
			}
			nb := notebookOrCtx(notebookID)
			if err := requireNotebook(nb); err != nil {
				return err
			}
			opts := notebooklm.GenerateAudioOptions{
				SourceIDs:    sourceIDs,
				Language:     language,
				Instructions: instructions,
			}
			switch formatStr {
			case "deep-dive":
				opts.Format = rpc.AudioDeepDive
			case "brief":
				opts.Format = rpc.AudioBrief
			case "critique":
				opts.Format = rpc.AudioCritique
			case "debate":
				opts.Format = rpc.AudioDebate
			}
			switch lengthStr {
			case "short":
				opts.Length = rpc.AudioShort
			case "long":
				opts.Length = rpc.AudioLong
			case "default":
				opts.Length = rpc.AudioDefault
			}
			status, err := c.Artifacts.GenerateAudio(context.Background(), nb, opts)
			if err != nil {
				return err
			}
			return waitAndDownload(c, nb, status, destPath)
		},
	}
	audioCmd.Flags().StringVarP(&notebookID, "notebook", "n", "", "notebook ID (required)")
	audioCmd.Flags().StringVarP(&language, "language", "l", "en", "output language (BCP 47)")
	audioCmd.Flags().StringVar(&instructions, "instructions", "", "custom instructions")
	audioCmd.Flags().BoolVarP(&waitFlag, "wait", "w", false, "wait for completion")
	audioCmd.Flags().StringVarP(&destPath, "output", "o", "", "download path when --wait is set")
	audioCmd.Flags().StringArrayVar(&sourceIDs, "source", nil, "source IDs to include (repeat for multiple)")
	audioCmd.Flags().StringVar(&formatStr, "format", "", "format: deep-dive, brief, critique, debate")
	audioCmd.Flags().StringVar(&lengthStr, "length", "", "length: short, default, long")

	videoCmd := &cobra.Command{
		Use:   "video",
		Short: "Generate a video overview",
		RunE: func(cmd *cobra.Command, args []string) error {
			c, err := client()
			if err != nil {
				return err
			}
			nb := notebookOrCtx(notebookID)
			if err := requireNotebook(nb); err != nil {
				return err
			}
			opts := notebooklm.GenerateVideoOptions{
				SourceIDs:    sourceIDs,
				Language:     language,
				Instructions: instructions,
			}
			switch formatStr {
			case "explainer":
				opts.Format = rpc.VideoExplainer
			case "brief":
				opts.Format = rpc.VideoBrief
			case "cinematic":
				opts.Format = rpc.VideoCinematic
			case "short":
				opts.Format = rpc.VideoShort
			}
			opts.StylePrompt = stylePrompt
			switch styleStr {
			case "classic":
				opts.Style = rpc.VideoStyleClassic
			case "whiteboard":
				opts.Style = rpc.VideoStyleWhiteboard
			case "kawaii":
				opts.Style = rpc.VideoStyleKawaii
			case "anime":
				opts.Style = rpc.VideoStyleAnime
			case "watercolor":
				opts.Style = rpc.VideoStyleWatercolor
			case "retro-print":
				opts.Style = rpc.VideoStyleRetroPrint
			case "heritage":
				opts.Style = rpc.VideoStyleHeritage
			case "paper-craft":
				opts.Style = rpc.VideoStylePaperCraft
			}
			status, err := c.Artifacts.GenerateVideo(context.Background(), nb, opts)
			if err != nil {
				return err
			}
			return waitAndDownload(c, nb, status, destPath)
		},
	}
	videoCmd.Flags().StringVarP(&notebookID, "notebook", "n", "", "notebook ID (required)")
	videoCmd.Flags().StringVarP(&language, "language", "l", "en", "output language (BCP 47)")
	videoCmd.Flags().StringVar(&instructions, "instructions", "", "custom instructions")
	videoCmd.Flags().BoolVarP(&waitFlag, "wait", "w", false, "wait for completion")
	videoCmd.Flags().StringVarP(&destPath, "output", "o", "", "download path when --wait is set")
	videoCmd.Flags().StringArrayVar(&sourceIDs, "source", nil, "source IDs to include")
	videoCmd.Flags().StringVar(&formatStr, "format", "", "format: explainer, brief, cinematic, short")
	videoCmd.Flags().StringVar(&stylePrompt, "style-prompt", "", "describe a custom visual style (overrides --style)")
	videoCmd.Flags().StringVar(&styleStr, "style", "", "style: classic, whiteboard, kawaii, anime, watercolor, retro-print, heritage, paper-craft")

	reportCmd := &cobra.Command{
		Use:   "report",
		Short: "Generate a report",
		RunE: func(cmd *cobra.Command, args []string) error {
			c, err := client()
			if err != nil {
				return err
			}
			nb := notebookOrCtx(notebookID)
			if err := requireNotebook(nb); err != nil {
				return err
			}
			opts := notebooklm.GenerateReportOptions{
				SourceIDs:         sourceIDs,
				Language:          language,
				CustomPrompt:      customPrompt,
				ExtraInstructions: extraInstr,
			}
			switch formatStr {
			case "briefing", "briefing-doc":
				opts.Format = rpc.ReportBriefingDoc
			case "study-guide":
				opts.Format = rpc.ReportStudyGuide
			case "blog-post":
				opts.Format = rpc.ReportBlogPost
			case "custom":
				opts.Format = rpc.ReportCustom
			default:
				opts.Format = rpc.ReportBriefingDoc
			}
			status, err := c.Artifacts.GenerateReport(context.Background(), nb, opts)
			if err != nil {
				return err
			}
			return waitAndDownload(c, nb, status, destPath)
		},
	}
	reportCmd.Flags().StringVarP(&notebookID, "notebook", "n", "", "notebook ID (required)")
	reportCmd.Flags().StringVarP(&language, "language", "l", "en", "output language (BCP 47)")
	reportCmd.Flags().BoolVarP(&waitFlag, "wait", "w", false, "wait for completion")
	reportCmd.Flags().StringVarP(&destPath, "output", "o", "", "download path when --wait is set")
	reportCmd.Flags().StringArrayVar(&sourceIDs, "source", nil, "source IDs to include")
	reportCmd.Flags().StringVar(&formatStr, "format", "", "format: briefing-doc, study-guide, blog-post, custom")
	reportCmd.Flags().StringVar(&customPrompt, "custom-prompt", "", "custom report prompt")
	reportCmd.Flags().StringVar(&extraInstr, "extra-instructions", "", "extra generation instructions")

	quizCmd := &cobra.Command{
		Use:   "quiz",
		Short: "Generate a quiz",
		RunE: func(cmd *cobra.Command, args []string) error {
			c, err := client()
			if err != nil {
				return err
			}
			nb := notebookOrCtx(notebookID)
			if err := requireNotebook(nb); err != nil {
				return err
			}
			opts := notebooklm.GenerateQuizOptions{
				SourceIDs: sourceIDs,
				Language:  language,
			}
			switch quantity {
			case "fewer":
				opts.Quantity = rpc.QuizFewer
			case "more":
				opts.Quantity = rpc.QuizMore
			default:
				opts.Quantity = rpc.QuizStandard
			}
			switch difficulty {
			case "easy":
				opts.Difficulty = rpc.QuizEasy
			case "hard":
				opts.Difficulty = rpc.QuizHard
			default:
				opts.Difficulty = rpc.QuizMedium
			}
			status, err := c.Artifacts.GenerateQuiz(context.Background(), nb, opts)
			if err != nil {
				return err
			}
			return waitAndDownload(c, nb, status, destPath)
		},
	}
	quizCmd.Flags().StringVarP(&notebookID, "notebook", "n", "", "notebook ID (required)")
	quizCmd.Flags().StringVarP(&language, "language", "l", "en", "output language (BCP 47)")
	quizCmd.Flags().BoolVarP(&waitFlag, "wait", "w", false, "wait for completion")
	quizCmd.Flags().StringVarP(&destPath, "output", "o", "", "download path when --wait is set")
	quizCmd.Flags().StringArrayVar(&sourceIDs, "source", nil, "source IDs to include")
	quizCmd.Flags().StringVar(&quantity, "quantity", "standard", "quantity: fewer, standard, more")
	quizCmd.Flags().StringVar(&difficulty, "difficulty", "medium", "difficulty: easy, medium, hard")

	flashcardsCmd := &cobra.Command{
		Use:   "flashcards",
		Short: "Generate flashcards",
		RunE: func(cmd *cobra.Command, args []string) error {
			c, err := client()
			if err != nil {
				return err
			}
			nb := notebookOrCtx(notebookID)
			if err := requireNotebook(nb); err != nil {
				return err
			}
			opts := notebooklm.GenerateQuizOptions{
				SourceIDs:  sourceIDs,
				Language:   language,
				Flashcards: true,
			}
			switch quantity {
			case "fewer":
				opts.Quantity = rpc.QuizFewer
			case "more":
				opts.Quantity = rpc.QuizMore
			default:
				opts.Quantity = rpc.QuizStandard
			}
			switch difficulty {
			case "easy":
				opts.Difficulty = rpc.QuizEasy
			case "hard":
				opts.Difficulty = rpc.QuizHard
			default:
				opts.Difficulty = rpc.QuizMedium
			}
			status, err := c.Artifacts.GenerateQuiz(context.Background(), nb, opts)
			if err != nil {
				return err
			}
			return waitAndDownload(c, nb, status, destPath)
		},
	}
	flashcardsCmd.Flags().StringVarP(&notebookID, "notebook", "n", "", "notebook ID (required)")
	flashcardsCmd.Flags().StringVarP(&language, "language", "l", "en", "output language (BCP 47)")
	flashcardsCmd.Flags().BoolVarP(&waitFlag, "wait", "w", false, "wait for completion")
	flashcardsCmd.Flags().StringVarP(&destPath, "output", "o", "", "download path when --wait is set")
	flashcardsCmd.Flags().StringArrayVar(&sourceIDs, "source", nil, "source IDs to include")
	flashcardsCmd.Flags().StringVar(&quantity, "quantity", "standard", "quantity: fewer, standard, more")
	flashcardsCmd.Flags().StringVar(&difficulty, "difficulty", "medium", "difficulty: easy, medium, hard")

	slideDeckCmd := &cobra.Command{
		Use:   "slide-deck",
		Short: "Generate a slide deck",
		RunE: func(cmd *cobra.Command, args []string) error {
			c, err := client()
			if err != nil {
				return err
			}
			nb := notebookOrCtx(notebookID)
			if err := requireNotebook(nb); err != nil {
				return err
			}
			opts := notebooklm.GenerateSlideDeckOptions{
				SourceIDs: sourceIDs,
				Language:  language,
			}
			switch formatStr {
			case "presenter":
				opts.Format = rpc.SlideDeckPresenter
			default:
				opts.Format = rpc.SlideDeckDetailed
			}
			switch lengthStr {
			case "short":
				opts.Length = rpc.SlideDeckShort
			default:
				opts.Length = rpc.SlideDeckDefault
			}
			status, err := c.Artifacts.GenerateSlideDeck(context.Background(), nb, opts)
			if err != nil {
				return err
			}
			return waitAndDownload(c, nb, status, destPath)
		},
	}
	slideDeckCmd.Flags().StringVarP(&notebookID, "notebook", "n", "", "notebook ID (required)")
	slideDeckCmd.Flags().StringVarP(&language, "language", "l", "en", "output language (BCP 47)")
	slideDeckCmd.Flags().BoolVarP(&waitFlag, "wait", "w", false, "wait for completion")
	slideDeckCmd.Flags().StringVarP(&destPath, "output", "o", "", "download path when --wait is set")
	slideDeckCmd.Flags().StringArrayVar(&sourceIDs, "source", nil, "source IDs to include")
	slideDeckCmd.Flags().StringVar(&formatStr, "format", "", "format: detailed, presenter")
	slideDeckCmd.Flags().StringVar(&lengthStr, "length", "", "length: default, short")

	infographicCmd := &cobra.Command{
		Use:   "infographic",
		Short: "Generate an infographic",
		RunE: func(cmd *cobra.Command, args []string) error {
			c, err := client()
			if err != nil {
				return err
			}
			nb := notebookOrCtx(notebookID)
			if err := requireNotebook(nb); err != nil {
				return err
			}
			opts := notebooklm.GenerateInfographicOptions{
				SourceIDs:    sourceIDs,
				Language:     language,
				Instructions: instructions,
			}
			switch orientStr {
			case "portrait":
				opts.Orientation = rpc.InfographicPortrait
			case "square":
				opts.Orientation = rpc.InfographicSquare
			default:
				opts.Orientation = rpc.InfographicLandscape
			}
			switch detailStr {
			case "concise":
				opts.DetailLevel = rpc.InfographicConcise
			case "detailed":
				opts.DetailLevel = rpc.InfographicDetailed
			default:
				opts.DetailLevel = rpc.InfographicStandard
			}
			switch styleStr {
			case "sketch-note":
				opts.Style = rpc.InfographicStyleSketchNote
			case "professional":
				opts.Style = rpc.InfographicStyleProfessional
			case "bento-grid":
				opts.Style = rpc.InfographicStyleBentoGrid
			case "editorial":
				opts.Style = rpc.InfographicStyleEditorial
			case "instructional":
				opts.Style = rpc.InfographicStyleInstructional
			case "bricks":
				opts.Style = rpc.InfographicStyleBricks
			case "clay":
				opts.Style = rpc.InfographicStyleClay
			case "anime":
				opts.Style = rpc.InfographicStyleAnime
			case "kawaii":
				opts.Style = rpc.InfographicStyleKawaii
			case "scientific":
				opts.Style = rpc.InfographicStyleScientific
			}
			status, err := c.Artifacts.GenerateInfographic(context.Background(), nb, opts)
			if err != nil {
				return err
			}
			return waitAndDownload(c, nb, status, destPath)
		},
	}
	infographicCmd.Flags().StringVarP(&notebookID, "notebook", "n", "", "notebook ID (required)")
	infographicCmd.Flags().StringVarP(&language, "language", "l", "en", "output language (BCP 47)")
	infographicCmd.Flags().StringVar(&instructions, "instructions", "", "custom instructions")
	infographicCmd.Flags().BoolVarP(&waitFlag, "wait", "w", false, "wait for completion")
	infographicCmd.Flags().StringVarP(&destPath, "output", "o", "", "download path when --wait is set")
	infographicCmd.Flags().StringArrayVar(&sourceIDs, "source", nil, "source IDs to include")
	infographicCmd.Flags().StringVar(&orientStr, "orientation", "", "orientation: landscape, portrait, square")
	infographicCmd.Flags().StringVar(&detailStr, "detail", "", "detail level: concise, standard, detailed")
	infographicCmd.Flags().StringVar(&styleStr, "style", "", "style: sketch-note, professional, bento-grid, editorial, instructional, bricks, clay, anime, kawaii, scientific")

	dataTableCmd := &cobra.Command{
		Use:   "data-table",
		Short: "Generate a data table",
		RunE: func(cmd *cobra.Command, args []string) error {
			c, err := client()
			if err != nil {
				return err
			}
			nb := notebookOrCtx(notebookID)
			if err := requireNotebook(nb); err != nil {
				return err
			}
			opts := notebooklm.GenerateDataTableOptions{
				SourceIDs:    sourceIDs,
				Language:     language,
				Instructions: instructions,
			}
			status, err := c.Artifacts.GenerateDataTable(context.Background(), nb, opts)
			if err != nil {
				return err
			}
			return waitAndDownload(c, nb, status, destPath)
		},
	}
	dataTableCmd.Flags().StringVarP(&notebookID, "notebook", "n", "", "notebook ID (required)")
	dataTableCmd.Flags().StringVarP(&language, "language", "l", "en", "output language (BCP 47)")
	dataTableCmd.Flags().StringVar(&instructions, "instructions", "", "custom instructions")
	dataTableCmd.Flags().BoolVarP(&waitFlag, "wait", "w", false, "wait for completion")
	dataTableCmd.Flags().StringVarP(&destPath, "output", "o", "", "download path when --wait is set")
	dataTableCmd.Flags().StringArrayVar(&sourceIDs, "source", nil, "source IDs to include")

	downloadCmd := &cobra.Command{
		Use:   "download <artifact-id> <dest-path>",
		Short: "Download a completed artifact",
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
			if err := c.Artifacts.Download(context.Background(), nb, args[0], args[1]); err != nil {
				return err
			}
			fmt.Printf("Downloaded to: %s\n", args[1])
			return nil
		},
	}
	downloadCmd.Flags().StringVarP(&notebookID, "notebook", "n", "", "notebook ID (required)")

	deleteCmd := &cobra.Command{
		Use:   "delete <artifact-id>",
		Short: "Delete an artifact",
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
			if err := c.Artifacts.Delete(context.Background(), nb, args[0]); err != nil {
				return err
			}
			fmt.Printf("Deleted artifact: %s\n", args[0])
			return nil
		},
	}
	deleteCmd.Flags().StringVarP(&notebookID, "notebook", "n", "", "notebook ID (required)")

	suggestedReportsCmd := &cobra.Command{
		Use:   "suggested-reports",
		Short: "Get AI-suggested report formats for a notebook",
		RunE: func(cmd *cobra.Command, args []string) error {
			c, err := client()
			if err != nil {
				return err
			}
			nb := notebookOrCtx(notebookID)
			if err := requireNotebook(nb); err != nil {
				return err
			}
			suggestions, err := c.Artifacts.GetSuggestedReports(context.Background(), nb)
			if err != nil {
				return err
			}
			if outputJSON {
				printJSON(suggestions)
				return nil
			}
			if len(suggestions) == 0 {
				fmt.Println("No suggestions found.")
				return nil
			}
			for _, s := range suggestions {
				fmt.Printf("Title: %s\nDescription: %s\n", s.Title, s.Description)
			}
			return nil
		},
	}
	suggestedReportsCmd.Flags().StringVarP(&notebookID, "notebook", "n", "", "notebook ID (required)")

	var exportTypeStr, exportTitle string
	exportCmd := &cobra.Command{
		Use:   "export <artifact-id>",
		Short: "Export an artifact to Docs or Sheets",
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
			var exportType rpc.ExportType
			if exportTypeStr == "sheets" {
				exportType = rpc.ExportSheets
			} else {
				exportType = rpc.ExportDocs
			}
			url, err := c.Artifacts.ExportArtifact(context.Background(), nb, args[0], exportTitle, exportType)
			if err != nil {
				return err
			}
			if outputJSON {
				printJSON(map[string]string{"url": url})
				return nil
			}
			if url == "" {
				fmt.Println("Export requested; open Google Drive to find the document.")
				return nil
			}
			fmt.Printf("Exported to: %s\n", url)
			return nil
		},
	}
	exportCmd.Flags().StringVarP(&notebookID, "notebook", "n", "", "notebook ID (required)")
	exportCmd.Flags().StringVar(&exportTypeStr, "type", "docs", "export type: docs, sheets")
	exportCmd.Flags().StringVar(&exportTitle, "title", "Export", "title of the exported document")

	shareLinkCmd := &cobra.Command{
		Use:   "share-link <artifact-id>",
		Short: "Get a shareable link for an artifact",
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
			url, err := c.Artifacts.ShareArtifact(context.Background(), nb, args[0])
			if err != nil {
				return err
			}
			if outputJSON {
				printJSON(map[string]string{"url": url})
				return nil
			}
			fmt.Printf("Share URL: %s\n", url)
			return nil
		},
	}
	shareLinkCmd.Flags().StringVarP(&notebookID, "notebook", "n", "", "notebook ID (required)")

	interactiveHTMLCmd := &cobra.Command{
		Use:   "interactive-html <artifact-id>",
		Short: "Get interactive HTML for an artifact",
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
			html, err := c.Artifacts.GetInteractiveHTML(context.Background(), nb, args[0])
			if err != nil {
				return err
			}
			if outputJSON {
				printJSON(map[string]string{"html": html})
				return nil
			}
			if destPath != "" {
				if err := os.WriteFile(destPath, []byte(html), 0644); err != nil {
					return err
				}
				fmt.Printf("Saved to: %s\n", destPath)
				return nil
			}
			fmt.Print(html)
			return nil
		},
	}
	interactiveHTMLCmd.Flags().StringVarP(&notebookID, "notebook", "n", "", "notebook ID (required)")
	interactiveHTMLCmd.Flags().StringVarP(&destPath, "output", "o", "", "file path to save HTML")

	reviseSlideCmd := &cobra.Command{
		Use:   "revise-slide <artifact-id> <slide-index> <instructions>",
		Short: "Revise a specific slide in a slide deck artifact",
		Args:  cobra.ExactArgs(3),
		RunE: func(cmd *cobra.Command, args []string) error {
			c, err := client()
			if err != nil {
				return err
			}
			nb := notebookOrCtx(notebookID)
			if err := requireNotebook(nb); err != nil {
				return err
			}
			slideIndex, err := strconv.Atoi(args[1])
			if err != nil {
				return fmt.Errorf("slide-index must be an integer: %w", err)
			}
			status, err := c.Artifacts.ReviseSlide(context.Background(), nb, args[0], slideIndex, args[2])
			if err != nil {
				return err
			}
			return waitAndDownload(c, nb, status, destPath)
		},
	}
	reviseSlideCmd.Flags().StringVarP(&notebookID, "notebook", "n", "", "notebook ID (required)")
	reviseSlideCmd.Flags().BoolVarP(&waitFlag, "wait", "w", false, "wait for completion")
	artifactCmd.AddCommand(
		listCmd, getCmd, renameCmd,
		audioCmd, videoCmd, reportCmd, quizCmd, flashcardsCmd,
		slideDeckCmd, infographicCmd, dataTableCmd,
		downloadCmd, deleteCmd,
		suggestedReportsCmd, exportCmd, shareLinkCmd, interactiveHTMLCmd, reviseSlideCmd,
	)
}
