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
	"strconv"
	"time"

	"kubedb.dev/apimachinery/apis/catalog/v1alpha1"
	"kubedb.dev/apimachinery/apis/kubedb"
	api "kubedb.dev/apimachinery/apis/kubedb/v1alpha2"
	"kubedb.dev/apimachinery/client/clientset/versioned/typed/kubedb/v1alpha2/util"

	"github.com/appscode/go/crypto/rand"
	"github.com/appscode/go/types"
	. "github.com/onsi/ginkgo"
	. "github.com/onsi/gomega"
	"gomodules.xyz/pointer"
	core "k8s.io/api/core/v1"
	policy "k8s.io/api/policy/v1beta1"
	kerr "k8s.io/apimachinery/pkg/api/errors"
	"k8s.io/apimachinery/pkg/api/resource"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/labels"
	meta_util "kmodules.xyz/client-go/meta"
	ofst "kmodules.xyz/offshoot-api/api/v1"
)

var (
	DBPvcStorageSize = "1Gi"
)

const (
	kindEviction = "Eviction"
)

func (fi *Invocation) MongoDBStandalone() *api.MongoDB {
	if InMemory {
		Skip("standalone doesn't support inmemory")
		return nil
	}

	return &api.MongoDB{
		ObjectMeta: metav1.ObjectMeta{
			Name:      rand.WithUniqSuffix("mongodb"),
			Namespace: fi.namespace,
			Labels: map[string]string{
				"app": fi.app,
			},
		},
		Spec: api.MongoDBSpec{
			Version: DBVersion,
			Storage: &core.PersistentVolumeClaimSpec{
				Resources: core.ResourceRequirements{
					Requests: core.ResourceList{
						core.ResourceStorage: resource.MustParse(DBPvcStorageSize),
					},
				},
				StorageClassName: pointer.StringP(fi.StorageClass),
			},
			SSLMode: api.SSLModeDisabled,

			TerminationPolicy: api.TerminationPolicyHalt,
		},
	}
}

func (fi *Invocation) MongoDBRS() *api.MongoDB {
	dbName := rand.WithUniqSuffix("mongo-rs")
	return &api.MongoDB{
		ObjectMeta: metav1.ObjectMeta{
			Name:      dbName,
			Namespace: fi.namespace,
			Labels: map[string]string{
				"app": fi.app,
			},
		},
		Spec: api.MongoDBSpec{
			Version:  DBVersion,
			Replicas: types.Int32P(3),
			ReplicaSet: &api.MongoDBReplicaSet{
				Name: dbName,
			},
			StorageEngine:     fi.getMongoStorageEngine(),
			StorageType:       fi.getStorageType(),
			Storage:           fi.getStorage(),
			TerminationPolicy: api.TerminationPolicyHalt,
		},
	}
}

func (fi *Invocation) MongoDBShard() *api.MongoDB {
	return &api.MongoDB{
		ObjectMeta: metav1.ObjectMeta{
			Name:      rand.WithUniqSuffix("mongo-sh"),
			Namespace: fi.namespace,
			Labels: map[string]string{
				"app": fi.app,
			},
		},
		Spec: api.MongoDBSpec{
			Version:       DBVersion,
			StorageEngine: fi.getMongoStorageEngine(),
			StorageType:   fi.getStorageType(),
			ShardTopology: &api.MongoDBShardingTopology{
				Shard: api.MongoDBShardNode{
					Shards: 2,
					MongoDBNode: api.MongoDBNode{
						Replicas: 3,
					},
					Storage: fi.getStorage(),
				},
				ConfigServer: api.MongoDBConfigNode{
					MongoDBNode: api.MongoDBNode{
						Replicas: 3,
					},
					Storage: fi.getStorage(),
				},
				Mongos: api.MongoDBMongosNode{
					MongoDBNode: api.MongoDBNode{
						Replicas: 2,
					},
				},
			},
			TerminationPolicy: api.TerminationPolicyHalt,
		},
	}
}

