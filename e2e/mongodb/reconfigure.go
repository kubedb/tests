package e2e_test

import (
	"fmt"

	api "kubedb.dev/apimachinery/apis/kubedb/v1alpha1"
	dbaapi "kubedb.dev/apimachinery/apis/ops/v1alpha1"
	"kubedb.dev/tests/e2e/framework"

	. "github.com/onsi/ginkgo"
	. "github.com/onsi/gomega"
	v1 "k8s.io/api/core/v1"
)

var _ = Describe("Reconfigure", func() {
	to := testOptions{}
	BeforeEach(func() {
		to.Invocation = framework.NewInvocation()
	})

	AfterEach(func() {
		err := to.CleanupTestResources()
		Expect(err).NotTo(HaveOccurred())
	})

	AfterEach(func() {
		//Delete MongoDB
		By("Delete mongodb")
		err := to.DeleteMongoDB(to.mongodb.ObjectMeta)
		Expect(err).NotTo(HaveOccurred())

		By("Delete mongodb ops request")
		err = to.DeleteMongoDBOpsRequest(to.mongoOpsReq.ObjectMeta)
		Expect(err).NotTo(HaveOccurred())

		By("Wait for mongodb resources to be wipedOut")
		to.EventuallyWipedOut(to.mongodb.ObjectMeta).Should(Succeed())
	})

	var prevMaxIncomingConnections = int32(10000)
	customConfigs := []string{
		fmt.Sprintf(`   maxIncomingConnections: %v`, prevMaxIncomingConnections),
	}

	var newMaxIncomingConnections = int32(20000)
	newCustomConfigs := []string{
		fmt.Sprintf(`   maxIncomingConnections: %v`, newMaxIncomingConnections),
	}
	data := map[string]string{
		"mongod.conf": fmt.Sprintf(`net:
   maxIncomingConnections: %v`, newMaxIncomingConnections),
	}

	Context("From Data", func() {
		var userConfig *v1.ConfigMap
		var newCustomConfig *dbaapi.MongoDBCustomConfig
		BeforeEach(func() {
			to.skipMessage = ""
			configName := to.App() + "-previous-config"
			userConfig = to.GetCustomConfig(customConfigs, configName)
			newCustomConfig = &dbaapi.MongoDBCustomConfig{
				Data: data,
			}
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

			to.createAndWaitForRunning()

			By("Checking maxIncomingConnections from mongodb config")
			to.EventuallyMaxIncomingConnections(to.mongodb.ObjectMeta).Should(Equal(prevMaxIncomingConnections))

			// Insert Data
			By("Insert Document Inside DB")
			to.EventuallyInsertDocument(to.mongodb.ObjectMeta, dbName, 3).Should(BeTrue())

			By("Checking Inserted Document")
			to.EventuallyDocumentExists(to.mongodb.ObjectMeta, dbName, 3).Should(BeTrue())

			// Update Database
			By("Updating MongoDB")
			err = to.CreateMongoDBOpsRequest(to.mongoOpsReq)
			Expect(err).NotTo(HaveOccurred())

			By("Waiting for MongoDB Ops Request Phase to be Successful")
			to.EventuallyMongoDBOpsRequestPhase(to.mongoOpsReq.ObjectMeta).Should(Equal(dbaapi.OpsRequestPhaseSuccessful))

			// Retrieve Inserted Data
			By("Checking Inserted Document after update")
			to.EventuallyDocumentExists(to.mongodb.ObjectMeta, dbName, 3).Should(BeTrue())

			By("Checking updated maxIncomingConnections from mongodb config")
			to.EventuallyMaxIncomingConnections(to.mongodb.ObjectMeta).Should(Equal(newMaxIncomingConnections))
		}

		Context("Standalone MongoDB", func() {
			BeforeEach(func() {
				to.mongodb = to.MongoDBStandalone()
				to.mongodb.Spec.Version = framework.DBVersion
				to.mongodb.Spec.TerminationPolicy = api.TerminationPolicyWipeOut
				to.mongodb.Spec.ConfigSource = &v1.VolumeSource{
					ConfigMap: &v1.ConfigMapVolumeSource{
						LocalObjectReference: v1.LocalObjectReference{
							Name: userConfig.Name,
						},
					},
				}
				to.mongoOpsReq = to.MongoDBOpsRequestReconfigure(to.mongodb.Name, to.mongodb.Namespace, newCustomConfig, nil, nil, nil, nil)
			})

			It("should run successfully", runWithUserProvidedConfig)
		})

		Context("With Replica Set", func() {
			BeforeEach(func() {
				to.mongodb = to.MongoDBRS()
				to.mongodb.Spec.Version = framework.DBVersion
				to.mongodb.Spec.TerminationPolicy = api.TerminationPolicyWipeOut
				to.mongodb.Spec.ConfigSource = &v1.VolumeSource{
					ConfigMap: &v1.ConfigMapVolumeSource{
						LocalObjectReference: v1.LocalObjectReference{
							Name: userConfig.Name,
						},
					},
				}
				to.mongoOpsReq = to.MongoDBOpsRequestReconfigure(to.mongodb.Name, to.mongodb.Namespace, nil, newCustomConfig, nil, nil, nil)
			})

			It("should run successfully", runWithUserProvidedConfig)
		})

		Context("With Sharding", func() {
			BeforeEach(func() {
				to.mongodb = to.MongoDBShard()
				to.mongodb.Spec.Version = framework.DBVersion
				to.mongodb.Spec.TerminationPolicy = api.TerminationPolicyWipeOut
				to.mongodb.Spec.ShardTopology.Shard.ConfigSource = &v1.VolumeSource{
					ConfigMap: &v1.ConfigMapVolumeSource{
						LocalObjectReference: v1.LocalObjectReference{
							Name: userConfig.Name,
						},
					},
				}
				to.mongodb.Spec.ShardTopology.ConfigServer.ConfigSource = &v1.VolumeSource{
					ConfigMap: &v1.ConfigMapVolumeSource{
						LocalObjectReference: v1.LocalObjectReference{
							Name: userConfig.Name,
						},
					},
				}
				to.mongodb.Spec.ShardTopology.Mongos.ConfigSource = &v1.VolumeSource{
					ConfigMap: &v1.ConfigMapVolumeSource{
						LocalObjectReference: v1.LocalObjectReference{
							Name: userConfig.Name,
						},
					},
				}

				to.mongoOpsReq = to.MongoDBOpsRequestReconfigure(to.mongodb.Name, to.mongodb.Namespace, nil, nil, newCustomConfig, newCustomConfig, newCustomConfig)

			})

			It("should run successfully", runWithUserProvidedConfig)
		})
	})

	Context("From New ConfigMap", func() {
		var userConfig *v1.ConfigMap
		var newUserConfig *v1.ConfigMap
		var newCustomConfig *dbaapi.MongoDBCustomConfig

		BeforeEach(func() {
			prevConfigName := to.App() + "-previous-config"
			newConfigName := to.App() + "-new-config"
			userConfig = to.GetCustomConfig(customConfigs, prevConfigName)
			newUserConfig = to.GetCustomConfig(newCustomConfigs, newConfigName)
			newCustomConfig = &dbaapi.MongoDBCustomConfig{
				ConfigMap: &v1.LocalObjectReference{
					Name: newUserConfig.Name,
				},
			}
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

			By("Creating configMap: " + newUserConfig.Name)
			err = to.CreateConfigMap(newUserConfig)
			Expect(err).NotTo(HaveOccurred())

			to.createAndWaitForRunning()

			By("Checking maxIncomingConnections from mongodb config")
			to.EventuallyMaxIncomingConnections(to.mongodb.ObjectMeta).Should(Equal(prevMaxIncomingConnections))

			By("Insert Document Inside DB")
			to.EventuallyInsertDocument(to.mongodb.ObjectMeta, dbName, 3).Should(BeTrue())

			By("Checking Inserted Document")
			to.EventuallyDocumentExists(to.mongodb.ObjectMeta, dbName, 3).Should(BeTrue())

			By("Updating MongoDB")
			err = to.CreateMongoDBOpsRequest(to.mongoOpsReq)
			Expect(err).NotTo(HaveOccurred())

			By("Waiting for MongoDB Ops Request Phase to be Successful")
			to.EventuallyMongoDBOpsRequestPhase(to.mongoOpsReq.ObjectMeta).Should(Equal(dbaapi.OpsRequestPhaseSuccessful))

			// Retrieve Inserted Data
			By("Checking Inserted Document after update")
			to.EventuallyDocumentExists(to.mongodb.ObjectMeta, dbName, 3).Should(BeTrue())

			By("Checking updated maxIncomingConnections from mongodb config")
			to.EventuallyMaxIncomingConnections(to.mongodb.ObjectMeta).Should(Equal(newMaxIncomingConnections))
		}

		Context("Standalone MongoDB", func() {
			BeforeEach(func() {
				to.mongodb = to.MongoDBStandalone()
				to.mongodb.Spec.Version = framework.DBVersion
				to.mongodb.Spec.TerminationPolicy = api.TerminationPolicyWipeOut
				to.mongodb.Spec.ConfigSource = &v1.VolumeSource{
					ConfigMap: &v1.ConfigMapVolumeSource{
						LocalObjectReference: v1.LocalObjectReference{
							Name: userConfig.Name,
						},
					},
				}
				to.mongoOpsReq = to.MongoDBOpsRequestReconfigure(to.mongodb.Name, to.mongodb.Namespace, newCustomConfig, nil, nil, nil, nil)
			})

			It("should run successfully", runWithUserProvidedConfig)
		})

		Context("With Replica Set", func() {
			BeforeEach(func() {
				to.mongodb = to.MongoDBRS()
				to.mongodb.Spec.ConfigSource = &v1.VolumeSource{
					ConfigMap: &v1.ConfigMapVolumeSource{
						LocalObjectReference: v1.LocalObjectReference{
							Name: userConfig.Name,
						},
					},
				}
				to.mongoOpsReq = to.MongoDBOpsRequestReconfigure(to.mongodb.Name, to.mongodb.Namespace, nil, newCustomConfig, nil, nil, nil)
			})

			It("should run successfully", runWithUserProvidedConfig)
		})

		Context("With Sharding", func() {
			BeforeEach(func() {
				to.mongodb = to.MongoDBShard()
				to.mongodb.Spec.Version = framework.DBVersion
				to.mongodb.Spec.TerminationPolicy = api.TerminationPolicyWipeOut
				to.mongodb.Spec.ShardTopology.Shard.ConfigSource = &v1.VolumeSource{
					ConfigMap: &v1.ConfigMapVolumeSource{
						LocalObjectReference: v1.LocalObjectReference{
							Name: userConfig.Name,
						},
					},
				}
				to.mongodb.Spec.ShardTopology.ConfigServer.ConfigSource = &v1.VolumeSource{
					ConfigMap: &v1.ConfigMapVolumeSource{
						LocalObjectReference: v1.LocalObjectReference{
							Name: userConfig.Name,
						},
					},
				}
				to.mongodb.Spec.ShardTopology.Mongos.ConfigSource = &v1.VolumeSource{
					ConfigMap: &v1.ConfigMapVolumeSource{
						LocalObjectReference: v1.LocalObjectReference{
							Name: userConfig.Name,
						},
					},
				}

				to.mongoOpsReq = to.MongoDBOpsRequestReconfigure(to.mongodb.Name, to.mongodb.Namespace, nil, nil, newCustomConfig, newCustomConfig, newCustomConfig)
			})

			It("should run successfully", runWithUserProvidedConfig)
		})
	})
})
