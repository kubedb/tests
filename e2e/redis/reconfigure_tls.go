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

package redis

import (
	"fmt"
	"strings"

	api "kubedb.dev/apimachinery/apis/kubedb/v1alpha2"
	dbaapi "kubedb.dev/apimachinery/apis/ops/v1alpha1"
	"kubedb.dev/tests/e2e/framework"

	"github.com/appscode/go/types"
	cm_api "github.com/jetstack/cert-manager/pkg/apis/certmanager/v1beta1"
	. "github.com/onsi/ginkgo"
	. "github.com/onsi/gomega"
	core "k8s.io/api/core/v1"
	kerr "k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	kmapi "kmodules.xyz/client-go/api/v1"
)

var _ = Describe("Reconfigure Redis TLS", func() {
	to := testOptions{}
	testName := framework.ReconfigureTLS
	BeforeEach(func() {
		to.Invocation = framework.NewInvocation()
		if !runTestEnterprise(testName) {
			Skip(fmt.Sprintf("Provide test profile `%s` or `all` or `enterprise` to test this.", testName))
		}
		if framework.SSLEnabled && !strings.HasPrefix(framework.DBVersion, "6.") {
			Skip(fmt.Sprintf("TLS is not supported for version `%s` in redis", framework.DBVersion))
		}
	})

	AfterEach(func() {
		By("Check if Redis " + to.redis.Name + " exists.")
		rd, err := to.GetRedis(to.redis.ObjectMeta)
		if err != nil {
			if kerr.IsNotFound(err) {
				// Redis was not created. Hence, rest of cleanup is not necessary.
				return
			}
			Expect(err).NotTo(HaveOccurred())
		}

		By("Update redis to set spec.terminationPolicy = WipeOut")
		_, err = to.PatchRedis(rd.ObjectMeta, func(in *api.Redis) *api.Redis {
			in.Spec.TerminationPolicy = api.TerminationPolicyWipeOut
			return in
		})
		Expect(err).NotTo(HaveOccurred())

		//Delete Redis
		By("Delete redis")
		err = to.DeleteRedis(to.redis.ObjectMeta)
		Expect(err).NotTo(HaveOccurred())

		By("Delete RedisOpsRequest")
		err = to.DeleteRedisOpsRequest(to.redisOpsReq.ObjectMeta)
		Expect(err).NotTo(HaveOccurred())

		By("Wait for redis resources to be wipedOut")
		to.EventuallyWipedOut(to.redis.ObjectMeta, api.ResourceKindRedis).Should(Succeed())
	})
	Context("Remove TLS", func() {
		tls := &dbaapi.TLSSpec{
			Remove: true,
		}
		Context("Standalone Redis", func() {
			BeforeEach(func() {
				to.TestConfig().UseTLS = true
				to.redis = to.RedisStandalone()
				to.redisOpsReq = to.RedisOpsRequestTLSReconfiguration(to.redis.Name, to.redis.Namespace, tls)
			})

			It("Should Remove Redis TLS", func() {
				to.shouldTestOpsReq()
			})
		})

		Context("Redis Cluster", func() {
			BeforeEach(func() {
				to.TestConfig().UseTLS = true
				to.redis = to.RedisCluster(nil, nil)
				to.redisOpsReq = to.RedisOpsRequestTLSReconfiguration(to.redis.Name, to.redis.Namespace, tls)
			})

			It("Should Remove Redis TLS", func() {
				to.shouldTestOpsReq()
			})
		})
	})

	Context("Rotate Certificate", func() {
		tls := &dbaapi.TLSSpec{
			RotateCertificates: true,
			TLSConfig: kmapi.TLSConfig{
				Certificates: []kmapi.CertificateSpec{
					{
						Alias: "client",
						EmailAddresses: []string{
							"anis028@appscode.com",
						},
					},
				},
			},
		}
		Context("Standalone Redis", func() {
			BeforeEach(func() {
				to.TestConfig().UseTLS = true
				to.redis = to.RedisStandalone()
				to.redisOpsReq = to.RedisOpsRequestTLSReconfiguration(to.redis.Name, to.redis.Namespace, tls)
			})

			It("Should Rotate Redis TLS", func() {
				to.shouldTestOpsReq()
			})
		})
		Context("Redis Cluster", func() {
			BeforeEach(func() {
				to.TestConfig().UseTLS = true
				to.redis = to.RedisCluster(nil, nil)
				to.redisOpsReq = to.RedisOpsRequestTLSReconfiguration(to.redis.Name, to.redis.Namespace, tls)
			})

			It("Should Rotate Redis TLS", func() {
				to.shouldTestOpsReq()

			})
		})
	})
	Context("Add TLS", func() {
		var makeTlsSpec = func(meta metav1.ObjectMeta) *dbaapi.TLSSpec {
			issuer, err := to.EnsureIssuer(meta, api.ResourceKindRedis)
			Expect(err).NotTo(HaveOccurred())
			tls := &dbaapi.TLSSpec{
				TLSConfig: kmapi.TLSConfig{
					IssuerRef: &core.TypedLocalObjectReference{
						Name:     issuer.Name,
						Kind:     "Issuer",
						APIGroup: types.StringP(cm_api.SchemeGroupVersion.Group), //cert-manger.io
					},
					Certificates: []kmapi.CertificateSpec{
						{
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
				},
			}
			return tls
		}
		Context("Standalone Redis", func() {
			BeforeEach(func() {
				to.TestConfig().UseTLS = false
				to.redis = to.RedisStandalone()
				to.redisOpsReq = to.RedisOpsRequestTLSReconfiguration(to.redis.Name, to.redis.Namespace, makeTlsSpec(to.redis.ObjectMeta))
			})

			It("Should Add Redis TLS", func() {
				to.shouldTestOpsReq()
			})
		})
		Context("Redis Cluster", func() {
			BeforeEach(func() {
				to.TestConfig().UseTLS = false
				to.redis = to.RedisCluster(nil, nil)
				to.redisOpsReq = to.RedisOpsRequestTLSReconfiguration(to.redis.Name, to.redis.Namespace, makeTlsSpec(to.redis.ObjectMeta))
			})

			It("Should Add Redis TLS", func() {
				to.shouldTestOpsReq()
			})
		})
	})
})
