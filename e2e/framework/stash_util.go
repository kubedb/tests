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
	"strings"

	. "github.com/onsi/ginkgo"
	. "github.com/onsi/gomega"
	batch "k8s.io/api/batch/v1beta1"
	kerr "k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"
	"k8s.io/client-go/kubernetes/scheme"
	v1 "kmodules.xyz/client-go/core/v1"
	"kmodules.xyz/client-go/discovery"
	dm_util "kmodules.xyz/client-go/dynamic"
	meta_util "kmodules.xyz/client-go/meta"
	appcat "kmodules.xyz/custom-resources/apis/appcatalog/v1alpha1"
	"stash.appscode.dev/apimachinery/apis/stash"
	stash_v1alpha1 "stash.appscode.dev/apimachinery/apis/stash/v1alpha1"
	stash_v1beta1 "stash.appscode.dev/apimachinery/apis/stash/v1beta1"
)

const (
	ParamKeyArgs = "args"
)

// StashInstalled function check whether Stash has been installed in the cluster or not.
// Its verify Stash installation by checking the presence of RestoreSession CRD in the cluster.
func (f *Framework) StashInstalled() bool {
	return discovery.ExistsGroupKind(f.kubeClient.Discovery(), stash.GroupName, stash_v1beta1.ResourceKindRestoreSession)
}

func (i *Invocation) AddonExist() (bool, error) {
	// check whether the respective addon has been installed or not
	_, err := i.StashClient.StashV1beta1().Tasks().Get(context.TODO(), getBackupAddonName(), metav1.GetOptions{})
	if err != nil {
		if !kerr.IsNotFound(err) {
			return false, err
		} else {
			return false, nil
		}
	}
	return true, nil
}

func (i *Invocation) BackupDatabase(dbMeta metav1.ObjectMeta, expectedSnapshotCount int32, transformFuncs ...func(bc *stash_v1beta1.BackupConfiguration)) (*appcat.AppBinding, *stash_v1alpha1.Repository) {
	By("Configuring backup")
	appBinding, err := i.GetAppBinding(dbMeta)
	Expect(err).NotTo(HaveOccurred())
	backupConfig, repo, err := i.SetupDatabaseBackup(appBinding, transformFuncs...)
	Expect(err).NotTo(HaveOccurred())

	// Simulate a backup run
	i.SimulateBackupRun(backupConfig.ObjectMeta, repo.ObjectMeta, expectedSnapshotCount)

	return appBinding, repo
}

func (i *Invocation) SimulateBackupRun(backupConfig, repoMeta metav1.ObjectMeta, expectedSnapshotCount int32) {
	By("Triggering an instant backup")
	backupSession, err := i.TriggerInstantBackup(backupConfig, stash_v1beta1.BackupInvokerRef{
		Name: backupConfig.Name,
		Kind: stash_v1beta1.ResourceKindBackupConfiguration,
	})
	Expect(err).NotTo(HaveOccurred())
	i.AppendToCleanupList(backupSession)

	By("Waiting for the backup to complete")
	i.EventuallyBackupProcessCompleted(backupSession.ObjectMeta).Should(BeTrue())

	By("Verifying that the backup has succeeded")
	completedBS, err := i.StashClient.StashV1beta1().BackupSessions(backupSession.Namespace).Get(context.TODO(), backupSession.Name, metav1.GetOptions{})
	Expect(err).NotTo(HaveOccurred())
	Expect(completedBS.Status.Phase).Should(Equal(stash_v1beta1.BackupSessionSucceeded))

	By(fmt.Sprintf("Verifying that number of backup snapshots = %d", expectedSnapshotCount))
	repo, err := i.StashClient.StashV1alpha1().Repositories(repoMeta.Namespace).Get(context.TODO(), repoMeta.Name, metav1.GetOptions{})
	Expect(err).NotTo(HaveOccurred())
	Expect(repo.Status.SnapshotCount).Should(BeEquivalentTo(expectedSnapshotCount))
}

