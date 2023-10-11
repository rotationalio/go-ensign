package ensign_test

import (
	"context"
	"sync"
	"testing"

	sdk "github.com/rotationalio/go-ensign"
	api "github.com/rotationalio/go-ensign/api/v1beta1"
	"github.com/rotationalio/go-ensign/auth"
	"github.com/rotationalio/go-ensign/auth/authtest"
	"github.com/rotationalio/go-ensign/mock"
	"github.com/stretchr/testify/suite"
	"google.golang.org/grpc"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/credentials/insecure"
	"google.golang.org/grpc/status"
)

type sdkTestSuite struct {
	suite.Suite
	client      *sdk.Client
	auth        *auth.Client
	mock        *mock.Ensign
	quarterdeck *authtest.Server
}

func TestEnsignSDK(t *testing.T) {
	suite.Run(t, &sdkTestSuite{})
}

func (s *sdkTestSuite) SetupSuite() {
	var err error
	assert := s.Assert()

	// Create an authtest server for authentication
	s.quarterdeck, err = authtest.NewServer()
	assert.NoError(err, "could not create authtest server")

	// Create a mock Ensign server for testing
	s.mock = mock.New(nil)

	// Create an auth client
	s.auth, err = auth.New(s.quarterdeck.URL(), true)
	assert.NoError(err, "could not create auth client")

	// Create a client that is mocked
	s.client, err = sdk.New(
		sdk.WithMock(
			s.mock,
			grpc.WithTransportCredentials(insecure.NewCredentials()),
			grpc.WithUnaryInterceptor(s.auth.UnaryAuthenticate),
			grpc.WithStreamInterceptor(s.auth.StreamAuthenticate),
		),
		sdk.WithAuthenticator(s.quarterdeck.URL(), true),
	)
	assert.NoError(err, "could not create mocked ensign client")
}

func (s *sdkTestSuite) TearDownSuite() {
	s.client.Close()
	s.quarterdeck.Close()
	s.mock.Shutdown()
}

func (s *sdkTestSuite) AfterTest(_, _ string) {
	s.auth.Reset()
	s.mock.Reset()
}

// Check an error response from the gRPC Ensign client, ensuring that it is a) a status
// error, b) has the code specified, and c) (if supplied) that the message matches.
func (s *sdkTestSuite) GRPCErrorIs(err error, code codes.Code, msg string) {
	require := s.Require()
	require.Error(err, "expected an error but none was returned")

	serr, ok := status.FromError(err)
	require.True(ok, "err is not a grpc status error: %s", err)
	require.Equal(code, serr.Code(), "status code %s did not match expected %s", serr.Code(), code)
	if msg != "" {
		require.Equal(msg, serr.Message(), "status message did not match the expected message")
	}
}

// Authenticate is a one step method to ensure the client is logged into Ensign
func (s *sdkTestSuite) Authenticate(ctx context.Context) error {
	clientID, clientSecret := s.quarterdeck.Register()
	if _, err := s.auth.Login(ctx, clientID, clientSecret); err != nil {
		return err
	}
	return nil
}

// Test WithCallOptions
func (s *sdkTestSuite) TestWithCallOptions() {
	require := s.Require()
	assert := s.Assert()
	ctx := context.Background()

	// Authenticate the client for info tests
	err := s.Authenticate(ctx)
	require.NoError(err, "must be able to authenticate")

	clone := s.client.WithCallOptions(grpc.CallContentSubtype("json"), auth.PerRPCToken("token", false))
	require.NotSame(s.client, clone, "expected a clone returned not the same object")

	// The codec call option should create an error
	_, err = clone.Info(ctx)
	s.GRPCErrorIs(err, codes.Internal, "no codec registered for content-subtype json")

	// Return just an authenticated clone
	clone = s.client.WithCallOptions(auth.PerRPCToken("token", false))

	// Update the background mock method to count the call options
	s.mock.OnInfo = func(context.Context, *api.InfoRequest) (*api.ProjectInfo, error) {
		return &api.ProjectInfo{}, nil
	}

	// The call option should be sent to the mock from the clone and should be thread-safe
	var wg sync.WaitGroup
	wg.Add(2)

	go func() {
		defer wg.Done()
		for i := 0; i < 10; i++ {
			_, err := clone.Info(context.Background())
			assert.NoError(err, "could not make info request from clone")
		}
	}()

	go func() {
		defer wg.Done()
		for i := 0; i < 10; i++ {
			_, err := s.client.Info(ctx)
			assert.NoError(err, "could not make info request from client")
		}
	}()

	wg.Wait()
	require.Equal(20, s.mock.Calls[mock.InfoRPC], "expected 20 calls to info rpc")

	// This must happen last for the test to pass
	require.NotPanics(func() { clone.Close() }, "expected clone to not panic on close")
}