func (fi *Invocation) MongoDBWithFlexibleProbeTimeout(db *api.MongoDB) *api.MongoDB {
	dbVersion, err := fi.GetMongoDBVersion(DBVersion)
	Expect(err).NotTo(HaveOccurred())
	db.SetDefaults(dbVersion, fi.topology)

	if db.Spec.ShardTopology != nil {
		db.Spec.ShardTopology.Mongos.PodTemplate.Spec.ReadinessProbe = &core.Probe{}
		db.Spec.ShardTopology.Mongos.PodTemplate.Spec.LivenessProbe = &core.Probe{}
		db.Spec.ShardTopology.Shard.PodTemplate.Spec.ReadinessProbe = &core.Probe{}
		db.Spec.ShardTopology.Shard.PodTemplate.Spec.LivenessProbe = &core.Probe{}
		db.Spec.ShardTopology.ConfigServer.PodTemplate.Spec.ReadinessProbe = &core.Probe{}
		db.Spec.ShardTopology.ConfigServer.PodTemplate.Spec.LivenessProbe = &core.Probe{}
	} else if db.Spec.PodTemplate != nil && db.Spec.PodTemplate.Spec.ReadinessProbe != nil {
		db.Spec.PodTemplate.Spec.ReadinessProbe = &core.Probe{}
		db.Spec.PodTemplate.Spec.LivenessProbe = &core.Probe{}
	}
	return db
}

func IsRepSet(db *api.MongoDB) bool {
	return db.Spec.ReplicaSet != nil
}

// ClusterAuthModeP returns a pointer to the int32 value passed in.
func ClusterAuthModeP(v api.ClusterAuthMode) *api.ClusterAuthMode {
	return &v
}

// SSLModeP returns a pointer to the int32 value passed in.
func SSLModeP(v api.SSLMode) *api.SSLMode {
	return &v
}

func (fi *Invocation) CreateMongoDB(obj *api.MongoDB) error {
	_, err := fi.dbClient.KubedbV1alpha2().MongoDBs(obj.Namespace).Create(context.TODO(), obj, metav1.CreateOptions{})
	return err
}

func (f *Framework) GetMongoDB(meta metav1.ObjectMeta) (*api.MongoDB, error) {
	return f.dbClient.KubedbV1alpha2().MongoDBs(meta.Namespace).Get(context.TODO(), meta.Name, metav1.GetOptions{})
}

func (f *Framework) PatchMongoDB(meta metav1.ObjectMeta, transform func(*api.MongoDB) *api.MongoDB) (*api.MongoDB, error) {
	mongodb, err := f.dbClient.KubedbV1alpha2().MongoDBs(meta.Namespace).Get(context.TODO(), meta.Name, metav1.GetOptions{})
	if err != nil {
		return nil, err
	}
	mongodb, _, err = util.PatchMongoDB(context.TODO(), f.dbClient.KubedbV1alpha2(), mongodb, transform, metav1.PatchOptions{})
	return mongodb, err
}

func (f *Framework) DeleteMongoDB(meta metav1.ObjectMeta) error {
	return f.dbClient.KubedbV1alpha2().MongoDBs(meta.Namespace).Delete(context.TODO(), meta.Name, meta_util.DeleteInBackground())
}

func (f *Framework) GetMongoDBVersion(name string) (*v1alpha1.MongoDBVersion, error) {
	return f.dbClient.CatalogV1alpha1().MongoDBVersions().Get(context.TODO(), name, metav1.GetOptions{})
}

