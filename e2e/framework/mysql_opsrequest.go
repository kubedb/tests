/*
Copyright The KubeDB Authors.
Licensed under the Apache License, Version 2.0 (the "License");
you may not use this file except in compliance with the License.
You may obtain a copy of the License at
    http://www.apache.org/licenses/LICENSE-2.0
Unless required by applicable law or agreed to in writing, software
distributed under the License is distributed on an "AS IS" BASIS,
WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
See the License for the specific language governing permissions and
limitations under the License.
*/
package framework

import (
	"context"

	opsapi "kubedb.dev/apimachinery/apis/ops/v1alpha1"

	"github.com/appscode/go/crypto/rand"
	. "github.com/onsi/ginkgo"
	. "github.com/onsi/gomega"
	v1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	meta_util "kmodules.xyz/client-go/meta"
)

func (fi *Invocation) MySQLOpsRequestDefinition(dbName string) *opsapi.MySQLOpsRequest {
	return &opsapi.MySQLOpsRequest{
		ObjectMeta: metav1.ObjectMeta{
			Name:      rand.WithUniqSuffix("myops"),
			Namespace: fi.namespace,
			Labels: map[string]string{
				"app": fi.app,
			},
		},
		Spec: opsapi.MySQLOpsRequestSpec{
			DatabaseRef: v1.LocalObjectReference{
				Name: dbName,
			},
		},
	}
}

func (f *Framework) CreateMySQLOpsRequest(obj *opsapi.MySQLOpsRequest) (*opsapi.MySQLOpsRequest, error) {
	return f.dbClient.OpsV1alpha1().MySQLOpsRequests(obj.Namespace).Create(context.TODO(), obj, metav1.CreateOptions{})
}

func (f *Framework) DeleteMySQLOpsRequest(meta metav1.ObjectMeta) error {
	return f.dbClient.OpsV1alpha1().MySQLOpsRequests(meta.Namespace).Delete(context.TODO(), meta.Name, meta_util.DeleteInForeground())
}

func (f *Framework) EventuallyMySQLOpsRequestCompleted(meta metav1.ObjectMeta) GomegaAsyncAssertion {
	return Eventually(
		func() bool {
			db, err := f.dbClient.OpsV1alpha1().MySQLOpsRequests(meta.Namespace).Get(context.TODO(), meta.Name, metav1.GetOptions{})
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

func (fi *Invocation) CreateMySQLOpsRequestsAndWaitForSuccess(dbName string, transformFuncs ...func(in *opsapi.MySQLOpsRequest)) *opsapi.MySQLOpsRequest {
	var err error
	// Generate MySQL Ops Request
	myOR := fi.MySQLOpsRequestDefinition(dbName)

	// transformFunc provide a function that made test specific change on the MySQL OpsRequest
	// apply these test specific changes
	for _, fn := range transformFuncs {
		fn(myOR)
	}

	By("Creating MySQL Ops Request")
	myOR, err = fi.CreateMySQLOpsRequest(myOR)
	Expect(err).NotTo(HaveOccurred())
	fi.AppendToCleanupList(myOR)

	By("Waiting for MySQL Ops Request to be Completed")
	fi.EventuallyMySQLOpsRequestCompleted(myOR.ObjectMeta).Should(BeTrue())

	By("Verify that MySQL OpsRequest Phase has Succeeded")
	myOR, err = fi.dbClient.OpsV1alpha1().MySQLOpsRequests(myOR.Namespace).Get(context.TODO(), myOR.Name, metav1.GetOptions{})
	Expect(err).NotTo(HaveOccurred())
	Expect(myOR.Status.Phase).Should(Equal(opsapi.OpsRequestPhaseSuccessful))

	return myOR
}
