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

var _ = Describe("Exporter", func() {
	var err error
	to := testOptions{}
	testName := framework.Exporter

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
			to.PrintDebugHelper()
		}
	})
	Context("Standalone", func() {
		BeforeEach(func() {
			to.mongodb = to.MongoDBStandalone()
		})
		It("should export selected metrics", func() {
			By("Add monitoring configurations to mongodb")
			to.AddMonitor(to.mongodb)
			// Create MongoDB
			to.createAndWaitForRunning()
			By("Verify exporter")
			err = to.VerifyExporter(to.mongodb.ObjectMeta)
			Expect(err).NotTo(HaveOccurred())
			By("Done")
		})
	})

	Context("Replicas", func() {
		BeforeEach(func() {
			to.mongodb = to.MongoDBRS()
		})
		It("should export selected metrics", func() {
			By("Add monitoring configurations to mongodb")
			to.AddMonitor(to.mongodb)
			// Create MongoDB
			to.createAndWaitForRunning()
			By("Verify exporter")
			err = to.VerifyExporter(to.mongodb.ObjectMeta)
			Expect(err).NotTo(HaveOccurred())
			By("Done")
		})
	})

	Context("Shards", func() {
		BeforeEach(func() {
			to.mongodb = to.MongoDBShard()
		})
		It("should export selected metrics", func() {
			By("Add monitoring configurations to mongodb")
			to.AddMonitor(to.mongodb)
			// Create MongoDB
			to.createAndWaitForRunning()
			By("Verify exporter")
			err = to.VerifyShardExporters(to.mongodb.ObjectMeta)
			Expect(err).NotTo(HaveOccurred())
			By("Done")
		})
	})
})