func (f *Framework) EvictPodsFromStatefulSet(meta metav1.ObjectMeta, fqn string) error {
	var err error
	labelSelector := labels.Set{
		meta_util.ManagedByLabelKey: kubedb.GroupName,
		meta_util.NameLabelKey:      fqn,
		meta_util.InstanceLabelKey:  meta.Name,
	}
	// get sts in the namespace
	stsList, err := f.kubeClient.AppsV1().StatefulSets(meta.Namespace).List(context.TODO(), metav1.ListOptions{LabelSelector: labelSelector.String()})
	if err != nil {
		return err
	}
	for _, sts := range stsList.Items {
		// if PDB is not found, send error
		var pdb *policy.PodDisruptionBudget
		pdb, err = f.kubeClient.PolicyV1beta1().PodDisruptionBudgets(sts.Namespace).Get(context.TODO(), sts.Name, metav1.GetOptions{})
		if err != nil {
			return err
		}
		eviction := &policy.Eviction{
			TypeMeta: metav1.TypeMeta{
				APIVersion: policy.SchemeGroupVersion.String(),
				Kind:       kindEviction,
			},
			ObjectMeta: metav1.ObjectMeta{
				Name:      sts.Name,
				Namespace: sts.Namespace,
			},
			DeleteOptions: &metav1.DeleteOptions{},
		}

		if pdb.Spec.MaxUnavailable == nil {
			return fmt.Errorf("found pdb %s spec.maxUnavailable nil", pdb.Name)
		}

		// try to evict as many pod as allowed in pdb. No err should occur
		maxUnavailable := pdb.Spec.MaxUnavailable.IntValue()
		for i := 0; i < maxUnavailable; i++ {
			eviction.Name = sts.Name + "-" + strconv.Itoa(i)

			err := f.kubeClient.PolicyV1beta1().Evictions(eviction.Namespace).Evict(context.TODO(), eviction)
			if err != nil {
				return err
			}
		}

		// try to evict one extra pod. TooManyRequests err should occur
		eviction.Name = sts.Name + "-" + strconv.Itoa(maxUnavailable)
		err = f.kubeClient.PolicyV1beta1().Evictions(eviction.Namespace).Evict(context.TODO(), eviction)
		if kerr.IsTooManyRequests(err) {
			err = nil
		} else if err != nil {
			return err
		} else {
			return fmt.Errorf("expected pod %s/%s to be not evicted due to pdb %s", sts.Namespace, eviction.Name, pdb.Name)
		}
	}
	return err
}

func (f *Framework) EvictPodsFromDeployment(meta metav1.ObjectMeta) error {
	var err error
	deployName := meta.Name + "-mongos"
	//if PDB is not found, send error
	pdb, err := f.kubeClient.PolicyV1beta1().PodDisruptionBudgets(meta.Namespace).Get(context.TODO(), deployName, metav1.GetOptions{})
	if err != nil {
		return err
	}
	if pdb.Spec.MinAvailable == nil {
		return fmt.Errorf("found pdb %s spec.minAvailable nil", pdb.Name)
	}

	podSelector := labels.Set{
		api.MongoDBMongosLabelKey: meta.Name + "-mongos",
	}
	pods, err := f.kubeClient.CoreV1().Pods(meta.Namespace).List(context.TODO(), metav1.ListOptions{LabelSelector: podSelector.String()})
	eviction := &policy.Eviction{
		TypeMeta: metav1.TypeMeta{
			APIVersion: policy.SchemeGroupVersion.String(),
			Kind:       kindEviction,
		},
		ObjectMeta: metav1.ObjectMeta{
			Namespace: meta.Namespace,
		},
		DeleteOptions: &metav1.DeleteOptions{},
	}

	// try to evict as many pods as allowed in pdb
	minAvailable := pdb.Spec.MinAvailable.IntValue()
	podCount := len(pods.Items)
	for i, pod := range pods.Items {
		eviction.Name = pod.Name
		err = f.kubeClient.PolicyV1beta1().Evictions(eviction.Namespace).Evict(context.TODO(), eviction)
		if i < (podCount - minAvailable) {
			if err != nil {
				return err
			}
		} else {
			// This pod should not get evicted
			if kerr.IsTooManyRequests(err) {
				err = nil
				break
			} else if err != nil {
				return err
			} else {
				return fmt.Errorf("expected pod %s/%s to be not evicted due to pdb %s", meta.Namespace, eviction.Name, pdb.Name)
			}
		}
	}
	return err
}

