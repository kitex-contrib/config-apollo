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

package apollo

import (
	"bytes"
	"sync"
	"text/template"

	"github.com/apolloconfig/agollo/v4/component/log"
	"github.com/cloudwego/kitex/pkg/klog"
	"github.com/shima-park/agollo"
)

// Client the wrapper of apollo client.
type Client interface {
	SetParser(ConfigParser)
	ClientConfigParam(cpc *ConfigParamConfig) (ConfigParam, error)
	ServerConfigParam(cpc *ConfigParamConfig) (ConfigParam, error)
	RegisterConfigCallback(ConfigParam, func(string, ConfigParser))
	DeregisterConfig() error
}

type ConfigParam struct {
	Key       string
	nameSpace string
	Cluster   string
	Type      ConfigType
}

type client struct {
	acli agollo.Agollo
	// support customise parser
	parser            ConfigParser
	stop              chan bool
	clusterTemplate   *template.Template
	serverKeyTemplate *template.Template
	clientKeyTemplate *template.Template
}

const (
	RetryConfigName          = "retry"
	RpcTimeoutConfigName     = "rpc_timeout"
	CircuitBreakerConfigName = "circuit_break"

	LimiterConfigName = "limit"
)

var Close sync.Once

type Options struct {
	ConfigServerURL string
	AppID           string
	Cluster         string
	ServerKeyFormat string
	ClientKeyFormat string
	ApolloOptions   []agollo.Option
	CustomLogger    log.LoggerInterface
	ConfigParser    ConfigParser
}

type OptionFunc func(option *Options)

func NewClient(opts Options, optsfunc ...OptionFunc) (Client, error) {
	if opts.ConfigServerURL == "" {
		opts.ConfigServerURL = ApolloDefaultConfigServerURL
	}
	if opts.CustomLogger == nil {
		opts.CustomLogger = NewCustomApolloLogger()
	}
	if opts.ConfigParser == nil {
		opts.ConfigParser = defaultConfigParse()
	}
	if opts.AppID == "" {
		opts.AppID = ApolloDefaultAppId
	}
	if opts.Cluster == "" {
		opts.Cluster = ApolloDefaultCluster
		opts.ApolloOptions = append(opts.ApolloOptions, agollo.Cluster(opts.Cluster))
	}
	if opts.ServerKeyFormat == "" {
		opts.ServerKeyFormat = ApolloDefaultServerKey
	}
	if opts.ClientKeyFormat == "" {
		opts.ClientKeyFormat = ApolloDefaultClientKey
	}
	opts.ApolloOptions = append(opts.ApolloOptions,
		agollo.AutoFetchOnCacheMiss(),
		agollo.FailTolerantOnBackupExists(),
	)
	for _, option := range optsfunc {
		option(&opts)
	}
	apolloCli, err := agollo.New(opts.ConfigServerURL, opts.AppID, opts.ApolloOptions...)
	if err != nil {
		return nil, err
	}
	clusterTemplate, err := template.New("cluster").Parse(opts.Cluster)
	if err != nil {
		return nil, err
	}
	serverKeyTemplate, err := template.New("serverKey").Parse(opts.ServerKeyFormat)
	if err != nil {
		return nil, err
	}
	clientKeyTemplate, err := template.New("clientKey").Parse(opts.ClientKeyFormat)
	if err != nil {
		return nil, err
	}
	cli := &client{
		acli:              apolloCli,
		parser:            opts.ConfigParser,
		stop:              make(chan bool),
		clusterTemplate:   clusterTemplate,
		serverKeyTemplate: serverKeyTemplate,
		clientKeyTemplate: clientKeyTemplate,
	}

	return cli, nil
}

func WithApolloOption(apolloOption ...agollo.Option) OptionFunc {
	return func(option *Options) {
		option.ApolloOptions = append(option.ApolloOptions, apolloOption...)
	}
}

func (c *client) SetParser(parser ConfigParser) {
	c.parser = parser
}

