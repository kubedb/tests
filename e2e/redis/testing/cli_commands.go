/*
Copyright AppsCode Inc. and Contributors

Licensed under the AppsCode Free Trial License 1.0.0 (the "License");
you may not use this file except in compliance with the License.
You may obtain a copy of the License at

    https://github.com/appscode/licenses/raw/1.0.0/AppsCode-Free-Trial-1.0.0.md

Unless required by applicable law or agreed to in writing, software
distributed under the License is distributed on an "AS IS" BASIS,
WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
See the License for the specific language governing permissions and
limitations under the License.
*/

package testing

import (
	"fmt"

	api "kubedb.dev/apimachinery/apis/kubedb/v1alpha2"
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

func (testConfig *TestConfig) cmdGetRedisCLI(redisMode api.RedisMode) []string {
	var command = []string{"redis-cli"}
	if testConfig.UseTLS {
		command = append(command, tlsArgs...)
	}

	if redisMode == api.RedisModeCluster {
		command = append(command, "-c")
	}
	return command
}

// ping redis node
func (testConfig *TestConfig) cmdPing(redisMode api.RedisMode) []string {
	command := testConfig.cmdGetRedisCLI(redisMode)
	command = append(command, "PING")
	return command
}

// set item in redis db
func (testConfig *TestConfig) cmdSetItem(redisMode api.RedisMode, key string, value string) []string {
	command := testConfig.cmdGetRedisCLI(redisMode)
	command = append(command, "SET", key, value)
	return command
}

// get item in redis db
func (testConfig *TestConfig) cmdGetItem(redisMode api.RedisMode, key string) []string {
	command := testConfig.cmdGetRedisCLI(redisMode)
	command = append(command, "GET", key)
	return command
}

// delete item in redis db
func (testConfig *TestConfig) cmdDeleteItem(redisMode api.RedisMode, key string) []string {
	command := testConfig.cmdGetRedisCLI(redisMode)
	command = append(command, "DEL", key)
	return command
}

// get random key in redis db
func (testConfig *TestConfig) cmdRandomKey(redisMode api.RedisMode) []string {
	command := testConfig.cmdGetRedisCLI(redisMode)
	command = append(command, "RANDOMKEY")
	return command
}

// get dbSize  in a individual redis node
func (testConfig *TestConfig) cmdGetDBSize() []string {
	command := testConfig.cmdGetRedisCLI(api.RedisModeStandalone)
	command = append(command, "DBSIZE")
	return command
}

// get config  in a individual redis node
func (testConfig *TestConfig) cmdConfigGet(param string) []string {
	command := testConfig.cmdGetRedisCLI(api.RedisModeStandalone)
	command = append(command, "config", "get", param)
	return command
}

// flush the individual redis node
func (testConfig *TestConfig) cmdFlushDB() []string {
	//flushing a node doesn't require to be done with cluster flag ( -c )
	//for this reason always passing redis standalone mode inside cmdGetRedisCli() func
	command := testConfig.cmdGetRedisCLI(api.RedisModeStandalone)
	command = append(command, "flushDB")
	return command
}

// redis cluster info command
func (testConfig *TestConfig) cmdClusterInfo() []string {
	command := testConfig.cmdGetRedisCLI(api.RedisModeCluster)
	command = append(command, "CLUSTER", "INFO")
	return command
}

func (testConfig *TestConfig) cmdClusterNodes() []string {
	command := testConfig.cmdGetRedisCLI(api.RedisModeCluster)
	command = append(command, "CLUSTER", "NODES")
	return command
}
func (testConfig *TestConfig) cmdClusterSaveConfig() []string {
	command := testConfig.cmdGetRedisCLI(api.RedisModeCluster)
	command = append(command, "CLUSTER", "SAVECONFIG")
	return command
}
func (testConfig *TestConfig) cmdClusterCountKeysInSlot(slot int) []string {
	command := testConfig.cmdGetRedisCLI(api.RedisModeCluster)
	command = append(command, "CLUSTER", "COUNTKEYSINSLOT", fmt.Sprintf("%d", slot))
	return command
}

//ClusterCountFailureReports
func (testConfig *TestConfig) cmdClusterCountFailureReports(nodeId string) []string {
	command := testConfig.cmdGetRedisCLI(api.RedisModeCluster)
	command = append(command, "CLUSTER", "COUNT-failure-reports", nodeId)
	return command
}

func (testConfig *TestConfig) cmdClusterSlaves(nodeId string) []string {
	command := testConfig.cmdGetRedisCLI(api.RedisModeCluster)
	command = append(command, "CLUSTER", "SLAVES", nodeId)
	return command
}

func (testConfig *TestConfig) cmdClusterFailOver() []string {
	command := testConfig.cmdGetRedisCLI(api.RedisModeCluster)
	command = append(command, "CLUSTER", "FAILOVER")
	return command
}

func (testConfig *TestConfig) cmdClusterSlots() []string {
	command := testConfig.cmdGetRedisCLI(api.RedisModeCluster)
	command = append(command, "CLUSTER", "SLOTS")
	return command
}