func (f *Framework) EventuallyMongoDB(meta metav1.ObjectMeta) GomegaAsyncAssertion {
	return Eventually(
		func() bool {
			_, err := f.dbClient.KubedbV1alpha2().MongoDBs(meta.Namespace).Get(context.TODO(), meta.Name, metav1.GetOptions{})
			if err != nil {
				if kerr.IsNotFound(err) {
					return false
				}
				Expect(err).NotTo(HaveOccurred())
			}
			return true
		},
		time.Minute*13,
		time.Second*5,
	)
}

func (f *Framework) EventuallyMongoDBPhase(meta metav1.ObjectMeta) GomegaAsyncAssertion {
	return Eventually(
		func() api.DatabasePhase {
			db, err := f.dbClient.KubedbV1alpha2().MongoDBs(meta.Namespace).Get(context.TODO(), meta.Name, metav1.GetOptions{})
			Expect(err).NotTo(HaveOccurred())
			return db.Status.Phase
		},
		time.Minute*13,
		time.Second*5,
	)
}

func (f *Framework) EventuallyMongoDBReady(meta metav1.ObjectMeta) GomegaAsyncAssertion {
	return Eventually(
		func() bool {
			mongodb, err := f.dbClient.KubedbV1alpha2().MongoDBs(meta.Namespace).Get(context.TODO(), meta.Name, metav1.GetOptions{})
			Expect(err).NotTo(HaveOccurred())
			return mongodb.Status.Phase == api.DatabasePhaseReady
		},
		time.Minute*13,
		time.Second*5,
	)
}

func (f *Framework) EventuallyCheckTLSMongoDB(meta metav1.ObjectMeta, sslMode api.SSLMode) GomegaAsyncAssertion {
	return Eventually(
		func() bool {
			mongodb, err := f.dbClient.KubedbV1alpha2().MongoDBs(meta.Namespace).Get(context.TODO(), meta.Name, metav1.GetOptions{})
			Expect(err).NotTo(HaveOccurred())
			return mongodb.Spec.SSLMode == sslMode
		},
		time.Minute*1,
		time.Second*5,
	)
}

func (f *Framework) CleanMongoDB() {
	mongodbList, err := f.dbClient.KubedbV1alpha2().MongoDBs(f.namespace).List(context.TODO(), metav1.ListOptions{})
	if err != nil {
		return
	}
	for _, e := range mongodbList.Items {
		if _, _, err := util.PatchMongoDB(context.TODO(), f.dbClient.KubedbV1alpha2(), &e, func(in *api.MongoDB) *api.MongoDB {
			in.ObjectMeta.Finalizers = nil
			in.Spec.TerminationPolicy = api.TerminationPolicyWipeOut
			return in
		}, metav1.PatchOptions{}); err != nil {
			fmt.Printf("error Patching MongoDB. error: %v", err)
		}
	}
	if err := f.dbClient.KubedbV1alpha2().MongoDBs(f.namespace).DeleteCollection(context.TODO(), meta_util.DeleteInForeground(), metav1.ListOptions{}); err != nil {
		fmt.Printf("error in deletion of MongoDB. Error: %v", err)
	}
}

