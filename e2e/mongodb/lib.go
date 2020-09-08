package e2e_test


import (
	api "kubedb.dev/apimachinery/apis/kubedb/v1alpha1"
	"kubedb.dev/tests/e2e/framework"

	"github.com/appscode/go/types"
	cm_api "github.com/jetstack/cert-manager/pkg/apis/certmanager/v1alpha2"
	. "github.com/onsi/ginkgo"
	. "github.com/onsi/gomega"
	core "k8s.io/api/core/v1"
	kerr "k8s.io/apimachinery/pkg/api/errors"
	kmapi "kmodules.xyz/client-go/api/v1"
)

func shouldRunWithPVC2(	f                *framework.Invocation, mongodb *api.MongoDB) interface{} {
	return func() {
		if skipMessage != "" {
			Skip(skipMessage)
		}
		// Create MongoDB
		createAndWaitForRunning()

		By("Checking SSL settings (if enabled any)")
		f.EventuallyUserSSLSettings(mongodb.ObjectMeta, clusterAuthMode, sslMode).Should(BeTrue())

		if enableSharding {
			By("Enable sharding for db:" + dbName)
			f.EventuallyEnableSharding(mongodb.ObjectMeta, dbName).Should(BeTrue())
		}
		if verifySharding {
			By("Check if db " + dbName + " is set to partitioned")
			f.EventuallyCollectionPartitioned(mongodb.ObjectMeta, dbName).Should(Equal(enableSharding))
		}

		By("Insert Document Inside DB")
		f.EventuallyInsertDocument(mongodb.ObjectMeta, dbName, 3).Should(BeTrue())

		By("Checking Inserted Document")
		f.EventuallyDocumentExists(mongodb.ObjectMeta, dbName, 3).Should(BeTrue())

		By("Delete mongodb")
		err = f.DeleteMongoDB(mongodb.ObjectMeta)
		Expect(err).NotTo(HaveOccurred())

		By("Wait for mongodb to be paused")
		f.EventuallyMongoDB(mongodb.ObjectMeta).Should(BeFalse())

		// Create MongoDB object again to resume it
		By("Create MongoDB: " + mongodb.Name)
		err = f.CreateMongoDB(mongodb)
		Expect(err).NotTo(HaveOccurred())

		By("Wait for Running mongodb")
		f.EventuallyMongoDBRunning(mongodb.ObjectMeta).Should(BeTrue())

		By("Ping mongodb database")
		f.EventuallyPingMongo(mongodb.ObjectMeta)

		if verifySharding {
			By("Check if db " + dbName + " is set to partitioned")
			f.EventuallyCollectionPartitioned(mongodb.ObjectMeta, dbName).Should(Equal(enableSharding))
		}

		By("Checking Inserted Document")
		f.EventuallyDocumentExists(mongodb.ObjectMeta, dbName, 3).Should(BeTrue())
	}
}