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

var _ = Describe("Exporter With SSL", func() {
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
			clusterAuthMode:  nil,
			sslMode:          nil,
			garbageCASecrets: []*core.Secret{},
		}

		if to.StorageClass == "" {
			Skip("Missing StorageClassName. Provide as flag to test this.")
		}
		if !framework.SSLEnabled {
			Skip("Enable SSL to test this")
		}
		if !framework.RunTestCommunity(testName) {
			Skip(fmt.Sprintf("Provide test profile `%s` or `all` to test this.", testName))
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

	var verifyExporter = func() {
		to.mongodb.Spec.SSLMode = *to.sslMode
		By("Add monitoring configurations to mongodb")
		to.AddMonitor(to.mongodb)
		// Create MongoDB
		to.createAndWaitForReady(true)
		By("Verify exporter")
		err := to.VerifyMongoDBExporter(to.mongodb.ObjectMeta)
		Expect(err).NotTo(HaveOccurred())
		By("Done")
	}

	var verifyShardedExporter = func() {
		to.mongodb.Spec.SSLMode = *to.sslMode
		By("Add monitoring configurations to mongodb")
		to.AddMonitor(to.mongodb)
		// Create MongoDB
		to.createAndWaitForReady(true)
		By("Verify exporter")
		err := to.VerifyShardExporters(to.mongodb.ObjectMeta)
		Expect(err).NotTo(HaveOccurred())
		By("Done")
	}

	Context("Standalone", func() {
		BeforeEach(func() {
			to.mongodb = to.MongoDBStandalone()
		})

		Context("With sslMode requireSSL", func() {
			BeforeEach(func() {
				to.sslMode = framework.SSLModeP(api.SSLModeRequireSSL)
			})
			It("Should verify Exporter", func() {
				verifyExporter()
			})
		})

		Context("With sslMode preferSSL", func() {
			BeforeEach(func() {
				to.sslMode = framework.SSLModeP(api.SSLModePreferSSL)
			})
			It("Should verify Exporter", func() {
				verifyExporter()
			})
		})

		Context("With sslMode allowSSL", func() {
			BeforeEach(func() {
				to.sslMode = framework.SSLModeP(api.SSLModeAllowSSL)
			})
			It("Should verify Exporter", func() {
				verifyExporter()
			})
		})

		Context("With sslMode disableSSL", func() {
			BeforeEach(func() {
				to.sslMode = framework.SSLModeP(api.SSLModeDisabled)
			})
			It("Should verify Exporter", func() {
				verifyExporter()
			})
		})
	})

	Context("Replicas", func() {
		BeforeEach(func() {
			to.mongodb = to.MongoDBRS()
		})

		Context("With sslMode requireSSL", func() {
			BeforeEach(func() {
				to.sslMode = framework.SSLModeP(api.SSLModeRequireSSL)
			})
			It("Should verify Exporter", func() {
				verifyExporter()
			})
		})

		Context("With sslMode preferSSL", func() {
			BeforeEach(func() {
				to.sslMode = framework.SSLModeP(api.SSLModePreferSSL)
			})
			It("Should verify Exporter", func() {
				verifyExporter()
			})
		})

		Context("With sslMode allowSSL", func() {
			BeforeEach(func() {
				to.sslMode = framework.SSLModeP(api.SSLModeAllowSSL)
			})
			It("Should verify Exporter", func() {
				verifyExporter()
			})
		})

		Context("With sslMode disableSSL", func() {
			BeforeEach(func() {
				to.sslMode = framework.SSLModeP(api.SSLModeDisabled)
			})
			It("Should verify Exporter", func() {
				verifyExporter()
			})
		})
	})

	Context("Shards", func() {
		BeforeEach(func() {
			to.mongodb = to.MongoDBShard()
		})
		Context("With sslMode requireSSL", func() {
			BeforeEach(func() {
				to.sslMode = framework.SSLModeP(api.SSLModeRequireSSL)
			})
			It("Should verify Exporter", func() {
				verifyShardedExporter()
			})
		})

		Context("With sslMode preferSSL", func() {
			BeforeEach(func() {
				to.sslMode = framework.SSLModeP(api.SSLModePreferSSL)
			})
			It("Should verify Exporter", func() {
				verifyShardedExporter()
			})
		})

		Context("With sslMode allowSSL", func() {
			BeforeEach(func() {
				to.sslMode = framework.SSLModeP(api.SSLModeAllowSSL)
			})
			It("Should verify Exporter", func() {
				verifyShardedExporter()
			})
		})

		Context("With sslMode disableSSL", func() {
			BeforeEach(func() {
				to.sslMode = framework.SSLModeP(api.SSLModeDisabled)
			})
			It("Should verify Exporter", func() {
				verifyShardedExporter()
			})
		})
	})
})
