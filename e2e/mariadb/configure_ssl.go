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

package mariadb

import (
	"fmt"

	api "kubedb.dev/apimachinery/apis/kubedb/v1alpha2"
	"kubedb.dev/tests/e2e/framework"

	"github.com/appscode/go/crypto/rand"
	"github.com/appscode/go/types"
	_ "github.com/go-sql-driver/mysql"
	cm_api "github.com/jetstack/cert-manager/pkg/apis/certmanager/v1"
	. "github.com/onsi/ginkgo"
	. "github.com/onsi/gomega"
	core "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	kmapi "kmodules.xyz/client-go/api/v1"
)

var _ = Describe("MariaDB TLS/SSL", func() {
	var fi *framework.Invocation

	BeforeEach(func() {
		fi = framework.NewInvocation()

		if !runTestDatabaseType() {
			Skip(fmt.Sprintf("Provide test for database `%s`", api.ResourceSingularMariaDB))
		}
		if !runTestEnterprise(framework.Enterprise) {
			Skip(fmt.Sprintf("Provide test profile `%s` or `all` or `enterprise` to test this.", framework.Enterprise))
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

	Describe("Test", func() {
		Context("Exporter", func() {
			Context("Standalone", func() {
				It("Should verify Exporter", func() {
					// MariaDB objectMeta
					mdMeta := metav1.ObjectMeta{
						Name:      rand.WithUniqSuffix("mariadb"),
						Namespace: fi.Namespace(),
					}
					issuer, err := fi.InsureIssuer(mdMeta, api.MariaDB{}.ResourceFQN())
					Expect(err).NotTo(HaveOccurred())
					// Create MariaDB standalone with SSL secured and wait for running
					md, err := fi.CreateMariaDBAndWaitForRunning(framework.DBVersion, func(in *api.MariaDB) {
						in.Name = mdMeta.Name
						in.Namespace = mdMeta.Namespace
						// configure TLS issuer to MariaDB CRD
						in.Spec.RequireSSL = true
						in.Spec.TLS = &kmapi.TLSConfig{
							IssuerRef: &core.TypedLocalObjectReference{
								Name:     issuer.Name,
								Kind:     "Issuer",
								APIGroup: types.StringP(cm_api.SchemeGroupVersion.Group), //cert-manger.io
							},
							Certificates: []kmapi.CertificateSpec{
								{
									Alias: string(api.MariaDBServerCert),
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
						// Add Monitor
						fi.AddMariaDBMonitor(in)
					})
					Expect(err).NotTo(HaveOccurred())
					dbInfo := framework.DatabaseConnectionInfo{
						StatefulSetOrdinal: 0,
						ClientPodIndex:     0,
						DatabaseName:       framework.DBMySQL,
						User:               framework.MySQLRootUser,
						Param:              fmt.Sprintf("tls=%s", framework.TLSCustomConfig),
					}
					fi.EventuallyDBReadyMD(md, dbInfo)

					By("Verify exporter")
					err = fi.VerifyMariaDBExporter(md)
					Expect(err).NotTo(HaveOccurred())
					By("Done")
				})
			})

			Context("Galera Cluster", func() {
				It("Should verify Exporter", func() {
					// MySQL objectMeta
					mdMeta := metav1.ObjectMeta{
						Name:      rand.WithUniqSuffix("mairadb"),
						Namespace: fi.Namespace(),
					}
					issuer, err := fi.InsureIssuer(mdMeta, api.MariaDB{}.ResourceFQN())
					Expect(err).NotTo(HaveOccurred())
					// Create MariaDB galera cluster with SSL secured and wait for running
					md, err := fi.CreateMariaDBAndWaitForRunning(framework.DBVersion, func(in *api.MariaDB) {
						in.Name = mdMeta.Name
						in.Namespace = mdMeta.Namespace
						in.Spec.Replicas = types.Int32P(api.MariaDBDefaultClusterSize)
						// configure TLS issuer to MariaDB CRD
						in.Spec.RequireSSL = true
						in.Spec.TLS = &kmapi.TLSConfig{
							IssuerRef: &core.TypedLocalObjectReference{
								Name:     issuer.Name,
								Kind:     "Issuer",
								APIGroup: types.StringP(cm_api.SchemeGroupVersion.Group), //cert-manger.io
							},
							Certificates: []kmapi.CertificateSpec{
								{
									Alias: string(api.MariaDBServerCert),
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
						fi.AddMariaDBMonitor(in)
					})
					Expect(err).NotTo(HaveOccurred())
					dbInfo := framework.DatabaseConnectionInfo{
						StatefulSetOrdinal: 0,
						ClientPodIndex:     0,
						DatabaseName:       framework.DBMySQL,
						User:               framework.MySQLRootUser,
						Param:              fmt.Sprintf("tls=%s", framework.TLSCustomConfig),
					}
					fi.EventuallyDBReadyMD(md, dbInfo)

					By("Verify exporter")
					err = fi.VerifyMariaDBExporter(md)
					Expect(err).NotTo(HaveOccurred())
					By("Done")
				})
			})
		})
		// TODO: Done So far

		Context("General", func() {
			Context("with requireSSL true", func() {
				Context("Standalone", func() {
					It("should run successfully", func() {
						// MariaDB objectMeta
						mdMeta := metav1.ObjectMeta{
							Name:      rand.WithUniqSuffix("mariadb"),
							Namespace: fi.Namespace(),
						}
						issuer, err := fi.InsureIssuer(mdMeta, api.MariaDB{}.ResourceFQN())
						Expect(err).NotTo(HaveOccurred())
						// Create MariaDB standalone with SSL secured and wait for running
						md, err := fi.CreateMariaDBAndWaitForRunning(framework.DBVersion, func(in *api.MariaDB) {
							in.Name = mdMeta.Name
							in.Namespace = mdMeta.Namespace
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
										Alias: string(api.MariaDBServerCert),
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
						dbInfo := framework.DatabaseConnectionInfo{
							StatefulSetOrdinal: 0,
							ClientPodIndex:     0,
							DatabaseName:       framework.DBMySQL,
							User:               framework.MySQLRootUser,
							Param:              fmt.Sprintf("tls=%s", framework.TLSCustomConfig),
						}
						fi.EventuallyDBReadyMD(md, dbInfo)

						// Create a mariadb User with required SSL
						By("Create mysql User with required SSL")
						fi.EventuallyCreateUserWithRequiredSSLMD(md.ObjectMeta, dbInfo).Should(BeTrue())
						dbInfo.User = framework.MySQLRequiredSSLUser
						fi.EventuallyCheckConnectionRequiredSSLUserMD(md, dbInfo)

						By("Creating Table")
						fi.EventuallyCreateTableMD(md.ObjectMeta, dbInfo).Should(BeTrue())

						By("Inserting Rows")
						fi.EventuallyInsertRowMD(md.ObjectMeta, dbInfo, 3).Should(BeTrue())

						By("Checking Row Count of Table")
						fi.EventuallyCountRowMD(md.ObjectMeta, dbInfo).Should(Equal(3))
					})
				})

				Context("Galera Cluster", func() {
					It("should run successfully", func() {
						// MariaDB objectMeta
						myMeta := metav1.ObjectMeta{
							Name:      rand.WithUniqSuffix("mairadb"),
							Namespace: fi.Namespace(),
						}
						issuer, err := fi.InsureIssuer(myMeta, api.MariaDB{}.ResourceFQN())
						Expect(err).NotTo(HaveOccurred())
						// Create MariaDB standalone with SSL secured and wait for running
						md, err := fi.CreateMariaDBAndWaitForRunning(framework.DBVersion, func(in *api.MariaDB) {
							in.Name = myMeta.Name
							in.Namespace = myMeta.Namespace
							in.Spec.Replicas = types.Int32P(api.MariaDBDefaultClusterSize)
							// configure TLS issuer to MariaDB CRD
							in.Spec.RequireSSL = true
							in.Spec.TLS = &kmapi.TLSConfig{
								IssuerRef: &core.TypedLocalObjectReference{
									Name:     issuer.Name,
									Kind:     "Issuer",
									APIGroup: types.StringP(cm_api.SchemeGroupVersion.Group), //cert-manger.io
								},
								Certificates: []kmapi.CertificateSpec{
									{
										Alias: string(api.MariaDBServerCert),
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
						dbInfo := framework.DatabaseConnectionInfo{
							StatefulSetOrdinal: 0,
							ClientPodIndex:     0,
							DatabaseName:       framework.DBMySQL,
							User:               framework.MySQLRootUser,
							Param:              fmt.Sprintf("tls=%s", framework.TLSCustomConfig),
						}
						fi.EventuallyDBReadyMD(md, dbInfo)

						// Create a mysql User with required SSL
						By("Create mysql User with required SSL")
						fi.EventuallyCreateUserWithRequiredSSLMD(md.ObjectMeta, dbInfo).Should(BeTrue())
						dbInfo.User = framework.MySQLRequiredSSLUser
						fi.EventuallyCheckConnectionRequiredSSLUserMD(md, dbInfo)

						By("Creating Table")
						fi.EventuallyCreateTableMD(md.ObjectMeta, dbInfo).Should(BeTrue())

						By("Inserting Rows")
						fi.EventuallyInsertRowMD(md.ObjectMeta, dbInfo, 3).Should(BeTrue())

						By("Checking Row Count of Table")
						fi.EventuallyCountRowMD(md.ObjectMeta, dbInfo).Should(Equal(3))
					})
				})
			})

			Context("with requireSSL false", func() {
				Context("Standalone", func() {
					It("should run successfully", func() {
						// MariaDB objectMeta
						mdMeta := metav1.ObjectMeta{
							Name:      rand.WithUniqSuffix("mariadb"),
							Namespace: fi.Namespace(),
						}
						issuer, err := fi.InsureIssuer(mdMeta, api.MySQL{}.ResourceFQN())
						Expect(err).NotTo(HaveOccurred())
						// Create MariaDB standalone with SSL secured and wait for running
						md, err := fi.CreateMariaDBAndWaitForRunning(framework.DBVersion, func(in *api.MariaDB) {
							in.Name = mdMeta.Name
							in.Namespace = mdMeta.Namespace
							// configure TLS issuer to MariaDB CRD
							in.Spec.RequireSSL = false
							in.Spec.TLS = &kmapi.TLSConfig{
								IssuerRef: &core.TypedLocalObjectReference{
									Name:     issuer.Name,
									Kind:     "Issuer",
									APIGroup: types.StringP(cm_api.SchemeGroupVersion.Group), //cert-manger.io
								},
								Certificates: []kmapi.CertificateSpec{
									{
										Alias: string(api.MariaDBServerCert),
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
						dbInfo := framework.DatabaseConnectionInfo{
							StatefulSetOrdinal: 0,
							ClientPodIndex:     0,
							DatabaseName:       framework.DBMySQL,
							User:               framework.MySQLRootUser,
							Param:              fmt.Sprintf("tls=%s", framework.TLSCustomConfig),
						}
						fi.EventuallyDBReadyMD(md, dbInfo)

						// Create a mysql User with required SSL
						By("Create mysql User with required SSL")
						fi.EventuallyCreateUserWithRequiredSSLMD(md.ObjectMeta, dbInfo).Should(BeTrue())
						dbInfo.User = framework.MySQLRequiredSSLUser
						fi.EventuallyCheckConnectionRequiredSSLUserMD(md, dbInfo)

						By("Creating Table")
						fi.EventuallyCreateTableMD(md.ObjectMeta, dbInfo).Should(BeTrue())

						By("Inserting Rows")
						fi.EventuallyInsertRowMD(md.ObjectMeta, dbInfo, 3).Should(BeTrue())

						By("Checking Row Count of Table")
						fi.EventuallyCountRowMD(md.ObjectMeta, dbInfo).Should(Equal(3))
					})
				})
				// TODO: Done so far(1)
				Context("Galera CLuster", func() {
					It("should run successfully", func() {
						// MariaDB objectMeta
						mdMeta := metav1.ObjectMeta{
							Name:      rand.WithUniqSuffix("mariadb"),
							Namespace: fi.Namespace(),
						}
						issuer, err := fi.InsureIssuer(mdMeta, api.MariaDB{}.ResourceFQN())
						Expect(err).NotTo(HaveOccurred())
						// Create MariaDB standalone with SSL secured and wait for running
						md, err := fi.CreateMariaDBAndWaitForRunning(framework.DBVersion, func(in *api.MariaDB) {
							in.Name = mdMeta.Name
							in.Namespace = mdMeta.Namespace
							in.Spec.Replicas = types.Int32P(api.MariaDBDefaultClusterSize)
							// configure TLS issuer to MySQL CRD
							in.Spec.RequireSSL = false
							in.Spec.TLS = &kmapi.TLSConfig{
								IssuerRef: &core.TypedLocalObjectReference{
									Name:     issuer.Name,
									Kind:     "Issuer",
									APIGroup: types.StringP(cm_api.SchemeGroupVersion.Group), //cert-manger.io
								},
								Certificates: []kmapi.CertificateSpec{
									{
										Alias: string(api.MariaDBServerCert),
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
						dbInfo := framework.DatabaseConnectionInfo{
							StatefulSetOrdinal: 0,
							ClientPodIndex:     0,
							DatabaseName:       framework.DBMySQL,
							User:               framework.MySQLRootUser,
							Param:              fmt.Sprintf("tls=%s", framework.TLSCustomConfig),
						}
						fi.EventuallyDBReadyMD(md, dbInfo)

						// Create a mysql User with required SSL
						By("Create mysql User with required SSL")
						fi.EventuallyCreateUserWithRequiredSSLMD(md.ObjectMeta, dbInfo).Should(BeTrue())
						dbInfo.User = framework.MySQLRequiredSSLUser
						fi.EventuallyCheckConnectionRequiredSSLUserMD(md, dbInfo)

						By("Creating Table")
						fi.EventuallyCreateTableMD(md.ObjectMeta, dbInfo).Should(BeTrue())

						By("Inserting Rows")
						fi.EventuallyInsertRowMD(md.ObjectMeta, dbInfo, 3).Should(BeTrue())

						By("Checking Row Count of Table")
						fi.EventuallyCountRowMD(md.ObjectMeta, dbInfo).Should(Equal(3))
					})
				})
			})
		})
	})
})
