package client

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"mime"
	"net/http"
	"net/url"
	"strings"

	"github.com/hugobrenet/opensvc-daemon-mcp/internal/auth"
)

const (
	maxResponseBodySize       = 10 << 20
	maxStreamResponseBodySize = 2 << 20
)

type Client struct {
	baseURL    *url.URL
	httpClient *http.Client
}

func New(rawBaseURL string, httpClient *http.Client) (*Client, error) {
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
	return &Client{baseURL: baseURL, httpClient: httpClient}, nil
}

func (c *Client) GetJSON(ctx context.Context, path string, query url.Values, output any) error {
	return c.doJSON(ctx, http.MethodGet, path, query, nil, output)
}

func (c *Client) PostJSON(ctx context.Context, path string, query url.Values, input any, output any) error {
	var body io.Reader
	if input != nil {
		payload, err := json.Marshal(input)
		if err != nil {
			return fmt.Errorf("encode OpenSVC daemon POST request body: %w", err)
		}
		body = bytes.NewReader(payload)
	}
	return c.doJSON(ctx, http.MethodPost, path, query, body, output)
}

// GetStream sends an authenticated GET request and delivers bounded opaque
// response chunks to consume. The OpenSVC container-log endpoint declares
// text/event-stream but returns raw log bytes rather than SSE framing.
func (c *Client) GetStream(ctx context.Context, path string, query url.Values, consume func([]byte) error) error {
	if consume == nil {
		return fmt.Errorf("OpenSVC daemon stream consumer is nil")
	}
	endpoint := c.baseURL.JoinPath(strings.TrimPrefix(path, "/"))
	endpoint.RawQuery = query.Encode()
	request, err := http.NewRequestWithContext(ctx, http.MethodGet, endpoint.String(), nil)
	if err != nil {
		return fmt.Errorf("create OpenSVC daemon GET request: %w", err)
	}
	request.Header.Set("Accept", "text/event-stream")
	if err := auth.ApplyBearerFromContext(request); err != nil {
		return fmt.Errorf("authenticate OpenSVC daemon request: %w", err)
	}

	response, err := c.httpClient.Do(request)
	if err != nil {
		return fmt.Errorf("request OpenSVC daemon %s: %w", path, err)
	}
	defer response.Body.Close()
	if response.StatusCode < http.StatusOK || response.StatusCode >= http.StatusMultipleChoices {
		return newAPIError(http.MethodGet, path, response)
	}
	mediaType, _, err := mime.ParseMediaType(response.Header.Get("Content-Type"))
	if err != nil || mediaType != "text/event-stream" {
		return fmt.Errorf("OpenSVC daemon %s returned unexpected content type %q", path, response.Header.Get("Content-Type"))
	}

	reader := io.LimitReader(response.Body, maxStreamResponseBodySize+1)
	buffer := make([]byte, 32<<10)
	total := 0
	for {
		n, readErr := reader.Read(buffer)
		if n > 0 {
			total += n
			if total > maxStreamResponseBodySize {
				return fmt.Errorf("OpenSVC daemon %s stream exceeds %d bytes", path, maxStreamResponseBodySize)
			}
			if err := consume(buffer[:n]); err != nil {
				return fmt.Errorf("consume OpenSVC daemon stream: %w", err)
			}
		}
		if readErr != nil {
			if readErr == io.EOF {
				return nil
			}
			return fmt.Errorf("read OpenSVC daemon %s stream: %w", path, readErr)
		}
	}
}

func (c *Client) doJSON(ctx context.Context, method string, path string, query url.Values, body io.Reader, output any) error {
	endpoint := c.baseURL.JoinPath(strings.TrimPrefix(path, "/"))
	endpoint.RawQuery = query.Encode()

	request, err := http.NewRequestWithContext(ctx, method, endpoint.String(), body)
	if err != nil {
		return fmt.Errorf("create OpenSVC daemon %s request: %w", method, err)
	}
	request.Header.Set("Accept", "application/json")
	if body != nil {
		request.Header.Set("Content-Type", "application/json")
	}
	if err := auth.ApplyBearerFromContext(request); err != nil {
		return fmt.Errorf("authenticate OpenSVC daemon request: %w", err)
	}

	response, err := c.httpClient.Do(request)
	if err != nil {
		return fmt.Errorf("request OpenSVC daemon %s: %w", path, err)
	}
	defer response.Body.Close()

	if response.StatusCode < http.StatusOK || response.StatusCode >= http.StatusMultipleChoices {
		return newAPIError(method, path, response)
	}

	if output == nil {
		return nil
	}
	decoder := json.NewDecoder(io.LimitReader(response.Body, maxResponseBodySize))
	if err := decoder.Decode(output); err != nil {
		return fmt.Errorf("decode OpenSVC daemon %s response: %w", path, err)
	}
	return nil
}
