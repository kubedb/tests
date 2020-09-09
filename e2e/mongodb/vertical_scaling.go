package e2e_test

import (
	"fmt"

	api "kubedb.dev/apimachinery/apis/kubedb/v1alpha1"
	"kubedb.dev/tests/e2e/framework"

	. "github.com/onsi/ginkgo"
	. "github.com/onsi/gomega"
	v1 "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/api/resource"
)

var _ = Describe("Vertical Scaling", func() {
	to := testOptions{}
	testName := framework.VerticalScaling
	BeforeEach(func() {
		to.Invocation = framework.NewInvocation()
		if !runTestEnterprise(testName) {
			Skip(fmt.Sprintf("Provide test profile `%s` or `all` or `enterprise` to test this.", testName))
		}
	})

	AfterEach(func() {
		err := to.CleanupTestResources()
		Expect(err).NotTo(HaveOccurred())
		By("Delete mongodb")
		err = to.DeleteMongoDB(to.mongodb.ObjectMeta)
		Expect(err).NotTo(HaveOccurred())

		By("Delete mongodb ops request")
		err = to.DeleteMongoDBOpsRequest(to.mongoOpsReq.ObjectMeta)
		Expect(err).NotTo(HaveOccurred())

		By("Wait for mongodb resources to be wipedOut")
		to.EventuallyWipedOut(to.mongodb.ObjectMeta).Should(Succeed())
	})

	Context("Scaling StandAlone Mongodb Resources", func() {
		BeforeEach(func() {
			to.mongodb = to.MongoDBStandalone()
			to.mongodb.Spec.Version = framework.DBVersion
			to.mongodb.Spec.TerminationPolicy = api.TerminationPolicyWipeOut
			standalone := &v1.ResourceRequirements{
				Limits: map[v1.ResourceName]resource.Quantity{
					v1.ResourceMemory: resource.MustParse("300Mi"),
					v1.ResourceCPU:    resource.MustParse(".2"),
				},
				Requests: map[v1.ResourceName]resource.Quantity{
					v1.ResourceMemory: resource.MustParse("200Mi"),
					v1.ResourceCPU:    resource.MustParse(".1"),
				},
			}
			to.mongoOpsReq = to.MongoDBOpsRequestVerticalScale(to.mongodb.Name, to.mongodb.Namespace, standalone, nil, nil, nil, nil, nil)
		})

		It("Should Scale StandAlone Mongodb Resources", func() {
			to.shouldTestOpsRequest()
		})

	})
	Context("Scaling ReplicaSet Resources", func() {
		BeforeEach(func() {
			to.mongodb = to.MongoDBRS()
			to.mongodb.Spec.Version = framework.DBVersion
			to.mongodb.Spec.TerminationPolicy = api.TerminationPolicyWipeOut
			replicaset := &v1.ResourceRequirements{
				Limits: map[v1.ResourceName]resource.Quantity{
					v1.ResourceMemory: resource.MustParse("300Mi"),
					v1.ResourceCPU:    resource.MustParse(".2"),
				},
				Requests: map[v1.ResourceName]resource.Quantity{
					v1.ResourceMemory: resource.MustParse("200Mi"),
					v1.ResourceCPU:    resource.MustParse(".1"),
				},
			}
			to.mongoOpsReq = to.MongoDBOpsRequestVerticalScale(to.mongodb.Name, to.mongodb.Namespace, nil, replicaset, nil, nil, nil, nil)
		})

		It("Should Scale ReplicaSet Resources", func() {
			to.shouldTestOpsRequest()

		})

	})
	Context("Scaling Mongos Resources", func() {
		BeforeEach(func() {
			to.mongodb = to.MongoDBShard()
			to.mongodb.Spec.Version = framework.DBVersion
			to.mongodb.Spec.TerminationPolicy = api.TerminationPolicyWipeOut
			mongos := &v1.ResourceRequirements{
				Limits: map[v1.ResourceName]resource.Quantity{
					v1.ResourceMemory: resource.MustParse("300Mi"),
					v1.ResourceCPU:    resource.MustParse(".2"),
				},
				Requests: map[v1.ResourceName]resource.Quantity{
					v1.ResourceMemory: resource.MustParse("200Mi"),
					v1.ResourceCPU:    resource.MustParse(".1"),
				},
			}
			to.mongoOpsReq = to.MongoDBOpsRequestVerticalScale(to.mongodb.Name, to.mongodb.Namespace, nil, nil, mongos, nil, nil, nil)
		})

		It("Should Scale Mongos Resources", func() {
			to.shouldTestOpsRequest()
		})

	})
	Context("Scaling ConfigServer Resources", func() {
		BeforeEach(func() {
			to.mongodb = to.MongoDBShard()
			to.mongodb.Spec.Version = framework.DBVersion
			to.mongodb.Spec.TerminationPolicy = api.TerminationPolicyWipeOut
			configServer := &v1.ResourceRequirements{
				Limits: map[v1.ResourceName]resource.Quantity{
					v1.ResourceMemory: resource.MustParse("300Mi"),
					v1.ResourceCPU:    resource.MustParse(".2"),
				},
				Requests: map[v1.ResourceName]resource.Quantity{
					v1.ResourceMemory: resource.MustParse("200Mi"),
					v1.ResourceCPU:    resource.MustParse(".1"),
				},
			}
			to.mongoOpsReq = to.MongoDBOpsRequestVerticalScale(to.mongodb.Name, to.mongodb.Namespace, nil, nil, nil, configServer, nil, nil)
		})

		It("Should Scale ConfigServer Resources", func() {
			to.shouldTestOpsRequest()
		})

	})
	Context("Scaling Shard Resources", func() {
		BeforeEach(func() {
			to.mongodb = to.MongoDBShard()
			to.mongodb.Spec.Version = framework.DBVersion
			to.mongodb.Spec.TerminationPolicy = api.TerminationPolicyWipeOut
			shard := &v1.ResourceRequirements{
				Limits: map[v1.ResourceName]resource.Quantity{
					v1.ResourceMemory: resource.MustParse("300Mi"),
					v1.ResourceCPU:    resource.MustParse(".2"),
				},
				Requests: map[v1.ResourceName]resource.Quantity{
					v1.ResourceMemory: resource.MustParse("200Mi"),
					v1.ResourceCPU:    resource.MustParse(".1"),
				},
			}
			to.mongoOpsReq = to.MongoDBOpsRequestVerticalScale(to.mongodb.Name, to.mongodb.Namespace, nil, nil, nil, nil, shard, nil)
		})

		It("Should Scale Shard Resources", func() {
			to.shouldTestOpsRequest()
		})

	})
	Context("Scaling All Mongos,ConfigServer and Shard Resources", func() {
		BeforeEach(func() {
			to.mongodb = to.MongoDBShard()
			to.mongodb.Spec.Version = framework.DBVersion
			to.mongodb.Spec.TerminationPolicy = api.TerminationPolicyWipeOut
			resource := &v1.ResourceRequirements{
				Limits: map[v1.ResourceName]resource.Quantity{
					v1.ResourceMemory: resource.MustParse("300Mi"),
					v1.ResourceCPU:    resource.MustParse(".2"),
				},
				Requests: map[v1.ResourceName]resource.Quantity{
					v1.ResourceMemory: resource.MustParse("200Mi"),
					v1.ResourceCPU:    resource.MustParse(".1"),
				},
			}
			to.mongoOpsReq = to.MongoDBOpsRequestVerticalScale(to.mongodb.Name, to.mongodb.Namespace, nil, nil, resource, resource, resource, nil)
		})

		It("Should Scale All Mongos,ConfigServer and Shard Resources", func() {
			to.shouldTestOpsRequest()
		})

	})
})
