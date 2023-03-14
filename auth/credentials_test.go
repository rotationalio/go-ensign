package auth_test

import (
	"context"
	"strings"
	"testing"

	api "github.com/rotationalio/go-ensign/api/v1beta1"
	"github.com/rotationalio/go-ensign/auth"
	"github.com/rotationalio/go-ensign/mock"
	"github.com/stretchr/testify/require"
	"google.golang.org/grpc"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/credentials/insecure"
	"google.golang.org/grpc/metadata"
	"google.golang.org/grpc/status"
)

const (
	header      = "authorization" // MUST BE LOWER CASE!
	bearer      = "Bearer "       // MUST INCLUDE TRAILING SPACE!
	dialerToken = "dialeraccesstoken"
	callToken   = "percallaccesstoken"
)

func TestInsecureCredentials(t *testing.T) {
	mock := mock.New(nil)
	defer mock.Shutdown()

	var actualToken string
	mock.OnListTopics = func(ctx context.Context, in *api.PageInfo) (*api.TopicsPage, error) {
		var (
			md metadata.MD
			ok bool
		)

		// Get token from the context.
		if md, ok = metadata.FromIncomingContext(ctx); !ok {
			return nil, status.Error(codes.Unauthenticated, "missing credentials")
		}

		// Extract the authorization credentials (we expect [at least] 1 JWT token)
		values := md[header]
		if len(values) == 0 {
			return nil, status.Error(codes.Unauthenticated, "missing credentials")
		}

		// Loop through credentials to find the first valid claims
		// NOTE: we only expect one token but are trying to future-proof the interceptor
		for _, value := range values {
			if !strings.HasPrefix(value, bearer) {
				continue
			}

			actualToken = strings.TrimPrefix(value, bearer)
			if actualToken != dialerToken && actualToken != callToken {
				return nil, status.Error(codes.Unauthenticated, "incorrect token in request")
			}
		}

		return &api.TopicsPage{}, nil
	}

	client, err := mock.Client(context.Background(), auth.WithPerRPCToken(dialerToken, true), grpc.WithTransportCredentials(insecure.NewCredentials()))
	require.NoError(t, err, "could not create mock client to connect to server with")

	// Should be able to connect with the dialeraccess token
	_, err = client.ListTopics(context.Background(), &api.PageInfo{})
	require.NoError(t, err, "could not list topics")
	require.Equal(t, dialerToken, actualToken)

	// Should be able to make per-call requests
	_, err = client.ListTopics(context.Background(), &api.PageInfo{}, auth.PerRPCToken(callToken, true))
	require.NoError(t, err, "could not list topics")
	require.Equal(t, callToken, actualToken)
}