func (f *Framework) EventuallyWipedOut(meta metav1.ObjectMeta, fqn string) GomegaAsyncAssertion {
	return Eventually(
		func() error {
			labelMap := map[string]string{
				meta_util.NameLabelKey:     fqn,
				meta_util.InstanceLabelKey: meta.Name,
			}
			labelSelector := labels.SelectorFromSet(labelMap)

			// check if pvcs is wiped out
			pvcList, err := f.kubeClient.CoreV1().PersistentVolumeClaims(meta.Namespace).List(
				context.TODO(),
				metav1.ListOptions{
					LabelSelector: labelSelector.String(),
				},
			)
			if err != nil {
				return err
			}
			if len(pvcList.Items) > 0 {
				fmt.Println("PVCs have not wiped out yet")
				return fmt.Errorf("PVCs have not wiped out yet")
			}

			// check if secrets are wiped out
			secretList, err := f.kubeClient.CoreV1().Secrets(meta.Namespace).List(
				context.TODO(),
				metav1.ListOptions{
					LabelSelector: labelSelector.String(),
				},
			)
			if err != nil {
				return err
			}
			if len(secretList.Items) > 0 {
				fmt.Println("secrets have not wiped out yet")
				return fmt.Errorf("secrets have not wiped out yet")
			}

			// check if appbinds are wiped out
			appBindingList, err := f.appCatalogClient.AppBindings(meta.Namespace).List(
				context.TODO(),
				metav1.ListOptions{
					LabelSelector: labelSelector.String(),
				},
			)
			if err != nil {
				return err
			}
			if len(appBindingList.Items) > 0 {
				fmt.Println("appBindings have not wiped out yet")
				return fmt.Errorf("appBindings have not wiped out yet")
			}

			return nil
		},
		WaitTimeOut,
		PullInterval,
	)
}

// DeployMongoDB creates a MongoDB object. It accepts an array of functions
// called transform function. The transform functions make test specific modification on
// a generic MongoDB definition.
func (fi *Invocation) DeployMongoDB(transformFuncs ...func(in *api.MongoDB)) *api.MongoDB {
	// A generic MongoDB definition
	mg := &api.MongoDB{
		ObjectMeta: metav1.ObjectMeta{
			Name:      meta_util.NameWithSuffix("mongodb", fi.app),
			Namespace: fi.namespace,
			Labels: map[string]string{
				labelApp: fi.app,
			},
		},
		Spec: api.MongoDBSpec{
			Version: DBVersion,
			Storage: &core.PersistentVolumeClaimSpec{
				Resources: core.ResourceRequirements{
					Requests: core.ResourceList{
						core.ResourceStorage: resource.MustParse(DBPvcStorageSize),
					},
				},
				StorageClassName: pointer.StringP(fi.StorageClass),
			},
			SSLMode:           api.SSLModeDisabled,
			TerminationPolicy: api.TerminationPolicyDelete,
		},
	}

	// apply the transform functions to obtain the desired MongoDB from the generic definition.
	for _, fn := range transformFuncs {
		fn(mg)
	}

	By("Deploying MongoDB: " + mg.Name)
	createdMongo, err := fi.dbClient.KubedbV1alpha2().MongoDBs(mg.Namespace).Create(context.TODO(), mg, metav1.CreateOptions{})
	Expect(err).NotTo(HaveOccurred())
	fi.AppendToCleanupList(createdMongo)

	// If "spec.Init.WaitForInitialRestore" is set to "true", database will stuck in "Provisioning" state until initial restore done.
	if shouldWaitForInitialRestore(createdMongo) {
		By("Waiting for MongoDB: " + createdMongo.Name + " to accept connection")
		fi.EventuallyPingMongo(createdMongo.ObjectMeta).Should(BeTrue())
	} else {
		By("Waiting for MongoDB: " + createdMongo.Name + " to be ready")
		fi.EventuallyMongoDBReady(mg.ObjectMeta).Should(BeTrue())
	}

	// if SSL is being used, verify SSL settings
	if sslEnabledMongo(mg) {
		By("Verifying SSL settings")
		fi.EventuallyUserSSLSettings(mg.ObjectMeta, &mg.Spec.ClusterAuthMode, &mg.Spec.SSLMode).Should(BeTrue())
	}
	return createdMongo
}

