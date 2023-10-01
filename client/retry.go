package client

import (
	"github.com/cloudwego/kitex/client"
	"github.com/cloudwego/kitex/pkg/klog"
	"github.com/cloudwego/kitex/pkg/retry"
	"github.com/kitex-contrib/config-apollo/apollo"
	"github.com/kitex-contrib/config-apollo/utils"
)

func WithRetryPolicy(dest, src string, apolloClient apollo.Client,
	cfs ...apollo.CustomFunction,
) []client.Option {
	param, err := apolloClient.ClientConfigParam(&apollo.ConfigParamConfig{
		Category:          apollo.RetryConfigName,
		ServerServiceName: dest,
		ClientServiceName: src,
	}, cfs...)
	if err != nil {
		panic(err)
	}

	return []client.Option{
		client.WithRetryContainer(initRetryContainer(param, dest, apolloClient)),
		client.WithCloseCallbacks(func() error {
			// cancel the configuration listener when client is closed.
			return apolloClient.DeregisterConfig(param)
		}),
	}
}

func initRetryContainer(param apollo.ConfigParam, dest string,
	apolloClient apollo.Client,
) *retry.Container {
	retryContainer := retry.NewRetryContainer()

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
