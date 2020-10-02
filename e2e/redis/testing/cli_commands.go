/*
Copyright AppsCode Inc. and Contributors

Licensed under the AppsCode Community License 1.0.0 (the "License");
you may not use this file except in compliance with the License.
You may obtain a copy of the License at

    https://github.com/appscode/licenses/raw/1.0.0/AppsCode-Community-1.0.0.md

Unless required by applicable law or agreed to in writing, software
distributed under the License is distributed on an "AS IS" BASIS,
WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
See the License for the specific language governing permissions and
limitations under the License.
*/

package testing

import (
	"fmt"
	"strings"

	api "kubedb.dev/apimachinery/apis/kubedb/v1alpha1"
)

var (
	tlsArgs = []string{
		"--tls",
		"--cert",
		"/certs/client.crt",
		"--key",
		"/certs/client.key",
		"--cacert",
		"/certs/ca.crt",
	}
)

func splitOff(input *string, delim string) {
	if parts := strings.SplitN(*input, delim, 2); len(parts) == 2 {
		*input = parts[0]
	}
}

func (testConfig *TestConfig) cmdGetRedisCLI(redisMode api.RedisMode) ([]string, error) {
	var command = []string{"redis-cli"}
	if testConfig.WithTLS {
		command = append(command, tlsArgs...)
	}

	if redisMode == api.RedisModeCluster {
		command = append(command, "-c")
	}
	return command, nil
}

// ping redis node
func (testConfig *TestConfig) cmdPing(redisMode api.RedisMode) ([]string, error) {
	command, err := testConfig.cmdGetRedisCLI(redisMode)
	if err != nil {
		return nil, err
	}
	command = append(command, "PING")
	return command, nil
}

// set item in redis db
func (testConfig *TestConfig) cmdSetItem(redisMode api.RedisMode, key string, value string) ([]string, error) {
	command, err := testConfig.cmdGetRedisCLI(redisMode)
	if err != nil {
		return nil, err
	}
	command = append(command, "SET", key, value)
	return command, nil
}

// get item in redis db
func (testConfig *TestConfig) cmdGetItem(redisMode api.RedisMode, key string) ([]string, error) {
	command, err := testConfig.cmdGetRedisCLI(redisMode)
	if err != nil {
		return nil, err
	}
	command = append(command, "GET", key)
	return command, nil
}

// get item in redis db
func (testConfig *TestConfig) cmdDeleteItem(redisMode api.RedisMode, key string) ([]string, error) {
	command, err := testConfig.cmdGetRedisCLI(redisMode)
	if err != nil {
		return nil, err
	}
	command = append(command, "DEL", key)
	return command, nil
}

// get item in redis db
func (testConfig *TestConfig) cmdRandomKey(redisMode api.RedisMode) ([]string, error) {
	command, err := testConfig.cmdGetRedisCLI(redisMode)
	if err != nil {
		return nil, err
	}
	command = append(command, "RANDOMKEY")
	return command, nil
}

// get dbSize  in a individual redis node
func (testConfig *TestConfig) cmdGetDBSize() ([]string, error) {
	command, err := testConfig.cmdGetRedisCLI(api.RedisModeStandalone)
	if err != nil {
		return nil, err
	}
	command = append(command, "DBSIZE")
	return command, nil
}

// get dbSize  in a individual redis node
func (testConfig *TestConfig) cmdConfigGet(param string) ([]string, error) {
	command, err := testConfig.cmdGetRedisCLI(api.RedisModeStandalone)
	if err != nil {
		return nil, err
	}
	command = append(command, "config", "get", param)
	return command, nil
}

// flash the individual redis node
func (testConfig *TestConfig) cmdFlushDB() ([]string, error) {
	//flushing a node doesn't require to be done with cluster flag ( -c )
	//for this reason always passing redis standalone mode inside cmdGetRedisCli() func
	command, err := testConfig.cmdGetRedisCLI(api.RedisModeStandalone)
	if err != nil {
		return nil, err
	}
	command = append(command, "flushDB")
	return command, nil
}

// redis cluster info command
func (testConfig *TestConfig) cmdClusterInfo() ([]string, error) {
	command, err := testConfig.cmdGetRedisCLI(api.RedisModeCluster)
	if err != nil {
		return nil, err
	}
	command = append(command, "CLUSTER", "INFO")
	return command, nil
}

func (testConfig *TestConfig) cmdClusterNodes() ([]string, error) {
	command, err := testConfig.cmdGetRedisCLI(api.RedisModeCluster)
	if err != nil {
		return nil, err
	}
	command = append(command, "CLUSTER", "NODES")
	return command, nil
}
func (testConfig *TestConfig) cmdClusterSaveConfig() ([]string, error) {
	command, err := testConfig.cmdGetRedisCLI(api.RedisModeCluster)
	if err != nil {
		return nil, err
	}
	command = append(command, "CLUSTER", "SAVECONFIG")
	return command, nil
}
func (testConfig *TestConfig) cmdClusterCountKeysInSlot(slot int) ([]string, error) {
	command, err := testConfig.cmdGetRedisCLI(api.RedisModeCluster)
	if err != nil {
		return nil, err
	}
	command = append(command, "CLUSTER", "COUNTKEYSINSLOT", fmt.Sprintf("%d", slot))
	return command, nil
}

//ClusterCountFailureReports

func (testConfig *TestConfig) cmdClusterCountFailureReports(nodeId string) ([]string, error) {
	command, err := testConfig.cmdGetRedisCLI(api.RedisModeCluster)
	if err != nil {
		return nil, err
	}
	command = append(command, "CLUSTER", "COUNT-failure-reports", nodeId)
	return command, nil
}

func (testConfig *TestConfig) cmdClusterSlaves(nodeId string) ([]string, error) {
	command, err := testConfig.cmdGetRedisCLI(api.RedisModeCluster)
	if err != nil {
		return nil, err
	}
	command = append(command, "CLUSTER", "SLAVES", nodeId)
	return command, nil
}
func (testConfig *TestConfig) cmdClusterFailOver() ([]string, error) {
	command, err := testConfig.cmdGetRedisCLI(api.RedisModeCluster)
	if err != nil {
		return nil, err
	}
	command = append(command, "CLUSTER", "FAILOVER")
	return command, nil
}

func (testConfig *TestConfig) cmdClusterSlots() ([]string, error) {
	command, err := testConfig.cmdGetRedisCLI(api.RedisModeCluster)
	if err != nil {
		return nil, err
	}
	command = append(command, "CLUSTER", "SLOTS")
	return command, nil
}