// PopulateMongoDB insert some sample data into the MongoDB when it is ready.
func (fi *Invocation) PopulateMongoDB(mg *api.MongoDB, databases ...string) {
	// If sharding is being used, then enable sharding in the database
	if shardedMongo(mg) {
		for i := range databases {
			By("Enabling sharding for db:" + databases[i])
			fi.EventuallyEnableSharding(mg.ObjectMeta, databases[i]).Should(BeTrue())

			By("Checking if db " + databases[i] + " is set to partitioned")
			fi.EventuallyCollectionPartitioned(mg.ObjectMeta, databases[i]).Should(BeTrue())
		}
	}
	// Insert sample data
	for i := range databases {
		By("Inserting sample data in db: " + databases[i])
		fi.EventuallyInsertCollection(mg.ObjectMeta, databases[i], SampleCollection, AnotherCollection).Should(BeTrue())
	}
}

func (fi *Invocation) UpdateCollection(meta metav1.ObjectMeta, dbName string, collection Collection) {
	By("Updating collection: " + collection.Name + " of db: " + dbName)
	fi.EventuallyUpdateCollection(meta, dbName, collection).Should(BeTrue())

	By("Verifying that the collection has been updated")
	resp, err := fi.GetDocument(meta, dbName, collection.Name)
	Expect(err).NotTo(HaveOccurred())
	Expect(resp.State).Should(Equal(collection.Document.State))
}

// SimulateMongoDBDisaster simulates accidental database deletion. It drops the database specified by "dbName" variable.
func (fi *Invocation) SimulateMongoDBDisaster(meta metav1.ObjectMeta, databases ...string) {
	for i := range databases {
		By("Deleting database: " + databases[i])
		fi.EventuallyDropDatabase(meta, databases[i]).Should(BeTrue())

		By("Verifying that database: " + databases[i] + " has been removed")
		dbExist, err := fi.DatabaseExists(meta, databases[i])
		Expect(err).NotTo(HaveOccurred())
		Expect(dbExist).To(BeFalse())
	}
}

func (fi *Invocation) VerifyMongoDBRestore(meta metav1.ObjectMeta, databases ...string) {
	By("Verify that database is healthy after restore")
	fi.EventuallyMongoDBReady(meta).Should(BeTrue())

	for i := range databases {
		By("Verifying that db: " + databases[i] + " has been restored")
		// Verify that "sampleCollection" has been restored
		document, err := fi.GetDocument(meta, databases[i], SampleCollection.Name)
		Expect(err).NotTo(HaveOccurred())
		Expect(documentMatches(*document, SampleCollection.Document)).Should(BeTrue())

		// Verify that "anotherCollection" has been restored
		document, err = fi.GetDocument(meta, databases[i], SampleCollection.Name)
		Expect(err).NotTo(HaveOccurred())
		Expect(documentMatches(*document, SampleCollection.Document)).Should(BeTrue())
	}
}

func (fi *Invocation) EnableMongoSSL(mg *api.MongoDB, sslMode api.SSLMode, transformFuncs ...func(in *api.MongoDB)) {
	// Create Issuer
	issuer, err := fi.InsureIssuer(mg.ObjectMeta, api.ResourceKindMongoDB)
	Expect(err).NotTo(HaveOccurred())

	// Enable SSL in the MongoDB
	mg.Spec.SSLMode = sslMode
	mg.Spec.TLS = NewTLSConfiguration(issuer)

	// apply test specific modification
	for _, fn := range transformFuncs {
		fn(mg)
	}
}

func (fi *Invocation) EnableMongoReplication(mg *api.MongoDB, transformFuncs ...func(in *api.MongoDB)) {
	// set storage engine
	mg.Spec.StorageEngine = fi.getMongoStorageEngine()
	// set storage type
	mg.Spec.StorageType = fi.getStorageType()
	// set pvc specification
	mg.Spec.Storage = fi.getStorage()
	// set replicas
	mg.Spec.Replicas = pointer.Int32P(2)
	mg.Spec.ReplicaSet = &api.MongoDBReplicaSet{
		Name: fi.app,
	}
	// apply test specific modification
	for _, fn := range transformFuncs {
		fn(mg)
	}
}

