package client

import (
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"strings"

	"github.com/hugobrenet/opensvc-daemon-mcp/internal/auth"
)

const maxResponseBodySize = 10 << 20

type Client struct {
	baseURL       *url.URL
	httpClient    *http.Client
	authenticator auth.Authenticator
}

func New(rawBaseURL string, httpClient *http.Client, authenticator auth.Authenticator) (*Client, error) {
	baseURL, err := url.Parse(rawBaseURL)
	if err != nil {
		return nil, fmt.Errorf("parse OpenSVC daemon URL: %w", err)
	}
	if baseURL.Scheme != "http" && baseURL.Scheme != "https" {
		return nil, fmt.Errorf("OpenSVC daemon URL must use http or https")
	}
	if baseURL.Host == "" {
		return nil, fmt.Errorf("OpenSVC daemon URL must include a host")
	}
	if httpClient == nil {
		httpClient = http.DefaultClient
	}
	if authenticator == nil {
		return nil, fmt.Errorf("OpenSVC daemon authenticator is required")
	}
	return &Client{baseURL: baseURL, httpClient: httpClient, authenticator: authenticator}, nil
}

func (c *Client) GetJSON(ctx context.Context, path string, query url.Values, output any) error {
	endpoint := c.baseURL.JoinPath(strings.TrimPrefix(path, "/"))
	endpoint.RawQuery = query.Encode()

	request, err := http.NewRequestWithContext(ctx, http.MethodGet, endpoint.String(), nil)
	if err != nil {
		return fmt.Errorf("create OpenSVC daemon GET request: %w", err)
	}
	request.Header.Set("Accept", "application/json")
	if err := c.authenticator.Apply(request); err != nil {
		return fmt.Errorf("authenticate OpenSVC daemon request: %w", err)
	}

	response, err := c.httpClient.Do(request)
	if err != nil {
		return fmt.Errorf("request OpenSVC daemon %s: %w", path, err)
	}
	defer response.Body.Close()

	if response.StatusCode < http.StatusOK || response.StatusCode >= http.StatusMultipleChoices {
		return fmt.Errorf("OpenSVC daemon %s returned HTTP %s", path, response.Status)
	}

	decoder := json.NewDecoder(io.LimitReader(response.Body, maxResponseBodySize))
	if err := decoder.Decode(output); err != nil {
		return fmt.Errorf("decode OpenSVC daemon %s response: %w", path, err)
	}
	return nil
}
