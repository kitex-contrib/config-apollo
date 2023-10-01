package client

import (
	"github.com/cloudwego/kitex/client"
	"github.com/kitex-contrib/config-apollo/apollo"
)

type ApolloClientSuite struct {
	apolloClient apollo.Client
	service      string
	client       string
	fns          []apollo.CustomFunction
}

func NewSuite(service, client string, cli apollo.Client,
	cfs ...apollo.CustomFunction,
) *ApolloClientSuite {
	return &ApolloClientSuite{
		service:      service,
		client:       client,
		apolloClient: cli,
		fns:          cfs,
	}
}
func (s *ApolloClientSuite) Options() []client.Option {
	opts := make([]client.Option, 0, 7)
	opts = append(opts, WithRetryPolicy(s.service, s.client, s.apolloClient, s.fns...)...)
	opts = append(opts, WithRPCTimeout(s.service, s.client, s.apolloClient, s.fns...)...)
	opts = append(opts, WithCircuitBreaker(s.service, s.client, s.apolloClient, s.fns...)...)
	return opts
}
