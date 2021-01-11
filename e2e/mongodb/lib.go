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

	"kubedb.dev/apimachinery/apis/autoscaling/v1alpha1"
	api "kubedb.dev/apimachinery/apis/kubedb/v1alpha2"
	dbaapi "kubedb.dev/apimachinery/apis/ops/v1alpha1"
	"kubedb.dev/tests/e2e/framework"

	"github.com/appscode/go/types"
	cm_api "github.com/jetstack/cert-manager/pkg/apis/certmanager/v1beta1"
	. "github.com/onsi/ginkgo"
	. "github.com/onsi/gomega"
	core "k8s.io/api/core/v1"
	kerr "k8s.io/apimachinery/pkg/api/errors"
	kmapi "kmodules.xyz/client-go/api/v1"
)

const (
	MONGO_INITDB_ROOT_USERNAME = "MONGO_INITDB_ROOT_USERNAME"
	MONGO_INITDB_ROOT_PASSWORD = "MONGO_INITDB_ROOT_PASSWORD"
	MONGO_INITDB_DATABASE      = "MONGO_INITDB_DATABASE"
)

var (
	dbName                     = "kubedb"
	newMaxIncomingConnections  = int32(20000)
	prevMaxIncomingConnections = int32(10000)
	customConfigs              = []string{
		fmt.Sprintf(`   maxIncomingConnections: %v`, prevMaxIncomingConnections),
	}
	newCustomConfigs = []string{
		fmt.Sprintf(`   maxIncomingConnections: %v`, newMaxIncomingConnections),
	}
	inlineConfig = fmt.Sprintf(`net:
   maxIncomingConnections: %v`, newMaxIncomingConnections)
)

type testOptions struct {
	*framework.Invocation
	mongodb          *api.MongoDB
	mongoOpsReq      *dbaapi.MongoDBOpsRequest
	mgAutoscaler     *v1alpha1.MongoDBAutoscaler
	skipMessage      string
	garbageMongoDB   *api.MongoDBList
	snapshotPVC      *core.PersistentVolumeClaim
	secret           *core.Secret
	verifySharding   bool
	enableSharding   bool
	clusterAuthMode  *api.ClusterAuthMode
	sslMode          *api.SSLMode
	garbageCASecrets []*core.Secret
}

func (to *testOptions) addIssuerRef() {
	//create cert-manager ca secret
	issuer, err := to.InsureIssuer(to.mongodb.ObjectMeta, api.MongoDB{}.ResourceFQN())
	Expect(err).NotTo(HaveOccurred())
	to.mongodb.Spec.TLS = &kmapi.TLSConfig{
		IssuerRef: &core.TypedLocalObjectReference{
			Name:     issuer.Name,
			Kind:     "Issuer",
			APIGroup: types.StringP(cm_api.SchemeGroupVersion.Group), //cert-manger.io
		},
	}
}

func (to *testOptions) createAndWaitForReady(ignoreSSL ...bool) {
	if to.skipMessage != "" {
		Skip(to.skipMessage)
	}

	if framework.SSLEnabled && len(ignoreSSL) == 0 {
		to.mongodb.Spec.SSLMode = api.SSLModeRequireSSL
		to.addIssuerRef()
	}

	By("Create MongoDB: " + to.mongodb.Name)
	err := to.CreateMongoDB(to.mongodb)
	Expect(err).NotTo(HaveOccurred())

	By("Wait for Ready mongodb")
	to.EventuallyMongoDBReady(to.mongodb.ObjectMeta).Should(BeTrue())

	By("Wait for AppBinding to create")
	to.EventuallyAppBinding(to.mongodb.ObjectMeta).Should(BeTrue())

	By("Check valid AppBinding Specs")
	err = to.CheckMongoDBAppBindingSpec(to.mongodb.ObjectMeta)
	Expect(err).NotTo(HaveOccurred())

	By("Ping mongodb database")
	to.EventuallyPingMongo(to.mongodb.ObjectMeta)
}

func (to *testOptions) createAndInsertData() {
	// Create MongoDB
	to.createAndWaitForReady()

	By("Insert Document Inside DB")
	to.EventuallyInsertDocument(to.mongodb.ObjectMeta, dbName, 1).Should(BeTrue())

	By("Checking Inserted Document")
	to.EventuallyDocumentExists(to.mongodb.ObjectMeta, dbName, 1).Should(BeTrue())
}

func (to *testOptions) shouldTestOpsRequest() {
	// Create MongoDB
	to.createAndWaitForReady()

	// Insert Data
	By("Insert Document Inside DB")
	to.EventuallyInsertDocument(to.mongodb.ObjectMeta, dbName, 3).Should(BeTrue())

	By("Checking Inserted Document")
	to.EventuallyDocumentExists(to.mongodb.ObjectMeta, dbName, 3).Should(BeTrue())

	// Update Database
	By("Updating MongoDB")
	err := to.CreateMongoDBOpsRequest(to.mongoOpsReq)
	Expect(err).NotTo(HaveOccurred())

	By("Waiting for MongoDB Ops Request Phase to be Successful")
	to.EventuallyMongoDBOpsRequestPhase(to.mongoOpsReq.ObjectMeta).Should(Equal(dbaapi.OpsRequestPhaseSuccessful))

	// Retrieve Inserted Data
	By("Checking Inserted Document after update")
	to.EventuallyDocumentExists(to.mongodb.ObjectMeta, dbName, 3).Should(BeTrue())

	By("Checking DB is Resumed")
	to.EventuallyDatabaseResumed(to.mongodb).Should(BeTrue())
}

