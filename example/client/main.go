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
//

package main

import (
	"context"
	"log"
	"time"

	"github.com/cloudwego/kitex-examples/kitex_gen/api"
	"github.com/cloudwego/kitex-examples/kitex_gen/api/echo"
	"github.com/cloudwego/kitex/client"
	"github.com/cloudwego/kitex/pkg/klog"
	"github.com/kitex-contrib/config-apollo/apollo"
	apolloclient "github.com/kitex-contrib/config-apollo/client"
)

func main() {
	klog.SetLevel(klog.LevelDebug)

	apolloClient, err := apollo.NewClient(apollo.Options{})
	if err != nil {
		panic(err)
	}

	fn := func(cp *apollo.ConfigParam) {
		klog.Infof("apollo config %v", cp)
		klog.Infof("apollo namespace: %v", cp.NameSpace)
		klog.Infof("apollo key: %v", cp.Key)
		klog.Infof("apollo cluster: %v", cp.Cluster)
	}

	serviceName := "ServiceName"
	clientName := "ClientName"
	client, err := echo.NewClient(
		serviceName,
		client.WithHostPorts("0.0.0.0:8899"),
		client.WithSuite(apolloclient.NewSuite(serviceName, clientName, apolloClient, fn)),
	)
	if err != nil {
		log.Fatal(err)
	}
	for {
		req := &api.Request{Message: "my request"}
		resp, err := client.Echo(context.Background(), req)
		if err != nil {
			klog.Errorf("take request error: %v", err)
		} else {
			klog.Infof("receive response %v", resp)
		}
		time.Sleep(time.Second * 10)
	}
}
