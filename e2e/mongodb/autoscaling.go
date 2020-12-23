package e2e_test

import (
	"fmt"
	"time"

	"kubedb.dev/apimachinery/apis/autoscaling/v1alpha1"
	api "kubedb.dev/apimachinery/apis/kubedb/v1alpha2"
	"kubedb.dev/tests/e2e/framework"

	. "github.com/onsi/ginkgo"
	. "github.com/onsi/gomega"
	v1 "k8s.io/apimachinery/pkg/apis/meta/v1"
)

var _ = Describe("MongoDB Autoscaling", func() {
	to := testOptions{}
	provisioner := "topolvm-provisioner"
	testName := framework.Autoscaling
	computeAutoscalerSpec := v1alpha1.ComputeAutoscalerSpec{
		Trigger:                v1alpha1.AutoscalerTriggerOn,
		ResourceDiffPercentage: 5,
		PodLifeTimeThreshold: v1.Duration{
			Duration: 3 * time.Minute,
		},
	}

	storageAutoscalerSpec := v1alpha1.StorageAutoscalerSpec{
		Trigger:          v1alpha1.AutoscalerTriggerOn,
		UsageThreshold:   50,
		ScalingThreshold: 100,
	}

	BeforeEach(func() {
		to.Invocation = framework.NewInvocation()
		if !framework.RunTestEnterprise(testName) {
			Skip(fmt.Sprintf("Provide test profile `%s` or `all` or `enterprise` to test this.", testName))
		}
	})

	AfterEach(func() {
		err := to.CleanupTestResources()
		Expect(err).NotTo(HaveOccurred())
		//Delete MongoDB
		By("Delete mongodb")
		err = to.DeleteMongoDB(to.mongodb.ObjectMeta)
		Expect(err).NotTo(HaveOccurred())

		By("Delete mongodb Autoscaler")
		err = to.DeleteMongoDBAutoscaler(to.mgAutoscaler.ObjectMeta)
		Expect(err).NotTo(HaveOccurred())

		By("Wait for mongodb resources to be wipedOut")
		to.EventuallyWipedOut(to.mongodb.ObjectMeta, api.ResourceKindMongoDB).Should(Succeed())
	})

	Context("Compute", func() {
		Context("Standalone Instance", func() {
			BeforeEach(func() {
				to.mongodb = to.MongoDBStandalone()
				to.mongodb.Spec.Version = framework.DBVersion
				to.mongodb.Spec.TerminationPolicy = api.TerminationPolicyWipeOut
				to.mgAutoscaler = to.MongoDBAutoscalerCompute(to.mongodb.Name, to.mongodb.Namespace, &computeAutoscalerSpec, nil, nil, nil, nil)
			})

			It("Should Scale StandAlone Mongodb Resources", func() {
				to.shouldTestComputeAutoscaler()
			})

		})
		Context("ReplicaSet Cluster", func() {
			BeforeEach(func() {
				to.mongodb = to.MongoDBRS()
				to.mongodb.Spec.Version = framework.DBVersion
				to.mongodb.Spec.TerminationPolicy = api.TerminationPolicyWipeOut
				to.mgAutoscaler = to.MongoDBAutoscalerCompute(to.mongodb.Name, to.mongodb.Namespace, nil, &computeAutoscalerSpec, nil, nil, nil)
			})

			It("Should Scale ReplicaSet Resources", func() {
				to.shouldTestComputeAutoscaler()
			})
		})
		Context("Scaling ConfigServer Resources", func() {
			BeforeEach(func() {
				to.mongodb = to.MongoDBShard()
				to.mongodb.Spec.Version = framework.DBVersion
				to.mongodb.Spec.TerminationPolicy = api.TerminationPolicyWipeOut
				to.mgAutoscaler = to.MongoDBAutoscalerCompute(to.mongodb.Name, to.mongodb.Namespace, nil, nil, nil, &computeAutoscalerSpec, nil)
			})

			It("Should Scale ConfigServer Resources", func() {
				to.shouldTestComputeAutoscaler()
			})

		})
		Context("Scaling Shard Resources", func() {
			BeforeEach(func() {
				to.mongodb = to.MongoDBShard()
				to.mongodb.Spec.Version = framework.DBVersion
				to.mongodb.Spec.TerminationPolicy = api.TerminationPolicyWipeOut
				to.mgAutoscaler = to.MongoDBAutoscalerCompute(to.mongodb.Name, to.mongodb.Namespace, nil, nil, &computeAutoscalerSpec, nil, nil)
			})

			It("Should Scale Shard Resources", func() {
				to.shouldTestComputeAutoscaler()
			})
		})
		Context("Scaling Mongos Resources", func() {
			BeforeEach(func() {
				to.mongodb = to.MongoDBShard()
				to.mongodb.Spec.Version = framework.DBVersion
				to.mongodb.Spec.TerminationPolicy = api.TerminationPolicyWipeOut
				to.mgAutoscaler = to.MongoDBAutoscalerCompute(to.mongodb.Name, to.mongodb.Namespace, nil, nil, nil, nil, &computeAutoscalerSpec)
			})

			It("Should Scale Mongos Resources", func() {
				to.shouldTestComputeAutoscaler()
			})
		})
	})

	Context("Storage", func() {
		Context("Standalone Instance", func() {
			BeforeEach(func() {
				to.mongodb = to.MongoDBStandalone()
				to.mongodb.Spec.Storage.StorageClassName = &provisioner
				to.mongodb.Spec.Version = framework.DBVersion
				to.mongodb.Spec.TerminationPolicy = api.TerminationPolicyWipeOut
				to.mgAutoscaler = to.MongoDBAutoscalerStorage(to.mongodb.Name, to.mongodb.Namespace, &storageAutoscalerSpec, nil, nil, nil)
			})

			It("Should Scale StandAlone Mongodb Resources", func() {
				to.shouldTestStorageAutoscaler()
			})

		})
		Context("ReplicaSet Cluster", func() {
			BeforeEach(func() {
				to.mongodb = to.MongoDBRS()
				to.mongodb.Spec.Storage.StorageClassName = &provisioner
				to.mongodb.Spec.Version = framework.DBVersion
				to.mongodb.Spec.TerminationPolicy = api.TerminationPolicyWipeOut
				to.mgAutoscaler = to.MongoDBAutoscalerStorage(to.mongodb.Name, to.mongodb.Namespace, nil, &storageAutoscalerSpec, nil, nil)
			})

			It("Should Scale ReplicaSet Resources", func() {
				to.shouldTestStorageAutoscaler()
			})
		})
		Context("Scaling ConfigServer Resources", func() {
			BeforeEach(func() {
				to.mongodb = to.MongoDBShard()
				to.mongodb.Spec.ShardTopology.ConfigServer.Storage.StorageClassName = &provisioner
				to.mongodb.Spec.Version = framework.DBVersion
				to.mongodb.Spec.TerminationPolicy = api.TerminationPolicyWipeOut
				to.mgAutoscaler = to.MongoDBAutoscalerStorage(to.mongodb.Name, to.mongodb.Namespace, nil, nil, nil, &storageAutoscalerSpec)
			})

			It("Should Scale ConfigServer Resources", func() {
				to.shouldTestStorageAutoscaler()
			})

		})
		Context("Scaling Shard Resources", func() {
			BeforeEach(func() {
				to.mongodb = to.MongoDBShard()
				to.mongodb.Spec.ShardTopology.Shard.Storage.StorageClassName = &provisioner
				to.mongodb.Spec.Version = framework.DBVersion
				to.mongodb.Spec.TerminationPolicy = api.TerminationPolicyWipeOut
				to.mgAutoscaler = to.MongoDBAutoscalerStorage(to.mongodb.Name, to.mongodb.Namespace, nil, nil, &storageAutoscalerSpec, nil)
			})

			It("Should Scale Shard Resources", func() {
				to.shouldTestStorageAutoscaler()
			})
		})
	})
})