func (i *Invocation) RestoreDatabase(appBinding *appcat.AppBinding, repo *stash_v1alpha1.Repository, transformFuncs ...func(rs *stash_v1beta1.RestoreSession)) {
	By("Restoring database from backup")
	restoreSession, err := i.SetupDatabaseRestore(appBinding, repo, transformFuncs...)
	Expect(err).NotTo(HaveOccurred())

	By("Waiting for restore process to complete")
	i.EventuallyRestoreProcessCompleted(restoreSession.ObjectMeta).Should(BeTrue())

	By("Verifying that restore process has succeeded")
	completedRS, err := i.StashClient.StashV1beta1().RestoreSessions(restoreSession.Namespace).Get(context.TODO(), restoreSession.Name, metav1.GetOptions{})
	Expect(err).NotTo(HaveOccurred())
	Expect(completedRS.Status.Phase).Should(Equal(stash_v1beta1.RestoreSucceeded))
}

func (i *Invocation) ConfigureAutoBackup(db interface{}, meta metav1.ObjectMeta, transformFuncs ...func(annotations map[string]string)) (*stash_v1beta1.BackupConfiguration, *stash_v1alpha1.Repository) {
	// Create BackupBlueprint
	bb := i.CreateBackupBlueprint()

	annotations := map[string]string{
		stash_v1beta1.KeyBackupBlueprint: bb.Name,
	}
	// Add test specific custom annotation
	for _, fn := range transformFuncs {
		fn(annotations)
	}

	// Add add auto-backup annotations into the target
	err := i.AddAutoBackupAnnotations(db, meta, annotations)
	Expect(err).NotTo(HaveOccurred())

	// Verify Repository and BackupConfiguration has been created
	return i.VerifyAutoBackupConfigured(meta)
}

func (i *Invocation) AddAutoBackupAnnotations(db interface{}, meta metav1.ObjectMeta, annotations map[string]string) error {
	By("Adding auto-backup specific annotations to the Target")
	err := i.AddAnnotations(annotations, db)
	if err != nil {
		return err
	}

	By("Verifying that the auto-backup annotations has been passed to AppBinding")
	i.EventuallyAnnotationsPassed(meta, annotations).Should(BeTrue())
	return nil
}

func (i *Invocation) VerifyAutoBackupConfigured(meta metav1.ObjectMeta) (*stash_v1beta1.BackupConfiguration, *stash_v1alpha1.Repository) {
	// BackupBlueprint create BackupConfiguration and Repository such that
	// the name of the BackupConfiguration and Repository will follow
	// the patter: <lower case of the workload kind>-<workload name>.
	// we will form the meta name and namespace for farther process.
	objMeta := metav1.ObjectMeta{
		Namespace: i.Namespace(),
		Name:      meta_util.NameWithPrefix("app", meta.Name),
	}

	By("Waiting for Repository")
	i.EventuallyRepositoryCreated(objMeta).Should(BeTrue())
	repo, err := i.StashClient.StashV1alpha1().Repositories(objMeta.Namespace).Get(context.TODO(), objMeta.Name, metav1.GetOptions{})
	Expect(err).NotTo(HaveOccurred())
	i.AppendToCleanupList(repo)

	By("Waiting for BackupConfiguration")
	i.EventuallyBackupConfigurationCreated(objMeta).Should(BeTrue())
	backupConfig, err := i.StashClient.StashV1beta1().BackupConfigurations(objMeta.Namespace).Get(context.TODO(), objMeta.Name, metav1.GetOptions{})
	Expect(err).NotTo(HaveOccurred())
	i.AppendToCleanupList(backupConfig)

	By("Verifying that backup triggering CronJob has been created")
	i.EventuallyCronJobCreated(backupConfig).Should(BeTrue())

	return backupConfig, repo
}

func (i *Invocation) AddAnnotations(annotations map[string]string, db interface{}) error {
	schm := scheme.Scheme
	gvr, _, err := getGVRAndObjectMeta(db)
	if err != nil {
		return err
	}

	// convert db into unstructured object
	cur := &unstructured.Unstructured{}
	err = schm.Convert(db, cur, nil)
	if err != nil {
		return err
	}

	// add annotations
	mod := cur.DeepCopy()
	mod.SetAnnotations(annotations)
	_, _, err = dm_util.PatchObject(context.TODO(), i.dmClient, gvr, cur, mod, metav1.PatchOptions{})
	return err
}

