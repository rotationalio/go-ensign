package ensign_test

import (
	"bufio"
	"bytes"
	"context"
	"fmt"
	"math/rand"
	"os"
	"regexp"
	"strconv"
	"strings"
	"sync"
	"testing"
	"time"

	"github.com/oklog/ulid/v2"
	"github.com/rotationalio/go-ensign"
	api "github.com/rotationalio/go-ensign/api/v1beta1"
	mimetype "github.com/rotationalio/go-ensign/mimetype/v1beta1"
	"github.com/stretchr/testify/suite"
)

// Staging Tests execute real Ensign commands agains the Ensign Staging environment and
// are meant to simulate live integration testing with Ensign nodes rather than using
// mocks to test the SDK. These tests only run if the $ENSIGN_TEST_STAGING environment
// variable is set to 1 or true; otherwise the tests are skipped. The tests will fail if
// valid credentials to the staging environment are not set.
type stagingTestSuite struct {
	suite.Suite
	client *ensign.Client
}

func TestStaging(t *testing.T) {
	// Load the .env file if it exists
	loadDotEnv()

	// Check if the tests are enabled
	if !parseBool(os.Getenv("ENSIGN_TEST_STAGING")) {
		t.Skip("set the $ENSIGN_TEST_STAGING environment variable to execute this test suite")
		return
	}

	// Try to create the Ensign staging client
	client, err := ensign.New(
		ensign.WithEnsignEndpoint("ensign.ninja:443", false),
		ensign.WithAuthenticator("https://auth.ensign.world", false),
	)

	if err != nil {
		t.Errorf("could not create ensign staging client: %s", err)
		return
	}

	suite.Run(t, &stagingTestSuite{client: client})
}

var semver *regexp.Regexp = regexp.MustCompile(`^v?(?P<major>\d+)\.(?P<minor>\d+)\.(?P<patch>\d+)(-[a-zA-Z0-9\.]+)?(\s+\((?P<revision>[a-f0-9]{7})\))?$`)

func (s *stagingTestSuite) TestPingVersion() {
	require := s.Require()

	// Check that we can connect to staging and that the SDK versions match.
	state, err := s.client.Status(context.Background())

	require.NoError(err, "could not reach ensign staging")
	require.Equal(api.ServiceState_HEALTHY, state.Status, "expected ok status")
	require.NotEmpty(state.Version, "no version information was returned")

	// Parse the version
	major, minor, err := parseSemVer(state.Version)
	require.NoError(err, "could not parse version info %q", state.Version)
	require.Equal(ensign.VersionMajor, major, "major version mismatch")
	require.Equal(ensign.VersionMinor, minor, "minor version mismatch")
}

