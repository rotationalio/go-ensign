package sdk

import (
	"context"
	"encoding/base64"
	"errors"

	"github.com/oklog/ulid/v2"
	api "github.com/rotationalio/go-ensign/api/v1beta1"
	"github.com/spaolacci/murmur3"
)

const DefaultPageSize uint32 = 100

// Check if a topic with the specified name exists in the project or not.
func (c *Client) TopicExists(ctx context.Context, topicName string) (_ bool, err error) {
	var info *api.TopicExistsInfo
	if info, err = c.api.TopicExists(ctx, &api.TopicName{Name: topicName}); err != nil {
		return false, err
	}
	return info.Exists, nil
}

// Create topic with the specified name and return the topic ID if there was no error.
func (c *Client) CreateTopic(ctx context.Context, topic string) (_ string, err error) {
	var reply *api.Topic
	if reply, err = c.api.CreateTopic(ctx, &api.Topic{Name: topic}); err != nil {
		// TODO: do a better job of categorizing the error
		return "", err
	}

	// Convert the topic ID into a ULID string for user consumption.
	var topicID ulid.ULID
	if err = topicID.UnmarshalBinary(reply.Id); err != nil {
		// TODO: do a better job of categorizing the error
		return "", err
	}
	return topicID.String(), nil
}

func (c *Client) ListTopics(ctx context.Context) (topics []*api.Topic, err error) {
	// TODO: return an iterator rather than materializing all of the topics
	topics = make([]*api.Topic, 0)
	query := &api.PageInfo{PageSize: DefaultPageSize}

	// Request all topics pages making each request in succession.
	var page *api.TopicsPage
	for page == nil || page.NextPageToken != "" {
		// If the context is done, stop requesting new pages
		select {
		case <-ctx.Done():
			return nil, ctx.Err()
		default:
		}

		// Make the topics page request
		if page, err = c.api.ListTopics(ctx, query); err != nil {
			// TODO: do a better job of categorizing the error
			return nil, err
		}

		// Update the query and append the topics to the request
		topics = append(topics, page.Topics...)
		query.NextPageToken = page.NextPageToken
	}

	return topics, nil
}

// Archive a topic marking it as read-only.
func (c *Client) ArchiveTopic(ctx context.Context, topicID string) (err error) {
	return errors.New("not implemented yet")
}

// Destroy a topic removing it and all of its data.
func (c *Client) DestroyTopic(ctx context.Context, topicID string) (err error) {
	return errors.New("not implemented yet")
}

// Find a topic ID from a topic name.
// TODO: automate and cache this on the client for easier lookups.
func (c *Client) TopicID(ctx context.Context, topicName string) (_ string, err error) {
	// Create a base64 encoded murmur3 hash of the topic name
	hash := murmur3.New128()
	hash.Write([]byte(topicName))
	topicHash := base64.RawURLEncoding.EncodeToString(hash.Sum(nil))

	// List the topic names until the topic ID is found
	var page *api.TopicNamesPage
	query := &api.PageInfo{PageSize: uint32(100)}

	for page == nil || page.NextPageToken != "" {
		if page, err = c.api.TopicNames(ctx, query); err != nil {
			return "", err
		}

		for _, topic := range page.TopicNames {
			if topic.Name == topicHash {
				return topic.TopicId, nil
			}
		}
		query.NextPageToken = page.NextPageToken
	}

	return "", ErrTopicNameNotFound
}
