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

package mysql

import (
	"context"
	"fmt"
	"time"

	api "kubedb.dev/apimachinery/apis/kubedb/v1alpha2"
	opsapi "kubedb.dev/apimachinery/apis/ops/v1alpha1"
	"kubedb.dev/apimachinery/pkg/util"
	"kubedb.dev/tests/e2e/framework"
	"kubedb.dev/tests/e2e/matcher"

	"github.com/appscode/go/crypto/rand"
	"github.com/appscode/go/types"
	cm_api "github.com/jetstack/cert-manager/pkg/apis/certmanager/v1"
	. "github.com/onsi/ginkgo"
	. "github.com/onsi/gomega"
	core "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	kmapi "kmodules.xyz/client-go/api/v1"
)

var _ = Describe("MySQL", func() {
	var fi *framework.Invocation

	BeforeEach(func() {
		fi = framework.NewInvocation()

		if !runTestDatabaseType() {
			Skip(fmt.Sprintf("Provide test for database `%s`", api.ResourceSingularMySQL))
		}
		if !runTestEnterprise(framework.ReconfigureTLS) {
			Skip(fmt.Sprintf("Provide test profile `%s` or `all` or `enterprise` to test this.", framework.Upgrade))
		}
		if !framework.SSLEnabled {
			Skip("Enable SSL to test this")
		}
	})

	JustAfterEach(func() {
		fi.PrintDebugInfoOnFailure()
	})

	AfterEach(func() {
		err := fi.CleanupTestResources()
		Expect(err).NotTo(HaveOccurred())

	})

	Context("Reconfigure TLS/SSL", func() {
		Context("MySQL Standalone", func() {
			Context("Remove TLS/SSL", func() {
				It("Should remove TLS/SSL", func() {
					// MySQL objectMeta
					myMeta := metav1.ObjectMeta{
						Name:      rand.WithUniqSuffix("mysql"),
						Namespace: fi.Namespace(),
					}
					issuer, err := fi.InsureIssuer(myMeta, api.ResourceKindMySQL)
					Expect(err).NotTo(HaveOccurred())
					// Create MySQL standalone with tls secured and wait for running
					my, err := fi.CreateMySQLAndWaitForRunning(framework.DBVersion, func(in *api.MySQL) {
						in.Name = myMeta.Name
						// configure TLS issuer to MySQL CRD
						in.Spec.RequireSSL = true
						in.Spec.TLS = &kmapi.TLSConfig{
							IssuerRef: &core.TypedLocalObjectReference{
								Name:     issuer.Name,
								Kind:     "Issuer",
								APIGroup: types.StringP(cm_api.SchemeGroupVersion.Group), //cert-manger.io
							},
							Certificates: []kmapi.CertificateSpec{
								{
									Alias: string(api.MySQLServerCert),
									Subject: &kmapi.X509Subject{
										Organizations: []string{
											"kubedb:server",
										},
									},
									DNSNames: []string{
										"localhost",
									},
									IPAddresses: []string{
										"127.0.0.1",
									},
								},
							},
						}
					})
					Expect(err).NotTo(HaveOccurred())
					// Database connection information
					dbInfo := framework.DatabaseConnectionInfo{
						StatefulSetOrdinal: 0,
						ClientPodIndex:     0,
						DatabaseName:       framework.DBMySQL,
						User:               framework.MySQLRootUser,
						Param:              fmt.Sprintf("tls=%s", framework.TLSCustomConfig),
					}
					fi.EventuallyDBReady(my, dbInfo)

					// Create a mysql User with required SSL
					By("Create mysql User with required SSL")
					fi.EventuallyCreateUserWithRequiredSSL(my.ObjectMeta, dbInfo).Should(BeTrue())
					dbInfo.User = framework.MySQLRequiredSSLUser
					fi.EventuallyCheckConnectionRequiredSSLUser(my, dbInfo)

					By("Creating Table")
					fi.EventuallyCreateTable(my.ObjectMeta, dbInfo).Should(BeTrue())

					By("Inserting Rows")
					fi.EventuallyInsertRow(my.ObjectMeta, dbInfo, 3).Should(BeTrue())

					By("Checking Row Count of Table")
					fi.EventuallyCountRow(my.ObjectMeta, dbInfo).Should(Equal(3))
					// Removing TLS/SSL and waiting for the success
					_ = fi.CreateMySQLOpsRequestsAndWaitForSuccess(my.Name, func(in *opsapi.MySQLOpsRequest) {
						in.Spec.Type = opsapi.OpsRequestTypeReconfigureTLSs
						in.Spec.TLS = &opsapi.MySQLTLSSpec{
							TLSSpec: opsapi.TLSSpec{
								Remove: true,
							},
						}
					})
					// ReconfigureTLS OpsRequest is succeeded, That's why TLS configuration have been removed.
					// we need to set `User` and `Param` to access database without TLSCustom config.
					dbInfo.User = framework.MySQLRootUser
					dbInfo.Param = ""
					By("Checking MySQL TLS config have removed")
					my, err = fi.DBClient().KubedbV1alpha2().MySQLs(fi.Namespace()).Get(context.TODO(), my.Name, metav1.GetOptions{})
					Expect(err).NotTo(HaveOccurred())
					sts, err := fi.KubeClient().AppsV1().StatefulSets(fi.Namespace()).Get(context.TODO(), my.OffshootName(), metav1.GetOptions{})
					Expect(err).NotTo(HaveOccurred())
					util.CheckMySQLTLSConfigRemoved(my, sts)

					// Retrieve Inserted Data
					By("Checking Row Count of Table")
					fi.EventuallyCountRow(my.ObjectMeta, dbInfo).Should(Equal(3))
				})
			})

		})
		Context("MySQL Group Replication", func() {
			Context("Remove TLS/SSL", func() {
				It("Should remove TLS/SSL", func() {
					// MySQL objectMeta
					myMeta := metav1.ObjectMeta{
						Name:      rand.WithUniqSuffix("mysql"),
						Namespace: fi.Namespace(),
					}
					issuer, err := fi.InsureIssuer(myMeta, api.ResourceKindMySQL)
					Expect(err).NotTo(HaveOccurred())
					// Create MySQL Group Replication with tls secured and wait for running
					my, err := fi.CreateMySQLAndWaitForRunning(framework.DBVersion, func(in *api.MySQL) {
						in.Spec.Replicas = types.Int32P(api.MySQLDefaultGroupSize)
						clusterMode := api.MySQLClusterModeGroup
						in.Spec.Topology = &api.MySQLClusterTopology{
							Mode: &clusterMode,
							Group: &api.MySQLGroupSpec{
								Name:         "dc002fc3-c412-4d18-b1d4-66c1fbfbbc9b",
								BaseServerID: types.Int64P(api.MySQLDefaultBaseServerID),
							},
						}
						// configure TLS issuer to MySQL CRD
						in.Spec.RequireSSL = true
						in.Spec.TLS = &kmapi.TLSConfig{
							IssuerRef: &core.TypedLocalObjectReference{
								Name:     issuer.Name,
								Kind:     "Issuer",
								APIGroup: types.StringP(cm_api.SchemeGroupVersion.Group), //cert-manger.io
							},
							Certificates: []kmapi.CertificateSpec{
								{
									Alias: string(api.MySQLServerCert),
									Subject: &kmapi.X509Subject{
										Organizations: []string{
											"kubedb:server",
										},
									},
									DNSNames: []string{
										"localhost",
									},
									IPAddresses: []string{
										"127.0.0.1",
									},
								},
							},
						}
					})
					Expect(err).NotTo(HaveOccurred())
					// Database connection information
					dbInfo := framework.DatabaseConnectionInfo{
						StatefulSetOrdinal: 0,
						ClientPodIndex:     0,
						DatabaseName:       framework.DBMySQL,
						User:               framework.MySQLRootUser,
						Param:              fmt.Sprintf("tls=%s", framework.TLSCustomConfig),
					}
					fi.EventuallyDBReady(my, dbInfo)

					// Create a mysql User with required SSL
					By("Create mysql User with required SSL")
					fi.EventuallyCreateUserWithRequiredSSL(my.ObjectMeta, dbInfo).Should(BeTrue())
					dbInfo.User = framework.MySQLRequiredSSLUser
					fi.EventuallyCheckConnectionRequiredSSLUser(my, dbInfo)

					By("Creating Table")
					fi.EventuallyCreateTable(my.ObjectMeta, dbInfo).Should(BeTrue())

					By("Inserting Rows")
					fi.EventuallyInsertRow(my.ObjectMeta, dbInfo, 3).Should(BeTrue())

					By("Checking Row Count of Table")
					fi.EventuallyCountRow(my.ObjectMeta, dbInfo).Should(Equal(3))

					// Removing TLS/SSL and waiting for the success
					_ = fi.CreateMySQLOpsRequestsAndWaitForSuccess(my.Name, func(in *opsapi.MySQLOpsRequest) {
						in.Spec.Type = opsapi.OpsRequestTypeReconfigureTLSs
						in.Spec.TLS = &opsapi.MySQLTLSSpec{
							TLSSpec: opsapi.TLSSpec{
								Remove: true,
							},
						}
					})
					// ReconfigureTLS OpsRequest is succeeded, That's why TLS configuration have been removed.
					// we need to set `User` and `Param` to access database without TLSCustom config.
					dbInfo.User = framework.MySQLRootUser
					dbInfo.Param = ""
					By("Checking MySQL TLS config have removed")
					my, err = fi.DBClient().KubedbV1alpha2().MySQLs(fi.Namespace()).Get(context.TODO(), my.Name, metav1.GetOptions{})
					Expect(err).NotTo(HaveOccurred())
					sts, err := fi.KubeClient().AppsV1().StatefulSets(fi.Namespace()).Get(context.TODO(), my.OffshootName(), metav1.GetOptions{})
					Expect(err).NotTo(HaveOccurred())
					util.CheckMySQLTLSConfigRemoved(my, sts)

					// Retrieve Inserted Data
					By("Checking Row Count of Table")
					fi.EventuallyCountRow(my.ObjectMeta, dbInfo).Should(Equal(3))
				})
			})

			Context("Add TLS/SSL", func() {
				It("Should add TLS/SSL", func() {
					// Create MySQL Group Replication and wait for running
					my, err := fi.CreateMySQLAndWaitForRunning(framework.DBVersion, func(in *api.MySQL) {
						in.Spec.Replicas = types.Int32P(api.MySQLDefaultGroupSize)
						clusterMode := api.MySQLClusterModeGroup
						in.Spec.Topology = &api.MySQLClusterTopology{
							Mode: &clusterMode,
							Group: &api.MySQLGroupSpec{
								Name:         "dc002fc3-c412-4d18-b1d4-66c1fbfbbc9b",
								BaseServerID: types.Int64P(api.MySQLDefaultBaseServerID),
							},
						}
					})
					Expect(err).NotTo(HaveOccurred())
					// Database connection information
					dbInfo := framework.DatabaseConnectionInfo{
						StatefulSetOrdinal: 0,
						ClientPodIndex:     0,
						DatabaseName:       framework.DBMySQL,
						User:               framework.MySQLRootUser,
						Param:              "",
					}
					fi.EventuallyDBReady(my, dbInfo)

					By("Creating Table")
					fi.EventuallyCreateTable(my.ObjectMeta, dbInfo).Should(BeTrue())

					By("Inserting Rows")
					fi.EventuallyInsertRow(my.ObjectMeta, dbInfo, 3).Should(BeTrue())

					By("Checking Row Count of Table")
					fi.EventuallyCountRow(my.ObjectMeta, dbInfo).Should(Equal(3))

					issuerMeta := metav1.ObjectMeta{
						Name:      rand.WithUniqSuffix("mysql"),
						Namespace: fi.Namespace(),
					}
					issuer, err := fi.InsureIssuer(issuerMeta, api.ResourceKindMySQL)
					Expect(err).NotTo(HaveOccurred())

					// Adding TLS/SSL and waiting for the success
					requireSSL := true
					_ = fi.CreateMySQLOpsRequestsAndWaitForSuccess(my.Name, func(in *opsapi.MySQLOpsRequest) {
						in.Spec.Type = opsapi.OpsRequestTypeReconfigureTLSs
						in.Spec.TLS = &opsapi.MySQLTLSSpec{
							RequireSSL: &requireSSL,
							TLSSpec: opsapi.TLSSpec{
								TLSConfig: kmapi.TLSConfig{
									IssuerRef: &core.TypedLocalObjectReference{
										Name:     issuer.Name,
										Kind:     "Issuer",
										APIGroup: types.StringP(cm_api.SchemeGroupVersion.Group), //cert-manger.io
									},
								},
							},
						}
					})

					// ReconfigureTLS OpsRequest is succeeded, That's why TLS configuration have been removed.
					// we need to set `User` and `Param` to access database without TLSCustom config.
					dbInfo.User = framework.MySQLRootUser
					dbInfo.Param = fmt.Sprintf("tls=%s", framework.TLSCustomConfig)
					By("Checking MySQL TLS config have added")
					my, err = fi.DBClient().KubedbV1alpha2().MySQLs(fi.Namespace()).Get(context.TODO(), my.Name, metav1.GetOptions{})
					Expect(err).NotTo(HaveOccurred())
					sts, err := fi.KubeClient().AppsV1().StatefulSets(fi.Namespace()).Get(context.TODO(), my.OffshootName(), metav1.GetOptions{})
					Expect(err).NotTo(HaveOccurred())
					util.CheckMySQLTLSConfigAdded(my, sts)

					// Retrieve Inserted Data
					By("Checking Row Count of Table")
					fi.EventuallyCountRow(my.ObjectMeta, dbInfo).Should(Equal(3))
				})
			})

			Context("Rotate TLS/SSL Certificates", func() {
				It("Should rotate TLS/SSL Certificates", func() {
					// MySQL objectMeta
					myMeta := metav1.ObjectMeta{
						Name:      rand.WithUniqSuffix("mysql"),
						Namespace: fi.Namespace(),
					}
					issuer, err := fi.InsureIssuer(myMeta, api.ResourceKindMySQL)
					Expect(err).NotTo(HaveOccurred())
					// Create MySQL Group Replication with tls secured and wait for running
					my, err := fi.CreateMySQLAndWaitForRunning(framework.DBVersion, func(in *api.MySQL) {
						in.Spec.Replicas = types.Int32P(api.MySQLDefaultGroupSize)
						clusterMode := api.MySQLClusterModeGroup
						in.Spec.Topology = &api.MySQLClusterTopology{
							Mode: &clusterMode,
							Group: &api.MySQLGroupSpec{
								Name:         "dc002fc3-c412-4d18-b1d4-66c1fbfbbc9b",
								BaseServerID: types.Int64P(api.MySQLDefaultBaseServerID),
							},
						}
						// configure TLS issuer to MySQL CRD
						in.Spec.RequireSSL = true
						in.Spec.TLS = &kmapi.TLSConfig{
							IssuerRef: &core.TypedLocalObjectReference{
								Name:     issuer.Name,
								Kind:     "Issuer",
								APIGroup: types.StringP(cm_api.SchemeGroupVersion.Group), //cert-manger.io
							},
							Certificates: []kmapi.CertificateSpec{
								{
									Alias: string(api.MySQLServerCert),
									Subject: &kmapi.X509Subject{
										Organizations: []string{
											"kubedb:server",
										},
									},
									DNSNames: []string{
										"localhost",
									},
									IPAddresses: []string{
										"127.0.0.1",
									},
								},
							},
						}
					})
					Expect(err).NotTo(HaveOccurred())
					// Database connection information
					dbInfo := framework.DatabaseConnectionInfo{
						StatefulSetOrdinal: 0,
						ClientPodIndex:     0,
						DatabaseName:       framework.DBMySQL,
						User:               framework.MySQLRootUser,
						Param:              fmt.Sprintf("tls=%s", framework.TLSCustomConfig),
					}
					fi.EventuallyDBReady(my, dbInfo)

					// Create a mysql User with required SSL
					By("Create mysql User with required SSL")
					fi.EventuallyCreateUserWithRequiredSSL(my.ObjectMeta, dbInfo).Should(BeTrue())
					dbInfo.User = framework.MySQLRequiredSSLUser
					fi.EventuallyCheckConnectionRequiredSSLUser(my, dbInfo)

					By("Creating Table")
					fi.EventuallyCreateTable(my.ObjectMeta, dbInfo).Should(BeTrue())

					By("Inserting Rows")
					fi.EventuallyInsertRow(my.ObjectMeta, dbInfo, 3).Should(BeTrue())

					By("Checking Row Count of Table")
					fi.EventuallyCountRow(my.ObjectMeta, dbInfo).Should(Equal(3))

					// Collect certs revision to verify next that certs issuing are updated
					revisions, err := fi.GetAllCertsRevision(my)
					Expect(err).NotTo(HaveOccurred())

					// Renew Certificates and waiting for the success
					duration, err := time.ParseDuration("24h30m")
					Expect(err).NotTo(HaveOccurred())
					_ = fi.CreateMySQLOpsRequestsAndWaitForSuccess(my.Name, func(in *opsapi.MySQLOpsRequest) {
						in.Spec.Type = opsapi.OpsRequestTypeReconfigureTLSs
						in.Spec.TLS = &opsapi.MySQLTLSSpec{
							TLSSpec: opsapi.TLSSpec{
								RotateCertificates: true,
								TLSConfig: kmapi.TLSConfig{
									Certificates: []kmapi.CertificateSpec{
										{
											Alias: string(api.MySQLServerCert),
											RenewBefore: &metav1.Duration{
												Duration: duration,
											},
											EmailAddresses: []string{
												"abc@appscode.com",
												"test@appscode.com",
											},
										},
									},
								},
							},
						}
					})

					By("Verify issuing certs are updated with new revision")
					my, err = fi.DBClient().KubedbV1alpha2().MySQLs(fi.Namespace()).Get(context.TODO(), my.Name, metav1.GetOptions{})
					Expect(err).NotTo(HaveOccurred())
					updatedRevisions, err := fi.GetAllCertsRevision(my)
					Expect(err).NotTo(HaveOccurred())
					_, err = fi.CheckAllCertsRevisionUpdated(revisions, updatedRevisions)
					Expect(err).NotTo(HaveOccurred())

					// Retrieve Inserted Data
					By("Checking Row Count of Table")
					fi.EventuallyCountRow(my.ObjectMeta, dbInfo).Should(Equal(3))
				})
			})

			Context("Set require secure transport OFF", func() {
				FIt("Should set secure require transport OFF", func() {
					// MySQL objectMeta
					myMeta := metav1.ObjectMeta{
						Name:      rand.WithUniqSuffix("mysql"),
						Namespace: fi.Namespace(),
					}
					issuer, err := fi.InsureIssuer(myMeta, api.ResourceKindMySQL)
					Expect(err).NotTo(HaveOccurred())
					// Create MySQL Group Replication with tls secured and wait for running
					my, err := fi.CreateMySQLAndWaitForRunning(framework.DBVersion, func(in *api.MySQL) {
						in.Spec.Replicas = types.Int32P(api.MySQLDefaultGroupSize)
						clusterMode := api.MySQLClusterModeGroup
						in.Spec.Topology = &api.MySQLClusterTopology{
							Mode: &clusterMode,
							Group: &api.MySQLGroupSpec{
								Name:         "dc002fc3-c412-4d18-b1d4-66c1fbfbbc9b",
								BaseServerID: types.Int64P(api.MySQLDefaultBaseServerID),
							},
						}
						// configure TLS issuer to MySQL CRD
						in.Spec.RequireSSL = true
						in.Spec.TLS = &kmapi.TLSConfig{
							IssuerRef: &core.TypedLocalObjectReference{
								Name:     issuer.Name,
								Kind:     "Issuer",
								APIGroup: types.StringP(cm_api.SchemeGroupVersion.Group), //cert-manger.io
							},
							Certificates: []kmapi.CertificateSpec{
								{
									Alias: string(api.MySQLServerCert),
									Subject: &kmapi.X509Subject{
										Organizations: []string{
											"kubedb:server",
										},
									},
									DNSNames: []string{
										"localhost",
									},
									IPAddresses: []string{
										"127.0.0.1",
									},
								},
							},
						}
					})
					Expect(err).NotTo(HaveOccurred())
					// Database connection information
					dbInfo := framework.DatabaseConnectionInfo{
						StatefulSetOrdinal: 0,
						ClientPodIndex:     0,
						DatabaseName:       framework.DBMySQL,
						User:               framework.MySQLRootUser,
						Param:              fmt.Sprintf("tls=%s", framework.TLSCustomConfig),
					}
					fi.EventuallyDBReady(my, dbInfo)

					// Create a mysql User with required SSL
					By("Create mysql User with required SSL")
					fi.EventuallyCreateUserWithRequiredSSL(my.ObjectMeta, dbInfo).Should(BeTrue())
					dbInfo.User = framework.MySQLRequiredSSLUser
					fi.EventuallyCheckConnectionRequiredSSLUser(my, dbInfo)

					By("Creating Table")
					fi.EventuallyCreateTable(my.ObjectMeta, dbInfo).Should(BeTrue())

					By("Inserting Rows")
					fi.EventuallyInsertRow(my.ObjectMeta, dbInfo, 3).Should(BeTrue())

					By("Checking Row Count of Table")
					fi.EventuallyCountRow(my.ObjectMeta, dbInfo).Should(Equal(3))

					// Set require secure transport OFF and waiting for the success
					requireSSL := false
					_ = fi.CreateMySQLOpsRequestsAndWaitForSuccess(my.Name, func(in *opsapi.MySQLOpsRequest) {
						in.Spec.Type = opsapi.OpsRequestTypeReconfigureTLSs
						in.Spec.TLS = &opsapi.MySQLTLSSpec{
							RequireSSL: &requireSSL,
						}
					})

					By("Verify requireSSL mode off")
					// require secure transport is set to OFF that's why param is set to tls=false
					dbInfo.Param = fmt.Sprintf("tls=%s", framework.TLSSkibVerify)
					cfg := "require_secure_transport=OFF"
					fi.EventuallyMySQLVariable(my.ObjectMeta, dbInfo, cfg).Should(matcher.UseCustomConfig(cfg))

					// Retrieve Inserted Data
					By("Checking Row Count of Table")
					fi.EventuallyCountRow(my.ObjectMeta, dbInfo).Should(Equal(3))
				})
			})
		})
	})
})
