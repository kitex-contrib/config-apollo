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
	"errors"
	"text/template"

	"github.com/apolloconfig/agollo/v4/component/log"
	"github.com/cloudwego/kitex/pkg/klog"
	"github.com/shima-park/agollo"
)

// Client the wrapper of nacos client.
type Client interface {
	SetParser(ConfigParser)
	ClientConfigParam(cpc *ConfigParamConfig, cfs ...CustomFunction) (ConfigParam, error)
	ServerConfigParam(cpc *ConfigParamConfig, cfs ...CustomFunction) (ConfigParam, error)
	RegisterConfigCallback(ConfigParam, func(string, ConfigParser))
	DeregisterConfig(ConfigParam) error
}

type ConfigParam struct {
	Key       string
	NameSpace string
	Cluster   string
	Content   string
	Type      ConfigType
}

type client struct {
	acli agollo.Agollo
	// support customise parser
	parser            ConfigParser
	clusterTemplate   *template.Template
	namespaceTemplate *template.Template
	serverKeyTemplate *template.Template
	clientKeyTemplate *template.Template
}

const (
	RetryConfigName          = "retry"
	RpcTimeoutConfigName     = "rpc_timeout"
	CircuitBreakerConfigName = "circuit_break"
)

type Options struct {
	ConfigServerURL string
	NamespaceID     string
	AppID           string
	Cluster         string
	ServerKeyFormat string
	ClientKeyFormat string
	IsPrivate       bool
	AccessKey       string
	CustomLogger    log.LoggerInterface
	ConfigParser    ConfigParser
}

func New(opts Options) (Client, error) {
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
	// TODO
	if opts.NamespaceID == "" {
		opts.NamespaceID = ApolloNameSpace
	}
	if opts.Cluster == "" {
		opts.Cluster = ApolloDefaultCluster
	}
	if opts.ServerKeyFormat == "" {
		opts.ServerKeyFormat = ApolloDefaultServerKey
	}
	if opts.ClientKeyFormat == "" {
		opts.ClientKeyFormat = ApolloDefaultClientKey
	}
	agolloOption := []agollo.Option{
		agollo.Cluster(opts.Cluster),
		agollo.PreloadNamespaces([]string{RetryConfigName, RpcTimeoutConfigName, CircuitBreakerConfigName}...),
		agollo.AutoFetchOnCacheMiss(),
	}
	if opts.IsPrivate {
		if opts.AccessKey == "" {
			return nil, errors.New("[apollo] need accesskey for private namespace")
		}
		agolloOption = append(agolloOption, agollo.AccessKey(opts.AccessKey))
	}
	// 默认是properties格式的文件， 如果是json或者yml需要指定后缀，例如 namespace.json
	// agollo.PreloadNamespaces("namespace"), // 预加载命名空间----配置文件名
	// agollo.AccessKey("screct")} // 访问私有
	// 默认是properties格式的文件， 如果是json或者yml需要指定后缀，例如 namespace.json
	// agollo.PreloadNamespaces("namespace"...),// 预加载命名空间----配置文件名
	// agollo.AccessKey("screct")}// 访问私有

	apolloCli, err := agollo.New(opts.ConfigServerURL, opts.AppID, agolloOption...)
	if err != nil {
		return nil, err
	}
	// TODO
	clusterTemplate, err := template.New("cluster").Parse(opts.Cluster)
	if err != nil {
		return nil, err
	}
	namespaceTemplate, err := template.New("namespace").Parse(opts.NamespaceID)
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
		clusterTemplate:   clusterTemplate,
		namespaceTemplate: namespaceTemplate,
		serverKeyTemplate: serverKeyTemplate,
		clientKeyTemplate: clientKeyTemplate,
	}

	return cli, nil
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

func (c *client) ServerConfigParam(cpc *ConfigParamConfig, cfs ...CustomFunction) (ConfigParam, error) {
	return c.configParam(cpc, c.serverKeyTemplate, cfs...)
}

// ClientConfigParam render client config parameters
func (c *client) ClientConfigParam(cpc *ConfigParamConfig, cfs ...CustomFunction) (ConfigParam, error) {
	return c.configParam(cpc, c.clientKeyTemplate, cfs...)
}

// configParam render config parameters. All the parameters can be customized with CustomFunction.
// ConfigParam explain:
//  1. Type: key format, support JSON and YAML, JSON by default. Could extend it by implementing the ConfigParser interface.
//  2. Content: empty by default. Customize with CustomFunction.
//  3. NameSpace: {{.Category}} by default.
//  4. ServerKey: {{.ServerServiceName}} by default.
//     ClientKey: {{.ClientServiceName}}.{{.ServerServiceName}} by default.
//  5. Cluster: DEFAULT_CLUSTER by default
func (c *client) configParam(cpc *ConfigParamConfig, t *template.Template, cfs ...CustomFunction) (ConfigParam, error) {
	param := ConfigParam{
		Type:    JSON,
		Content: defaultContent,
	}
	var err error
	param.Key, err = c.render(cpc, t)
	if err != nil {
		return param, err
	}
	param.NameSpace, err = c.render(cpc, c.namespaceTemplate)
	if err != nil {
		return param, err
	}
	param.Cluster, err = c.render(cpc, c.clusterTemplate)
	if err != nil {
		return param, err
	}
	for _, cf := range cfs {
		cf(&param)
	}
	return param, nil
}

// DeregisterConfig deregister the config.
func (c *client) DeregisterConfig(cfg ConfigParam) error {
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

	configMap := c.acli.GetNameSpace(param.NameSpace)
	data, ok := configMap[param.Key]
	if !ok {
		klog.Info("key:", param.Key)
		klog.Info("CONFIG:", configMap)
		panic(errors.New("kitex config : key not found"))
	}
	// data := c.acli.Get(param.Key, agollo.WithNamespace(param.NameSpace))
	callback(data.(string), c.parser)

	go c.listenConfig(param, onChange)
}

func (c *client) listenConfig(param ConfigParam, callback func(namespace, cluster, key, data string)) {
	errorsCh := c.acli.Start()
	apolloRespCh := c.acli.WatchNamespace(param.NameSpace, make(chan bool))

	for {
		select {
		case resp := <-apolloRespCh:
			data, ok := resp.NewValue[param.Key]
			if !ok {
				klog.Errorf("[apollo] config %s error, namespace %s cluster %s key %s : error : key not found",
					param.NameSpace, param.NameSpace, param.Cluster, param.Key)
				klog.Error("[apollo] please recover key remote config")
				continue
			}
			klog.Info("[apollo] config update")
			callback(param.NameSpace, param.Cluster, param.Key, data.(string))
		case err := <-errorsCh:
			klog.Errorf("[apollo] config %s error, namespace %s cluster %s key %s : error %s",
				param.NameSpace, param.NameSpace, param.Cluster, param.Key, err.Err.Error())
			return
		}
	}
}
