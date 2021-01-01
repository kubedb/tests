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

	api "kubedb.dev/apimachinery/apis/kubedb/v1alpha2"
	"kubedb.dev/tests/e2e/framework"

	. "github.com/onsi/ginkgo"
	. "github.com/onsi/gomega"
	core "k8s.io/api/core/v1"
	kerr "k8s.io/apimachinery/pkg/api/errors"
)

var _ = Describe("General SSL", func() {
	to := testOptions{}
	testName := framework.General
	var err error
	BeforeEach(func() {
		f := framework.NewInvocation()
		to = testOptions{
			Invocation:       f,
			mongodb:          f.MongoDBStandalone(),
			mongoOpsReq:      nil,
			skipMessage:      "",
			garbageMongoDB:   new(api.MongoDBList),
			snapshotPVC:      nil,
			secret:           nil,
			verifySharding:   false,
			enableSharding:   false,
			clusterAuthMode:  nil,
			sslMode:          nil,
			garbageCASecrets: []*core.Secret{},
		}
		if !framework.SSLEnabled {
			Skip("Enable SSL to test this")
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
	})

	JustAfterEach(func() {
		if CurrentGinkgoTestDescription().Failed {
			to.PrintDebugHelpers()
		}
	})

	var shouldRunWithPVC = func() {
		if to.skipMessage != "" {
			Skip(to.skipMessage)
		}
		// Create MongoDB
		to.createAndWaitForRunning(true)

		By("Checking SSL settings (if enabled any)")
		to.EventuallyUserSSLSettings(to.mongodb.ObjectMeta, to.clusterAuthMode, to.sslMode).Should(BeTrue())

		if to.enableSharding {
			By("Enable sharding for db:" + dbName)
			to.EventuallyEnableSharding(to.mongodb.ObjectMeta, dbName).Should(BeTrue())
		}
		if to.verifySharding {
			By("Check if db " + dbName + " is set to partitioned")
			to.EventuallyCollectionPartitioned(to.mongodb.ObjectMeta, dbName).Should(Equal(to.enableSharding))
		}

		By("Insert Document Inside DB")
		to.EventuallyInsertDocument(to.mongodb.ObjectMeta, dbName, 3).Should(BeTrue())

		By("Checking Inserted Document")
		to.EventuallyDocumentExists(to.mongodb.ObjectMeta, dbName, 3).Should(BeTrue())

		By("Delete mongodb")
		err := to.DeleteMongoDB(to.mongodb.ObjectMeta)
		Expect(err).NotTo(HaveOccurred())

		By("Wait for mongodb to be paused")
		to.EventuallyMongoDB(to.mongodb.ObjectMeta).Should(BeFalse())

		// Create MongoDB object again to resume it
		By("Create MongoDB: " + to.mongodb.Name)
		err = to.CreateMongoDB(to.mongodb)
		Expect(err).NotTo(HaveOccurred())

		By("Wait for Running mongodb")
		to.EventuallyMongoDBRunning(to.mongodb.ObjectMeta).Should(BeTrue())

		By("Ping mongodb database")
		to.EventuallyPingMongo(to.mongodb.ObjectMeta)

		if to.verifySharding {
			By("Check if db " + dbName + " is set to partitioned")
			to.EventuallyCollectionPartitioned(to.mongodb.ObjectMeta, dbName).Should(Equal(to.enableSharding))
		}

		By("Checking Inserted Document")
		to.EventuallyDocumentExists(to.mongodb.ObjectMeta, dbName, 3).Should(BeTrue())
	}

	var shouldFailToCreateDB = func() {
		By("Create MongoDB: " + to.mongodb.Name)
		err = to.CreateMongoDB(to.mongodb)
		Expect(err).To(HaveOccurred())
	}

	var shouldInitializeFromScript = func() {
		// Create and wait for running MongoDB
		to.createAndWaitForRunning(true)

		By("Checking SSL settings (if enabled any)")
		to.EventuallyUserSSLSettings(to.mongodb.ObjectMeta, to.clusterAuthMode, to.sslMode).Should(BeTrue())

		if to.enableSharding {
			By("Enable sharding for db:" + dbName)
			to.EventuallyEnableSharding(to.mongodb.ObjectMeta, dbName).Should(BeTrue())
		}
		if to.verifySharding {
			By("Check if db " + dbName + " is set to partitioned")
			to.EventuallyCollectionPartitioned(to.mongodb.ObjectMeta, dbName).Should(Equal(to.enableSharding))
		}

		By("Checking Inserted Document from initialization part")
		to.EventuallyDocumentExists(to.mongodb.ObjectMeta, dbName, 1).Should(BeTrue())

		By("Insert Document Inside DB")
		to.EventuallyInsertDocument(to.mongodb.ObjectMeta, dbName, 50).Should(BeTrue())

		By("Checking Inserted Document")
		to.EventuallyDocumentExists(to.mongodb.ObjectMeta, dbName, 50).Should(BeTrue())
	}

	BeforeEach(func() {
		if to.StorageClass == "" {
			Skip("Missing StorageClassName. Provide as flag to test this.")
		}
	})

	AfterEach(func() {
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

	Context("General SSL", func() {

		Context("With sslMode requireSSL", func() {

			BeforeEach(func() {
				to.sslMode = framework.SSLModeP(api.SSLModeRequireSSL)
			})

			Context("Standalone", func() {
				BeforeEach(func() {
					to.mongodb = to.MongoDBStandalone()
					to.mongodb.Spec.SSLMode = *to.sslMode
				})

				It("should run successfully", shouldRunWithPVC)

				// Snapshot doesn't work yet for requireSSL SSLMode
			})

			Context("With ClusterAuthMode keyfile", func() {

				BeforeEach(func() {
					to.clusterAuthMode = framework.ClusterAuthModeP(api.ClusterAuthModeKeyFile)
				})

				Context("With Replica Set", func() {

					BeforeEach(func() {
						to.mongodb = to.MongoDBRS()
						to.mongodb.Spec.ClusterAuthMode = *to.clusterAuthMode
						to.mongodb.Spec.SSLMode = *to.sslMode
					})

					It("should run successfully", shouldRunWithPVC)
				})

				Context("With Sharding", func() {

					BeforeEach(func() {
						to.verifySharding = true
						to.enableSharding = true

						to.mongodb = to.MongoDBShard()
						to.mongodb.Spec.ClusterAuthMode = *to.clusterAuthMode
						to.mongodb.Spec.SSLMode = *to.sslMode
					})

					It("should run successfully", shouldRunWithPVC)
				})
			})

			Context("With ClusterAuthMode x509", func() {

				BeforeEach(func() {
					to.clusterAuthMode = framework.ClusterAuthModeP(api.ClusterAuthModeX509)
				})

				Context("With Replica Set", func() {

					BeforeEach(func() {
						to.mongodb = to.MongoDBRS()
						to.mongodb.Spec.ClusterAuthMode = *to.clusterAuthMode
						to.mongodb.Spec.SSLMode = *to.sslMode
					})

					It("should run successfully", shouldRunWithPVC)
				})

				Context("With Sharding", func() {

					BeforeEach(func() {
						to.verifySharding = true
						to.enableSharding = true

						to.mongodb = to.MongoDBShard()
						to.mongodb.Spec.ClusterAuthMode = *to.clusterAuthMode
						to.mongodb.Spec.SSLMode = *to.sslMode
					})

					It("should run successfully", shouldRunWithPVC)
				})
			})

			Context("With ClusterAuthMode sendkeyfile", func() {

				BeforeEach(func() {
					to.clusterAuthMode = framework.ClusterAuthModeP(api.ClusterAuthModeSendKeyFile)
				})

				Context("With Replica Set", func() {

					BeforeEach(func() {
						to.mongodb = to.MongoDBRS()
						to.mongodb.Spec.ClusterAuthMode = *to.clusterAuthMode
						to.mongodb.Spec.SSLMode = *to.sslMode
					})

					It("should run successfully", shouldRunWithPVC)

				})

				Context("With Sharding", func() {

					BeforeEach(func() {
						to.verifySharding = true
						to.enableSharding = true

						to.mongodb = to.MongoDBShard()
						to.mongodb.Spec.ClusterAuthMode = *to.clusterAuthMode
						to.mongodb.Spec.SSLMode = *to.sslMode
					})

					It("should run successfully", shouldRunWithPVC)
				})
			})

			Context("With ClusterAuthMode sendX509", func() {

				BeforeEach(func() {
					to.clusterAuthMode = framework.ClusterAuthModeP(api.ClusterAuthModeSendX509)
				})

				Context("With Replica Set", func() {

					BeforeEach(func() {
						to.mongodb = to.MongoDBRS()
						to.mongodb.Spec.ClusterAuthMode = *to.clusterAuthMode
						to.mongodb.Spec.SSLMode = *to.sslMode
					})

					It("should run successfully", shouldRunWithPVC)
				})

				Context("With Sharding", func() {

					BeforeEach(func() {
						to.verifySharding = true
						to.enableSharding = true

						to.mongodb = to.MongoDBShard()
						to.mongodb.Spec.ClusterAuthMode = *to.clusterAuthMode
						to.mongodb.Spec.SSLMode = *to.sslMode
					})

					It("should run successfully", shouldRunWithPVC)
				})
			})
		})

		Context("With sslMode preferSSL", func() {

			BeforeEach(func() {
				to.sslMode = framework.SSLModeP(api.SSLModePreferSSL)
			})

			Context("Standalone", func() {

				BeforeEach(func() {
					to.mongodb = to.MongoDBStandalone()
					to.mongodb.Spec.SSLMode = *to.sslMode
				})

				It("should run successfully", shouldRunWithPVC)

				Context("Initialization - script & snapshot", func() {
					var configMap *core.ConfigMap

					BeforeEach(func() {
						configMap = to.ConfigMapForInitialization()
						err := to.CreateConfigMap(configMap)
						Expect(err).NotTo(HaveOccurred())
					})

					AfterEach(func() {
						err := to.DeleteConfigMap(configMap.ObjectMeta)
						Expect(err).NotTo(HaveOccurred())
					})

					BeforeEach(func() {
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

					It("should run successfully", shouldInitializeFromScript)
				})
			})

			Context("With ClusterAuthMode keyfile", func() {

				BeforeEach(func() {
					to.clusterAuthMode = framework.ClusterAuthModeP(api.ClusterAuthModeKeyFile)
				})

				Context("With Replica Set", func() {

					BeforeEach(func() {
						to.mongodb = to.MongoDBRS()
						to.mongodb.Spec.ClusterAuthMode = *to.clusterAuthMode
						to.mongodb.Spec.SSLMode = *to.sslMode
					})

					It("should run successfully", shouldRunWithPVC)

					Context("Initialization - script & snapshot", func() {
						var configMap *core.ConfigMap

						BeforeEach(func() {
							configMap = to.ConfigMapForInitialization()
							err := to.CreateConfigMap(configMap)
							Expect(err).NotTo(HaveOccurred())
						})

						AfterEach(func() {
							err := to.DeleteConfigMap(configMap.ObjectMeta)
							Expect(err).NotTo(HaveOccurred())
						})

						BeforeEach(func() {
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

						It("should initialize database successfully", shouldInitializeFromScript)
					})
				})

				Context("With Sharding", func() {

					BeforeEach(func() {
						to.verifySharding = true
						to.enableSharding = true

						to.mongodb = to.MongoDBShard()
						to.mongodb.Spec.ClusterAuthMode = *to.clusterAuthMode
						to.mongodb.Spec.SSLMode = *to.sslMode
					})

					It("should run successfully", shouldRunWithPVC)

					Context("Initialization - script & snapshot", func() {
						var configMap *core.ConfigMap

						AfterEach(func() {
							err := to.DeleteConfigMap(configMap.ObjectMeta)
							Expect(err).NotTo(HaveOccurred())
						})

						BeforeEach(func() {
							configMap = to.ConfigMapForInitialization()
							err := to.CreateConfigMap(configMap)
							Expect(err).NotTo(HaveOccurred())

							to.verifySharding = true
							to.enableSharding = true
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

							to.mongodb = to.MongoDBWithFlexibleProbeTimeout(to.mongodb)
						})

						It("should initialize database successfully", shouldInitializeFromScript)
					})
				})
			})

			Context("With ClusterAuthMode x509", func() {

				BeforeEach(func() {
					to.clusterAuthMode = framework.ClusterAuthModeP(api.ClusterAuthModeX509)
				})

				Context("With Replica Set", func() {
					BeforeEach(func() {
						to.mongodb = to.MongoDBRS()
						to.mongodb.Spec.ClusterAuthMode = *to.clusterAuthMode
						to.mongodb.Spec.SSLMode = *to.sslMode
					})

					It("should run successfully", shouldRunWithPVC)
				})

				Context("With Sharding", func() {

					BeforeEach(func() {
						to.verifySharding = true
						to.enableSharding = true

						to.mongodb = to.MongoDBShard()
						to.mongodb.Spec.ClusterAuthMode = *to.clusterAuthMode
						to.mongodb.Spec.SSLMode = *to.sslMode
					})

					It("should run successfully", shouldRunWithPVC)

					Context("Initialization - script & snapshot", func() {
						var configMap *core.ConfigMap

						AfterEach(func() {
							err := to.DeleteConfigMap(configMap.ObjectMeta)
							Expect(err).NotTo(HaveOccurred())
						})

						BeforeEach(func() {
							configMap = to.ConfigMapForInitialization()
							err := to.CreateConfigMap(configMap)
							Expect(err).NotTo(HaveOccurred())

							to.verifySharding = true
							to.enableSharding = true
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

							to.mongodb = to.MongoDBWithFlexibleProbeTimeout(to.mongodb)
						})

						It("should initialize database successfully", shouldInitializeFromScript)
					})
				})
			})

			Context("With ClusterAuthMode sendkeyfile", func() {

				BeforeEach(func() {
					to.clusterAuthMode = framework.ClusterAuthModeP(api.ClusterAuthModeSendKeyFile)
				})

				Context("With Replica Set", func() {

					BeforeEach(func() {
						to.mongodb = to.MongoDBRS()
						to.mongodb.Spec.ClusterAuthMode = *to.clusterAuthMode
						to.mongodb.Spec.SSLMode = *to.sslMode
					})

					It("should run successfully", shouldRunWithPVC)

					Context("Initialization - script & snapshot", func() {
						var configMap *core.ConfigMap

						BeforeEach(func() {
							configMap = to.ConfigMapForInitialization()
							err := to.CreateConfigMap(configMap)
							Expect(err).NotTo(HaveOccurred())
						})

						AfterEach(func() {
							err := to.DeleteConfigMap(configMap.ObjectMeta)
							Expect(err).NotTo(HaveOccurred())
						})

						BeforeEach(func() {
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

						It("should initialize database successfully", shouldInitializeFromScript)
					})
				})

				Context("With Sharding", func() {

					BeforeEach(func() {
						to.verifySharding = true
						to.enableSharding = true

						to.mongodb = to.MongoDBShard()
						to.mongodb.Spec.ClusterAuthMode = *to.clusterAuthMode
						to.mongodb.Spec.SSLMode = *to.sslMode
					})

					It("should run successfully", shouldRunWithPVC)

					Context("Initialization - script & snapshot", func() {
						var configMap *core.ConfigMap

						AfterEach(func() {
							err := to.DeleteConfigMap(configMap.ObjectMeta)
							Expect(err).NotTo(HaveOccurred())
						})

						BeforeEach(func() {
							configMap = to.ConfigMapForInitialization()
							err := to.CreateConfigMap(configMap)
							Expect(err).NotTo(HaveOccurred())

							to.verifySharding = true
							to.enableSharding = true
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

							to.mongodb = to.MongoDBWithFlexibleProbeTimeout(to.mongodb)
						})

						It("should initialize database successfully", shouldInitializeFromScript)
					})
				})
			})

			Context("With ClusterAuthMode sendX509", func() {

				BeforeEach(func() {
					to.clusterAuthMode = framework.ClusterAuthModeP(api.ClusterAuthModeSendX509)
				})

				Context("With Replica Set", func() {

					BeforeEach(func() {
						to.mongodb = to.MongoDBRS()
						to.mongodb.Spec.ClusterAuthMode = *to.clusterAuthMode
						to.mongodb.Spec.SSLMode = *to.sslMode
					})

					It("should run successfully", shouldRunWithPVC)

					Context("Initialization - script & snapshot", func() {
						var configMap *core.ConfigMap

						BeforeEach(func() {
							configMap = to.ConfigMapForInitialization()
							err := to.CreateConfigMap(configMap)
							Expect(err).NotTo(HaveOccurred())
						})

						AfterEach(func() {
							err := to.DeleteConfigMap(configMap.ObjectMeta)
							Expect(err).NotTo(HaveOccurred())
						})

						BeforeEach(func() {
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

						It("should initialize database successfully", shouldInitializeFromScript)
					})
				})

				Context("With Sharding", func() {

					BeforeEach(func() {
						to.verifySharding = true
						to.enableSharding = true

						to.mongodb = to.MongoDBShard()
						to.mongodb.Spec.ClusterAuthMode = *to.clusterAuthMode
						to.mongodb.Spec.SSLMode = *to.sslMode
					})

					It("should run successfully", shouldRunWithPVC)

					Context("Initialization - script & snapshot", func() {
						var configMap *core.ConfigMap

						AfterEach(func() {
							err := to.DeleteConfigMap(configMap.ObjectMeta)
							Expect(err).NotTo(HaveOccurred())
						})

						BeforeEach(func() {
							configMap = to.ConfigMapForInitialization()
							err := to.CreateConfigMap(configMap)
							Expect(err).NotTo(HaveOccurred())

							to.verifySharding = true
							to.enableSharding = true
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

							to.mongodb = to.MongoDBWithFlexibleProbeTimeout(to.mongodb)
						})

						It("should initialize database successfully", shouldInitializeFromScript)
					})
				})
			})
		})

		Context("With sslMode allowSSL", func() {

			BeforeEach(func() {
				to.sslMode = framework.SSLModeP(api.SSLModeAllowSSL)
			})

			Context("Standalone", func() {

				BeforeEach(func() {
					to.mongodb = to.MongoDBStandalone()
					to.mongodb.Spec.SSLMode = *to.sslMode
				})

				It("should run successfully", shouldRunWithPVC)

				Context("Initialization - script & snapshot", func() {
					var configMap *core.ConfigMap

					BeforeEach(func() {
						configMap = to.ConfigMapForInitialization()
						err := to.CreateConfigMap(configMap)
						Expect(err).NotTo(HaveOccurred())
					})

					AfterEach(func() {
						err := to.DeleteConfigMap(configMap.ObjectMeta)
						Expect(err).NotTo(HaveOccurred())
					})
					BeforeEach(func() {
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

					It("should run successfully", shouldInitializeFromScript)
				})
			})

			Context("With ClusterAuthMode keyFile", func() {

				BeforeEach(func() {
					to.clusterAuthMode = framework.ClusterAuthModeP(api.ClusterAuthModeKeyFile)
				})

				Context("With Replica Set", func() {

					BeforeEach(func() {
						to.mongodb = to.MongoDBRS()
						to.mongodb.Spec.ClusterAuthMode = *to.clusterAuthMode
						to.mongodb.Spec.SSLMode = *to.sslMode
					})

					It("should run successfully", shouldRunWithPVC)

					Context("Initialization - script & snapshot", func() {
						var configMap *core.ConfigMap

						BeforeEach(func() {
							configMap = to.ConfigMapForInitialization()
							err := to.CreateConfigMap(configMap)
							Expect(err).NotTo(HaveOccurred())
						})

						AfterEach(func() {
							err := to.DeleteConfigMap(configMap.ObjectMeta)
							Expect(err).NotTo(HaveOccurred())
						})

						BeforeEach(func() {
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

						It("should initialize database successfully", shouldInitializeFromScript)
					})
				})

				Context("With Sharding", func() {

					BeforeEach(func() {
						to.verifySharding = true
						to.enableSharding = true

						to.mongodb = to.MongoDBShard()
						to.mongodb.Spec.ClusterAuthMode = *to.clusterAuthMode
						to.mongodb.Spec.SSLMode = *to.sslMode
					})

					It("should run successfully", shouldRunWithPVC)

					Context("Initialization - script & snapshot", func() {
						var configMap *core.ConfigMap

						AfterEach(func() {
							err := to.DeleteConfigMap(configMap.ObjectMeta)
							Expect(err).NotTo(HaveOccurred())
						})

						BeforeEach(func() {
							configMap = to.ConfigMapForInitialization()
							err := to.CreateConfigMap(configMap)
							Expect(err).NotTo(HaveOccurred())

							to.verifySharding = true
							to.enableSharding = true
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

							to.mongodb = to.MongoDBWithFlexibleProbeTimeout(to.mongodb)
						})

						It("should initialize database successfully", shouldInitializeFromScript)
					})
				})
			})

			// should fail. error: BadValue: cannot have x.509 cluster authentication in allowSSL mode
			Context("With ClusterAuthMode x509", func() {

				BeforeEach(func() {
					to.clusterAuthMode = framework.ClusterAuthModeP(api.ClusterAuthModeX509)
				})

				Context("With Replica Set", func() {
					BeforeEach(func() {
						to.mongodb = to.MongoDBRS()
						to.mongodb.Spec.ClusterAuthMode = *to.clusterAuthMode
						to.mongodb.Spec.SSLMode = *to.sslMode
					})

					It("should fail creating mongodb object", shouldFailToCreateDB)
				})

				Context("With Sharding", func() {

					BeforeEach(func() {
						to.verifySharding = true
						to.enableSharding = true

						to.mongodb = to.MongoDBShard()
						to.mongodb.Spec.ClusterAuthMode = *to.clusterAuthMode
						to.mongodb.Spec.SSLMode = *to.sslMode
					})

					It("should fail creating mongodb object", shouldFailToCreateDB)
				})
			})

			Context("With ClusterAuthMode sendkeyfile", func() {

				BeforeEach(func() {
					to.clusterAuthMode = framework.ClusterAuthModeP(api.ClusterAuthModeSendKeyFile)
				})

				Context("With Replica Set", func() {

					BeforeEach(func() {
						to.mongodb = to.MongoDBRS()
						to.mongodb.Spec.ClusterAuthMode = *to.clusterAuthMode
						to.mongodb.Spec.SSLMode = *to.sslMode
					})

					It("should run successfully", shouldRunWithPVC)

					Context("Initialization - script & snapshot", func() {
						var configMap *core.ConfigMap

						BeforeEach(func() {
							configMap = to.ConfigMapForInitialization()
							err := to.CreateConfigMap(configMap)
							Expect(err).NotTo(HaveOccurred())
						})

						AfterEach(func() {
							err := to.DeleteConfigMap(configMap.ObjectMeta)
							Expect(err).NotTo(HaveOccurred())
						})

						BeforeEach(func() {
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

						It("should initialize database successfully", shouldInitializeFromScript)
					})
				})

				Context("With Sharding", func() {

					BeforeEach(func() {
						to.verifySharding = true
						to.enableSharding = true

						to.mongodb = to.MongoDBShard()
						to.mongodb.Spec.ClusterAuthMode = *to.clusterAuthMode
						to.mongodb.Spec.SSLMode = *to.sslMode
					})

					It("should run successfully", shouldRunWithPVC)

					Context("Initialization - script & snapshot", func() {
						var configMap *core.ConfigMap

						AfterEach(func() {
							err := to.DeleteConfigMap(configMap.ObjectMeta)
							Expect(err).NotTo(HaveOccurred())
						})

						BeforeEach(func() {
							configMap = to.ConfigMapForInitialization()
							err := to.CreateConfigMap(configMap)
							Expect(err).NotTo(HaveOccurred())

							to.verifySharding = true
							to.enableSharding = true
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

							to.mongodb = to.MongoDBWithFlexibleProbeTimeout(to.mongodb)
						})

						It("should initialize database successfully", shouldInitializeFromScript)
					})
				})
			})

			//should fail. error: BadValue: cannot have x.509 cluster authentication in allowSSL mode
			Context("With ClusterAuthMode sendX509", func() {

				BeforeEach(func() {
					to.clusterAuthMode = framework.ClusterAuthModeP(api.ClusterAuthModeSendX509)
				})

				Context("With Replica Set", func() {
					BeforeEach(func() {
						to.mongodb = to.MongoDBRS()
						to.mongodb.Spec.ClusterAuthMode = *to.clusterAuthMode
						to.mongodb.Spec.SSLMode = *to.sslMode
					})

					It("should fail creating mongodb object", shouldFailToCreateDB)
				})

				Context("With Sharding", func() {

					BeforeEach(func() {
						to.verifySharding = true
						to.enableSharding = true

						to.mongodb = to.MongoDBShard()
						to.mongodb.Spec.ClusterAuthMode = *to.clusterAuthMode
						to.mongodb.Spec.SSLMode = *to.sslMode
					})

					It("should fail creating mongodb object", shouldFailToCreateDB)
				})
			})

		})

		Context("With sslMode disabled", func() {

			BeforeEach(func() {
				to.sslMode = framework.SSLModeP(api.SSLModeDisabled)
			})

			Context("Standalone", func() {
				BeforeEach(func() {
					to.mongodb = to.MongoDBStandalone()
					to.mongodb.Spec.SSLMode = *to.sslMode
				})

				It("should run successfully", shouldRunWithPVC)

				Context("Initialization - script & snapshot", func() {
					var configMap *core.ConfigMap

					AfterEach(func() {
						err := to.DeleteConfigMap(configMap.ObjectMeta)
						Expect(err).NotTo(HaveOccurred())
					})

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

						to.mongodb = to.MongoDBWithFlexibleProbeTimeout(to.mongodb)
					})

					It("should run successfully", shouldInitializeFromScript)
				})
			})

			Context("With ClusterAuthMode keyfile", func() {

				BeforeEach(func() {
					to.clusterAuthMode = framework.ClusterAuthModeP(api.ClusterAuthModeKeyFile)
				})

				Context("With Replica Set", func() {

					BeforeEach(func() {
						to.mongodb = to.MongoDBRS()
						to.mongodb.Spec.ClusterAuthMode = *to.clusterAuthMode
						to.mongodb.Spec.SSLMode = *to.sslMode
					})

					It("should run successfully", shouldRunWithPVC)

					Context("Initialization - script & snapshot", func() {
						var configMap *core.ConfigMap

						AfterEach(func() {
							err := to.DeleteConfigMap(configMap.ObjectMeta)
							Expect(err).NotTo(HaveOccurred())
						})

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

							to.mongodb = to.MongoDBWithFlexibleProbeTimeout(to.mongodb)
						})

						It("should initialize database successfully", shouldInitializeFromScript)
					})
				})

				Context("With Sharding", func() {

					BeforeEach(func() {
						to.verifySharding = true
						to.enableSharding = true

						to.mongodb = to.MongoDBShard()
						to.mongodb.Spec.ClusterAuthMode = *to.clusterAuthMode
						to.mongodb.Spec.SSLMode = *to.sslMode
					})

					It("should run successfully", shouldRunWithPVC)

					Context("Initialization - script & snapshot", func() {
						var configMap *core.ConfigMap

						AfterEach(func() {
							err := to.DeleteConfigMap(configMap.ObjectMeta)
							Expect(err).NotTo(HaveOccurred())
						})

						BeforeEach(func() {
							configMap = to.ConfigMapForInitialization()
							err := to.CreateConfigMap(configMap)
							Expect(err).NotTo(HaveOccurred())

							to.verifySharding = true
							to.enableSharding = true
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

							to.mongodb = to.MongoDBWithFlexibleProbeTimeout(to.mongodb)
						})

						It("should initialize database successfully", shouldInitializeFromScript)
					})
				})
			})

			// should fail. error: BadValue: need to enable SSL via the sslMode flag
			Context("With ClusterAuthMode x509", func() {

				BeforeEach(func() {
					to.clusterAuthMode = framework.ClusterAuthModeP(api.ClusterAuthModeX509)
				})

				Context("With Replica Set", func() {
					BeforeEach(func() {
						to.mongodb = to.MongoDBRS()
						to.mongodb.Spec.ClusterAuthMode = *to.clusterAuthMode
						to.mongodb.Spec.SSLMode = *to.sslMode
					})

					It("should fail creating mongodb object", shouldFailToCreateDB)
				})

				Context("With Sharding", func() {

					BeforeEach(func() {
						to.verifySharding = true
						to.enableSharding = true

						to.mongodb = to.MongoDBShard()
						to.mongodb.Spec.ClusterAuthMode = *to.clusterAuthMode
						to.mongodb.Spec.SSLMode = *to.sslMode
					})

					It("should fail creating mongodb object", shouldFailToCreateDB)
				})
			})

			// should fail. error: BadValue: need to enable SSL via the sslMode flag
			Context("With ClusterAuthMode sendkeyfile", func() {

				BeforeEach(func() {
					to.clusterAuthMode = framework.ClusterAuthModeP(api.ClusterAuthModeSendKeyFile)
				})

				Context("With Replica Set", func() {
					BeforeEach(func() {
						to.mongodb = to.MongoDBRS()
						to.mongodb.Spec.ClusterAuthMode = *to.clusterAuthMode
						to.mongodb.Spec.SSLMode = *to.sslMode
					})

					It("should fail creating mongodb object", shouldFailToCreateDB)
				})

				Context("With Sharding", func() {

					BeforeEach(func() {
						to.verifySharding = true
						to.enableSharding = true

						to.mongodb = to.MongoDBShard()
						to.mongodb.Spec.ClusterAuthMode = *to.clusterAuthMode
						to.mongodb.Spec.SSLMode = *to.sslMode
					})

					It("should fail creating mongodb object", shouldFailToCreateDB)
				})
			})

			// should fail. error: need to enable SSL via the sslMode flag
			Context("With ClusterAuthMode sendX509", func() {

				BeforeEach(func() {
					to.clusterAuthMode = framework.ClusterAuthModeP(api.ClusterAuthModeSendX509)
				})

				// should fail. error: need to enable SSL via the sslMode flag
				Context("With Replica Set", func() {
					BeforeEach(func() {
						to.mongodb = to.MongoDBRS()
						to.mongodb.Spec.ClusterAuthMode = *to.clusterAuthMode
						to.mongodb.Spec.SSLMode = *to.sslMode
					})

					It("should fail creating mongodb object", shouldFailToCreateDB)
				})

				// should fail. error: need to enable SSL via the sslMode flag
				Context("With Sharding", func() {

					BeforeEach(func() {
						to.verifySharding = true
						to.enableSharding = true

						to.mongodb = to.MongoDBShard()
						to.mongodb.Spec.ClusterAuthMode = *to.clusterAuthMode
						to.mongodb.Spec.SSLMode = *to.sslMode
					})

					It("should fail creating mongodb object", shouldFailToCreateDB)
				})
			})
		})
	})
})
