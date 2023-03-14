package auth

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"net/http/cookiejar"
	"net/url"
	"time"

	"google.golang.org/grpc/credentials"
)

func New(authURL string, insecure bool) (client *Client, err error) {
	client = &Client{
		insecure: insecure,
		api: &http.Client{
			Transport:     nil,
			CheckRedirect: nil,
			Timeout:       30 * time.Second,
		},
	}

	if client.endpoint, err = url.Parse(authURL); err != nil {
		return nil, fmt.Errorf("could not parse auth url: %w", err)
	}

	if client.api.Jar, err = cookiejar.New(nil); err != nil {
		return nil, fmt.Errorf("could not create cookiejar: %w", err)
	}

	return client, nil
}

type Client struct {
	endpoint *url.URL
	api      *http.Client
	apikey   *APIKey
	tokens   *Tokens
	insecure bool
}

type APIKey struct {
	ClientID     string `json:"client_id"`
	ClientSecret string `json:"client_secret"`
}

type Tokens struct {
	AccessToken  string `json:"access_token,omitempty"`
	RefreshToken string `json:"refresh_token,omitempty"`
	LastLogin    string `json:"last_login,omitempty"`
}

func (c *Client) Login(ctx context.Context, clientID, clientSecret string) (_ credentials.PerRPCCredentials, err error) {
	c.apikey = &APIKey{
		ClientID:     clientID,
		ClientSecret: clientSecret,
	}

	if err = c.Authenticate(ctx); err != nil {
		return nil, err
	}

	creds := &Credentials{
		accessToken: c.tokens.AccessToken,
		insecure:    c.insecure,
	}

	return creds, nil
}

func (c *Client) Authenticate(ctx context.Context) (err error) {
	// TODO: ensure apikeys are available
	// TODO: check tokens to make sure they're not expired

	var req *http.Request
	if req, err = c.NewRequest(ctx, http.MethodPost, "/v1/authenticate", c.apikey); err != nil {
		return err
	}

	c.tokens = &Tokens{}
	if _, err = c.Do(req, c.tokens); err != nil {
		return err
	}

	return nil
}

func (c *Client) Refresh(ctx context.Context) (err error) {
	// TODO: check refresh token is available
	// TODO: check refresh token to make sure it is not expired

	tokens := &Tokens{
		RefreshToken: c.tokens.RefreshToken,
	}

	var req *http.Request
	if req, err = c.NewRequest(ctx, http.MethodPost, "/v1/refresh", tokens); err != nil {
		return err
	}

	c.tokens = &Tokens{}
	if _, err = c.Do(req, c.tokens); err != nil {
		return err
	}

	return nil
}

//===========================================================================
// Helper Methods
//===========================================================================

const (
	userAgent   = "Ensign Go-SDK Client Authentication/v1"
	accept      = "application/json"
	contentType = "application/json; charset=utf-8"
)

func (c *Client) NewRequest(ctx context.Context, method, path string, data interface{}) (req *http.Request, err error) {
	// Resolve the URL reference from the path
	url := c.endpoint.ResolveReference(&url.URL{Path: path})

	var body io.ReadWriter
	switch {
	case data == nil:
		body = nil
	default:
		body = &bytes.Buffer{}
		if err = json.NewEncoder(body).Encode(data); err != nil {
			return nil, fmt.Errorf("could not serialize request data as json: %s", err)
		}
	}

	// Create the http request
	if req, err = http.NewRequestWithContext(ctx, method, url.String(), body); err != nil {
		return nil, fmt.Errorf("could not create request: %s", err)
	}

	// Set the headers on the request
	req.Header.Add("User-Agent", userAgent)
	req.Header.Add("Accept", accept)
	req.Header.Add("Content-Type", contentType)

	// Add authentication if it's available (add Authorization header)
	if c.tokens != nil && c.tokens.AccessToken != "" {
		req.Header.Set("Authorization", "Bearer "+c.tokens.AccessToken)
	}

	// Add CSRF protection if its available
	if c.api.Jar != nil {
		cookies := c.api.Jar.Cookies(url)
		for _, cookie := range cookies {
			if cookie.Name == "csrf_token" {
				req.Header.Add("X-CSRF-TOKEN", cookie.Value)
			}
		}
	}

	return req, nil
}

// Do executes an http request against the server, performs error checking, and
// deserializes the response data into the specified struct.
func (c *Client) Do(req *http.Request, data interface{}) (rep *http.Response, err error) {
	if rep, err = c.api.Do(req); err != nil {
		return rep, fmt.Errorf("could not execute request: %s", err)
	}
	defer rep.Body.Close()

	// Detect http status errors if they've occurred
	if rep.StatusCode < 200 || rep.StatusCode >= 300 {
		// Attempt to read the error response from JSON, if available
		serr := &StatusError{
			StatusCode: rep.StatusCode,
		}

		if err = json.NewDecoder(rep.Body).Decode(&serr.Reply); err == nil {
			return rep, serr
		}

		serr.Reply = unsuccessful
		return rep, serr
	}

	// Deserialize the JSON data from the body
	if data != nil && rep.StatusCode >= 200 && rep.StatusCode < 300 && rep.StatusCode != http.StatusNoContent {
		// Check the content type to ensure data deserialization is possible
		if ct := rep.Header.Get("Content-Type"); ct != contentType {
			return rep, fmt.Errorf("unexpected content type: %q", ct)
		}

		if err = json.NewDecoder(rep.Body).Decode(data); err != nil {
			return nil, fmt.Errorf("could not deserialize response data: %s", err)
		}
	}

	return rep, nil
}
