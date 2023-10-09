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
	"strings"

	"github.com/cloudwego/kitex/client"
	"github.com/cloudwego/kitex/pkg/circuitbreak"
	"github.com/cloudwego/kitex/pkg/klog"
	"github.com/cloudwego/kitex/pkg/rpcinfo"

	"github.com/kitex-contrib/config-apollo/apollo"
	"github.com/kitex-contrib/config-apollo/utils"
)

// WithCircuitBreaker sets the circuit breaker policy from apollo configuration center.
func WithCircuitBreaker(dest, src string, apolloClient apollo.Client,
	opts utils.Options,
) []client.Option {
	param, err := apolloClient.ClientConfigParam(&apollo.ConfigParamConfig{
		Category:          apollo.CircuitBreakerConfigName,
		ServerServiceName: dest,
		ClientServiceName: src,
	})
	if err != nil {
		panic(err)
	}

	for _, f := range opts.ApolloCustomFunctions {
		f(&param)
	}

	cbSuite := initCircuitBreaker(param, dest, src, apolloClient)

	return []client.Option{
		client.WithCircuitBreaker(cbSuite),
		client.WithCloseCallbacks(func() error {
			err := cbSuite.Close()
			if err != nil {
				return err
			}
			// cancel the configuration listener when client is closed.
			return apolloClient.DeregisterConfig(param)
		}),
	}
}

// keep consistent when initialising the circuit breaker suit and updating
// the circuit breaker policy.
func genServiceCBKeyWithRPCInfo(ri rpcinfo.RPCInfo) string {
	if ri == nil {
		return ""
	}
	return genServiceCBKey(ri.To().ServiceName(), ri.To().Method())
}

func genServiceCBKey(toService, method string) string {
	sum := len(toService) + len(method) + 2
	var buf strings.Builder
	buf.Grow(sum)
	buf.WriteString(toService)
	buf.WriteByte('/')
	buf.WriteString(method)
	return buf.String()
}

func initCircuitBreaker(param apollo.ConfigParam, dest, src string,
	apolloClient apollo.Client,
) *circuitbreak.CBSuite {
	cb := circuitbreak.NewCBSuite(genServiceCBKeyWithRPCInfo)
	lcb := utils.ThreadSafeSet{}

	onChangeCallback := func(data string, parser apollo.ConfigParser) {
		set := utils.Set{}
		configs := map[string]circuitbreak.CBConfig{}
		err := parser.Decode(param.Type, data, &configs)
		if err != nil {
			klog.Warnf("[apollo] %s client apollo rpc timeout: unmarshal data %s failed: %s, skip...", dest, data, err)
			return
		}

		for method, config := range configs {
			set[method] = true
			key := genServiceCBKey(dest, method)
			cb.UpdateServiceCBConfig(key, config)
		}

		for _, method := range lcb.DiffAndEmplace(set) {
			key := genServiceCBKey(dest, method)
			// For deleted method configs, set to default policy
			cb.UpdateServiceCBConfig(key, circuitbreak.GetDefaultCBConfig())
		}
	}

	apolloClient.RegisterConfigCallback(param, onChangeCallback)

	return cb
}
