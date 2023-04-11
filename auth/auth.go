/*
Package auth enables an Ensign client to authenticate with Ensign's Authn and Authz
service called Quarterdeck. Every Ensign RPC must include access tokens with the
ProjectID and permissions assigned to the API key; the access tokens are signed by
Quarterdeck's private keys which can be verified on an Ensign server using Quarterdeck's
public keys. If the RPC credentials have incorrect permissions or attempt to access a
resource that does not belong to the project then a gRPC Unauthorized error is returned.

Note: API Keys have access to all the topics in the specified project with the
permissions assigned to the key; you cannot limit topics other than by creating new
projects.

This package provides a Client that wraps APIKeys and makes requests to Quarterdeck in
order to authenticate and refresh Ensign credentials. The client is intended to be used
by the Ensign client to maintain an authenticated connection to Ensign for long-running
processes (e.g. publishers and subscribers) and will make requests to Quarterdeck in an
on-demand fashion to maintain authentication without logging out. The Ensign SDK must
ensure that it requests credentials for every RPC call that it makes.
*/
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

	"github.com/cenkalti/backoff/v4"
	"google.golang.org/grpc"
	"google.golang.org/grpc/credentials"
)

const (
	AuthenticateEP = "/v1/authenticate"
	RefreshEP      = "/v1/refresh"
	StatusEP       = "/v1/status"
)

// Client connects to the Quarterdeck authentication service in order to authenticate
// API Keys and to refresh access tokens for Ensign access. The Client maintains the
// API Keys and tokens so that it can hand out credentials in long running processes,
// ensuring that the Ensign client can stay logged into Ensign for as long as possible.
type Client struct {
	endpoint *url.URL
	api      *http.Client
	apikey   *APIKey
	tokens   *Tokens
	insecure bool
}

// Create a new authentication client to connect to Quarterdeck. The authURL should be
// the endpoint of the Quarterdeck service and must be a parseable URL. The insecure
// flag tells the client to create Ensign credentials that are insecure; e.g. not
// requiring a TLS connection. The insecure flag should only be true in development.
// After creating a Quarterdeck client, ensure to call Login() to prepare it to hand out
// credentials to connect to Ensign.
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

// Login to Quarterdeck, storing the API credentials on the client and making a login
// request to Quarterdeck to fetch access and refresh tokens. Ensure that a context
// with a deadline is specified in order to reduce how long the client attempts to login
// for. Once logged in, the authentication client can hand out credentials on demand.
// Credentials are returned from this method in case users want to add the credentials
// as a DialOption; however this is only good for short duration process (e.g. processes
// that will stop before the access token expires). Long running processes should use
// the UnaryInterceptor and StreamInterceptor methods or call Credentials to get a
// PerRPCCredentials CallOption to add to every RPC call.
func (c *Client) Login(ctx context.Context, clientID, clientSecret string) (creds credentials.PerRPCCredentials, err error) {
	// Require both clientID and clientSecret
	if clientID == "" || clientSecret == "" {
		return nil, ErrIncompleteCreds
	}

	// Store the API key on the client so that authentication can happen again.
	c.apikey = &APIKey{
		ClientID:     clientID,
		ClientSecret: clientSecret,
	}

	// Authenticate and store the tokens on the client to cache for each call.
	if c.tokens, err = c.Authenticate(ctx, c.apikey); err != nil {
		return nil, err
	}

	// Return credentials for dial options.
	return c.Credentials(ctx)
}

// Credentials returns the PerRPC credentials to make a gRPC request. If the tokens are
// expired, this method will refresh them by making a request to Quarterdeck. An error
// is returned if the client is not logged in. This method should be called before every
// Ensign RPC in order to ensure the RPC has valid credentials.
func (c *Client) Credentials(ctx context.Context) (_ credentials.PerRPCCredentials, err error) {
	// Check if tokens exist; if they don't exist, then authenticate.
	if c.tokens == nil || c.tokens.AccessToken == "" || c.tokens.RefreshToken == "" {
		// Tokens are missing or are partial, authenticate to get new tokens
		if c.tokens, err = c.Authenticate(ctx, c.apikey); err != nil {
			return nil, err
		}
	}

	// Check if the access token is valid
	var accessValid bool
	if accessValid, err = c.tokens.AccessValid(); err != nil {
		// Returning an error here is acceptable because we checked if the access tokens
		// were missing in an above step.
		return nil, err
	}

	// If the access token is not valid, attempt to use the refresh token to validate.
	if !accessValid {
		var refreshValid bool
		if refreshValid, err = c.tokens.RefreshValid(); err != nil {
			// Returning an error is acceptable here because we checked if the refresh
			// tokens were missing in an above step.
			return nil, err
		}

		// If the refresh tokens are valid, use it to refresh the access token,
		// otherwise reauthenticate using the credentials.
		if refreshValid {
			if c.tokens, err = c.Refresh(ctx, c.tokens); err != nil {
				return nil, err
			}
		} else {
			if c.tokens, err = c.Authenticate(ctx, c.apikey); err != nil {
				return nil, err
			}
		}
	}

	// At this point we should have a valid access token one way or another ...
	return &Credentials{
		accessToken: c.tokens.AccessToken,
		insecure:    c.insecure,
	}, nil
}

// An interceptor that adds credentials on every unary request made by the gRPC client.
func (c *Client) UnaryAuthenticate(ctx context.Context, method string, req, reply interface{}, cc *grpc.ClientConn, invoker grpc.UnaryInvoker, opts ...grpc.CallOption) (err error) {
	var creds credentials.PerRPCCredentials
	if creds, err = c.Credentials(ctx); err != nil {
		return err
	}

	opts = append(opts, grpc.PerRPCCredentials(creds))
	return invoker(ctx, method, req, reply, cc, opts...)
}

