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
	"context"
	"fmt"

	api "kubedb.dev/apimachinery/apis/kubedb/v1alpha2"
	"kubedb.dev/apimachinery/client/clientset/versioned/typed/kubedb/v1alpha2/util"
	"kubedb.dev/tests/e2e/framework"

	. "github.com/onsi/ginkgo"
	. "github.com/onsi/gomega"
	core "k8s.io/api/core/v1"
	kerr "k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	ofst "kmodules.xyz/offshoot-api/api/v1"
)

var _ = Describe("Environment Variables", func() {
	var err error
	to := testOptions{}
	testName := framework.EnvironmentVariable

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
		if !framework.RunTestCommunity(testName) {
			Skip(fmt.Sprintf("Provide test profile `%s` or `all` to test this.", testName))
		}
	})

	AfterEach(func() {
		// Cleanup
		By("Cleanup Left Overs")
		By("Delete left over MongoDB objects")
		to.CleanMongoDB()
		By("Delete left over workloads if exists any")
		to.CleanWorkloadLeftOvers(api.MongoDB{}.ResourceFQN())
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

	Context("With allowed Envs", func() {
		var configMap *core.ConfigMap

		BeforeEach(func() {
			configMap = to.ConfigMapForInitialization()
			err := to.CreateConfigMap(configMap)
			Expect(err).NotTo(HaveOccurred())

			to.mongodb.Spec.Init = &api.InitSpec{
				Script: &api.ScriptSourceSpec{
					VolumeSource: core.VolumeSource{
						ConfigMap: &core.ConfigMapVolumeSource{
							LocalObjectReference: core.LocalObjectReference{
								Name: configMap.Name,
							},
						},
					},
				},
			}
		})

		AfterEach(func() {
			err := to.DeleteConfigMap(configMap.ObjectMeta)
			Expect(err).NotTo(HaveOccurred())
		})

		var withAllowedEnvs = func() {
			dbName = "envDB"
			envs := []core.EnvVar{
				{
					Name:  MONGO_INITDB_DATABASE,
					Value: dbName,
				},
			}
			if to.mongodb.Spec.ShardTopology != nil {
				to.mongodb.Spec.ShardTopology.Shard.PodTemplate.Spec.Env = envs
				to.mongodb.Spec.ShardTopology.ConfigServer.PodTemplate.Spec.Env = envs
				to.mongodb.Spec.ShardTopology.Mongos.PodTemplate.Spec.Env = envs

			} else {
				if to.mongodb.Spec.PodTemplate == nil {
					to.mongodb.Spec.PodTemplate = new(ofst.PodTemplateSpec)
				}
				to.mongodb.Spec.PodTemplate.Spec.Env = envs
			}

			// Create MongoDB
			to.createAndWaitForReady()

			By("Checking Inserted Document")
			to.EventuallyDocumentExists(to.mongodb.ObjectMeta, dbName, 1).Should(BeTrue())
		}

		It("should initialize database specified by env", withAllowedEnvs)

		Context("With Replica Set", func() {
			BeforeEach(func() {
				to.mongodb = to.MongoDBRS()
				to.mongodb.Spec.Init = &api.InitSpec{
					Script: &api.ScriptSourceSpec{
						VolumeSource: core.VolumeSource{
							ConfigMap: &core.ConfigMapVolumeSource{
								LocalObjectReference: core.LocalObjectReference{
									Name: configMap.Name,
								},
							},
						},
					},
				}
			})
			It("should initialize database specified by env", withAllowedEnvs)
		})

		Context("With Sharding", func() {
			BeforeEach(func() {
				to.mongodb = to.MongoDBShard()
				to.mongodb.Spec.Init = &api.InitSpec{
					Script: &api.ScriptSourceSpec{
						VolumeSource: core.VolumeSource{
							ConfigMap: &core.ConfigMapVolumeSource{
								LocalObjectReference: core.LocalObjectReference{
									Name: configMap.Name,
								},
							},
						},
					},
				}
				//to.mongodb = to.MongoDBWithFlexibleProbeTimeout(to.mongodb)
			})
			It("should initialize database specified by env", withAllowedEnvs)
		})

	})

	Context("With forbidden Envs", func() {

		var withForbiddenEnvs = func() {

			By("Create MongoDB with " + MONGO_INITDB_ROOT_USERNAME + " env var")
			envs := []core.EnvVar{
				{
					Name:  MONGO_INITDB_ROOT_USERNAME,
					Value: "mg-user",
				},
			}
			if to.mongodb.Spec.ShardTopology != nil {
				to.mongodb.Spec.ShardTopology.Shard.PodTemplate.Spec.Env = envs
				to.mongodb.Spec.ShardTopology.ConfigServer.PodTemplate.Spec.Env = envs
				to.mongodb.Spec.ShardTopology.Mongos.PodTemplate.Spec.Env = envs

			} else {
				if to.mongodb.Spec.PodTemplate == nil {
					to.mongodb.Spec.PodTemplate = new(ofst.PodTemplateSpec)
				}
				to.mongodb.Spec.PodTemplate.Spec.Env = envs
			}
			err = to.CreateMongoDB(to.mongodb)
			Expect(err).To(HaveOccurred())

			By("Create MongoDB with " + MONGO_INITDB_ROOT_PASSWORD + " env var")
			envs = []core.EnvVar{
				{
					Name:  MONGO_INITDB_ROOT_PASSWORD,
					Value: "not@secret",
				},
			}
			if to.mongodb.Spec.ShardTopology != nil {
				to.mongodb.Spec.ShardTopology.Shard.PodTemplate.Spec.Env = envs
				to.mongodb.Spec.ShardTopology.ConfigServer.PodTemplate.Spec.Env = envs
				to.mongodb.Spec.ShardTopology.Mongos.PodTemplate.Spec.Env = envs

			} else {
				if to.mongodb.Spec.PodTemplate == nil {
					to.mongodb.Spec.PodTemplate = new(ofst.PodTemplateSpec)
				}
				to.mongodb.Spec.PodTemplate.Spec.Env = envs
			}
			err = to.CreateMongoDB(to.mongodb)
			Expect(err).To(HaveOccurred())
		}

		It("should reject to create MongoDB crd", withForbiddenEnvs)

		Context("With Replica Set", func() {
			BeforeEach(func() {
				to.mongodb = to.MongoDBRS()
			})
			It("should take Snapshot successfully", withForbiddenEnvs)
		})

		Context("With Sharding", func() {
			BeforeEach(func() {
				to.mongodb = to.MongoDBShard()
				//to.mongodb = to.MongoDBWithFlexibleProbeTimeout(to.mongodb)
			})
			It("should take Snapshot successfully", withForbiddenEnvs)
		})
	})

	Context("Update Envs", func() {
		var configMap *core.ConfigMap

		BeforeEach(func() {
			configMap = to.ConfigMapForInitialization()
			err := to.CreateConfigMap(configMap)
			Expect(err).NotTo(HaveOccurred())

			to.mongodb.Spec.Init = &api.InitSpec{
				Script: &api.ScriptSourceSpec{
					VolumeSource: core.VolumeSource{
						ConfigMap: &core.ConfigMapVolumeSource{
							LocalObjectReference: core.LocalObjectReference{
								Name: configMap.Name,
							},
						},
					},
				},
			}
		})

		AfterEach(func() {
			err := to.DeleteConfigMap(configMap.ObjectMeta)
			Expect(err).NotTo(HaveOccurred())
		})

		var withUpdateEnvs = func() {

			dbName = "envDB"
			envs := []core.EnvVar{
				{
					Name:  MONGO_INITDB_DATABASE,
					Value: dbName,
				},
			}
			if to.mongodb.Spec.ShardTopology != nil {
				to.mongodb.Spec.ShardTopology.Shard.PodTemplate.Spec.Env = envs
				to.mongodb.Spec.ShardTopology.ConfigServer.PodTemplate.Spec.Env = envs
				to.mongodb.Spec.ShardTopology.Mongos.PodTemplate.Spec.Env = envs

			} else {
				if to.mongodb.Spec.PodTemplate == nil {
					to.mongodb.Spec.PodTemplate = new(ofst.PodTemplateSpec)
				}
				to.mongodb.Spec.PodTemplate.Spec.Env = envs
			}

			// Create MongoDB
			to.createAndWaitForReady()

			By("Checking Inserted Document")
			to.EventuallyDocumentExists(to.mongodb.ObjectMeta, dbName, 1).Should(BeTrue())

			_, _, err = util.PatchMongoDB(context.TODO(), to.DBClient().KubedbV1alpha2(), to.mongodb, func(in *api.MongoDB) *api.MongoDB {
				envs = []core.EnvVar{
					{
						Name:  MONGO_INITDB_DATABASE,
						Value: "patched-db",
					},
				}
				if in.Spec.ShardTopology != nil {
					in.Spec.ShardTopology.Shard.PodTemplate.Spec.Env = envs
					in.Spec.ShardTopology.ConfigServer.PodTemplate.Spec.Env = envs
					in.Spec.ShardTopology.Mongos.PodTemplate.Spec.Env = envs

				} else {
					if in.Spec.PodTemplate == nil {
						in.Spec.PodTemplate = new(ofst.PodTemplateSpec)
					}
					in.Spec.PodTemplate.Spec.Env = envs
				}
				return in
			}, metav1.PatchOptions{})
			Expect(err).NotTo(HaveOccurred())
		}

		It("should not reject to update EnvVar", withUpdateEnvs)

		Context("With Replica Set", func() {
			BeforeEach(func() {
				to.mongodb = to.MongoDBRS()
				to.mongodb.Spec.Init = &api.InitSpec{
					Script: &api.ScriptSourceSpec{
						VolumeSource: core.VolumeSource{
							ConfigMap: &core.ConfigMapVolumeSource{
								LocalObjectReference: core.LocalObjectReference{
									Name: configMap.Name,
								},
							},
						},
					},
				}
			})

			It("should not reject to update EnvVar", withUpdateEnvs)
		})

		Context("With Sharding", func() {
			BeforeEach(func() {
				to.mongodb = to.MongoDBShard()
				to.mongodb.Spec.Init = &api.InitSpec{
					Script: &api.ScriptSourceSpec{
						VolumeSource: core.VolumeSource{
							ConfigMap: &core.ConfigMapVolumeSource{
								LocalObjectReference: core.LocalObjectReference{
									Name: configMap.Name,
								},
							},
						},
					},
				}
				//to.mongodb = to.MongoDBWithFlexibleProbeTimeout(to.mongodb)
			})

			It("should not reject to update EnvVar", withUpdateEnvs)
		})
	})
})
