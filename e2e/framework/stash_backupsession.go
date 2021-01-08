/*
Copyright AppsCode Inc. and Contributors

Licensed under the AppsCode Free Trial License 1.0.0 (the "License");
you may not use this file except in compliance with the License.
You may obtain a copy of the License at

    https://github.com/appscode/licenses/raw/1.0.0/AppsCode-Free-Trial-1.0.0.md

Unless required by applicable law or agreed to in writing, software
distributed under the License is distributed on an "AS IS" BASIS,
WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
See the License for the speciic language governing permissions and
limitations under the License.
*/

package framework

import (
	"context"

	. "github.com/onsi/gomega"
	"gomodules.xyz/x/crypto/rand"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	stash_apis "stash.appscode.dev/apimachinery/apis"
	stash_v1beta1 "stash.appscode.dev/apimachinery/apis/stash/v1beta1"
)

func (fi *Invocation) TriggerInstantBackup(objMeta metav1.ObjectMeta, invokerRef stash_v1beta1.BackupInvokerRef) (*stash_v1beta1.BackupSession, error) {
	backupSession := &stash_v1beta1.BackupSession{
		ObjectMeta: metav1.ObjectMeta{
			Name:      rand.WithUniqSuffix(objMeta.Name),
			Namespace: objMeta.Namespace,
			Labels: map[string]string{
				stash_apis.LabelApp:         stash_apis.AppLabelStash,
				stash_apis.LabelInvokerType: invokerRef.Kind,
				stash_apis.LabelInvokerName: invokerRef.Name,
			},
		},
		Spec: stash_v1beta1.BackupSessionSpec{
			Invoker: stash_v1beta1.BackupInvokerRef{
				APIGroup: stash_v1beta1.SchemeGroupVersion.Group,
				Kind:     invokerRef.Kind,
				Name:     invokerRef.Name,
			},
		},
	}
	return fi.StashClient.StashV1beta1().BackupSessions(backupSession.Namespace).Create(context.TODO(), backupSession, metav1.CreateOptions{})
}

func (f *Framework) EventuallyBackupProcessCompleted(meta metav1.ObjectMeta) GomegaAsyncAssertion {
	return Eventually(
		func() bool {
			bs, err := f.StashClient.StashV1beta1().BackupSessions(meta.Namespace).Get(context.TODO(), meta.Name, metav1.GetOptions{})
			if err != nil {
				Expect(err).NotTo(HaveOccurred())
				return false
			}
			if bs.Status.Phase == stash_v1beta1.BackupSessionSucceeded ||
				bs.Status.Phase == stash_v1beta1.BackupSessionFailed ||
				bs.Status.Phase == stash_v1beta1.BackupSessionUnknown {
				return true
			}
			return false
		}, WaitTimeOut, PullInterval)
}
