package sdk

import (
	"context"
	"errors"

	"github.com/oklog/ulid/v2"
	api "github.com/rotationalio/go-ensign/api/v1beta1"
)

const DefaultPageSize uint32 = 100

// Check if a topic with the specified name exists in the project or not.
func (c *Client) TopicExists(ctx context.Context, topicName string) (_ bool, err error) {
	// TODO: more directly use an exists RPC call for faster checking with an index.
	var topics []*api.Topic
	if topics, err = c.ListTopics(ctx); err != nil {
		return false, err
	}

	for _, topic := range topics {
		if topic.Name == topicName {
			return true, nil
		}
	}
	return false, nil
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
	for page != nil && page.NextPageToken != "" {
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
