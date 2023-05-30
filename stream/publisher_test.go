package stream_test

import (
	"testing"

	"github.com/stretchr/testify/suite"
)

type publisherTestSuite struct {
	suite.Suite
	mock *MockConnectionObserver
}

// Create the bufconn and mock when the suite starts.
func (s *publisherTestSuite) SetupSuite() {
	var err error
	s.mock, err = NewMockConnectionObserver()
	s.Assert().NoError(err, "unable to setup mock suite")
}

// When the suite is done teardown the bufconn and mock connections.
func (s *publisherTestSuite) TearDownSuite() {
	s.mock.conn.Close()
	s.mock.server.Shutdown()
	s.mock.sock.Close()
}

// After each test make sure the mock server is reset.
func (s *publisherTestSuite) AfterTest(suiteName, testName string) {
	s.mock.server.Reset()
}

func TestPublisher(t *testing.T) {
	suite.Run(t, &publisherTestSuite{})
}