func (s *stagingTestSuite) TestEnsignIntegration() {
	// Runs an integration test, creating a topic, listing the topic, and publishing
	// and subscribing to the topic to ensure the client SDK is working as expected.
	require := s.Require()
	assert := s.Assert()

	ctx, cancel := context.WithTimeout(context.Background(), 45*time.Second)
	defer cancel()

	// Create a topic with a random name and assert it does not exist
	topicName := fmt.Sprintf("testing.random.%s", strings.ToLower(ulid.Make().String()))
	exists, err := s.client.TopicExists(ctx, topicName)
	require.NoError(err, "could not query topic exists")
	require.False(exists, "random topic already exists")

	// Create the topic in Ensign
	topicID, err := s.client.CreateTopic(ctx, topicName)
	require.NoError(err, "could not create topic in Ensign")
	require.NotEmpty(topicID, "no topic id was returned")

	// Topic should exist now
	exists, err = s.client.TopicExists(ctx, topicName)
	require.NoError(err, "could not query topic exists")
	require.True(exists, "random topic already exists")

	// Should be able to lookup a topic ID
	cmprID, err := s.client.TopicID(ctx, topicName)
	require.NoError(err, "could not get topicID from topic name")
	require.Equal(topicID, cmprID, "topic ID does not match created return and topic id lookup")

	// Should be able to list topics in the project
	topicList, err := s.client.ListTopics(ctx)
	require.NoError(err, "could not list topics")
	require.GreaterOrEqual(len(topicList), 1, "expected at least one topic back from list")

	// Convert the topicID to a ULID
	topicULID, err := ulid.Parse(topicID)
	require.NoError(err, "Could not parse topic ulid")

	found := false
	for _, topic := range topicList {
		if bytes.Equal(topic.Id, topicULID[:]) {
			require.Equal(topicName, topic.Name, "unexpected topic name")
			found = true
			break
		}
	}
	require.True(found, "could not identify topic in topic list")

	// Create a subscriber to listen for events on
	sub, err := s.client.Subscribe(context.Background(), topicID)
	require.NoError(err, "could not subscribe to topic")

	var wg sync.WaitGroup
	nsent, nrecv := 0, 0

	// Consume events as they come
	wg.Add(1)
	go func() {
		defer wg.Done()
		defer sub.Close()

		for event := range sub.C {
			nrecv++
			acked, err := event.Ack()
			assert.NoError(err, "could not acknowledge consumed message")
			require.True(acked, "message should be acked")

			if done := event.Metadata.Get("done"); done != "" {
				return
			}
		}
	}()

	// Publish events to the topic
	wg.Add(1)
	go func() {
		defer wg.Done()
		for i := 0; i < 10; i++ {
			event := &ensign.Event{
				Metadata: make(ensign.Metadata),
				Data:     make([]byte, 0, 512),
				Mimetype: mimetype.ApplicationOctetStream,
			}

			event.Metadata["msg"] = strconv.Itoa(i + 1)
			rand.Read(event.Data)

			err := s.client.Publish(topicID, event)
			assert.NoError(err, "could not publish event")
		}

		event := &ensign.Event{
			Metadata: make(ensign.Metadata),
			Data:     []byte("done"),
			Mimetype: mimetype.TextPlain,
		}
		event.Metadata["done"] = "true"
		err := s.client.Publish(topicID, event)
		assert.NoError(err, "could not publish done event")
	}()

	wg.Wait()
	require.Equal(nsent, nrecv, "the number of messages published does not equal those consumed")

	// TODO: test archiving the topic
	// TODO: delete the topic so we are not wasting resources
}

// A lightweight mechanism to load a .env file without adding godotenv as a dependency.
// This method is not as robust as godotenv and some valid .env files may not load.
func loadDotEnv() (err error) {
	var f *os.File
	if f, err = os.Open(".env"); err != nil {
		return err
	}
	defer f.Close()

	reader := bufio.NewScanner(f)
	for reader.Scan() {
		line := strings.TrimSpace(reader.Text())
		if strings.HasPrefix(line, "#") {
			continue
		}

		parts := strings.Split(line, "=")
		if len(parts) != 2 {
			continue
		}

		key := strings.TrimSpace(parts[0])
		val := strings.TrimSpace(parts[1])

		if err = os.Setenv(key, val); err != nil {
			return err
		}
	}
	return reader.Err()
}

func parseBool(s string) bool {
	switch strings.TrimSpace(s) {
	case "", "0", "n", "f", "no", "false":
		return false
	case "1", "t", "y", "yes", "true":
		return true
	default:
		return false
	}
}

func parseSemVer(s string) (major, minor int, err error) {
	if !semver.MatchString(s) {
		return 0, 0, fmt.Errorf("could not parse semver from %q", s)
	}

	match := semver.FindStringSubmatch(s)
	result := make(map[string]string)
	for i, name := range semver.SubexpNames() {
		if i != 0 && name != "" {
			result[name] = match[i]
		}
	}

	if major, err = strconv.Atoi(result["major"]); err != nil {
		return 0, 0, err
	}

	if minor, err = strconv.Atoi(result["minor"]); err != nil {
		return 0, 0, err
	}

	return major, minor, nil
}
