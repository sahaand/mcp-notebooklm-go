# notebooklm-go

An unofficial Go client for [Google NotebookLM](https://notebooklm.google.com/).

> **Disclaimer:** This uses undocumented Google APIs that can change without notice. Use for prototypes, research, and personal projects — not production systems.

## Features

* **Notebook management** — list, create, rename, delete, describe

* **Source management** — add URLs, YouTube videos, PDFs, text, Google Drive files

* **Artifact generation** — audio overviews, video overviews, reports, quizzes, flashcards, infographics, slide decks, data tables, mind maps

* **Chat** — ask questions with citations, configure personas and response styles

* **Sharing** — public/private links, user-level permissions

* **Research** — fast and deep web research with source import

* **Settings** — output language configuration

***

## Installation

### Prerequisites

* Go 1.22+

* A Google account with access to [notebooklm.google.com](https://notebooklm.google.com/)

### Install the CLI

```bash
go install github.com/saeedata/notebooklm-go/cmd/notebooklm@latest
```

Or clone and build manually:

```bash
git clone https://github.com/saeedata/notebooklm-go
cd notebooklm-go
go build -o notebooklm ./cmd/notebooklm/
```

### Use as a Go library

```bash
go get github.com/saeedata/notebooklm-go
```

***

## Authentication

Authentication requires exporting your Google session cookies from a browser. There are two methods:

### Method 1: Playwright storage state (recommended)

1. Install Node.js and Playwright:

```bash
npm install -g playwright
playwright install chromium
```

1. Export your session after logging in to NotebookLM:

```bash
npx playwright codegen --save-storage ~/.notebooklm/storage_state.json https://notebooklm.google.com/
```

Log in when the browser opens, then close it. The cookies are saved automatically.

1. Verify:

```bash
notebooklm notebook list
```

### Method 2: Environment variable (CI/CD friendly)

Set `NOTEBOOKLM_AUTH_JSON` to the contents of your storage state JSON — no file needed:

```bash
export NOTEBOOKLM_AUTH_JSON='{"cookies":[{"name":"SID","value":"...","domain":".google.com",...}]}'
notebooklm notebook list
```

### Custom storage path

```bash
notebooklm --storage /path/to/storage_state.json notebook list
```

***

## CLI Usage

### Notebooks

```bash
# List all notebooks
notebooklm notebook list

# Create a notebook
notebooklm notebook create "My Research"

# Get notebook details
notebooklm notebook get <notebook-id>

# Rename
notebooklm notebook rename <notebook-id> "Better Title"

# Delete
notebooklm notebook delete <notebook-id>

# AI-generated summary + suggested topics
notebooklm notebook describe <notebook-id>
```

### Sources

```bash
# List sources in a notebook
notebooklm source list -n <notebook-id>

# Add a web URL
notebooklm source add-url -n <notebook-id> https://example.com

# Add a YouTube video
notebooklm source add-url -n <notebook-id> https://youtu.be/dQw4w9WgXcQ

# Add pasted text
notebooklm source add-text -n <notebook-id> "My Title" "Content goes here..."

# Upload a file (PDF, TXT, MD, DOCX)
notebooklm source add-file -n <notebook-id> ./paper.pdf

# Delete a source
notebooklm source delete -n <notebook-id> <source-id>
```

### Artifacts (Audio, Video, Reports, etc.)

```bash
# List artifacts
notebooklm artifact list -n <notebook-id>

# Generate audio overview (Deep Dive podcast format)
notebooklm artifact audio -n <notebook-id>

# Generate and wait for completion, then download
notebooklm artifact audio -n <notebook-id> --wait --output podcast.mp4

# With custom instructions and language
notebooklm artifact audio -n <notebook-id> \
  --instructions "Focus on practical applications" \
  --language es \
  --wait --output podcast_es.mp4

# Download a completed artifact
notebooklm artifact download -n <notebook-id> <artifact-id> ./output.mp4

# Delete an artifact
notebooklm artifact delete -n <notebook-id> <artifact-id>
```

### Chat

```bash
# Ask a question
notebooklm chat ask -n <notebook-id> "What are the key themes?"

# Ask with specific sources only
notebooklm chat ask -n <notebook-id> \
  --source <source-id-1> --source <source-id-2> \
  "Summarize these two papers"

# Output as JSON (includes citations)
notebooklm chat ask -n <notebook-id> --json "What is the main argument?"
```

### Settings

```bash
# Get current settings
notebooklm settings get

# Set output language (BCP 47 code)
notebooklm settings set-language es
notebooklm settings set-language ja
```

### Global flags

```bash
--json          # Output all results as formatted JSON
--storage PATH  # Custom path to storage_state.json
```

***

## Go Library Usage

### Quick start

```go
package main

import (
    "context"
    "fmt"
    "log"

    notebooklm "github.com/saeedata/notebooklm-go"
)

func main() {
    ctx := context.Background()

    // Create client from storage state
    client, err := notebooklm.NewClientFromStorage(ctx, "")
    if err != nil {
        log.Fatal(err)
    }

    // Create a notebook
    nb, err := client.Notebooks.Create(ctx, "My Research")
    if err != nil {
        log.Fatal(err)
    }
    fmt.Printf("Created: %s (%s)\n", nb.Title, nb.ID)

    // Add a source
    src, err := client.Sources.AddURL(ctx, nb.ID, "https://example.com/paper.pdf")
    if err != nil {
        log.Fatal(err)
    }

    // Wait for source to finish processing
    _, err = client.Sources.WaitUntilReady(ctx, nb.ID, src.ID, 2*time.Minute)
    if err != nil {
        log.Fatal(err)
    }

    // Ask a question
    result, err := client.Chat.Ask(ctx, nb.ID, "What is this about?", notebooklm.AskOptions{})
    if err != nil {
        log.Fatal(err)
    }
    fmt.Println(result.Answer)

    // Generate audio overview and download
    status, err := client.Artifacts.GenerateAudio(ctx, nb.ID, notebooklm.GenerateAudioOptions{
        Language: "en",
    })
    if err != nil {
        log.Fatal(err)
    }
    status, err = client.Artifacts.WaitForCompletion(ctx, nb.ID, status.TaskID, 10*time.Minute)
    if err != nil {
        log.Fatal(err)
    }
    client.Artifacts.Download(ctx, nb.ID, status.TaskID, "podcast.mp4")
}
```

***

## Integration with Nym

[Nym](https://github.com/saeedata/nym) is a Go-based AI agent platform. Add `notebooklm-go` as a built-in tool so any agent can interact with NotebookLM.

### 1. Add the dependency

In `nym/go.mod`:

```bash
cd /path/to/nym
go get github.com/saeedata/notebooklm-go
```

### 2. Create a Nym tool wrapper

Create `internal/tools/notebooklm.go` in the Nym repo:

```go
package tools

import (
    "context"
    "encoding/json"
    "fmt"

    notebooklm "github.com/saeedata/notebooklm-go"
)

func init() {
    Register(Tool{
        Name:        "notebooklm_ask",
        Description: "Ask a question to a NotebookLM notebook and get an answer with citations from its sources.",
        Parameters: map[string]any{
            "type": "object",
            "properties": map[string]any{
                "notebook_id": map[string]any{"type": "string", "description": "NotebookLM notebook ID"},
                "question":    map[string]any{"type": "string", "description": "Question to ask"},
            },
            "required": []string{"notebook_id", "question"},
        },
        Execute: func(ctx context.Context, args map[string]any) (string, error) {
            client, err := notebooklm.NewClientFromStorage(ctx, "")
            if err != nil {
                return "", err
            }
            notebookID, _ := args["notebook_id"].(string)
            question, _ := args["question"].(string)
            result, err := client.Chat.Ask(ctx, notebookID, question, notebooklm.AskOptions{})
            if err != nil {
                return "", err
            }
            return result.Answer, nil
        },
    })

    Register(Tool{
        Name:        "notebooklm_create_notebook",
        Description: "Create a new NotebookLM notebook.",
        Parameters: map[string]any{
            "type": "object",
            "properties": map[string]any{
                "title": map[string]any{"type": "string", "description": "Notebook title"},
            },
            "required": []string{"title"},
        },
        Execute: func(ctx context.Context, args map[string]any) (string, error) {
            client, err := notebooklm.NewClientFromStorage(ctx, "")
            if err != nil {
                return "", err
            }
            title, _ := args["title"].(string)
            nb, err := client.Notebooks.Create(ctx, title)
            if err != nil {
                return "", err
            }
            data, _ := json.Marshal(nb)
            return string(data), nil
        },
    })

    Register(Tool{
        Name:        "notebooklm_add_source",
        Description: "Add a URL or YouTube video as a source to a NotebookLM notebook.",
        Parameters: map[string]any{
            "type": "object",
            "properties": map[string]any{
                "notebook_id": map[string]any{"type": "string"},
                "url":         map[string]any{"type": "string", "description": "URL or YouTube link to add"},
            },
            "required": []string{"notebook_id", "url"},
        },
        Execute: func(ctx context.Context, args map[string]any) (string, error) {
            client, err := notebooklm.NewClientFromStorage(ctx, "")
            if err != nil {
                return "", err
            }
            notebookID, _ := args["notebook_id"].(string)
            url, _ := args["url"].(string)
            src, err := client.Sources.AddURL(ctx, notebookID, url)
            if err != nil {
                return "", err
            }
            return fmt.Sprintf("Added source %s", src.ID), nil
        },
    })

    Register(Tool{
        Name:        "notebooklm_generate_audio",
        Description: "Generate an audio overview (podcast) from a NotebookLM notebook.",
        Parameters: map[string]any{
            "type": "object",
            "properties": map[string]any{
                "notebook_id": map[string]any{"type": "string"},
                "language":    map[string]any{"type": "string", "description": "BCP 47 language code, e.g. 'en', 'es'"},
            },
            "required": []string{"notebook_id"},
        },
        Execute: func(ctx context.Context, args map[string]any) (string, error) {
            client, err := notebooklm.NewClientFromStorage(ctx, "")
            if err != nil {
                return "", err
            }
            notebookID, _ := args["notebook_id"].(string)
            lang, _ := args["language"].(string)
            if lang == "" {
                lang = "en"
            }
            status, err := client.Artifacts.GenerateAudio(ctx, notebookID, notebooklm.GenerateAudioOptions{Language: lang})
            if err != nil {
                return "", err
            }
            return fmt.Sprintf("Audio generation started. Task ID: %s", status.TaskID), nil
        },
    })
}
```

### 3. Set authentication

```bash
export NOTEBOOKLM_AUTH_JSON="$(cat ~/.notebooklm/storage_state.json)"
```

Or configure it in Nym's `.env` / config file.

***

## Integration with Claude Code

Add `notebooklm-go` as a Claude Code skill so agents can use it during coding sessions.

### 1. Build and install the CLI

```bash
cd notebooklm-go
go build -o notebooklm ./cmd/notebooklm/
sudo mv notebooklm /usr/local/bin/
```

### 2. Create a Claude Code skill

Create `~/.claude/skills/notebooklm.md`:

```markdown
# NotebookLM Skill

You have access to Google NotebookLM via the `notebooklm` CLI.

## Available commands

### Notebooks
- `notebooklm notebook list` — list all notebooks
- `notebooklm notebook create "<title>"` — create a notebook
- `notebooklm notebook describe <id>` — AI summary + suggested topics

### Sources
- `notebooklm source add-url -n <id> <url>` — add a URL/YouTube source
- `notebooklm source add-text -n <id> "<title>" "<content>"` — add text
- `notebooklm source add-file -n <id> <path>` — upload PDF/MD/DOCX

### Chat
- `notebooklm chat ask -n <id> "<question>"` — ask the notebook

### Artifacts
- `notebooklm artifact audio -n <id> --wait --output out.mp4` — generate audio podcast

## Authentication
Requires `NOTEBOOKLM_AUTH_JSON` env var or `~/.notebooklm/storage_state.json`.

## Example workflow
1. Create a notebook for the current task
2. Add relevant documentation URLs as sources
3. Ask questions to get answers grounded in the sources
4. Generate an audio summary for async review
```

### 3. Use in a Claude Code session

```text
/notebooklm
```

Or just start using it — Claude Code will invoke the `notebooklm` CLI directly via bash.

***

## Integration with MCP (Model Context Protocol)

Expose NotebookLM as MCP tools that any MCP-compatible client (Claude Desktop, Cursor, etc.) can use.

### MCP server implementation

```go
// cmd/mcp-server/main.go
package main

import (
    "context"
    notebooklm "github.com/saeedata/notebooklm-go"
    // Use your preferred MCP server library
)

// Register tools: notebooklm_ask, notebooklm_create_notebook,
// notebooklm_add_source, notebooklm_list_notebooks, notebooklm_generate_audio
```

### Claude Desktop config (`~/Library/Application Support/Claude/claude_desktop_config.json`)

```json
{
  "mcpServers": {
    "notebooklm": {
      "command": "/usr/local/bin/notebooklm-mcp",
      "env": {
        "NOTEBOOKLM_AUTH_JSON": "<your-storage-state-json>"
      }
    }
  }
}
```

***

## Environment Variables

| Variable               | Description                                                    |
| ---------------------- | -------------------------------------------------------------- |
| `NOTEBOOKLM_AUTH_JSON` | Playwright storage state JSON (inline, no file needed)         |
| `NOTEBOOKLM_HOME`      | Override default storage directory (default: `~/.notebooklm/`) |

***

## API Reference

### Client creation

```go
// From storage file (default: ~/.notebooklm/storage_state.json)
client, err := notebooklm.NewClientFromStorage(ctx, "")

// From explicit path
client, err := notebooklm.NewClientFromStorage(ctx, "/path/to/storage_state.json")

// From pre-loaded auth tokens
auth, err := notebooklm.NewAuthTokensFromStorage(ctx, "")
client := notebooklm.NewClient(auth)
```

### Sub-clients

| Client             | Methods                                                                                                                                                                                                 |
| ------------------ | ------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------- |
| `client.Notebooks` | `List`, `Create`, `Get`, `Rename`, `Delete`, `GetDescription`, `GetMetadata`, `RemoveFromRecent`                                                                                                        |
| `client.Sources`   | `List`, `Get`, `AddURL`, `AddText`, `AddFile`, `AddDrive`, `Delete`, `Rename`, `Refresh`, `WaitUntilReady`, `GetGuide`, `GetFulltext`, `CheckFreshness`, `DiscoverSources`                              |
| `client.Artifacts` | `List`, `Get`, `Delete`, `Rename`, `GenerateAudio`, `GenerateVideo`, `GenerateReport`, `GenerateQuiz`, `GenerateInfographic`, `GenerateSlideDeck`, `GenerateDataTable`, `WaitForCompletion`, `Download` |
| `client.Chat`      | `Ask`, `Configure`, `SetMode`, `GetConversationTurns`, `GetLastConversationID`                                                                                                                          |
| `client.Sharing`   | `GetStatus`, `SetPublic`, `SetViewLevel`, `AddUser`, `UpdateUser`, `RemoveUser`                                                                                                                         |
| `client.Notes`     | `List`, `Create`, `Update`, `Delete`, `ListMindMaps`, `GenerateMindMap`                                                                                                                                 |
| `client.Research`  | `StartFast`, `StartDeep`, `Poll`, `Import`, `WaitForCompletion`                                                                                                                                         |
| `client.Settings`  | `Get`, `SetOutputLanguage`                                                                                                                                                                              |

***

## License

MIT — see [LICENSE](LICENSE).

Not affiliated with or endorsed by Google.

⠀