func (i *Invocation) RemoveAutoBackupAnnotations(db interface{}) {
	By("Removing auto-backup annotations")
	gvr, meta, err := getGVRAndObjectMeta(db)
	Expect(err).NotTo(HaveOccurred())

	// get the database
	cur, err := i.dmClient.Resource(gvr).Namespace(meta.Namespace).Get(context.TODO(), meta.Name, metav1.GetOptions{})
	Expect(err).NotTo(HaveOccurred())

	// remove auto-backup annotation
	mod := cur.DeepCopy()
	annotations := mod.GetAnnotations()
	annotations = meta_util.RemoveKey(annotations, stash_v1beta1.KeyBackupBlueprint)

	mod.SetAnnotations(annotations)
	_, _, err = dm_util.PatchObject(context.TODO(), i.dmClient, gvr, cur, mod, metav1.PatchOptions{})
	Expect(err).NotTo(HaveOccurred())

	By("Verifying that auto-backup annotations has been removed from the AppBinding")
	i.EventuallyAnnotationsRemoved(meta, []string{stash_v1beta1.KeyBackupBlueprint}).Should(BeTrue())
}

func (i *Invocation) EventuallyAnnotationsPassed(meta metav1.ObjectMeta, expectedAnnotations map[string]string) GomegaAsyncAssertion {
	return Eventually(
		func() bool {
			appBinding, err := i.appCatalogClient.AppBindings(meta.Namespace).Get(context.TODO(), meta.Name, metav1.GetOptions{})
			Expect(err).NotTo(HaveOccurred())

			annotations := appBinding.GetAnnotations()
			for k, v := range expectedAnnotations {
				if !(meta_util.HasKey(annotations, k) && annotations[k] == v) {
					return false
				}
			}
			return true
		},
		WaitTimeOut,
		PullInterval,
	)
}

func (i *Invocation) EventuallyAnnotationsRemoved(meta metav1.ObjectMeta, keys []string) GomegaAsyncAssertion {
	return Eventually(
		func() bool {
			appBinding, err := i.appCatalogClient.AppBindings(meta.Namespace).Get(context.TODO(), meta.Name, metav1.GetOptions{})
			Expect(err).NotTo(HaveOccurred())

			annotations := appBinding.GetAnnotations()
			for i := range keys {
				if meta_util.HasKey(annotations, keys[i]) {
					return false
				}
			}
			return true
		},
		WaitTimeOut,
		PullInterval,
	)
}

func (i *Invocation) EventuallyCronJobCreated(backupConfig *stash_v1beta1.BackupConfiguration) GomegaAsyncAssertion {
	return Eventually(
		func() bool {
			cronJobs, err := i.kubeClient.BatchV1beta1().CronJobs(backupConfig.Namespace).List(context.TODO(), metav1.ListOptions{})
			Expect(err).NotTo(HaveOccurred())
			for _, cr := range cronJobs.Items {
				if metav1.IsControlledBy(&cr, backupConfig) {
					return true
				}
			}
			return false
		},
		WaitTimeOut,
		PullInterval,
	)
}

func (i *Invocation) GetCronJob(meta metav1.ObjectMeta) (*batch.CronJob, error) {
	cronJobs, err := i.kubeClient.BatchV1beta1().CronJobs(meta.Namespace).List(context.TODO(), metav1.ListOptions{})
	if err != nil {
		return nil, err
	}
	for i := range cronJobs.Items {
		if yes, _ := v1.IsOwnedBy(&cronJobs.Items[i].ObjectMeta, &meta); yes {
			return &cronJobs.Items[i], nil
		}
	}
	return nil, nil
}

func getBackupAddonName() string {
	return fmt.Sprintf("%s-backup-%s",
		strings.TrimPrefix(StashAddonName, "stash-"),
		StashAddonVersion,
	)
}

func getRestoreAddonName() string {
	return fmt.Sprintf("%s-restore-%s",
		strings.TrimPrefix(StashAddonName, "stash-"),
		StashAddonVersion,
	)
}