func (c *client) render(cpc *ConfigParamConfig, t *template.Template) (string, error) {
	var tpl bytes.Buffer
	err := t.Execute(&tpl, cpc)
	if err != nil {
		return "", err
	}
	return tpl.String(), nil
}

func (c *client) ServerConfigParam(cpc *ConfigParamConfig) (ConfigParam, error) {
	return c.configParam(cpc, c.serverKeyTemplate)
}

// ClientConfigParam render client config parameters
func (c *client) ClientConfigParam(cpc *ConfigParamConfig) (ConfigParam, error) {
	return c.configParam(cpc, c.clientKeyTemplate)
}

// configParam render config parameters. All the parameters can be customized with CustomFunction.
// ConfigParam explain:
//  1. Type: key format, support JSON and YAML, JSON by default. Could extend it by implementing the ConfigParser interface.
//  2. Content: empty by default. Customize with CustomFunction.
//  3. NameSpace: select by user.
//  4. ServerKey: {{.ServerServiceName}} by default.
//     ClientKey: {{.ClientServiceName}}.{{.ServerServiceName}} by default.
//  5. Cluster: default by default
func (c *client) configParam(cpc *ConfigParamConfig, t *template.Template) (ConfigParam, error) {
	param := ConfigParam{
		Type:      JSON,
		nameSpace: cpc.Category,
	}
	var err error
	param.Key, err = c.render(cpc, t)
	if err != nil {
		return param, err
	}
	param.Cluster, err = c.render(cpc, c.clusterTemplate)
	if err != nil {
		return param, err
	}
	return param, nil
}

// DeregisterConfig deregister the config.
func (c *client) DeregisterConfig() error {
	Close.Do(func() {
		// close listen goroutine
		close(c.stop)
	})
	// close longpoll
	c.acli.Stop()
	return nil
}

// RegisterConfigCallback register the callback function to apollo client.
func (c *client) RegisterConfigCallback(param ConfigParam,
	callback func(string, ConfigParser),
) {
	onChange := func(namespace, cluster, key, data string) {
		klog.Debugf("[apollo] config %s updated, namespace %s cluster %s key %s data %s",
			namespace, namespace, cluster, key, data)
		callback(data, c.parser)
	}

	configMap := c.acli.GetNameSpace(param.nameSpace)
	data, ok := configMap[param.Key]
	if !ok {
		klog.Info("key:", param.Key)
		klog.Info("CONFIG:", configMap)
		klog.Warn("[apollo] key not found")
	} else {
		callback(data.(string), c.parser)
	}

	go c.listenConfig(param, c.stop, onChange)
}

func (c *client) listenConfig(param ConfigParam, stop chan bool, callback func(namespace, cluster, key, data string)) {
	defer func() {
		if err := recover(); err != nil {
			klog.Error("[apollo] listen goroutine error:", err)
		}
	}()
	errorsCh := c.acli.Start()
	apolloRespCh := c.acli.WatchNamespace(param.nameSpace, stop)

	for {
		select {
		case resp := <-apolloRespCh:
			klog.Info("[apollo] config update")
			data, ok := resp.NewValue[param.Key]
			if !ok {
				// Deal with delete config
				klog.Warnf("[apollo] config %s error, namespace %s cluster %s key %s : error : key not found",
					param.nameSpace, param.nameSpace, param.Cluster, param.Key)
				klog.Warn("[apollo] please recover key from remote config")
				callback(param.nameSpace, param.Cluster, param.Key, emptyConfig)
				continue
			}
			callback(param.nameSpace, param.Cluster, param.Key, data.(string))
		case err := <-errorsCh:
			klog.Errorf("[apollo] config %s error, namespace %s cluster %s key %s : error %s",
				param.nameSpace, param.nameSpace, param.Cluster, param.Key, err.Err.Error())
			return
		case <-stop:
			klog.Warnf("[apollo] config %s exit,namespace %s cluster %s key %s : exit",
				param.nameSpace, param.nameSpace, param.Cluster, param.Key)
			return
		}
	}
}
