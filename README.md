# Ensign Go SDK

[![Go Reference](https://pkg.go.dev/badge/github.com/rotationalio/go-ensign.svg)](https://pkg.go.dev/github.com/rotationalio/go-ensign)
[![Go Report Card](https://goreportcard.com/badge/github.com/rotationalio/go-ensign)](https://goreportcard.com/report/github.com/rotationalio/go-ensign)
[![CI](https://github.com/rotationalio/go-ensign/actions/workflows/test.yaml/badge.svg)](https://github.com/rotationalio/go-ensign/actions/workflows/test.yaml)

Welcome to go-ensign!

This repository contains the Ensign driver, SDK, and helpers for Go. For the main ensign repo, go [here](https://github.com/rotationalio/ensign). We also have SDKs for [Javascript](https://github.com/rotationalio/ensignjs) and [Python](https://github.com/rotationalio/pyensign).

The getting started guide and general documentation can be found at [https://ensign.rotational.dev](https://ensign.rotational.dev). You may also want to reference the [GoDoc Package Documentation](https://pkg.go.dev/github.com/rotationalio/go-ensign) as well.

## Creating an Ensign Client

To add the Go SDK as a dependency, either `go get` it or import it and run `go mod tidy`:

```
$ go get github.com/rotationalio/go-ensign
```

The Go SDK provides a client that is able to connect to an Ensign system in order to manage topics, publish events, and subscribe to an event stream. At a minimum, you need API credentials, which can be obtained by creating an account at [https://rotational.app](https://rotational.app). Once you've created an account and downloaded your credentials, you can instantiate a new Ensign client and check to make sure you're connected:

```go
package main

import (
	"context"
	"fmt"
	"log"

	"github.com/rotationalio/go-ensign"
)

func main() {
	client, err := ensign.New(ensign.WithCredentials("CLIENT ID", "CLIENT SECRET"))
	if err != nil {
		log.Fatal(err)
	}

	status, err := client.Status(context.Background())
	if err != nil {
		log.Fatal(err)
	}

	fmt.Printf("%+v\n", status)
}
```

You can also set the `$ENSIGN_CLIENT_ID` and `$ENSIGN_CLIENT_SECRET` environment
variables so that you can instantiate the Client without specifying credentials in code.

```go
// Assumes that $ENSIGN_CLIENT_ID and $ENSIGN_CLIENT_SECRET are set
client, err := ensign.New()
```

Finally, if you downloaded the `client.json` file from the app; you can load it by specifying the path to the JSON file:


```go
client, err := ensign.New(ensign.WithLoadCredentials("path/to/client.json"))
```

## Topic Management

Every topic that you work with is like a database table or collection of tables -- it is where your event data lives. Naturally, this is generally a starting place to interacting with Ensign. While you can create and manage topics from [Beacon](https://rotational.app) -- our Ensign UI -- you can also create and manage topics with the SDK.

Generally speaking, at the beginning of each Ensign program, you'll want to check if a topic exists and create it if it doesn't. This can be done with the following code:

```go
const TopicName = "my-awesome-topic"

var client *ensign.Client

func checkTopic() (err error) {
	ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
	defer cancel()

	var exists bool
	if exists, err = client.TopicExists(ctx, TopicName); err != nil {
		return err
	}

	if !exists {
		var topicID string
		if topicID, err = client.CreateTopic(ctx, TopicName); err != nil {
			return err
		}
		log.Printf("created topic %s with ID %s\n", TopicName, topicID)
	}
	return nil
}
```

You can also list all the topics in your current project as follows:

```go
func printAllTopics() error {
	ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
	defer cancel()

	topics, err := client.ListTopics(ctx)
	if err != nil {
		return err
	}

	for _, topic := range topics {
		fmt.Printf("%s: %s\n", topic.Id, topic.Name)
	}
	return nil
}
```

### Topic Cache

If you're going to be working with a lot of topics, you can use the `topics.Cache` to simplify topic management. Create the cache as follows:

```go

import (
	"github.com/rotationalio/ensign"
	ensignTopics "github.com/rotationalio/ensign/topics"
)

var (
	client *ensign.Client
	topics *ensignTopics.Cache
)

func connect() (err error) {
	if client, err = ensign.New(); err != nil {
		return err
	}

	topics = ensignTopics.NewCache(client)
	return nil
}
```

With the topics cache in place you can simplify the "check if topic exists and create if it doesn't" code by using the `Ensure()` method:

```go
const TopicName = "my-awesome-topic"

func checkTopic() (err error) {
	if _, err = topics.Ensure(TopicName); err != nil {
		return err
	}
	return nil
}
```

The cache also prevents repeated calls to Ensign, so you can use the `Exists` and `Get` method instead of `client.TopicExists` and `client.TopicID`.

### Quick Topics API Reference

- [`client.TopicExists`](https://pkg.go.dev/github.com/rotationalio/go-ensign#Client.TopicExists): check if a topic with the specified name exists in the project.
- [`client.CreateTopic`](https://pkg.go.dev/github.com/rotationalio/go-ensign#Client.CreateTopic): create a topic with the specified name in the project.
- [`client.ListTopics`](https://pkg.go.dev/github.com/rotationalio/go-ensign#Client.ListTopics): list all the topics in the project.
- [`client.TopicID`](https://pkg.go.dev/github.com/rotationalio/go-ensign#Client.TopicID): get the topic ID from a topic name.
- [`cache.Get`](https://pkg.go.dev/github.com/rotationalio/go-ensign/topics#Cache.Get): get a topic ID from a topic name.
- [`cache.Exists`](https://pkg.go.dev/github.com/rotationalio/go-ensign/topics#Cache.Exists): check if the specified topic exists.
- [`cache.Ensure`](https://pkg.go.dev/github.com/rotationalio/go-ensign/topics#Cache.Ensure): a helper for "create topic if it doesn't exist".
- [`cache.Clear`](https://pkg.go.dev/github.com/rotationalio/go-ensign/topics#Cache.Clear): empty the internal cache to fetch data from ensign again.
- [`cache.Length`](https://pkg.go.dev/github.com/rotationalio/go-ensign/topics#Cache.Length): returns the number of items in the topic cache.