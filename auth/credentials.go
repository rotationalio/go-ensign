package auth

import (
	"context"

	"google.golang.org/grpc"
)

// Credentials implement the credentials.PerRPCCredentials interface so that the access
// token can be embedded in the metadata of each RPC call for authentication and
// authorization. The credentials wrap an access token and whether or not the
// credentials can be used in insecure mode. Insecure should almost always be false;
// the only exception is when doing local development with an Ensign service running in
// docker compose or in CI tests. For staging and production, insecure should be false.
type Credentials struct {
	accessToken string
	insecure    bool
}

// GetRequestMetadata attaches the bearer access token to the authorization header.
func (t *Credentials) GetRequestMetadata(ctx context.Context, uri ...string) (map[string]string, error) {
	return map[string]string{
		"Authorization": "Bearer " + t.accessToken,
	}, nil
}

// RequireTransportSecurity should almost always return true unless accessing a local
// Ensign server in development or CI environments.
func (t *Credentials) RequireTransportSecurity() bool {
	return !t.insecure
}

// Equals compares credentials (primarily used for testing).
func (t *Credentials) Equals(o *Credentials) bool {
	return t.accessToken == o.accessToken && t.insecure == o.insecure
}

// PerRPCToken returns a CallOption to attach access tokens to a single RPC call.
// Because access tokens expire and need to be refreshed; this is the preferred way of
// attaching credentials to an RPC call.
func PerRPCToken(accessToken string, insecure bool) grpc.CallOption {
	return grpc.PerRPCCredentials(&Credentials{accessToken: accessToken, insecure: insecure})
}

// WithPerRPCToken returns a DialOption to ensure that the credentials are attached to
// every RPC call but only have to be specified once by the dialer. The issue with using
// this method is that access tokens expire; so unless you're expecting your Ensign
// session to be shorter than the access token duration (about an hour), then using the
// PerRPCToken CallOption is usually a better choice.
func WithPerRPCToken(accessToken string, insecure bool) grpc.DialOption {
	return grpc.WithPerRPCCredentials(&Credentials{accessToken: accessToken, insecure: insecure})
}