// An interceptor that adds credentials to every streaming request made by the gRPC client.
func (c *Client) StreamAuthenticate(ctx context.Context, desc *grpc.StreamDesc, cc *grpc.ClientConn, method string, streamer grpc.Streamer, opts ...grpc.CallOption) (_ grpc.ClientStream, err error) {
	var creds credentials.PerRPCCredentials
	if creds, err = c.Credentials(ctx); err != nil {
		return nil, err
	}

	opts = append(opts, grpc.PerRPCCredentials(creds))
	return streamer(ctx, desc, cc, method, opts...)
}

// Authenticate makes a request to the Quarterdeck server with the available API keys
// in order to fetch new access and refresh tokens. The tokens are returned directly.
func (c *Client) Authenticate(ctx context.Context, apikey *APIKey) (tokens *Tokens, err error) {
	// Check to make sure we have APIKeys in order to perform the authentication.
	if apikey == nil || apikey.ClientID == "" || apikey.ClientSecret == "" {
		return nil, ErrNoAPIKeys
	}

	var req *http.Request
	if req, err = c.newRequest(ctx, http.MethodPost, AuthenticateEP, apikey); err != nil {
		return nil, err
	}

	tokens = &Tokens{}
	if _, err = c.do(req, tokens); err != nil {
		return nil, err
	}

	return tokens, nil
}

// Refresh makes a request to the Quarterdeck server with the refresh token in order to
// fetch a new access token. If the refresh token is expired an error is returned. The
// new tokens are returned directly.
func (c *Client) Refresh(ctx context.Context, refresh *Tokens) (tokens *Tokens, err error) {
	var req *http.Request
	if req, err = c.newRequest(ctx, http.MethodPost, RefreshEP, refresh); err != nil {
		return nil, err
	}

	tokens = &Tokens{}
	if _, err = c.do(req, tokens); err != nil {
		return nil, err
	}

	return tokens, nil
}

// Status makes a request to the Quarterdeck server to check if the service is online
// and ready to make requests. The status check is returned directly.
func (c *Client) Status(ctx context.Context) (status *Status, err error) {
	var req *http.Request
	if req, err = c.newRequest(ctx, http.MethodPost, StatusEP, nil); err != nil {
		return nil, err
	}

	status = &Status{}
	if _, err = c.do(req, status); err != nil {
		return nil, err
	}

	return status, nil
}

// Wait for ready polls the Quarterdeck status endpoint until it responds with a 200,
// retrying with exponential backoff or until the context deadline is expired. If the
// input context does not have a deadline, then a default deadline of 5 minutes is used
// so this method does not block indefinitely. When the Quarterdeck service is ready
// then no error is returned; if the Quartdeck does not respond within the retry window
// an error is returned.
func (c *Client) WaitForReady(ctx context.Context) (err error) {
	// If context does not have a deadline, create a context with a default deadline
	if _, hasDeadline := ctx.Deadline(); !hasDeadline {
		var cancel context.CancelFunc
		ctx, cancel = context.WithTimeout(ctx, 5*time.Minute)
		defer cancel()
	}

	// Create the status request to send until ready
	var req *http.Request
	if req, err = c.newRequest(ctx, http.MethodGet, StatusEP, nil); err != nil {
		return err
	}

	// Create a closure to call the Quarterdeck status endpoint
	checkReady := func() (err error) {
		var rep *http.Response
		if rep, err = c.api.Do(req); err != nil {
			return err
		}
		defer rep.Body.Close()

		if rep.StatusCode < 200 || rep.StatusCode >= 300 {
			return &StatusError{StatusCode: rep.StatusCode, Reply: Reply{Success: false, Error: http.StatusText(rep.StatusCode)}}
		}
		return nil
	}

	// Create exponential backoff ticker for retries
	ticker := backoff.NewExponentialBackOff()

	// Keep checking if Quarterdeck is ready until it responds or the context expires.
	for {
		// Execute the status request
		if err = checkReady(); err == nil {
			// Success - Quarterdeck is ready for requests!
			return nil
		}

		// Delay until the next backoff retry or the context expires
		wait := time.After(ticker.NextBackOff())
		select {
		case <-ctx.Done():
			return ctx.Err()
		case <-wait:
		}
	}

}

//===========================================================================
// Testing Methods
//===========================================================================

// Reset removes the apikeys and tokens from the client (used for testing).
func (c *Client) Reset() {
	c.apikey = nil
	c.tokens = nil
}

// SetTokens allows the test suite to set the tokens on the client.
func (c *Client) SetTokens(tokens *Tokens) {
	c.tokens = tokens
}

// SetAPIKey allows the test suite to set the apikey on the client.
func (c *Client) SetAPIKey(key *APIKey) {
	c.apikey = key
}

//===========================================================================
// Helper Methods
//===========================================================================

const (
	userAgent   = "Ensign Go-SDK Client Authentication/v1"
	accept      = "application/json"
	contentType = "application/json; charset=utf-8"
)

// Create a new HTTP request to the Quarterdeck server with the correct headers and body.
func (c *Client) newRequest(ctx context.Context, method, path string, data interface{}) (req *http.Request, err error) {
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

// Execute an http request against the server, perform error checking, and
// deserialize the response data into the specified struct.
func (c *Client) do(req *http.Request, data interface{}) (rep *http.Response, err error) {
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
