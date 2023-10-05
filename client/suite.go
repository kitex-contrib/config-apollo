// Copyright 2023 CloudWeGo Authors
//
// Licensed under the Apache License, Version 2.0 (the "License");
// you may not use this file except in compliance with the License.
// You may obtain a copy of the License at
//
//     http://www.apache.org/licenses/LICENSE-2.0
//
// Unless required by applicable law or agreed to in writing, software
// distributed under the License is distributed on an "AS IS" BASIS,
// WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
// See the License for the specific language governing permissions and
// limitations under the License.

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

type ClientSuiteOption func(*ApolloClientSuite)

func NewSuite(service, client string, cli apollo.Client,
	options ...ClientSuiteOption,
) *ApolloClientSuite {
	client_suite := &ApolloClientSuite{
		service:      service,
		client:       client,
		apolloClient: cli,
	}
	for _, option := range options {
		option(client_suite)
	}
	return client_suite
}

func (s *ApolloClientSuite) Options() []client.Option {
	opts := make([]client.Option, 0, 7)
	opts = append(opts, WithRetryPolicy(s.service, s.client, s.apolloClient, s.fns...)...)
	opts = append(opts, WithRPCTimeout(s.service, s.client, s.apolloClient, s.fns...)...)
	opts = append(opts, WithCircuitBreaker(s.service, s.client, s.apolloClient, s.fns...)...)
	return opts
}

func WithApolloClient(cli apollo.Client) ClientSuiteOption {
	return func(s *ApolloClientSuite) {
		s.apolloClient = cli
	}
}
func WithClientName(client string) ClientSuiteOption {
	return func(s *ApolloClientSuite) {
		s.client = client
	}
}
func WithServiceName(service string) ClientSuiteOption {
	return func(s *ApolloClientSuite) {
		s.service = service
	}
}
func WithConfigParamFunc(fns ...apollo.CustomFunction) ClientSuiteOption {
	return func(s *ApolloClientSuite) {
		s.fns = append(s.fns, fns...)
	}
}
