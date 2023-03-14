package auth

import (
	"context"

	"google.golang.org/grpc"
)

type Credentials struct {
	accessToken string
	insecure    bool
}

func (t *Credentials) GetRequestMetadata(ctx context.Context, uri ...string) (map[string]string, error) {
	return map[string]string{
		"Authorization": "Bearer " + t.accessToken,
	}, nil
}

func (t *Credentials) RequireTransportSecurity() bool {
	return !t.insecure
}

func PerRPCToken(accessToken string, insecure bool) grpc.CallOption {
	return grpc.PerRPCCredentials(&Credentials{accessToken: accessToken, insecure: insecure})
}

func WithPerRPCToken(accessToken string, insecure bool) grpc.DialOption {
	return grpc.WithPerRPCCredentials(&Credentials{accessToken: accessToken, insecure: insecure})
}
