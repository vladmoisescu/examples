// Copyright 2019 Cisco Systems, Inc.
// SPDX-License-Identifier: Apache-2.0
//
// Licensed under the Apache License, Version 2.0 (the "License");
// you may not use this file except in compliance with the License.
// You may obtain a copy of the License at:
//
//     http://www.apache.org/licenses/LICENSE-2.0
//
// Unless required by applicable law or agreed to in writing, software
// distributed under the License is distributed on an "AS IS" BASIS,
// WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
// See the License for the specific language governing permissions and
// limitations under the License.

package main

import (
	"fmt"
	"github.com/networkservicemesh/examples/examples/universal-cnf/vppagent/cmd/config"
	"github.com/networkservicemesh/networkservicemesh/sdk/common"
	"github.com/networkservicemesh/networkservicemesh/sdk/endpoint"
	"github.com/sirupsen/logrus"
	"plugin"
)
/*
type CompositeEndpointPluginConf struct {
	pluginModuleFile   string
	plugin config.CompositeEndpointAddons
}

 */

type defaultCompositeEndpointAddon string

func (dcea defaultCompositeEndpointAddon) AddCompositeEndpoints(nsConfig *common.NSConfiguration) *[]endpoint.ChainedEndpoint {
	return nil
}

func GetPluginCompositeEndpoints(pluginModule string) (config.CompositeEndpointAddons, error) {
	if pluginModule == "" {
		var defCEAddon defaultCompositeEndpointAddon
		return defCEAddon, nil
	}
	// load module
	// 1. open the so file to load the symbols
	plug, err := plugin.Open(pluginModule)
	if err != nil {
		logrus.Errorf("Unable to find composite endpoint plugin '%s'", pluginModule)
		return nil, err
	}
	// 2. look up a symbol (an exported function or variable)
	// in this case, variable Greeter
	symCompositeEndpointPlugin, err := plug.Lookup("CompositeEndpointPlugin")
	if err != nil {
		logrus.Errorf("Unable to find plugin symbol '%s'", "CompositeEndpointPlugin")
		return nil, err
	}
	// 3. Assert that loaded symbol is of a desired type
	// in this case interface type Greeter (defined above)
	var compositeEndpointPlugin config.CompositeEndpointAddons
	compositeEndpointPlugin, ok := symCompositeEndpointPlugin.(config.CompositeEndpointAddons)
	if !ok {
		logrus.Errorf("Symbol '%s' doesn't implement the CompositeEndpointPlugin interface", "CompositeEndpointPlugin")
		return nil, fmt.Errorf("Symbol '%s' doesn't implement the CompositeEndpointPlugin interface", "CompositeEndpointPlugin")
	}

	//chainedEndpoints := compositeEndpointPlugin.AddCompositeEndpoints(nsConfig)
	return compositeEndpointPlugin, nil
}

/*
func (plugConfig *CompositeEndpointPluginConf) AddCompositeEndpoints(nsConfig *common.NSConfiguration) *[]endpoint.ChainedEndpoint {
	if plugConfig.plugin != nil {
		return plugConfig.plugin
	}
}

 */