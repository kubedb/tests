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

package redis

import (
	"fmt"
	"strings"
	"time"

	api "kubedb.dev/apimachinery/apis/kubedb/v1alpha2"
	"kubedb.dev/tests/e2e/framework"

	"github.com/appscode/go/crypto/rand"
	"github.com/appscode/go/types"
	. "github.com/onsi/ginkgo"
	. "github.com/onsi/gomega"
	core "k8s.io/api/core/v1"
	rbac "k8s.io/api/rbac/v1"
	kerr "k8s.io/apimachinery/pkg/api/errors"
	"k8s.io/klog/v2"
)

var _ = Describe("General Redis", func() {
	var (
		err   error
		key   string
		value string
	)
	to := testOptions{}
	testName := framework.General

	BeforeEach(func() {
		to.Invocation = framework.NewInvocation()
		if !runTestCommunity(testName) {
			Skip(fmt.Sprintf("Provide test profile `%s` or `all` or `community` to test this.", testName))
		}
		if framework.SSLEnabled && !strings.HasPrefix(framework.DBVersion, "6.") {
			Skip(fmt.Sprintf("TLS is not supported for version `%s` in redis", framework.DBVersion))
		}

		to.redis = to.RedisStandalone()
		to.skipMessage = ""
		key = rand.WithUniqSuffix("kubed-e2e")
		value = rand.GenerateTokenWithLength(10)
	})

	JustAfterEach(func() {
		if CurrentGinkgoTestDescription().Failed {
			to.PrintDebugHelpers()
		}
	})

	AfterEach(func() {
		err = to.CleanupTestResources()
		Expect(err).NotTo(HaveOccurred())

		if to.redis == nil {
			// No redis. So, no cleanup
			return
		}

		By("Check if Redis " + to.redis.Name + " exists.")
		rd, err := to.GetRedis(to.redis.ObjectMeta)
		if err != nil {
			if kerr.IsNotFound(err) {
				// Redis was not created. Hence, rest of cleanup is not necessary.
				return
			}
			Expect(err).NotTo(HaveOccurred())
		}

		By("Update redis to set spec.terminationPolicy = WipeOut")
		_, err = to.PatchRedis(rd.ObjectMeta, func(in *api.Redis) *api.Redis {
			in.Spec.TerminationPolicy = api.TerminationPolicyWipeOut
			return in
		})
		Expect(err).NotTo(HaveOccurred())

		By("Delete redis")
		err = to.DeleteRedis(to.redis.ObjectMeta)
		Expect(err).NotTo(HaveOccurred())

		By("Wait for redis to be deleted")
		to.EventuallyRedis(to.redis.ObjectMeta).Should(BeFalse())

		By("Wait for redis resources to be wipedOut")
		to.EventuallyWipedOut(to.redis.ObjectMeta, api.Redis{}.ResourceFQN()).Should(Succeed())
	})

	var shouldBeSuccessfullyRunning = func() {
		if to.skipMessage != "" {
			Skip(to.skipMessage)
		}

		to.createRedis()

		res, err := to.TestConfig().GetPingResult(to.redis)
		Expect(err).NotTo(HaveOccurred())
		Expect(res).To(Equal("PONG"))

		By("Inserting item into database")
		to.EventuallySetItem(to.redis, key, value).Should(BeTrue())

		By("Retrieving item from database")
		to.EventuallyGetItem(to.redis, key).Should(BeEquivalentTo(value))
	}

	Context("-", func() {
		It("should run successfully", func() {

			shouldBeSuccessfullyRunning()

			By("Delete redis")
			err = to.DeleteRedis(to.redis.ObjectMeta)
			Expect(err).NotTo(HaveOccurred())

			By("Wait for redis to be deleted")
			to.EventuallyRedis(to.redis.ObjectMeta).Should(BeFalse())

			// Create Redis object again to resume it
			to.createRedis()

			By("Retrieving item from database")
			to.EventuallyGetItem(to.redis, key).Should(BeEquivalentTo(value))

		})
	})

	Context("PDB", func() {

		It("should run eviction successfully", func() {
			// Create Redis
			By("Create DB")
			to.createRedis()
			//Evict Redis pod
			By("Try to evict Pod")
			err := to.EvictPodsFromStatefulSetRedis(to.redis.ObjectMeta)
			Expect(err).NotTo(HaveOccurred())
		})

		It("should run eviction with shard successfully", func() {
			to.redis = to.RedisCluster(types.Int32P(3), types.Int32P(3))

			// Create Redis
			By("Create DB")
			to.createRedis()

			// Wait for some more time until the 2nd thread of operator completes
			By(fmt.Sprintf("Wait for operator to complete processing the key %s/%s", to.redis.Namespace, to.redis.Name))
			time.Sleep(time.Minute * 1)

			//Evict a Redis pod from each sts and deploy
			By("Try to evict Pod")
			err := to.EvictPodsFromStatefulSetRedis(to.redis.ObjectMeta)
			Expect(err).NotTo(HaveOccurred())
		})
	})

	Context("Custom Resources", func() {

		Context("with custom SA Name", func() {
			BeforeEach(func() {
				to.redis.Spec.PodTemplate.Spec.ServiceAccountName = "my-custom-sa"
				to.redis.Spec.TerminationPolicy = api.TerminationPolicyHalt
			})

			It("should start and resume successfully", func() {
				//shouldTakeSnapshot()
				to.createRedis()
				By("Check if Redis " + to.redis.Name + " exists.")
				_, err := to.GetRedis(to.redis.ObjectMeta)
				if err != nil {
					if kerr.IsNotFound(err) {
						// Redis was not created. Hence, rest of cleanup is not necessary.
						return
					}
					Expect(err).NotTo(HaveOccurred())
				}

				By("Delete redis: " + to.redis.Name)
				err = to.DeleteRedis(to.redis.ObjectMeta)
				if err != nil {
					if kerr.IsNotFound(err) {
						// Redis was not created. Hence, rest of cleanup is not necessary.
						klog.Infof("Skipping rest of cleanup. Reason: Redis %s is not found.", to.redis.Name)
						return
					}
					Expect(err).NotTo(HaveOccurred())
				}

				By("Wait for redis to be deleted")
				to.EventuallyRedis(to.redis.ObjectMeta).Should(BeFalse())

				By("Resume DB")
				to.createRedis()
			})
		})

		Context("with custom SA", func() {
			var customSAForDB *core.ServiceAccount
			var customRoleForDB *rbac.Role
			var customRoleBindingForDB *rbac.RoleBinding
			BeforeEach(func() {
				customSAForDB = to.ServiceAccount()
				to.redis.Spec.PodTemplate.Spec.ServiceAccountName = customSAForDB.Name
				customRoleForDB = to.RoleForMongoDB(to.redis.ObjectMeta)
				customRoleBindingForDB = to.RoleBinding(customSAForDB.Name, customRoleForDB.Name)
			})
			It("should take snapshot successfully", func() {
				By("Create Database SA")
				err = to.CreateServiceAccount(customSAForDB)
				Expect(err).NotTo(HaveOccurred())

				By("Create Database Role")
				err = to.CreateRole(customRoleForDB)
				Expect(err).NotTo(HaveOccurred())

				By("Create Database RoleBinding")
				err = to.CreateRoleBinding(customRoleBindingForDB)
				Expect(err).NotTo(HaveOccurred())

				to.createRedis()
			})
		})
	})
})
