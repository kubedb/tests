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

package framework

import (
	"context"
	"fmt"

	opsapi "kubedb.dev/apimachinery/apis/ops/v1alpha1"

	"github.com/appscode/go/crypto/rand"
	. "github.com/onsi/ginkgo"
	. "github.com/onsi/gomega"
	v1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	meta_util "kmodules.xyz/client-go/meta"
)

func (fi *Invocation) MariaDBOpsRequestDefinition(dbName string) *opsapi.MariaDBOpsRequest {
	return &opsapi.MariaDBOpsRequest{
		ObjectMeta: metav1.ObjectMeta{
			Name:      rand.WithUniqSuffix("mdops"),
			Namespace: fi.namespace,
			Labels: map[string]string{
				"app": fi.app,
			},
		},
		Spec: opsapi.MariaDBOpsRequestSpec{
			DatabaseRef: v1.LocalObjectReference{
				Name: dbName,
			},
		},
	}
}

func (f *Framework) CreateMariaDBOpsRequest(obj *opsapi.MariaDBOpsRequest) (*opsapi.MariaDBOpsRequest, error) {
	return f.dbClient.OpsV1alpha1().MariaDBOpsRequests(obj.Namespace).Create(context.TODO(), obj, metav1.CreateOptions{})
}

func (f *Framework) DeleteMariaDBOpsRequest(meta metav1.ObjectMeta) error {
	return f.dbClient.OpsV1alpha1().MariaDBOpsRequests(meta.Namespace).Delete(context.TODO(), meta.Name, meta_util.DeleteInForeground())
}

func (f *Framework) EventuallyMariaDBOpsRequestCompleted(meta metav1.ObjectMeta) GomegaAsyncAssertion {
	return Eventually(
		func() bool {
			db, err := f.dbClient.OpsV1alpha1().MariaDBOpsRequests(meta.Namespace).Get(context.TODO(), meta.Name, metav1.GetOptions{})
			if err != nil {
				return false
			}
			if db.Status.Phase == opsapi.OpsRequestPhaseSuccessful ||
				db.Status.Phase == opsapi.OpsRequestPhaseFailed ||
				db.Status.Phase == opsapi.OpsRequestDenied {
				return true
			}
			return false
		},
		Timeout,
		RetryInterval,
	)
}

func (f *Framework) EventuallyMariaDBOpsRequestPhaseSuccessful(meta metav1.ObjectMeta) GomegaAsyncAssertion {
	return Eventually(
		func() bool {
			db, err := f.dbClient.OpsV1alpha1().MariaDBOpsRequests(meta.Namespace).Get(context.TODO(), meta.Name, metav1.GetOptions{})
			if err != nil {
				fmt.Println("Failed to get OpsReq CR. Error: ", err)
				return false
			}
			if db.Status.Phase == opsapi.OpsRequestPhaseSuccessful {
				return true
			}
			return false
		},
		Timeout,
		RetryInterval,
	)
}

func (fi *Invocation) CreateMariaDBOpsRequestsAndWaitForSuccess(dbName string, transformFuncs ...func(in *opsapi.MariaDBOpsRequest)) *opsapi.MariaDBOpsRequest {
	var err error
	// Generate MariaDB Ops Request
	mdOpsReq := fi.MariaDBOpsRequestDefinition(dbName)

	// transformFunc provide a function that made test specific change on the MariaDB OpsRequest
	// apply these test specific changes
	for _, fn := range transformFuncs {
		fn(mdOpsReq)
	}

	By("Creating MariaDB Ops Request")
	mdOpsReq, err = fi.CreateMariaDBOpsRequest(mdOpsReq)
	Expect(err).NotTo(HaveOccurred())
	fi.AppendToCleanupList(mdOpsReq)

	By("Waiting for MariaDB Ops Request to be Completed")
	fi.EventuallyMariaDBOpsRequestCompleted(mdOpsReq.ObjectMeta).Should(BeTrue())

	By("Verify that MariaDB OpsRequest Phase has Succeeded")
	fi.EventuallyMariaDBOpsRequestPhaseSuccessful(mdOpsReq.ObjectMeta).Should(BeTrue())

	return mdOpsReq
}
