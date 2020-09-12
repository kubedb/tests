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
	VerticalScaling   = "vertical_scaling"
	HorizontalScaling = "horizontal_scaling"
	Reconfigure       = "reconfigure"
	VolumeExpansion   = "volume_expansion"

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