func (fi *Invocation) EnableMongoSharding(mg *api.MongoDB, transformFuncs ...func(in *api.MongoDB)) {
	// remove the generic storage section
	mg.Spec.Storage = nil
	// set storage engine
	mg.Spec.StorageEngine = fi.getMongoStorageEngine()
	// set storage type
	mg.Spec.StorageType = fi.getStorageType()
	// add shard topology
	mg.Spec.ShardTopology = &api.MongoDBShardingTopology{
		Shard: api.MongoDBShardNode{
			Shards: 2,
			MongoDBNode: api.MongoDBNode{
				Replicas: 2,
				PodTemplate: ofst.PodTemplateSpec{
					Spec: ofst.PodSpec{
						Resources: fi.getResources(),
					},
				},
			},
			Storage: fi.getStorage(),
		},
		ConfigServer: api.MongoDBConfigNode{
			MongoDBNode: api.MongoDBNode{
				Replicas: 2,
				PodTemplate: ofst.PodTemplateSpec{
					Spec: ofst.PodSpec{
						Resources: fi.getResources(),
					},
				},
			},
			Storage: fi.getStorage(),
		},
		Mongos: api.MongoDBMongosNode{
			MongoDBNode: api.MongoDBNode{
				Replicas: 2,
				PodTemplate: ofst.PodTemplateSpec{
					Spec: ofst.PodSpec{
						Resources: fi.getResources(),
					},
				},
			},
		},
	}
	// apply test specific modification
	for _, fn := range transformFuncs {
		fn(mg)
	}
}

func (fi *Invocation) WaitForInitialRestore(init *api.InitSpec) *api.InitSpec {
	if init == nil {
		init = &api.InitSpec{}
	}
	init.WaitForInitialRestore = true
	return init
}

func shardedMongo(mg *api.MongoDB) bool {
	if mg.Spec.ShardTopology != nil {
		return mg.Spec.ShardTopology != nil
	}
	return false
}

func sslEnabledMongo(mg *api.MongoDB) bool {
	return mg.Spec.TLS != nil
}

func documentMatches(actual, expected Document) bool {
	return actual.Name == expected.Name &&
		actual.State == expected.State
}

func shouldWaitForInitialRestore(mg *api.MongoDB) bool {
	return mg != nil && mg.Spec.Init != nil && mg.Spec.Init.WaitForInitialRestore
}

func (fi *Invocation) getStorage() *core.PersistentVolumeClaimSpec {
	storage := &core.PersistentVolumeClaimSpec{
		Resources: core.ResourceRequirements{
			Requests: core.ResourceList{
				core.ResourceStorage: resource.MustParse(DBPvcStorageSize),
			},
		},
		StorageClassName: pointer.StringP(fi.StorageClass),
	}
	if InMemory {
		storage = nil
	}
	return storage
}

func (fi *Invocation) getResources() core.ResourceRequirements {
	return core.ResourceRequirements{
		Limits: core.ResourceList{
			core.ResourceCPU:    resource.MustParse("300m"),
			core.ResourceMemory: resource.MustParse("512Mi"),
		},
		Requests: core.ResourceList{
			core.ResourceCPU:    resource.MustParse("300m"),
			core.ResourceMemory: resource.MustParse("512Mi"),
		},
	}
}

func (fi *Invocation) getMongoStorageEngine() api.StorageEngine {
	if InMemory {
		return api.StorageEngineInMemory
	}
	return api.StorageEngineWiredTiger
}

func (fi *Invocation) getStorageType() api.StorageType {
	if InMemory {
		return api.StorageTypeEphemeral
	}
	return api.StorageTypeDurable
}
