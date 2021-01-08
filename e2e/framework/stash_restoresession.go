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

	. "github.com/onsi/ginkgo"
	. "github.com/onsi/gomega"
	"gomodules.xyz/x/crypto/rand"
	core "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	appcat "kmodules.xyz/custom-resources/apis/appcatalog/v1alpha1"
	stash_v1alpha1 "stash.appscode.dev/apimachinery/apis/stash/v1alpha1"
	stash_v1beta1 "stash.appscode.dev/apimachinery/apis/stash/v1beta1"
)

func (i *Invocation) NewRestoreSession(repoName string, transformFuncs ...func(bc *stash_v1beta1.RestoreSession)) *stash_v1beta1.RestoreSession {
	// A generic RestoreSession definition
	restoreSession := &stash_v1beta1.RestoreSession{
		ObjectMeta: metav1.ObjectMeta{
			Name:      rand.WithUniqSuffix(i.app),
			Namespace: i.namespace,
		},
		Spec: stash_v1beta1.RestoreSessionSpec{
			Repository: core.LocalObjectReference{
				Name: repoName,
			},
		},
	}
	// apply test specific modification
	for _, fn := range transformFuncs {
		fn(restoreSession)
	}

	return restoreSession
}

func (i *Invocation) SetupDatabaseRestore(appBinding *appcat.AppBinding, repo *stash_v1alpha1.Repository, transformFuncs ...func(rs *stash_v1beta1.RestoreSession)) (*stash_v1beta1.RestoreSession, error) {
	// Generate RestoreSession definition for database
	restoreSession := i.NewRestoreSession(repo.Name, func(rs *stash_v1beta1.RestoreSession) {
		rs.Labels = appBinding.Labels
		rs.Spec.Task = stash_v1beta1.TaskRef{
			Name: getRestoreAddonName(),
		}
		rs.Spec.Target = &stash_v1beta1.RestoreTarget{
			Alias: i.app,
			Ref: stash_v1beta1.TargetRef{
				APIVersion: appcat.SchemeGroupVersion.String(),
				Kind:       appcat.ResourceKindApp,
				Name:       appBinding.Name,
			},
			Rules: []stash_v1beta1.Rule{
				{
					Snapshots: []string{"latest"},
				},
			},
		}
	})

	// apply test specific modification
	for _, fn := range transformFuncs {
		fn(restoreSession)
	}

	By("Creating RestoreSession: " + restoreSession.Name)
	createdRS, err := i.StashClient.StashV1beta1().RestoreSessions(restoreSession.Namespace).Create(context.TODO(), restoreSession, metav1.CreateOptions{})
	if err != nil {
		return nil, err
	}
	i.AppendToCleanupList(createdRS)

	return createdRS, err
}

func (f *Framework) EventuallyRestoreProcessCompleted(meta metav1.ObjectMeta) GomegaAsyncAssertion {
	return Eventually(
		func() bool {
			rs, err := f.StashClient.StashV1beta1().RestoreSessions(meta.Namespace).Get(context.TODO(), meta.Name, metav1.GetOptions{})
			if err != nil {
				Expect(err).NotTo(HaveOccurred())
				return false
			}
			if rs.Status.Phase == stash_v1beta1.RestoreSucceeded ||
				rs.Status.Phase == stash_v1beta1.RestoreFailed ||
				rs.Status.Phase == stash_v1beta1.RestorePhaseUnknown {
				return true
			}
			return false
		}, WaitTimeOut, PullInterval)
}
