package ensign

import (
	"context"
	"fmt"

	"github.com/oklog/ulid/v2"
	api "github.com/rotationalio/go-ensign/api/v1beta1"
)

// Info returns summary statistics that describe the state of the project that you can
// connect to with your API key. Statistics include the number of topics, the number
// topics that are readonly (a subset of the number of topics) and the number of events.
// The project for the statistics is determined by the project your API key has access
// to (API keys are issued to projects). You can also specify a list of topicIDs to get
// the statistics for (e.g. filtering the statistics for one or more topics).
//
// TODO: allow users to specify either topic names or topic IDs.
func (c *Client) Info(ctx context.Context, topicIDs ...string) (info *api.ProjectInfo, err error) {
	req := &api.InfoRequest{
		Topics: make([][]byte, 0, len(topicIDs)),
	}

	for _, topicID := range topicIDs {
		var tid ulid.ULID
		if tid, err = ulid.Parse(topicID); err != nil {
			return nil, fmt.Errorf("could not parse %q as a topic id", topicID)
		}
		req.Topics = append(req.Topics, tid.Bytes())
	}

	if info, err = c.api.Info(ctx, req, c.copts...); err != nil {
		// TODO: do a better job of categorizing the error
		return nil, err
	}
	return info, nil
}
