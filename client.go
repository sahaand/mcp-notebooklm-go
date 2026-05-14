package notebooklm

import (
	"context"
	"fmt"
	"io"
	"net/http"
	"os"
	"strings"
	"sync"
	"time"

	"github.com/saeedata/notebooklm-go/rpc"
)

const defaultTimeout = 30 * time.Second

// Client is the main NotebookLM API client.
// Use NewClient or NewClientFromStorage to create one.
type Client struct {
	auth       *AuthTokens
	httpClient *http.Client
	mu         sync.Mutex

	// Sub-clients
	Notebooks *NotebooksAPI
	Sources   *SourcesAPI
	Artifacts *ArtifactsAPI
	Chat      *ChatAPI
	Sharing   *SharingAPI
	Notes     *NotesAPI
	Research  *ResearchAPI
	Settings  *SettingsAPI
}

// NewClient creates a Client with the provided AuthTokens.
func NewClient(auth *AuthTokens) *Client {
	c := &Client{
		auth: auth,
		httpClient: &http.Client{
			Timeout: defaultTimeout,
		},
	}
	c.Notebooks = &NotebooksAPI{client: c}
	c.Sources = &SourcesAPI{client: c}
	c.Notes = &NotesAPI{client: c}
	c.Artifacts = &ArtifactsAPI{client: c}
	c.Chat = &ChatAPI{client: c}
	c.Sharing = &SharingAPI{client: c}
	c.Research = &ResearchAPI{client: c}
	c.Settings = &SettingsAPI{client: c}
	return c
}

// NewClientFromStorage creates a Client by loading auth from Playwright storage state.
func NewClientFromStorage(ctx context.Context, storagePath string) (*Client, error) {
	auth, err := NewAuthTokensFromStorage(ctx, storagePath)
	if err != nil {
		return nil, fmt.Errorf("load auth: %w", err)
	}
	return NewClient(auth), nil
}

// RefreshAuth re-fetches CSRF and session tokens from the NotebookLM homepage.
func (c *Client) RefreshAuth(ctx context.Context) error {
	c.mu.Lock()
	defer c.mu.Unlock()

	csrf, sid, err := FetchTokens(ctx, c.auth.Cookies)
	if err != nil {
		return err
	}
	c.auth.CSRFToken = csrf
	c.auth.SessionID = sid
	return nil
}

// RPCCall executes a batchexecute RPC call and returns the decoded result.
func (c *Client) RPCCall(ctx context.Context, method rpc.RPCMethod, params any, sourcePath string, allowNull bool) (any, error) {
	return c.doRPC(ctx, method, params, sourcePath, allowNull, true)
}

func (c *Client) doRPC(ctx context.Context, method rpc.RPCMethod, params any, sourcePath string, allowNull, retry bool) (any, error) {
	encoded, err := rpc.EncodeRPCRequest(method, params)
	if err != nil {
		return nil, fmt.Errorf("encode RPC request: %w", err)
	}

	c.mu.Lock()
	csrf := c.auth.CSRFToken
	sid := c.auth.SessionID
	cookieHeader := c.auth.CookieHeader()
	c.mu.Unlock()

	body, err := rpc.BuildRequestBody(encoded, csrf)
	if err != nil {
		return nil, fmt.Errorf("build request body: %w", err)
	}

	urlParams := rpc.BuildURLParams(method, sourcePath, sid)
	reqURL := rpc.BatchExecuteURL + "?" + urlParams.Encode()

	req, err := http.NewRequestWithContext(ctx, "POST", reqURL, strings.NewReader(body))
	if err != nil {
		return nil, fmt.Errorf("create HTTP request: %w", err)
	}
	req.Header.Set("Content-Type", "application/x-www-form-urlencoded;charset=UTF-8")
	req.Header.Set("Cookie", cookieHeader)
	req.Header.Set("User-Agent", "Mozilla/5.0 (Macintosh; Intel Mac OS X 10_15_7) AppleWebKit/537.36")
	req.Header.Set("X-Same-Domain", "1")

	resp, err := c.httpClient.Do(req)
	if err != nil {
		return nil, fmt.Errorf("HTTP request failed: %w", err)
	}
	defer resp.Body.Close()

	// Handle auth expiry
	if resp.StatusCode == http.StatusUnauthorized || resp.StatusCode == http.StatusForbidden {
		if retry {
			if refreshErr := c.RefreshAuth(ctx); refreshErr == nil {
				return c.doRPC(ctx, method, params, sourcePath, allowNull, false)
			}
		}
		return nil, fmt.Errorf("authentication failed (HTTP %d) — run 'notebooklm login'", resp.StatusCode)
	}

	if resp.StatusCode == http.StatusTooManyRequests {
		return nil, &rpc.RPCError{Message: "rate limit exceeded — please wait before retrying", MethodID: string(method)}
	}

	if resp.StatusCode >= 500 {
		return nil, fmt.Errorf("server error (HTTP %d) — try again later", resp.StatusCode)
	}

	rawBody, err := io.ReadAll(resp.Body)
	if err != nil {
		return nil, fmt.Errorf("read response body: %w", err)
	}

	if os.Getenv("NOTEBOOKLM_DEBUG") != "" {
		fmt.Fprintf(os.Stderr, "[DEBUG] RPC %s status=%d body=%s\n", method, resp.StatusCode, string(rawBody))
	}

	result, err := rpc.DecodeResponse(string(rawBody), string(method))
	if err != nil {
		return nil, err
	}

	if result == nil && !allowNull {
		return nil, fmt.Errorf("RPC %s returned null result", method)
	}

	return result, nil
}

// Download performs an authenticated GET request and returns the response body.
func (c *Client) Download(ctx context.Context, url string) ([]byte, error) {
	c.mu.Lock()
	cookieHeader := c.auth.CookieHeader()
	c.mu.Unlock()

	req, err := http.NewRequestWithContext(ctx, "GET", url, nil)
	if err != nil {
		return nil, err
	}
	req.Header.Set("Cookie", cookieHeader)
	req.Header.Set("User-Agent", "Mozilla/5.0 (Macintosh; Intel Mac OS X 10_15_7) AppleWebKit/537.36")

	resp, err := c.httpClient.Do(req)
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		return nil, fmt.Errorf("download failed: HTTP %d", resp.StatusCode)
	}
	return io.ReadAll(resp.Body)
}
