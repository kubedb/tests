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

package framework

import "strings"

/*//go:generate enumer -type=TestProfile -json
type TestProfile int

const (
	TestGeneric TestProfile = 1 << iota
	TestStash
	TestInit
	TestExporter
	TestUpgrade
	TestHorizontalScaling
	TestVerticalScaling
	TestVolumeExpansion
	TestCustomConfig
	TestRotateCertificates
)*/

const (
	General             = "general"
	CustomConfig        = "custom_config"
	EnvironmentVariable = "env_variable"
	Exporter            = "exporter"
	Initialize          = "initialize"
	Resume              = "resume"
	StorageEngine       = "storage_engine"
	StorageType         = "storage_type"
	TerminationPolicy   = "termination_policy"

	Upgrade           = "upgrade"
	Scale             = "scale"
	VerticalScaling   = "vertical_scaling"
	HorizontalScaling = "horizontal_scaling"
	Reconfigure       = "reconfigure"
	VolumeExpansion   = "volume_expansion"

	RedisHorizontalScaling   = "redis_horizontal"
	RedisVerticalScaling     = "redis_vertical"
	RedisUpgrade             = "redis_upgrading"
	RedisGeneral             = "redis_general"
	RedisResume              = "redis_resume"
	RedisStorageType         = "redis_storage_type"
	RedisTerminationPolicy   = "redis_termination_policy"
	RedisEnvironmentVariable = "redis_env_variable"
	RedisCustomConfig        = "redis_custom_config"
	RedisCluster             = "redis_cluster"
	RedisAll                 = "redis_all"
	RedisEnterprise          = "redis_enterprise"
	RedisCommunity           = "redis_community"
	RedisVolumeExpansion     = "redis_volume_expansion"

	All        = "all"
	Community  = "community"
	Enterprise = "enterprise"
)

type stringSlice []string

func (stringSlice *stringSlice) String() string {
	return strings.Join(*stringSlice, ",")
}

func (stringSlice *stringSlice) Set(value string) error {
	s := strings.Split(value, ",")
	*stringSlice = append(*stringSlice, s...)
	return nil
}
