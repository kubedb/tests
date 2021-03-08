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

import (
	"fmt"
	"strings"

	. "github.com/onsi/ginkgo"
	"gomodules.xyz/x/arrays"
)

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
	RedisCluster        = "redis_cluster"

	Upgrade           = "upgrade"
	Scale             = "scale"
	VerticalScaling   = "vertical_scaling"
	HorizontalScaling = "horizontal_scaling"
	Reconfigure       = "reconfigure"
	VolumeExpansion   = "volume_expansion"
	ReconfigureTLS    = "reconfigure_tls"
	Autoscaling       = "autoscaling"

	All         = "all"
	Community   = "community"
	Enterprise  = "enterprise"
	StashBackup = "stash_backup"
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

func CoveredByTestProfiles(testType string) bool {
	runningThisProfile, _ := arrays.Contains(TestProfiles, testType)
	runningAllProfiles, _ := arrays.Contains(TestProfiles, All)
	return runningThisProfile || runningAllProfiles
}

func RunTestCommunity(testProfile string) bool {
	return strings.Contains(TestProfiles.String(), testProfile) ||
		TestProfiles.String() == All ||
		TestProfiles.String() == Community
}

func RunTestEnterprise(testProfile string) bool {
	return strings.Contains(TestProfiles.String(), testProfile) ||
		TestProfiles.String() == All ||
		TestProfiles.String() == Enterprise
}

func (f *Framework) DumpTestConfigurations() {
	By(fmt.Sprintf("Using the following test configurations:\n\t"+
		"Test Profiles: %s\n\t"+
		"Docker Registry: %s\n\t"+
		"StorageClass: %s\n\t"+
		"Database Type: %s\n\t"+
		"Database Version: %s\n\t"+
		"Updated Database Version: %s\n\t"+
		"SSL Enabled: %v\n\t"+
		"Stash Addon Name: %s\n\t"+
		"Stash Addon Version: %s\n\t",
		TestProfiles.String(),
		DockerRegistry,
		f.StorageClass,
		DBType,
		DBVersion,
		DBUpdatedVersion,
		SSLEnabled,
		StashAddonName,
		StashAddonVersion,
	),
	)
}
