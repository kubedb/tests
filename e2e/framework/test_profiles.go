package framework

//go:generate enumer -type=TestProfile -json
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
)