func (to *testOptions) shouldTestComputeAutoscaler() {
	// Create MongoDB
	to.createAndWaitForReady()

	// Insert Data
	By("Insert Document Inside DB")
	to.EventuallyInsertDocument(to.mongodb.ObjectMeta, dbName, 3).Should(BeTrue())

	By("Checking Inserted Document")
	to.EventuallyDocumentExists(to.mongodb.ObjectMeta, dbName, 3).Should(BeTrue())

	// Update Database
	By("Creating MongoDB Autoscaler")
	err := to.CreateMongoDBAutoscaler(to.mgAutoscaler)
	Expect(err).NotTo(HaveOccurred())

	By("Wait for Vertical Scaling")
	to.EventuallyVerticallyScaled(to.mongodb.ObjectMeta, to.mgAutoscaler.Spec.Compute).Should(BeTrue())

	// Retrieve Inserted Data
	By("Checking Inserted Document after scaling")
	to.EventuallyDocumentExists(to.mongodb.ObjectMeta, dbName, 3).Should(BeTrue())
}

func (to *testOptions) shouldTestStorageAutoscaler() {
	// Create MongoDB
	to.createAndWaitForReady()

	// Insert Data
	By("Insert Document Inside DB")
	to.EventuallyInsertDocument(to.mongodb.ObjectMeta, dbName, 3).Should(BeTrue())

	By("Checking Inserted Document")
	to.EventuallyDocumentExists(to.mongodb.ObjectMeta, dbName, 3).Should(BeTrue())

	// Update Database
	By("Creating MongoDB Autoscaler")
	err := to.CreateMongoDBAutoscaler(to.mgAutoscaler)
	Expect(err).NotTo(HaveOccurred())

	By("Fill Persistent Volume")
	err = to.FillDisk(to.mongodb, to.mgAutoscaler.Spec.Storage)
	Expect(err).NotTo(HaveOccurred())

	By("Wait for Volume Expansion")
	to.EventuallyVolumeExpanded(to.mongodb, to.mgAutoscaler.Spec.Storage).Should(BeTrue())

	By("Wait for Ready mongodb")
	to.EventuallyMongoDBReady(to.mongodb.ObjectMeta).Should(BeTrue())

	// Retrieve Inserted Data
	By("Checking Inserted Document after scaling")
	to.EventuallyDocumentExists(to.mongodb.ObjectMeta, dbName, 3).Should(BeTrue())
}

func (to *testOptions) deleteTestResource() {
	if to.mongodb == nil {
		Skip("Skipping")
	}

	By("Check if mongodb " + to.mongodb.Name + " exists.")
	mg, err := to.GetMongoDB(to.mongodb.ObjectMeta)
	if err != nil && kerr.IsNotFound(err) {
		// MongoDB was not created. Hence, rest of cleanup is not necessary.
		return
	}
	Expect(err).NotTo(HaveOccurred())

	By("Update mongodb to set spec.terminationPolicy = WipeOut")
	_, err = to.PatchMongoDB(mg.ObjectMeta, func(in *api.MongoDB) *api.MongoDB {
		in.Spec.TerminationPolicy = api.TerminationPolicyWipeOut
		return in
	})
	Expect(err).NotTo(HaveOccurred())

	By("Delete mongodb")
	err = to.DeleteMongoDB(to.mongodb.ObjectMeta)
	if err != nil && kerr.IsNotFound(err) {
		// MongoDB was not created. Hence, rest of cleanup is not necessary.
		return
	}
	Expect(err).NotTo(HaveOccurred())
	By("Delete CA secret")
	to.DeleteGarbageCASecrets(to.garbageCASecrets)

	By("Wait for mongodb to be deleted")
	to.EventuallyMongoDB(to.mongodb.ObjectMeta).Should(BeFalse())

	By("Wait for mongodb resources to be wipedOut")
	to.EventuallyWipedOut(to.mongodb.ObjectMeta, api.MongoDB{}.ResourceFQN()).Should(Succeed())
}

func (to *testOptions) runWithUserProvidedConfig(userConfig, newUserConfig *core.Secret) {
	if to.skipMessage != "" {
		Skip(to.skipMessage)
	}

	By("Creating secret: " + userConfig.Name)
	_, err := to.CreateSecret(userConfig)
	Expect(err).NotTo(HaveOccurred())

	if newUserConfig != nil {
		By("Creating secret: " + newUserConfig.Name)
		_, err = to.CreateSecret(newUserConfig)
		Expect(err).NotTo(HaveOccurred())
	}

	to.createAndWaitForReady()

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
