# Ensign Go SDK

[![Go Reference](https://pkg.go.dev/badge/github.com/rotationalio/go-ensign.svg)](https://pkg.go.dev/github.com/rotationalio/go-ensign)
[![Go Report Card](https://goreportcard.com/badge/github.com/rotationalio/go-ensign)](https://goreportcard.com/report/github.com/rotationalio/go-ensign)
[![CI](https://github.com/rotationalio/go-ensign/actions/workflows/test.yaml/badge.svg)](https://github.com/rotationalio/go-ensign/actions/workflows/test.yaml)

Welcome to go-ensign!

This repository contains the Ensign driver, SDK, and helpers for Go. For the main ensign repo, go [here](https://github.com/rotationalio/ensign). We also have SDKs for [Javascript](https://github.com/rotationalio/ensignjs) and [Python](https://github.com/rotationalio/pyensign).

The getting started guide and general documentation can be found at [https://ensign.rotational.dev](https://ensign.rotational.dev). You may also want to reference the [GoDoc Package Documentation](https://pkg.go.dev/github.com/rotationalio/go-ensign) as well.

## Quickstart

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