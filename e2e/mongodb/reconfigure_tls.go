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
	"net"
	"strings"

	api "kubedb.dev/apimachinery/apis/kubedb/v1alpha2"
	dbaapi "kubedb.dev/apimachinery/apis/ops/v1alpha1"
	opsapi "kubedb.dev/apimachinery/apis/ops/v1alpha1"
	"kubedb.dev/tests/e2e/framework"

	"github.com/appscode/go/crypto/rand"
	"github.com/appscode/go/types"
	cm_api "github.com/jetstack/cert-manager/pkg/apis/certmanager/v1"
	. "github.com/onsi/ginkgo"
	. "github.com/onsi/gomega"
	"github.com/pkg/errors"
	"gomodules.xyz/cert"
	core "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	kmapi "kmodules.xyz/client-go/api/v1"
	meta_util "kmodules.xyz/client-go/meta"
)

var _ = Describe("Reconfigure TLS", func() {
	to := testOptions{}
	testName := framework.ReconfigureTLS
	BeforeEach(func() {
		to.Invocation = framework.NewInvocation()
		if strings.ToLower(framework.DBType) != api.ResourceSingularMongoDB {
			Skip(fmt.Sprintf("Skipping MongoDB: %s tests...", testName))
		}
		if !framework.RunTestEnterprise(testName) {
			Skip(fmt.Sprintf("Provide test profile `%s` or `all` or `enterprise` to test this.", testName))
		}
		if !framework.SSLEnabled {
			Skip(fmt.Sprintf("Test profile `%s` is only supported when ssl flag is set", testName))
		}
	})

	AfterEach(func() {
		err := to.CleanupTestResources()
		Expect(err).NotTo(HaveOccurred())
		//Delete MongoDB
		By("Delete mongodb")
		err = to.DeleteMongoDB(to.mongodb.ObjectMeta)
		Expect(err).NotTo(HaveOccurred())

		By("Delete mongodb ops request")
		err = to.DeleteMongoDBOpsRequest(to.mongoOpsReq.ObjectMeta)
		Expect(err).NotTo(HaveOccurred())

		By("Wait for mongodb resources to be wipedOut")
		to.EventuallyWipedOut(to.mongodb.ObjectMeta, api.MongoDB{}.ResourceFQN()).Should(Succeed())
	})

	Context("Add", func() {
		shouldTestOpsRequest := func() {
			to.createAndInsertData()

			// Update Database
			By("Updating MongoDB")
			err := to.CreateMongoDBOpsRequest(to.mongoOpsReq)
			Expect(err).NotTo(HaveOccurred())

			By("Waiting for MongoDB Ops Request Phase to be Successful")
			to.EventuallyMongoDBOpsRequestPhase(to.mongoOpsReq.ObjectMeta).Should(Equal(dbaapi.OpsRequestPhaseSuccessful))

			By("Checking DB is Resumed")
			to.EventuallyDatabaseResumed(to.mongodb).Should(BeTrue())

			By("Checking for Ready MongoDB After Successful OpsRequest")
			to.EventuallyMongoDBReady(to.mongodb.ObjectMeta).Should(BeTrue())

			By("Checking ssl Enabled")
			to.EventuallyCheckTLSMongoDB(to.mongodb.ObjectMeta, api.SSLModeRequireSSL)

			By("Checking TLS user created")
			to.EventuallyTLSUserCreated(to.mongodb).Should(BeTrue())

			// Retrieve Inserted Data
			By("Checking Inserted Document after update")
			to.EventuallyDocumentExists(to.mongodb.ObjectMeta, dbName, 1).Should(BeTrue())
		}
		BeforeEach(func() {
			framework.SSLEnabled = false
		})
		AfterEach(func() {
			framework.SSLEnabled = true
		})
		Context("Standalone", func() {
			BeforeEach(func() {
				to.mongodb = to.MongoDBStandalone()
				to.mongodb.Spec.Version = framework.DBVersion
				to.mongodb.Spec.TerminationPolicy = api.TerminationPolicyWipeOut

				issuer, err := to.EnsureIssuer(to.mongodb.ObjectMeta, api.MongoDB{}.ResourceFQN())
				Expect(err).NotTo(HaveOccurred())
				tls := &opsapi.TLSSpec{
					TLSConfig: kmapi.TLSConfig{
						IssuerRef: &core.TypedLocalObjectReference{
							Name:     issuer.Name,
							Kind:     "Issuer",
							APIGroup: types.StringP(cm_api.SchemeGroupVersion.Group), //cert-manger.io
						},
					},
				}
				to.mongoOpsReq = to.MongoDBOpsRequestReconfigureTLS(to.mongodb.Name, to.mongodb.Namespace, tls)
			})

			It("should run successfully", func() {
				shouldTestOpsRequest()
			})
		})

		Context("Replicaset", func() {
			BeforeEach(func() {
				to.mongodb = to.MongoDBRS()
				to.mongodb.Spec.Version = framework.DBVersion
				to.mongodb.Spec.TerminationPolicy = api.TerminationPolicyWipeOut

				issuer, err := to.EnsureIssuer(to.mongodb.ObjectMeta, api.MongoDB{}.ResourceFQN())
				Expect(err).NotTo(HaveOccurred())
				tls := &opsapi.TLSSpec{
					TLSConfig: kmapi.TLSConfig{
						IssuerRef: &core.TypedLocalObjectReference{
							Name:     issuer.Name,
							Kind:     "Issuer",
							APIGroup: types.StringP(cm_api.SchemeGroupVersion.Group), //cert-manger.io
						},
					},
				}
				to.mongoOpsReq = to.MongoDBOpsRequestReconfigureTLS(to.mongodb.Name, to.mongodb.Namespace, tls)
			})

			It("should run successfully", func() {
				shouldTestOpsRequest()
			})
		})

		Context("Sharded", func() {
			BeforeEach(func() {
				to.mongodb = to.MongoDBShard()
				to.mongodb.Spec.Version = framework.DBVersion
				to.mongodb.Spec.TerminationPolicy = api.TerminationPolicyWipeOut

				issuer, err := to.EnsureIssuer(to.mongodb.ObjectMeta, api.MongoDB{}.ResourceFQN())
				Expect(err).NotTo(HaveOccurred())
				tls := &opsapi.TLSSpec{
					TLSConfig: kmapi.TLSConfig{
						IssuerRef: &core.TypedLocalObjectReference{
							Name:     issuer.Name,
							Kind:     "Issuer",
							APIGroup: types.StringP(cm_api.SchemeGroupVersion.Group), //cert-manger.io
						},
					},
				}
				to.mongoOpsReq = to.MongoDBOpsRequestReconfigureTLS(to.mongodb.Name, to.mongodb.Namespace, tls)
			})

			It("should run successfully", func() {
				shouldTestOpsRequest()
			})
		})
	})

	Context("Remove", func() {
		shouldTestOpsRequest := func() {
			to.createAndInsertData()

			// Update Database
			By("Updating MongoDB")
			err := to.CreateMongoDBOpsRequest(to.mongoOpsReq)
			Expect(err).NotTo(HaveOccurred())

			By("Waiting for MongoDB Ops Request Phase to be Successful")
			to.EventuallyMongoDBOpsRequestPhase(to.mongoOpsReq.ObjectMeta).Should(Equal(dbaapi.OpsRequestPhaseSuccessful))

			By("Checking DB is Resumed")
			to.EventuallyDatabaseResumed(to.mongodb).Should(BeTrue())

			By("Checking for Ready MongoDB After Successful OpsRequest")
			to.EventuallyMongoDBReady(to.mongodb.ObjectMeta).Should(BeTrue())

			By("Checking ssl Disabled")
			to.EventuallyCheckTLSMongoDB(to.mongodb.ObjectMeta, api.SSLModeDisabled)

			// Retrieve Inserted Data
			By("Checking Inserted Document after update")
			to.EventuallyDocumentExists(to.mongodb.ObjectMeta, dbName, 1).Should(BeTrue())
		}

		Context("Standalone", func() {
			BeforeEach(func() {
				to.mongodb = to.MongoDBStandalone()
				to.mongodb.Spec.Version = framework.DBVersion
				to.mongodb.Spec.TerminationPolicy = api.TerminationPolicyWipeOut

				tls := &opsapi.TLSSpec{
					Remove: true,
				}
				to.mongoOpsReq = to.MongoDBOpsRequestReconfigureTLS(to.mongodb.Name, to.mongodb.Namespace, tls)
			})

			It("should run successfully", func() {
				shouldTestOpsRequest()
			})
		})

		Context("Replicaset", func() {
			BeforeEach(func() {
				to.mongodb = to.MongoDBRS()
				to.mongodb.Spec.Version = framework.DBVersion
				to.mongodb.Spec.TerminationPolicy = api.TerminationPolicyWipeOut

				tls := &opsapi.TLSSpec{
					Remove: true,
				}
				to.mongoOpsReq = to.MongoDBOpsRequestReconfigureTLS(to.mongodb.Name, to.mongodb.Namespace, tls)
			})

			It("should run successfully", func() {
				shouldTestOpsRequest()
			})
		})

		Context("Sharded", func() {
			BeforeEach(func() {
				to.mongodb = to.MongoDBShard()
				to.mongodb.Spec.Version = framework.DBVersion
				to.mongodb.Spec.TerminationPolicy = api.TerminationPolicyWipeOut

				tls := &opsapi.TLSSpec{
					Remove: true,
				}
				to.mongoOpsReq = to.MongoDBOpsRequestReconfigureTLS(to.mongodb.Name, to.mongodb.Namespace, tls)
			})

			It("should run successfully", func() {
				shouldTestOpsRequest()
			})
		})
	})

	Context("Update", func() {
		Context("IssuerRef", func() {
			commonName := "kubedb-new"
			getSelfSignedCA := func() ([]byte, []byte, error) {
				cfg := cert.Config{
					CommonName:   commonName,
					Organization: []string{"kubedb"},
					AltNames: cert.AltNames{
						DNSNames: []string{"kubedb-new"},
						IPs:      []net.IP{net.ParseIP("127.0.0.1")},
					},
				}
				key, err := cert.NewPrivateKey()
				if err != nil {
					return nil, nil, errors.Wrap(err, "failed to generate private key")
				}
				crt, err := cert.NewSelfSignedCACert(cfg, key)
				if err != nil {
					return nil, nil, errors.Wrap(err, "failed to generate self-signed certificate")
				}

				return cert.EncodeCertPEM(crt), cert.EncodePrivateKeyPEM(key), nil
			}

			insureNewIssuer := func() (*cm_api.Issuer, error) {
				ca, key, err := getSelfSignedCA()
				if err != nil {
					return nil, err
				}
				fqn := api.MongoDB{}.ResourceFQN()
				clientCASecret := &core.Secret{
					ObjectMeta: metav1.ObjectMeta{
						Name:      rand.WithUniqSuffix(to.mongodb.Name + "-new-ca"),
						Namespace: to.mongodb.Namespace,
						Labels: map[string]string{
							meta_util.NameLabelKey:     fqn,
							meta_util.InstanceLabelKey: to.mongodb.Name,
						},
					},
					Data: map[string][]byte{
						"tls.crt": ca,
						"tls.key": key,
					},
				}
				clientCASecret, err = to.CreateSecret(clientCASecret)
				if err != nil {
					return nil, err
				}
				to.AppendToCleanupList(clientCASecret)
				//create issuer
				issuer := to.IssuerForDB(to.mongodb.ObjectMeta, clientCASecret.ObjectMeta, fqn)
				issuer, err = to.CreateIssuer(issuer)
				if err != nil {
					return nil, err
				}
				to.AppendToCleanupList(issuer)
				return issuer, err
			}

			shouldTestOpsRequest := func() {
				to.createAndInsertData()

				// Update Database
				By("Updating MongoDB")
				err := to.CreateMongoDBOpsRequest(to.mongoOpsReq)
				Expect(err).NotTo(HaveOccurred())

				By("Waiting for MongoDB Ops Request Phase to be Successful")
				to.EventuallyMongoDBOpsRequestPhase(to.mongoOpsReq.ObjectMeta).Should(Equal(dbaapi.OpsRequestPhaseSuccessful))

				By("Checking DB is Resumed")
				to.EventuallyDatabaseResumed(to.mongodb).Should(BeTrue())

				By("Checking for Ready MongoDB After Successful OpsRequest")
				to.EventuallyMongoDBReady(to.mongodb.ObjectMeta).Should(BeTrue())

				currentCert, err := to.GetCertificate(to.mongodb, "ca")
				Expect(err).NotTo(HaveOccurred())
				Expect(currentCert.Subject.CommonName).To(BeIdenticalTo(commonName))

				// Retrieve Inserted Data
				By("Checking Inserted Document after update")
				to.EventuallyDocumentExists(to.mongodb.ObjectMeta, dbName, 1).Should(BeTrue())
			}

			Context("Standalone", func() {
				BeforeEach(func() {
					to.mongodb = to.MongoDBStandalone()
					to.mongodb.Spec.Version = framework.DBVersion
					to.mongodb.Spec.TerminationPolicy = api.TerminationPolicyWipeOut

					newIssuer, err := insureNewIssuer()
					Expect(err).NotTo(HaveOccurred())
					tls := &opsapi.TLSSpec{
						TLSConfig: kmapi.TLSConfig{
							IssuerRef: &core.TypedLocalObjectReference{
								Name:     newIssuer.Name,
								Kind:     "Issuer",
								APIGroup: types.StringP(cm_api.SchemeGroupVersion.Group), //cert-manger.io
							},
						},
					}
					to.mongoOpsReq = to.MongoDBOpsRequestReconfigureTLS(to.mongodb.Name, to.mongodb.Namespace, tls)
				})

				It("should run successfully", func() {
					shouldTestOpsRequest()
				})
			})

			Context("Replicaset", func() {
				BeforeEach(func() {
					to.mongodb = to.MongoDBRS()
					to.mongodb.Spec.Version = framework.DBVersion
					to.mongodb.Spec.TerminationPolicy = api.TerminationPolicyWipeOut

					newIssuer, err := insureNewIssuer()
					Expect(err).NotTo(HaveOccurred())
					tls := &opsapi.TLSSpec{
						TLSConfig: kmapi.TLSConfig{
							IssuerRef: &core.TypedLocalObjectReference{
								Name:     newIssuer.Name,
								Kind:     "Issuer",
								APIGroup: types.StringP(cm_api.SchemeGroupVersion.Group), //cert-manger.io
							},
						},
					}
					to.mongoOpsReq = to.MongoDBOpsRequestReconfigureTLS(to.mongodb.Name, to.mongodb.Namespace, tls)
				})

				It("should run successfully", func() {
					shouldTestOpsRequest()
				})
			})

			Context("Sharded", func() {
				BeforeEach(func() {
					to.mongodb = to.MongoDBShard()
					to.mongodb.Spec.Version = framework.DBVersion
					to.mongodb.Spec.TerminationPolicy = api.TerminationPolicyWipeOut

					newIssuer, err := insureNewIssuer()
					Expect(err).NotTo(HaveOccurred())
					tls := &opsapi.TLSSpec{
						TLSConfig: kmapi.TLSConfig{
							IssuerRef: &core.TypedLocalObjectReference{
								Name:     newIssuer.Name,
								Kind:     "Issuer",
								APIGroup: types.StringP(cm_api.SchemeGroupVersion.Group), //cert-manger.io
							},
						},
					}
					to.mongoOpsReq = to.MongoDBOpsRequestReconfigureTLS(to.mongodb.Name, to.mongodb.Namespace, tls)
				})

				It("should run successfully", func() {
					shouldTestOpsRequest()
				})
			})
		})

		Context("Certificates", func() {
			shouldTestOpsRequest := func() {
				to.createAndInsertData()

				// Update Database
				By("Updating MongoDB")
				err := to.CreateMongoDBOpsRequest(to.mongoOpsReq)
				Expect(err).NotTo(HaveOccurred())

				By("Waiting for MongoDB Ops Request Phase to be Successful")
				to.EventuallyMongoDBOpsRequestPhase(to.mongoOpsReq.ObjectMeta).Should(Equal(dbaapi.OpsRequestPhaseSuccessful))

				By("Checking DB is Resumed")
				to.EventuallyDatabaseResumed(to.mongodb).Should(BeTrue())

				By("Checking for Ready MongoDB After Successful OpsRequest")
				to.EventuallyMongoDBReady(to.mongodb.ObjectMeta).Should(BeTrue())

				currentCert, err := to.GetCertificate(to.mongodb, "client")
				Expect(err).NotTo(HaveOccurred())
				Expect(len(currentCert.EmailAddresses)).To(BeIdenticalTo(2))

				// Retrieve Inserted Data
				By("Checking Inserted Document after update")
				to.EventuallyDocumentExists(to.mongodb.ObjectMeta, dbName, 1).Should(BeTrue())
			}
			tlsConfig := kmapi.TLSConfig{
				Certificates: []kmapi.CertificateSpec{
					{
						Alias: string(api.MongoDBClientCert),
						EmailAddresses: []string{
							"abc@appscode.com",
							"test@appscode.com",
						},
					},
				},
			}

			Context("Standalone", func() {
				BeforeEach(func() {
					to.mongodb = to.MongoDBStandalone()
					to.mongodb.Spec.Version = framework.DBVersion
					to.mongodb.Spec.TerminationPolicy = api.TerminationPolicyWipeOut

					tls := &opsapi.TLSSpec{
						TLSConfig: tlsConfig,
					}

					to.mongoOpsReq = to.MongoDBOpsRequestReconfigureTLS(to.mongodb.Name, to.mongodb.Namespace, tls)
				})

				It("should run successfully", func() {
					shouldTestOpsRequest()
				})
			})

			Context("Replicaset", func() {
				BeforeEach(func() {
					to.mongodb = to.MongoDBRS()
					to.mongodb.Spec.Version = framework.DBVersion
					to.mongodb.Spec.TerminationPolicy = api.TerminationPolicyWipeOut

					tls := &opsapi.TLSSpec{
						TLSConfig: tlsConfig,
					}

					to.mongoOpsReq = to.MongoDBOpsRequestReconfigureTLS(to.mongodb.Name, to.mongodb.Namespace, tls)
				})

				It("should run successfully", func() {
					shouldTestOpsRequest()
				})
			})

			Context("Sharded", func() {
				BeforeEach(func() {
					to.mongodb = to.MongoDBShard()
					to.mongodb.Spec.Version = framework.DBVersion
					to.mongodb.Spec.TerminationPolicy = api.TerminationPolicyWipeOut

					tls := &opsapi.TLSSpec{
						TLSConfig: tlsConfig,
					}

					to.mongoOpsReq = to.MongoDBOpsRequestReconfigureTLS(to.mongodb.Name, to.mongodb.Namespace, tls)
				})

				It("should run successfully", func() {
					shouldTestOpsRequest()
				})
			})
		})
	})

	Context("Rotate", func() {
		shouldTestOpsRequest := func() {
			to.createAndInsertData()

			prevCert, err := to.GetCertificate(to.mongodb, "client")
			Expect(err).NotTo(HaveOccurred())

			// Update Database
			By("Updating MongoDB")
			err = to.CreateMongoDBOpsRequest(to.mongoOpsReq)
			Expect(err).NotTo(HaveOccurred())

			By("Waiting for MongoDB Ops Request Phase to be Successful")
			to.EventuallyMongoDBOpsRequestPhase(to.mongoOpsReq.ObjectMeta).Should(Equal(dbaapi.OpsRequestPhaseSuccessful))

			By("Checking DB is Resumed")
			to.EventuallyDatabaseResumed(to.mongodb).Should(BeTrue())

			By("Checking for Ready MongoDB After Successful OpsRequest")
			to.EventuallyMongoDBReady(to.mongodb.ObjectMeta).Should(BeTrue())

			currentCert, err := to.GetCertificate(to.mongodb, "client")
			Expect(err).NotTo(HaveOccurred())
			Expect(currentCert.NotAfter.After(prevCert.NotAfter)).To(BeTrue())

			// Retrieve Inserted Data
			By("Checking Inserted Document after update")
			to.EventuallyDocumentExists(to.mongodb.ObjectMeta, dbName, 1).Should(BeTrue())
		}

		Context("Standalone", func() {
			BeforeEach(func() {
				to.mongodb = to.MongoDBStandalone()
				to.mongodb.Spec.Version = framework.DBVersion
				to.mongodb.Spec.TerminationPolicy = api.TerminationPolicyWipeOut

				tls := &opsapi.TLSSpec{
					RotateCertificates: true,
				}
				to.mongoOpsReq = to.MongoDBOpsRequestReconfigureTLS(to.mongodb.Name, to.mongodb.Namespace, tls)
			})

			It("should run successfully", func() {
				shouldTestOpsRequest()
			})
		})

		Context("Replicaset", func() {
			BeforeEach(func() {
				to.mongodb = to.MongoDBRS()
				to.mongodb.Spec.Version = framework.DBVersion
				to.mongodb.Spec.TerminationPolicy = api.TerminationPolicyWipeOut

				tls := &opsapi.TLSSpec{
					RotateCertificates: true,
				}
				to.mongoOpsReq = to.MongoDBOpsRequestReconfigureTLS(to.mongodb.Name, to.mongodb.Namespace, tls)
			})

			It("should run successfully", func() {
				shouldTestOpsRequest()
			})
		})

		Context("Sharded", func() {
			BeforeEach(func() {
				to.mongodb = to.MongoDBShard()
				to.mongodb.Spec.Version = framework.DBVersion
				to.mongodb.Spec.TerminationPolicy = api.TerminationPolicyWipeOut

				tls := &opsapi.TLSSpec{
					RotateCertificates: true,
				}
				to.mongoOpsReq = to.MongoDBOpsRequestReconfigureTLS(to.mongodb.Name, to.mongodb.Namespace, tls)
			})

			It("should run successfully", func() {
				shouldTestOpsRequest()
			})
		})
	})
})
