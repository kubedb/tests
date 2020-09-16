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

package e2e_test

import (
	"fmt"

	api "kubedb.dev/apimachinery/apis/kubedb/v1alpha1"
	"kubedb.dev/tests/e2e/framework"

	. "github.com/onsi/ginkgo"
	. "github.com/onsi/gomega"
	core "k8s.io/api/core/v1"
	kerr "k8s.io/apimachinery/pkg/api/errors"
)

var _ = Describe("Custom config", func() {
	to := testOptions{}
	testName := framework.CustomConfig
	BeforeEach(func() {
		f := framework.NewInvocation()
		to = testOptions{
			Invocation:       f,
			mongodb:          f.MongoDBStandalone(),
			skipMessage:      "",
			garbageMongoDB:   new(api.MongoDBList),
			snapshotPVC:      nil,
			secret:           nil,
			verifySharding:   false,
			enableSharding:   false,
			garbageCASecrets: []*core.Secret{},
		}
		if to.StorageClass == "" {
			Skip("Missing StorageClassName. Provide as flag to test this.")
		}
		if !runTestCommunity(testName) {
			Skip(fmt.Sprintf("Provide test profile `%s` or `all` to test this.", testName))
		}
	})

	AfterEach(func() {
		// Cleanup
		By("Cleanup Left Overs")
		By("Delete left over MongoDB objects")
		to.CleanMongoDB()
		By("Delete left over workloads if exists any")
		to.CleanWorkloadLeftOvers()
		if to.snapshotPVC != nil {
			err := to.DeletePersistentVolumeClaim(to.snapshotPVC.ObjectMeta)
			if err != nil && !kerr.IsNotFound(err) {
				Expect(err).NotTo(HaveOccurred())
			}
		}

		// Delete test resource
		to.deleteTestResource()

		for _, mg := range to.garbageMongoDB.Items {
			*to.mongodb = mg
			// Delete test resource
			to.deleteTestResource()
		}

		if to.secret != nil {
			err := to.DeleteSecret(to.secret.ObjectMeta)
			if !kerr.IsNotFound(err) {
				Expect(err).NotTo(HaveOccurred())
			}
		}
	})

	JustAfterEach(func() {
		if CurrentGinkgoTestDescription().Failed {
			to.PrintDebugHelpers()
		}
	})

	var maxIncomingConnections = int32(10000)
	customConfigs := []string{
		fmt.Sprintf(`   maxIncomingConnections: %v`, maxIncomingConnections),
	}

	Context("from configMap", func() {
		var userConfig *core.ConfigMap

		BeforeEach(func() {
			userConfig = to.GetCustomConfig(customConfigs, to.App())
		})

		AfterEach(func() {
			By("Deleting configMap: " + userConfig.Name)
			err := to.DeleteConfigMap(userConfig.ObjectMeta)
			Expect(err).NotTo(HaveOccurred())

		})

		runWithUserProvidedConfig := func() {
			if to.skipMessage != "" {
				Skip(to.skipMessage)
			}

			By("Creating configMap: " + userConfig.Name)
			err := to.CreateConfigMap(userConfig)
			Expect(err).NotTo(HaveOccurred())

			// Create MySQL
			to.createAndWaitForRunning()

			By("Checking maxIncomingConnections from mongodb config")
			to.EventuallyMaxIncomingConnections(to.mongodb.ObjectMeta).Should(Equal(maxIncomingConnections))
		}

		Context("Standalone MongoDB", func() {

			BeforeEach(func() {
				to.mongodb = to.MongoDBStandalone()
				to.mongodb.Spec.ConfigSource = &core.VolumeSource{
					ConfigMap: &core.ConfigMapVolumeSource{
						LocalObjectReference: core.LocalObjectReference{
							Name: userConfig.Name,
						},
					},
				}
			})

			It("should run successfully", runWithUserProvidedConfig)
		})

		Context("With Replica Set", func() {

			BeforeEach(func() {
				to.mongodb = to.MongoDBRS()
				to.mongodb.Spec.ConfigSource = &core.VolumeSource{
					ConfigMap: &core.ConfigMapVolumeSource{
						LocalObjectReference: core.LocalObjectReference{
							Name: userConfig.Name,
						},
					},
				}
			})

			It("should run successfully", runWithUserProvidedConfig)
		})

		Context("With Sharding", func() {

			BeforeEach(func() {
				to.mongodb = to.MongoDBShard()
				to.mongodb.Spec.ShardTopology.Shard.ConfigSource = &core.VolumeSource{
					ConfigMap: &core.ConfigMapVolumeSource{
						LocalObjectReference: core.LocalObjectReference{
							Name: userConfig.Name,
						},
					},
				}
				to.mongodb.Spec.ShardTopology.ConfigServer.ConfigSource = &core.VolumeSource{
					ConfigMap: &core.ConfigMapVolumeSource{
						LocalObjectReference: core.LocalObjectReference{
							Name: userConfig.Name,
						},
					},
				}
				to.mongodb.Spec.ShardTopology.Mongos.ConfigSource = &core.VolumeSource{
					ConfigMap: &core.ConfigMapVolumeSource{
						LocalObjectReference: core.LocalObjectReference{
							Name: userConfig.Name,
						},
					},
				}

				//to.mongodb = to.MongoDBWithFlexibleProbeTimeout(to.mongodb)

			})

			It("should run successfully", runWithUserProvidedConfig)
		})

	})
})
