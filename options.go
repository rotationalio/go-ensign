package sdk

import "github.com/kelseyhightower/envconfig"

// Options allows users to configure their connection to ensign.
type Options struct {
	Endpoint         string `default:"ensign.rotational.app:443"`
	ClientID         string `split_words:"true"`
	ClientSecret     string `split_words:"true"`
	Insecure         bool   `default:"false"`
	AuthURL          string `default:"https://auth.rotational.app"`
	NoAuthentication bool   `split_words:"true" default:"false"`
}

func NewOptions() (opts *Options) {
	opts = &Options{}
	if err := envconfig.Process("ensign", opts); err != nil {
		// TODO: instead of panic log the error and allow the user to control logging.
		panic(err)
	}
	return opts
}

func (o *Options) Validate() (err error) {
	o.SetDefaults()
	if o.Endpoint == "" {
		return ErrMissingEndpoint
	}

	if !o.NoAuthentication {
		if o.ClientID == "" {
			return ErrMissingClientID
		}

		if o.ClientSecret == "" {
			return ErrMissingClientSecret
		}

		if o.AuthURL == "" {
			return ErrMissingAuthURL
		}
	}

	return nil
}

func (o *Options) SetDefaults() {
	// TODO: use reflection do this more effectively.
	if o.Endpoint == "" {
		o.Endpoint = "ensign.rotational.app:443"
	}

	if o.AuthURL == "" {
		o.AuthURL = "https://auth.rotational.app"
	}
}
