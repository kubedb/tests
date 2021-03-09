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
	"path"

	"kubedb.dev/apimachinery/apis/autoscaling/v1alpha1"
	api "kubedb.dev/apimachinery/apis/kubedb/v1alpha2"
	dbaapi "kubedb.dev/apimachinery/apis/ops/v1alpha1"
	"kubedb.dev/tests/e2e/framework"

	"github.com/appscode/go/log"
	"github.com/google/go-cmp/cmp"
	cm_api "github.com/jetstack/cert-manager/pkg/apis/certmanager/v1"
	. "github.com/onsi/ginkgo"
	. "github.com/onsi/gomega"
	apps "k8s.io/api/apps/v1"
	core "k8s.io/api/core/v1"
	kerr "k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	kmapi "kmodules.xyz/client-go/api/v1"
)

type testOptions struct {
	*framework.Invocation
	db                  *api.Elasticsearch
	skipMessage         string
	configSecret        *core.Secret
	elasticsearchOpsReq *dbaapi.ElasticsearchOpsRequest
	issuer              *cm_api.Issuer
	esAutoscaler        *v1alpha1.ElasticsearchAutoscaler
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
	to.EventuallyElasticsearchIndicesCount(indicesCount, esClient).Should(BeTrue())
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

func (to *testOptions) createElasticsearchOpsRequestAndWaitForBeingSuccessful() {
	_, err := to.DBClient().OpsV1alpha1().ElasticsearchOpsRequests(to.Namespace()).Create(context.TODO(), to.elasticsearchOpsReq, metav1.CreateOptions{})
	Expect(err).NotTo(HaveOccurred())

	By("Waiting for Ops Request to be successful or failed...")
	to.EventuallyElasticsearchOpsRequestSuccessful(to.elasticsearchOpsReq.ObjectMeta, 4*framework.WaitTimeOut).Should(BeTrue())

	By("Waiting for Elasticsearch to be Ready after ops request is performed...")
	to.EventuallyElasticsearchReady(to.db.ObjectMeta).Should(BeTrue())
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
	err = esClient.CreateIndex(5)
	Expect(err).NotTo(HaveOccurred())
	indicesCount += 5

	By("Checking created indices")
	to.EventuallyElasticsearchIndicesCount(indicesCount, esClient).Should(BeTrue())

	return indicesCount
}

func (to *testOptions) verifyData(indicesCount int) {
	esClient, tunnel, err := to.GetElasticClient(to.db.ObjectMeta)
	Expect(err).NotTo(HaveOccurred())
	defer esClient.Stop()
	defer tunnel.Close()

	By("Checking indices...")
	to.EventuallyElasticsearchIndicesCount(indicesCount, esClient).Should(BeTrue())
}

func (to *testOptions) verifyResources() bool {
	reqSpec := to.elasticsearchOpsReq.Spec.VerticalScaling
	if reqSpec == nil {
		return false
	}
	db, err := to.DBClient().KubedbV1alpha2().Elasticsearches(to.db.Namespace).Get(context.TODO(), to.db.Name, metav1.GetOptions{})
	Expect(err).NotTo(HaveOccurred())
	dbSpec := db.Spec

	if reqSpec.Node != nil && dbSpec.Topology == nil {
		// Get StatefulSet
		sts, err := to.KubeClient().AppsV1().StatefulSets(db.Namespace).Get(context.TODO(), db.OffshootName(), metav1.GetOptions{})
		Expect(err).NotTo(HaveOccurred())
		container := GetElasticsearchContainer(sts, api.ElasticsearchContainerName)
		if !cmp.Equal(container.Resources, *reqSpec.Node) {
			log.Error("statefulSet....container.Resources and reqSpec.Node are not equal!")
			return false
		}

		if !cmp.Equal(dbSpec.PodTemplate.Spec.Resources, *reqSpec.Node) {
			log.Error("dbSpec.PodTemplate.Spec.Resources and reqSpec.Node are not equal!")
			return false
		}
	}

	if reqSpec.Topology != nil && dbSpec.Topology != nil {
		if reqSpec.Topology.Master != nil {
			// Get StatefulSet
			sts, err := to.KubeClient().AppsV1().StatefulSets(db.Namespace).Get(context.TODO(), db.MasterStatefulSetName(), metav1.GetOptions{})
			Expect(err).NotTo(HaveOccurred())
			container := GetElasticsearchContainer(sts, api.ElasticsearchContainerName)
			if !cmp.Equal(container.Resources, *reqSpec.Topology.Master) {
				log.Error("statefulSet....container.Resources and reqSpec.topology.master are not equal!")
				return false
			}

			if !cmp.Equal(*reqSpec.Topology.Master, dbSpec.Topology.Master.Resources) {
				log.Error("reqSpec.Topology.Master and dbSpec.Topology.Master.Resources are not equal!")
				return false
			}
		}

		if reqSpec.Topology.Data != nil {
			// Get StatefulSet
			sts, err := to.KubeClient().AppsV1().StatefulSets(db.Namespace).Get(context.TODO(), db.DataStatefulSetName(), metav1.GetOptions{})
			Expect(err).NotTo(HaveOccurred())
			container := GetElasticsearchContainer(sts, api.ElasticsearchContainerName)
			if !cmp.Equal(container.Resources, *reqSpec.Topology.Data) {
				log.Error("statefulSet....container.Resources and reqSpec.topology.data are not equal!")
				return false
			}

			if !cmp.Equal(*reqSpec.Topology.Data, dbSpec.Topology.Data.Resources) {
				log.Error("reqSpec.Topology.Data and dbSpec.Topology.Data.Resources are not equal!")
				return false
			}
		}

		if reqSpec.Topology.Ingest != nil {
			// Get StatefulSet
			sts, err := to.KubeClient().AppsV1().StatefulSets(db.Namespace).Get(context.TODO(), db.IngestStatefulSetName(), metav1.GetOptions{})
			Expect(err).NotTo(HaveOccurred())
			container := GetElasticsearchContainer(sts, api.ElasticsearchContainerName)
			if !cmp.Equal(container.Resources, *reqSpec.Topology.Ingest) {
				log.Error("statefulSet....container.Resources and reqSpec.topology.ingest are not equal!")
				return false
			}
			// Exporter sidecar of ingest node
			if reqSpec.Exporter != nil && dbSpec.Monitor != nil && dbSpec.Monitor.Prometheus != nil {
				container = GetElasticsearchContainer(sts, api.ElasticsearchExporterContainerName)
				if !cmp.Equal(container.Resources, *reqSpec.Exporter) {
					log.Error("statefulSet....container.Resources and reqSpec.exporter are not equal!")
					return false
				}
			}

			if !cmp.Equal(*reqSpec.Topology.Ingest, dbSpec.Topology.Ingest.Resources) {
				log.Error("reqSpec.Topology.Ingest and dbSpec.Topology.Ingest.Resources are not equal!")
				return false
			}
		}
	}
	if reqSpec.Exporter != nil && dbSpec.Monitor != nil && dbSpec.Monitor.Prometheus != nil {
		if !cmp.Equal(*reqSpec.Exporter, dbSpec.Monitor.Prometheus.Exporter.Resources) {
			log.Error("reqSpec.Exporter and dbSpec.Monitor.Prometheus.Exporter.Resources are not equal!")
			return false
		}
	}
	return true
}

func (to *testOptions) verifyStorage() bool {
	reqSpec := to.elasticsearchOpsReq.Spec.VolumeExpansion
	if reqSpec == nil {
		return false
	}
	db, err := to.DBClient().KubedbV1alpha2().Elasticsearches(to.db.Namespace).Get(context.TODO(), to.db.Name, metav1.GetOptions{})
	Expect(err).NotTo(HaveOccurred())
	dbSpec := db.Spec

	if reqSpec.Node != nil && dbSpec.Topology == nil {
		// Get StatefulSet
		sts, err := to.KubeClient().AppsV1().StatefulSets(db.Namespace).Get(context.TODO(), db.CombinedStatefulSetName(), metav1.GetOptions{})
		Expect(err).NotTo(HaveOccurred())

		// with statefulSet
		if !cmp.Equal(*sts.Spec.VolumeClaimTemplates[0].Spec.Resources.Requests.Storage(), *reqSpec.Node) {
			log.Error("VolumeClaimTemplates[0].Spec.Resources.Requests.Storage() and reqSpec.Node are not equal!")
			return false
		}

		// with PVCs
		pvcNames := GetPVCNamesForStatefulSet(sts)
		for _, pvcName := range pvcNames {
			pvc, err := to.KubeClient().CoreV1().PersistentVolumeClaims(to.db.Namespace).Get(context.TODO(), pvcName, metav1.GetOptions{})
			Expect(err).NotTo(HaveOccurred())
			if !cmp.Equal(*pvc.Spec.Resources.Requests.Storage(), *reqSpec.Node) {
				log.Error("pvc.Spec.Resources.Requests.Storage() and reqSpec.Node are not equal!")
				return false
			}
		}

		// with Elasticsearch CRD
		if !cmp.Equal(*dbSpec.Storage.Resources.Requests.Storage(), *reqSpec.Node) {
			log.Error("db.Spec.Storage.Resources.Requests.Storage() and reqSpec.Node are not equal!")
			return false
		}
	}

	if reqSpec.Topology != nil && dbSpec.Topology != nil {
		if reqSpec.Topology.Master != nil {
			// Get StatefulSet
			sts, err := to.KubeClient().AppsV1().StatefulSets(db.Namespace).Get(context.TODO(), db.MasterStatefulSetName(), metav1.GetOptions{})
			Expect(err).NotTo(HaveOccurred())

			// with StatefulSet
			if !cmp.Equal(*sts.Spec.VolumeClaimTemplates[0].Spec.Resources.Requests.Storage(), *reqSpec.Topology.Master) {
				log.Error("VolumeClaimTemplates[0].Spec.Resources.Requests.Storage() and reqSpec.Topology.Master are not equal!")
				return false
			}

			// with PVCs
			pvcNames := GetPVCNamesForStatefulSet(sts)
			for _, pvcName := range pvcNames {
				pvc, err := to.KubeClient().CoreV1().PersistentVolumeClaims(to.db.Namespace).Get(context.TODO(), pvcName, metav1.GetOptions{})
				Expect(err).NotTo(HaveOccurred())
				if !cmp.Equal(*pvc.Spec.Resources.Requests.Storage(), *reqSpec.Topology.Master) {
					log.Error("pvc.Spec.Resources.Requests.Storage() and reqSpec.Topology.Master are not equal!")
					return false
				}
			}

			// with Elasticsearch CRD
			if !cmp.Equal(*dbSpec.Topology.Master.Storage.Resources.Requests.Storage(), *reqSpec.Topology.Master) {
				log.Error("db.Spec.Topology.Master.Storage.Resources.Requests.Storage() and reqSpec.Topology.Master are not equal!")
				return false
			}
		}

		if reqSpec.Topology.Data != nil {
			// Get StatefulSet
			sts, err := to.KubeClient().AppsV1().StatefulSets(db.Namespace).Get(context.TODO(), db.DataStatefulSetName(), metav1.GetOptions{})
			Expect(err).NotTo(HaveOccurred())

			// with statefulSet
			if !cmp.Equal(*sts.Spec.VolumeClaimTemplates[0].Spec.Resources.Requests.Storage(), *reqSpec.Topology.Data) {
				log.Error("VolumeClaimTemplates[0].Spec.Resources.Requests.Storage() and reqSpec.topology.data are not equal!")
				return false
			}

			// with PVCs
			pvcNames := GetPVCNamesForStatefulSet(sts)
			for _, pvcName := range pvcNames {
				pvc, err := to.KubeClient().CoreV1().PersistentVolumeClaims(to.db.Namespace).Get(context.TODO(), pvcName, metav1.GetOptions{})
				Expect(err).NotTo(HaveOccurred())
				if !cmp.Equal(*pvc.Spec.Resources.Requests.Storage(), *reqSpec.Topology.Data) {
					log.Error("pvc.Spec.Resources.Requests.Storage() and reqSpec.Topology.Data are not equal!")
					return false
				}
			}

			// with Elasticsearch CRD
			if !cmp.Equal(*dbSpec.Topology.Data.Storage.Resources.Requests.Storage(), *reqSpec.Topology.Data) {
				log.Error("dbSpec.Topology.Data.Storage.Resources.Requests.Storage() and reqSpec.Topology.Data are not equal!")
				return false
			}
		}

		if reqSpec.Topology.Ingest != nil {
			// Get StatefulSet
			sts, err := to.KubeClient().AppsV1().StatefulSets(db.Namespace).Get(context.TODO(), db.IngestStatefulSetName(), metav1.GetOptions{})
			Expect(err).NotTo(HaveOccurred())

			// with statefulSet
			if !cmp.Equal(*sts.Spec.VolumeClaimTemplates[0].Spec.Resources.Requests.Storage(), *reqSpec.Topology.Ingest) {
				log.Error("sts.Spec.VolumeClaimTemplates[0].Spec.Resources.Requests.Storage() and reqSpec.topology.ingest are not equal!")
				return false
			}

			// with PVCs
			pvcNames := GetPVCNamesForStatefulSet(sts)
			for _, pvcName := range pvcNames {
				pvc, err := to.KubeClient().CoreV1().PersistentVolumeClaims(to.db.Namespace).Get(context.TODO(), pvcName, metav1.GetOptions{})
				Expect(err).NotTo(HaveOccurred())
				if !cmp.Equal(*pvc.Spec.Resources.Requests.Storage(), *reqSpec.Topology.Ingest) {
					log.Error("pvc.Spec.Resources.Requests.Storage() and reqSpec.Topology.Ingest are not equal!")
					return false
				}
			}

			// with Elasticsearch CRD
			if !cmp.Equal(*dbSpec.Topology.Ingest.Storage.Resources.Requests.Storage(), *reqSpec.Topology.Ingest) {
				log.Error("reqSpec.Topology.Ingest and dbSpec.Topology.Ingest.Resources are not equal!")
				return false
			}
		}
	}

	return true
}

func (to *testOptions) transformElasticsearch(db *api.Elasticsearch, transform func(in *api.Elasticsearch) *api.Elasticsearch) *api.Elasticsearch {
	return transform(db)
}

func (to *testOptions) checkUpdatedCertificates() {
	var pod *core.Pod
	var err error
	if to.db.Spec.Topology != nil {
		pod, err = to.KubeClient().CoreV1().Pods(to.db.Namespace).Get(context.TODO(), fmt.Sprintf("%s-0", to.db.IngestStatefulSetName()), metav1.GetOptions{})
		Expect(err).NotTo(HaveOccurred())
	} else {
		pod, err = to.KubeClient().CoreV1().Pods(to.db.Namespace).Get(context.TODO(), fmt.Sprintf("%s-0", to.db.CombinedStatefulSetName()), metav1.GetOptions{})
		Expect(err).NotTo(HaveOccurred())
	}

	// Get updated DB
	newDB, err := to.DBClient().KubedbV1alpha2().Elasticsearches(to.db.Namespace).Get(context.TODO(), to.db.Name, metav1.GetOptions{})
	Expect(err).NotTo(HaveOccurred())
	to.db = newDB

	// Check HTTP Certificate
	if kmapi.HasCertificate(to.db.Spec.TLS.Certificates, string(api.ElasticsearchHTTPCert)) {
		cert1, err := to.GetCertificateFromPod(pod, path.Join(to.db.CertSecretVolumeMountPath(api.ElasticsearchConfigDir, api.ElasticsearchHTTPCert), core.TLSCertKey))
		Expect(err).NotTo(HaveOccurred())
		cert2, err := to.GetCertificateFromSecret(to.db.MustCertSecretName(api.ElasticsearchHTTPCert), to.db.Namespace)
		Expect(err).NotTo(HaveOccurred())
		Expect(cmp.Equal(cert1, cert2)).Should(BeTrue())
	}

	// Check Transport Certificate
	if kmapi.HasCertificate(to.db.Spec.TLS.Certificates, string(api.ElasticsearchTransportCert)) {
		cert1, err := to.GetCertificateFromPod(pod, path.Join(to.db.CertSecretVolumeMountPath(api.ElasticsearchConfigDir, api.ElasticsearchTransportCert), core.TLSCertKey))
		Expect(err).NotTo(HaveOccurred())
		cert2, err := to.GetCertificateFromSecret(to.db.MustCertSecretName(api.ElasticsearchTransportCert), to.db.Namespace)
		Expect(err).NotTo(HaveOccurred())
		Expect(cmp.Equal(cert1, cert2)).Should(BeTrue())
	}
}

func GetElasticsearchContainer(sts *apps.StatefulSet, containerName string) core.Container {
	for _, c := range sts.Spec.Template.Spec.Containers {
		if c.Name == containerName {
			return c
		}
	}

	return core.Container{}
}

func GetPVCNamesForStatefulSet(sts *apps.StatefulSet) []string {
	replicas := *sts.Spec.Replicas
	var names []string
	for idx := int32(0); idx < replicas; idx++ {
		names = append(names, fmt.Sprintf("%s-%s-%d", api.DefaultVolumeClaimTemplateName, sts.Name, idx))
	}
	return names
}

func (to *testOptions) shouldTestComputeAutoscaler() {
	to.createElasticsearchAndWaitForBeingReady()

	indicesCount := to.insertData()

	to.verifyData(indicesCount)

	By("Creating Elasticsearch Autoscaler")
	err := to.CreateElasticsearchAutoscaler(to.esAutoscaler)
	Expect(err).NotTo(HaveOccurred())

	By("Wait for Vertical Scaling")
	to.EventuallyVerticallyScaledElasticsearch(to.db.ObjectMeta, to.esAutoscaler.Spec.Compute).Should(BeTrue())

	to.verifyData(indicesCount)
}

func (to *testOptions) shouldTestStorageAutoscaler() {
	to.createElasticsearchAndWaitForBeingReady()

	indicesCount := to.insertData()

	to.verifyData(indicesCount)

	By("Creating Elasticsearch Autoscaler")
	err := to.CreateElasticsearchAutoscaler(to.esAutoscaler)
	Expect(err).NotTo(HaveOccurred())

	By("Fill Persistent Volume")
	err = to.FillDiskElasticsearch(to.db, to.esAutoscaler.Spec.Storage)
	Expect(err).NotTo(HaveOccurred())

	By("Wait for Volume Expansion")
	to.EventuallyVolumeExpandedElasticsearch(to.db, to.esAutoscaler.Spec.Storage).Should(BeTrue())

	By("Wait for Ready elasticsearch")
	to.EventuallyElasticsearchReady(to.db.ObjectMeta).Should(BeTrue())

	to.verifyData(indicesCount)
}
