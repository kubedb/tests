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
	kerr "k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	appcat "kmodules.xyz/custom-resources/apis/appcatalog/v1alpha1"
	stash_v1alpha1 "stash.appscode.dev/apimachinery/apis/stash/v1alpha1"
	stash_v1beta1 "stash.appscode.dev/apimachinery/apis/stash/v1beta1"
)

func (fi *Invocation) NewBackupConfiguration(repoName string, transformFuncs ...func(bc *stash_v1beta1.BackupConfiguration)) *stash_v1beta1.BackupConfiguration {
	// a generic BackupConfiguration definition
	backupConfig := &stash_v1beta1.BackupConfiguration{
		ObjectMeta: metav1.ObjectMeta{
			Name:      rand.WithUniqSuffix(fi.app),
			Namespace: fi.namespace,
		},
		Spec: stash_v1beta1.BackupConfigurationSpec{
			// Instead of waiting for a schedule to appear, we are going to trigger the backup manually  to minimize the testing period.
			// Hence, we are setting a large interval so that the scheduled slot never appear during testing.
			Schedule: "0 0 * 12 *",
			Repository: core.LocalObjectReference{
				Name: repoName,
			},
			RetentionPolicy: stash_v1alpha1.RetentionPolicy{
				Name:     "keep-last-5",
				KeepLast: 5,
				Prune:    true,
			},
		},
	}
	// apply test specific modification
	for _, fn := range transformFuncs {
		fn(backupConfig)
	}

	return backupConfig
}

func (fi *Invocation) SetupDatabaseBackup(appBinding *appcat.AppBinding, transformFuncs ...func(bc *stash_v1beta1.BackupConfiguration)) (*stash_v1beta1.BackupConfiguration, *stash_v1alpha1.Repository, error) {
	// Setup Repository
	repo, err := fi.SetupMinioRepository()
	if err != nil {
		return nil, nil, err
	}

	// Generate BackupConfiguration definition for database
	backupConfig := fi.NewBackupConfiguration(repo.Name, func(bc *stash_v1beta1.BackupConfiguration) {
		bc.Spec.Task = stash_v1beta1.TaskRef{
			Name: getBackupAddonName(),
		}
		bc.Spec.Target = &stash_v1beta1.BackupTarget{
			Alias: fi.app,
			Ref: stash_v1beta1.TargetRef{
				APIVersion: appcat.SchemeGroupVersion.String(),
				Kind:       appcat.ResourceKindApp,
				Name:       appBinding.Name,
			},
		}
	})

	// apply test specific modification
	for _, fn := range transformFuncs {
		fn(backupConfig)
	}

	By("Creating BackupConfiguration: " + backupConfig.Name)
	createdBC, err := fi.StashClient.StashV1beta1().BackupConfigurations(backupConfig.Namespace).Create(context.TODO(), backupConfig, metav1.CreateOptions{})
	if err != nil {
		return nil, nil, err
	}
	fi.AppendToCleanupList(createdBC)

	By("Verifying that backup triggering CronJob has been created")
	fi.EventuallyCronJobCreated(createdBC).Should(BeTrue())

	return createdBC, repo, err
}

func (fi *Invocation) EventuallyBackupConfigurationCreated(meta metav1.ObjectMeta) GomegaAsyncAssertion {
	return Eventually(
		func() bool {
			_, err := fi.StashClient.StashV1beta1().BackupConfigurations(meta.Namespace).Get(context.TODO(), meta.Name, metav1.GetOptions{})
			if err == nil && !kerr.IsNotFound(err) {
				return true
			}
			return false
		},
		WaitTimeOut,
		PullInterval,
	)
}

func (fi *Invocation) VerifyCustomSchedule(backupConfig *stash_v1beta1.BackupConfiguration, schedule string) {
	By("Verifying that the BackupConfiguration is using schedule from the annotations")
	Expect(backupConfig.Spec.Schedule).Should(BeEquivalentTo(schedule))
}

func (fi *Invocation) VerifyParameterPassed(parameters []stash_v1beta1.Param, key, value string) {
	By("Verifying that the custom args has been passed to the BackupConfiguration")
	Expect(paramPassed(parameters, key, value)).Should(BeTrue())
}

func paramPassed(parameters []stash_v1beta1.Param, key, value string) bool {
	for _, p := range parameters {
		if p.Name == key && p.Value == value {
			return true
		}
	}
	return false
}
