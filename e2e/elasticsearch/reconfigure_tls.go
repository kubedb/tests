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
	"fmt"
	"strings"

	api "kubedb.dev/apimachinery/apis/kubedb/v1alpha2"
	dbaapi "kubedb.dev/apimachinery/apis/ops/v1alpha1"
	"kubedb.dev/tests/e2e/framework"

	. "github.com/onsi/ginkgo"
	. "github.com/onsi/gomega"
	"gomodules.xyz/oneliners"
	"gomodules.xyz/pointer"
	kmapi "kmodules.xyz/client-go/api/v1"
)

var _ = Describe("Reconfigure TLS", func() {
	to := testOptions{}
	testName := framework.ReconfigureTLS
	var f *framework.Invocation

	BeforeEach(func() {
		f = framework.NewInvocation()
		to = testOptions{
			Invocation: f,
			db:         f.StandaloneElasticsearch(),
		}
		if to.StorageClass == "" {
			Skip("Missing StorageClassName. Provide as flag to test this.")
		}

		if strings.ToLower(framework.DBType) != api.ResourceSingularElasticsearch {
			Skip(fmt.Sprintf("Skipping Elasticsearch: %s tests...", testName))
		}

		if !framework.RunTestEnterprise(testName) {
			Skip(fmt.Sprintf("Provide test profile `%s` or `all` to test this.", testName))
		}

		if !framework.SSLEnabled {
			Skip("Skipping Elasticsearch ReconfigureTLS test, because ssl is false")
		}

		issuer, err := f.EnsureIssuer(to.db.ObjectMeta, to.db.ResourceFQN())
		Expect(err).NotTo(HaveOccurred())
		to.issuer = issuer
	})

	AfterEach(func() {
		// Cleanup
		By("Cleanup Left Overs")
		By("Delete Issuer")
		err := f.DeleteIssuer(to.issuer.ObjectMeta)
		Expect(err).NotTo(HaveOccurred())
		By("Delete left over Elasticsearch objects")
		to.CleanElasticsearch()
		By("Delete left over Elasticsearch Ops Request objects")
		to.CleanElasticsearchOpsRequests()
		By("Delete left over workloads if exists any")
		to.CleanWorkloadLeftOvers(api.Elasticsearch{}.ResourceFQN())
		By("Delete left over secrets if exists any")
		to.CleanSecrets()
	})

	JustAfterEach(func() {
		if CurrentGinkgoTestDescription().Failed {
			to.PrintDebugHelpers()
		}
	})

	Context("Add TLS", func() {
		AfterEach(func() {
			to.wipeOutElasticsearch()
			err := to.DeleteElasticsearchOpsRequest(to.elasticsearchOpsReq.ObjectMeta)
			Expect(err).ShouldNot(HaveOccurred())
		})

		Context("EnableSSL:false --> EnableSSL: true/false", func() {

			It("Default TLS to Cert-Manager Managed TLS; Only Transport Cert", func() {
				to.db = to.transformElasticsearch(to.StandaloneElasticsearch(), func(in *api.Elasticsearch) *api.Elasticsearch {
					in.Spec.EnableSSL = false
					return in
				})
				to.createElasticsearchAndWaitForBeingReady()
				indicesCount := to.insertData()
				to.elasticsearchOpsReq = to.GetElasticsearchOpsRequestReconfigureTLS(to.db.ObjectMeta, &dbaapi.TLSSpec{
					TLSConfig: *framework.NewTLSConfiguration(to.issuer),
				})
				to.createElasticsearchOpsRequestAndWaitForBeingSuccessful()
				to.checkUpdatedCertificates()
				to.verifyData(indicesCount)
			})

			It("Default TLS to Cert-Manager Managed TLS; Add all certs", func() {
				to.db = to.transformElasticsearch(to.ClusterElasticsearch(), func(in *api.Elasticsearch) *api.Elasticsearch {
					in.Spec.EnableSSL = false
					in.Spec.TLS = framework.NewTLSConfiguration(to.issuer)
					return in
				})
				to.createElasticsearchAndWaitForBeingReady()
				indicesCount := to.insertData()
				to.elasticsearchOpsReq = to.GetElasticsearchOpsRequestReconfigureTLS(to.db.ObjectMeta, &dbaapi.TLSSpec{
					TLSConfig: kmapi.TLSConfig{
						Certificates: []kmapi.CertificateSpec{
							{
								Alias:   "http",
								Subject: &kmapi.X509Subject{Organizations: []string{"appscode.com"}},
							},
						},
					},
				})
				oneliners.PrettyJson(to.elasticsearchOpsReq)
				to.createElasticsearchOpsRequestAndWaitForBeingSuccessful()
				to.checkUpdatedCertificates()
				Expect(to.db.Spec.EnableSSL).Should(BeTrue())
				to.verifyData(indicesCount)
			})
		})

		Context("EnableSSL:true --> EnableSSL: true", func() {

			It("Default TLS to Cert-Manager Managed TLS ", func() {
				to.db = to.transformElasticsearch(to.StandaloneElasticsearch(), func(in *api.Elasticsearch) *api.Elasticsearch {
					in.Spec.EnableSSL = true
					in.Spec.Replicas = pointer.Int32P(3)
					return in
				})
				to.createElasticsearchAndWaitForBeingReady()
				indicesCount := to.insertData()
				to.elasticsearchOpsReq = to.GetElasticsearchOpsRequestReconfigureTLS(to.db.ObjectMeta, &dbaapi.TLSSpec{
					TLSConfig: *framework.NewTLSConfiguration(to.issuer),
				})
				to.createElasticsearchOpsRequestAndWaitForBeingSuccessful()
				to.checkUpdatedCertificates()
				to.verifyData(indicesCount)
			})

		})
	})

	Context("Remove TLS", func() {

		It("Combined cluster", func() {
			to.db = to.transformElasticsearch(to.StandaloneElasticsearch(), func(in *api.Elasticsearch) *api.Elasticsearch {
				in.Spec.EnableSSL = true
				in.Spec.Replicas = pointer.Int32P(2)
				in.Spec.TLS = framework.NewTLSConfiguration(to.issuer)
				return in
			})
			to.createElasticsearchAndWaitForBeingReady()
			indicesCount := to.insertData()
			to.elasticsearchOpsReq = to.GetElasticsearchOpsRequestReconfigureTLS(to.db.ObjectMeta, &dbaapi.TLSSpec{
				Remove: true,
			})
			to.createElasticsearchOpsRequestAndWaitForBeingSuccessful()
			to.checkUpdatedCertificates()
			Expect(to.db.Spec.EnableSSL).Should(BeFalse())
			to.verifyData(indicesCount)
		})

		It("Topology cluster", func() {
			to.db = to.transformElasticsearch(to.ClusterElasticsearch(), func(in *api.Elasticsearch) *api.Elasticsearch {
				in.Spec.EnableSSL = true
				in.Spec.TLS = framework.NewTLSConfiguration(to.issuer)
				return in
			})
			to.createElasticsearchAndWaitForBeingReady()
			indicesCount := to.insertData()
			to.elasticsearchOpsReq = to.GetElasticsearchOpsRequestReconfigureTLS(to.db.ObjectMeta, &dbaapi.TLSSpec{
				Remove: true,
			})
			to.createElasticsearchOpsRequestAndWaitForBeingSuccessful()
			to.checkUpdatedCertificates()
			Expect(to.db.Spec.EnableSSL).Should(BeFalse())
			to.verifyData(indicesCount)
		})

	})

	Context("Rotate TLS", func() {

		Context("EnableSSL:true; All Certificates", func() {

			It("Combined Cluster", func() {
				to.db = to.transformElasticsearch(to.StandaloneElasticsearch(), func(in *api.Elasticsearch) *api.Elasticsearch {
					in.Spec.EnableSSL = true
					in.Spec.Replicas = pointer.Int32P(2)
					in.Spec.TLS = framework.NewTLSConfiguration(to.issuer)
					return in
				})
				to.createElasticsearchAndWaitForBeingReady()
				indicesCount := to.insertData()
				to.elasticsearchOpsReq = to.GetElasticsearchOpsRequestReconfigureTLS(to.db.ObjectMeta, &dbaapi.TLSSpec{
					RotateCertificates: true,
				})
				to.createElasticsearchOpsRequestAndWaitForBeingSuccessful()
				to.checkUpdatedCertificates()
				to.verifyData(indicesCount)
			})

		})

		Context("EnableSSL:false; Only Transport Certificate", func() {

			It("Topology Cluster", func() {
				to.db = to.transformElasticsearch(to.ClusterElasticsearch(), func(in *api.Elasticsearch) *api.Elasticsearch {
					in.Spec.EnableSSL = false
					in.Spec.TLS = framework.NewTLSConfiguration(to.issuer)
					return in
				})
				to.createElasticsearchAndWaitForBeingReady()
				indicesCount := to.insertData()
				to.elasticsearchOpsReq = to.GetElasticsearchOpsRequestReconfigureTLS(to.db.ObjectMeta, &dbaapi.TLSSpec{
					RotateCertificates: true,
				})
				to.createElasticsearchOpsRequestAndWaitForBeingSuccessful()
				to.checkUpdatedCertificates()
				Expect(to.db.Spec.EnableSSL).Should(BeFalse())
				to.verifyData(indicesCount)
			})

		})
	})

	Context("Update TLS", func() {

		Context("EnableSSL: false; Only Transport Certificate", func() {
			It("Topology Cluster", func() {
				to.db = to.transformElasticsearch(to.ClusterElasticsearch(), func(in *api.Elasticsearch) *api.Elasticsearch {
					in.Spec.EnableSSL = false
					in.Spec.TLS = framework.NewTLSConfiguration(to.issuer)
					return in
				})
				to.createElasticsearchAndWaitForBeingReady()
				indicesCount := to.insertData()
				to.elasticsearchOpsReq = to.GetElasticsearchOpsRequestReconfigureTLS(to.db.ObjectMeta, &dbaapi.TLSSpec{
					TLSConfig: kmapi.TLSConfig{
						Certificates: []kmapi.CertificateSpec{
							{
								Alias: "transport",
								Subject: &kmapi.X509Subject{
									Organizations:       []string{"mydb.com"},
									OrganizationalUnits: []string{"engineering"},
								},
							},
						},
					},
				})
				to.createElasticsearchOpsRequestAndWaitForBeingSuccessful()
				to.checkUpdatedCertificates()
				Expect(to.db.Spec.EnableSSL).Should(BeFalse())
				to.verifyData(indicesCount)
			})
		})

		Context("EnableSSL: true; Both HTTP & Transport Certificates", func() {
			It("Combined Cluster", func() {
				to.db = to.transformElasticsearch(to.StandaloneElasticsearch(), func(in *api.Elasticsearch) *api.Elasticsearch {
					in.Spec.EnableSSL = true
					in.Spec.Replicas = pointer.Int32P(2)
					in.Spec.TLS = framework.NewTLSConfiguration(to.issuer)
					return in
				})
				to.createElasticsearchAndWaitForBeingReady()
				indicesCount := to.insertData()
				to.elasticsearchOpsReq = to.GetElasticsearchOpsRequestReconfigureTLS(to.db.ObjectMeta, &dbaapi.TLSSpec{
					TLSConfig: kmapi.TLSConfig{
						Certificates: []kmapi.CertificateSpec{
							{
								Alias: "transport",
								Subject: &kmapi.X509Subject{
									Organizations:       []string{"mydb.com"},
									OrganizationalUnits: []string{"engineering"},
								},
							},
							{
								Alias: "http",
								Subject: &kmapi.X509Subject{
									Organizations: []string{"mydb-server.com"},
								},
							},
						},
					},
				})
				to.createElasticsearchOpsRequestAndWaitForBeingSuccessful()
				to.checkUpdatedCertificates()
				to.verifyData(indicesCount)
			})
		})
	})

})
