# WIP — Remaining Work

Known gaps between this Go client and the Python reference
(`teng-lin/notebooklm-py`), and work still to do.

In October 2026 every RPC was re-audited against notebooklm-py at commit
`2aff0fc` (2026-10-09). Google's "Gemini-3.5" backend rollout had changed most
request shapes. Every request this client sends now matches the reference:
see commits `2d91c5b` (rpc), `17b86c5` (sources), `bfe53af` (chat, notes,
notebooks) and `c0bcf91` (artifacts, research), plus the sharing and settings
update that followed. Unit tests pin each payload to the reference shape.

---

## Critical

### 1. Live verification
The audit was done offline against the reference source. None of the migrated
calls have been run against a live account yet. Use a throwaway notebook with
`NOTEBOOKLM_DEBUG=1` and check, at minimum:

- `source add-url`, `add-text`, `add-file`, `list`, `get-fulltext`, `rename`, `delete`
- `chat ask` (with citations via `--json`), `chat history`
- `notes create`, `notes update`, `notes mind-map`
- `artifact report --wait -o r.md`, `artifact audio --wait -o a.mp4`
- `research fast` and `research deep --wait --import`
- `share status`, `settings get`

A rejected request now reports its gRPC status, for example
`server rejected the request (invalid argument)`. That status points at the
payload that needs another look.

---

## Medium priority

### 2. Structured quiz / flashcard export
`artifact download` saves quizzes, flashcards and interactive mind maps as
their HTML page. The reference also extracts the embedded app data and
renders it as JSON or Markdown (`_web/artifact/downloads.py`,
`_extract_app_data` / `_format_content`). Port that, and add
`--format json|markdown|html`.

### 3. Mind map tree type
Mind maps come back as notes whose `Content` is the JSON tree
(`{"name": ..., "children": [...]}`). Add a `MindMapNode` type and a parser,
so callers can walk the tree without decoding the JSON themselves.

### 4. Chat follow-up history
On follow-ups, the reference sends earlier turns from a local cache in the
`history` slot of the chat request. This client sends `null` and relies on the
server-side conversation. If follow-up answers lose context, fetch the turns
with `GetConversationTurns` and send them as
`[[answer, null, 2], [question, null, 1], ...]`.

---

## Low priority

### 5. Multi-account (`authuser`)
The reference sends `authuser` (URL param and `x-goog-authuser` header) when
the browser session has several Google accounts. This client always uses
account 0. NOT_FOUND / PERMISSION_DENIED errors already hint at this.

### 6. Host
The reference now defaults to `https://notebook.google.com`. The
`notebooklm.google.com` host this client uses is still served, but it may
need to become configurable.

### 7. Drive adds
Drive source adds still use the older flat request tail. The reference has not
migrated it yet either (its `TODO(#1546)`). Re-check when the reference does.

### 8. Not yet ported
These reference features have no Go equivalent: copy notebooks/sources/artifacts,
source labels and collections, Play Books sources, chat session status / cancel,
delete conversation, prompt and next-step suggestions, retry failed artifact,
customization choices, quota/usage.
