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
	"github.com/cloudwego/kitex/pkg/klog"
	"github.com/cloudwego/kitex/pkg/retry"
	"github.com/kitex-contrib/config-apollo/apollo"
	"github.com/kitex-contrib/config-apollo/utils"
)

func WithRetryPolicy(dest, src string, apolloClient apollo.Client,
	opts utils.Options,
) []client.Option {
	param, err := apolloClient.ClientConfigParam(&apollo.ConfigParamConfig{
		Category:          apollo.RetryConfigName,
		ServerServiceName: dest,
		ClientServiceName: src,
	})
	if err != nil {
		panic(err)
	}

	for _, f := range opts.ApolloCustomFunctions {
		f(&param)
	}

	rc := initRetryContainer(param, dest, apolloClient)
	return []client.Option{
		client.WithRetryContainer(rc),
		client.WithCloseCallbacks(rc.Close),
		client.WithCloseCallbacks(func() error {
			// cancel the configuration listener when client is closed.
			return apolloClient.DeregisterConfig()
		}),
	}
}

func initRetryContainer(param apollo.ConfigParam, dest string,
	apolloClient apollo.Client,
) *retry.Container {
	retryContainer := retry.NewRetryContainerWithPercentageLimit()

	ts := utils.ThreadSafeSet{}

	onChangeCallback := func(data string, parser apollo.ConfigParser) {
		// the key is method name, wildcard "*" can match anything.
		rcs := map[string]*retry.Policy{}
		err := parser.Decode(param.Type, data, &rcs)
		if err != nil {
			klog.Warnf("[apollo] %s client apollo retry: unmarshal data %s failed: %s, skip...", dest, data, err)
			return
		}

		set := utils.Set{}
		for method, policy := range rcs {
			set[method] = true
			if policy.BackupPolicy != nil && policy.FailurePolicy != nil {
				klog.Warnf("[apollo] %s client policy for method %s BackupPolicy and FailurePolicy must not be set at same time",
					dest, method)
				continue
			}
			if policy.BackupPolicy == nil && policy.FailurePolicy == nil {
				klog.Warnf("[apollo] %s client policy for method %s BackupPolicy and FailurePolicy must not be empty at same time",
					dest, method)
				continue
			}
			retryContainer.NotifyPolicyChange(method, *policy)
		}

		for _, method := range ts.DiffAndEmplace(set) {
			retryContainer.DeletePolicy(method)
		}
	}

	apolloClient.RegisterConfigCallback(param, onChangeCallback)

	return retryContainer
}
