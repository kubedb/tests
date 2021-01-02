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

package elasticsearch

import (
	"context"
	"fmt"

	api "kubedb.dev/apimachinery/apis/kubedb/v1alpha2"
	"kubedb.dev/tests/e2e/framework"

	. "github.com/onsi/ginkgo"
	. "github.com/onsi/gomega"
	core "k8s.io/api/core/v1"
	kerr "k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
)

type testOptions struct {
	*framework.Invocation
	db           *api.Elasticsearch
	skipMessage  string
	configSecret *core.Secret
}

func (to *testOptions) createAndHaltElasticsearchAndWaitForBeingReady() {
	if to.skipMessage != "" {
		Skip(to.skipMessage)
	}

	// Create and wait for running elasticsearch
	to.createElasticsearchAndWaitForBeingReady()

	// insert some data/indices
	indicesCount := to.insertData()

	By("Halt Elasticsearch: Update elasticsearch to set spec.halted = true")
	_, err := to.PatchElasticsearch(to.db.ObjectMeta, func(in *api.Elasticsearch) *api.Elasticsearch {
		in.Spec.Halted = true
		return in
	})
	Expect(err).NotTo(HaveOccurred())

	By("Wait for halted/paused elasticsearch")
	to.EventuallyElasticsearchPhase(to.db.ObjectMeta).Should(Equal(api.DatabasePhaseHalted))

	By("Resume Elasticsearch: Update elasticsearch to set spec.halted = false")
	_, err = to.PatchElasticsearch(to.db.ObjectMeta, func(in *api.Elasticsearch) *api.Elasticsearch {
		in.Spec.Halted = false
		return in
	})
	Expect(err).NotTo(HaveOccurred())

	By("Wait for Running elasticsearch")
	to.EventuallyElasticsearchReady(to.db.ObjectMeta).Should(BeTrue())

	By("Check for Elastic client")
	to.EventuallyElasticsearchClientReady(to.db.ObjectMeta).Should(BeTrue())

	esClient, tunnel, err := to.GetElasticClient(to.db.ObjectMeta)
	Expect(err).NotTo(HaveOccurred())
	defer esClient.Stop()
	defer tunnel.Close()

	By("Checking indices after recovery from halted.")
	to.EventuallyElasticsearchIndicesCount(esClient).Should(Equal(indicesCount))
}

func (to *testOptions) createElasticsearchAndWaitForBeingReady() {
	By("Creating Elasticsearch: " + to.db.Name)
	err := to.CreateElasticsearch(to.db)
	Expect(err).NotTo(HaveOccurred())

	By("Wait for AppBinding to create")
	to.EventuallyAppBinding(to.db.ObjectMeta).Should(BeTrue())

	By("Check valid AppBinding Specs")
	err = to.CheckElasticsearchAppBindingSpec(to.db.ObjectMeta)
	Expect(err).NotTo(HaveOccurred())

	By("Wait for Ready  elasticsearch")
	to.EventuallyElasticsearchReady(to.db.ObjectMeta).Should(BeTrue())

	By("Check for Elastic client")
	to.EventuallyElasticsearchClientReady(to.db.ObjectMeta).Should(BeTrue())

}

func (to *testOptions) createElasticsearchWithCustomConfigAndWaitForBeingReady() {
	to.configSecret = to.GetElasticsearchCustomConfig()
	to.configSecret.StringData = map[string]string{
		"elasticsearch.yml":        to.GetElasticsearchCommonConfig(),
		"master-elasticsearch.yml": to.GetElasticsearchMasterConfig(),
		"ingest-elasticsearch.yml": to.GetElasticsearchIngestConfig(),
		"data-elasticsearch.yml":   to.GetElasticsearchDataConfig(),
	}

	fmt.Println(to.configSecret.StringData)

	By("Creating secret: " + to.configSecret.Name)
	s, err := to.CreateSecret(to.configSecret)
	Expect(err).NotTo(HaveOccurred())

	fmt.Println(s.Data)

	to.db.Spec.ConfigSecret = &core.LocalObjectReference{
		Name: to.configSecret.Name,
	}

	to.createElasticsearchAndWaitForBeingReady()

	esClient, tunnel, err := to.GetElasticClient(to.db.ObjectMeta)
	Expect(err).NotTo(HaveOccurred())
	defer esClient.Stop()
	defer tunnel.Close()

	By("Reading Nodes information")
	settings, err := esClient.GetAllNodesInfo()
	Expect(err).NotTo(HaveOccurred())
	// TODO:
	// 	- setting API does not provide expected output, fix it
	fmt.Printf("%+v", settings)

	By("Checking nodes are using provided config")
	Expect(to.IsElasticsearchUsingProvidedConfig(settings)).Should(BeTrue())

}

func (to *testOptions) wipeOutElasticsearch() {
	if to.db == nil {
		Skip("Skipping...")
	}

	By("Checking if the Elasticsearch: " + to.db.Name + " exists.")
	db, err := to.DBClient().KubedbV1alpha2().Elasticsearches(to.Namespace()).Get(context.TODO(), to.db.Name, metav1.GetOptions{})
	if err != nil {
		if kerr.IsNotFound(err) {
			// Elasticsearch doesn't exist anymore.
			return
		}
		Expect(err).NotTo(HaveOccurred())
	}

	By("Updating Elasticsearch to set spec.terminationPolicy = WipeOut.")
	_, err = to.PatchElasticsearch(db.ObjectMeta, func(in *api.Elasticsearch) *api.Elasticsearch {
		in.Spec.TerminationPolicy = api.TerminationPolicyWipeOut
		return in
	})
	Expect(err).NotTo(HaveOccurred())

	By("Deleting Elasticsearch: " + db.Name)
	err = to.DeleteElasticsearch(db.ObjectMeta)
	if err != nil {
		if kerr.IsNotFound(err) {
			return
		}
		Expect(err).NotTo(HaveOccurred())
	}

	By("Wait for elasticsearch to be deleted")
	to.EventuallyElasticsearch(db.ObjectMeta).Should(BeFalse())

	By("Wait for elasticsearch services to be deleted")
	to.EventuallyServices(db.ObjectMeta, api.Elasticsearch{}.ResourceFQN()).Should(Succeed())

	By("Wait for elasticsearch resources to be wipedOut")
	to.EventuallyWipedOut(db.ObjectMeta, api.Elasticsearch{}.ResourceFQN()).Should(Succeed())
}

func (to *testOptions) insertData() int {
	esClient, tunnel, err := to.GetElasticClient(to.db.ObjectMeta)
	Expect(err).NotTo(HaveOccurred())
	defer tunnel.Close()

	indicesCount, err := to.ElasticsearchIndicesCount(esClient)
	Expect(err).NotTo(HaveOccurred())

	By("Creating indices in Elasticsearch")
	err = esClient.CreateIndex(2)
	Expect(err).NotTo(HaveOccurred())
	indicesCount += 2

	By("Checking created indices")
	to.EventuallyElasticsearchIndicesCount(esClient).Should(Equal(indicesCount))

	return indicesCount
}

func (to *testOptions) transformElasticsearch(db *api.Elasticsearch, transform func(in *api.Elasticsearch) *api.Elasticsearch) *api.Elasticsearch {
	return transform(db)
}
