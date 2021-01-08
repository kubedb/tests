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
	"bytes"
	"context"
	"crypto/tls"
	"fmt"
	"net/http"
	"os"
	"strings"
	"time"

	api "kubedb.dev/apimachinery/apis/kubedb/v1alpha2"
	"kubedb.dev/apimachinery/apis/ops/v1alpha1"
	opsapi "kubedb.dev/apimachinery/apis/ops/v1alpha1"

	"github.com/appscode/go/log"
	"github.com/aws/aws-sdk-go/aws"
	shell "github.com/codeskyblue/go-sh"
	cm_api "github.com/jetstack/cert-manager/pkg/apis/certmanager/v1beta1"
	. "github.com/onsi/ginkgo"
	promClient "github.com/prometheus/client_model/go"
	"github.com/prometheus/prom2json"
	"gomodules.xyz/pointer"
	core "k8s.io/api/core/v1"
	v1 "k8s.io/api/core/v1"
	kerr "k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/labels"
	"k8s.io/apimachinery/pkg/runtime/schema"
	"k8s.io/apimachinery/pkg/util/wait"
	kutil "kmodules.xyz/client-go"
	meta_util "kmodules.xyz/client-go/meta"
	"kmodules.xyz/client-go/tools/portforward"
	mona "kmodules.xyz/monitoring-agent-api/api/v1"
	"stash.appscode.dev/apimachinery/apis"
	stash_v1alpha1 "stash.appscode.dev/apimachinery/apis/stash/v1alpha1"
	stash_v1beta1 "stash.appscode.dev/apimachinery/apis/stash/v1beta1"
)

const (
	updateRetryInterval  = 10 * 1000 * 1000 * time.Nanosecond
	maxAttempts          = 5
	mongodbUpMetric      = "mongodb_up"
	metricsMatchedCount  = 2
	mongodbVersionMetric = "mongodb_version_info"
	labelApp             = "app"
)

func (f *Framework) DeleteCASecret(clientCASecret *v1.Secret) {
	err := f.CheckSecret(clientCASecret)
	if err != nil {
		return
	}
	if err := f.DeleteSecret(clientCASecret.ObjectMeta); err != nil && !kerr.IsNotFound(err) {
		fmt.Printf("error in deletion of CA secret. Error: %v", err)
	}
}

func (f *Framework) DeleteGarbageCASecrets(secretList []*v1.Secret) {
	if len(secretList) == 0 {
		return
	}
	for _, secret := range secretList {
		f.DeleteCASecret(secret)
	}
}

func (f *Framework) CleanWorkloadLeftOvers(fqn string) {
	// delete statefulset
	if err := f.kubeClient.AppsV1().StatefulSets(f.namespace).DeleteCollection(context.TODO(), meta_util.DeleteInForeground(), metav1.ListOptions{
		LabelSelector: labels.SelectorFromSet(map[string]string{
			meta_util.NameLabelKey: fqn,
		}).String(),
	}); err != nil && !kerr.IsNotFound(err) {
		fmt.Printf("error in deletion of Statefulset. Error: %v", err)
	}

	// delete pvc
	if err := f.kubeClient.CoreV1().PersistentVolumeClaims(f.namespace).DeleteCollection(context.TODO(), meta_util.DeleteInForeground(), metav1.ListOptions{
		LabelSelector: labels.SelectorFromSet(map[string]string{
			meta_util.NameLabelKey: fqn,
		}).String(),
	}); err != nil && !kerr.IsNotFound(err) {
		fmt.Printf("error in deletion of PVC. Error: %v", err)
	}
}

func (f *Framework) AddMonitor(obj *api.MongoDB) {
	obj.Spec.Monitor = &mona.AgentSpec{
		Prometheus: &mona.PrometheusSpec{
			Exporter: mona.PrometheusExporterSpec{
				Port:            mona.PrometheusExporterPortNumber,
				Resources:       v1.ResourceRequirements{},
				SecurityContext: nil,
			},
		},
		Agent: mona.AgentPrometheus,
	}
}

func (f *Framework) VerifyShardExporters(meta metav1.ObjectMeta) error {
	mongoDB, err := f.dbClient.KubedbV1alpha2().MongoDBs(meta.Namespace).Get(context.TODO(), meta.Name, metav1.GetOptions{})
	if err != nil {
		log.Infoln(err)
		return err
	}

	newMeta := metav1.ObjectMeta{
		Name:      "",
		Namespace: meta.Namespace,
	}
	// for config server
	newMeta.Name = mongoDB.ConfigSvrNodeName()
	err = f.VerifyMongoDBExporter(newMeta)
	if err != nil {
		log.Infoln(err)
		return err
	}
	// for shards
	newMeta.Name = mongoDB.ShardNodeName(int32(0))
	err = f.VerifyMongoDBExporter(newMeta)
	if err != nil {
		log.Infoln(err)
		return err
	}
	// for mongos
	newMeta.Name = mongoDB.MongosNodeName()
	err = f.VerifyMongoDBExporter(newMeta)
	if err != nil {
		log.Infoln(err)
		return err
	}

	return nil
}

func (f *Framework) VerifyInMemory(meta metav1.ObjectMeta) error {
	svcName := f.GetPrimaryService(meta)
	storageEngine, err := f.getStorageEngine(meta, string(core.ResourceServices), svcName)
	if err != nil {
		log.Infoln(err)
		return err
	}

	if storageEngine != string(api.StorageEngineInMemory) {
		return fmt.Errorf("storageEngine is not inMemory")
	}

	return nil
}

//VerifyMongoDBExporter uses metrics from given URL
//and check against known key and value
//to verify the connection is functioning as intended
func (f *Framework) VerifyMongoDBExporter(meta metav1.ObjectMeta) error {
	tunnel, err := f.ForwardToPort(meta, string(core.ResourcePods), fmt.Sprintf("%v-0", meta.Name), aws.Int(mona.PrometheusExporterPortNumber))
	if err != nil {
		log.Infoln(err)
		return err
	}
	defer tunnel.Close()
	return wait.PollImmediate(time.Second, kutil.ReadinessTimeout, func() (bool, error) {
		metricsURL := fmt.Sprintf("http://127.0.0.1:%d/metrics", tunnel.Local)
		mfChan := make(chan *promClient.MetricFamily, 1024)
		transport := makeTransport()

		err := prom2json.FetchMetricFamilies(metricsURL, mfChan, transport)
		if err != nil {
			log.Infoln(err)
			return false, nil
		}

		var count = 0
		for mf := range mfChan {
			if mf.Metric != nil && mf.Metric[0].Gauge != nil && mf.Metric[0].Gauge.Value != nil {
				if *mf.Name == mongodbVersionMetric && strings.Contains(DBVersion, *mf.Metric[0].Label[0].Value) {
					count++
				} else if *mf.Name == mongodbUpMetric && int(*mf.Metric[0].Gauge.Value) > 0 {
					count++
				}
			}
		}

		if count != metricsMatchedCount {
			return false, nil
		}
		log.Infoln("Found ", count, " metrics out of ", metricsMatchedCount)
		return true, nil
	})
}
func makeTransport() *http.Transport {
	return &http.Transport{
		TLSClientConfig: &tls.Config{InsecureSkipVerify: true},
	}
}

func (f *Framework) ForwardToPort(meta metav1.ObjectMeta, resource, name string, port *int) (*portforward.Tunnel, error) {
	if port == nil {
		port = pointer.IntP(mona.PrometheusExporterPortNumber)
	}

	tunnel := portforward.NewTunnel(
		portforward.TunnelOptions{
			Client:    f.kubeClient.CoreV1().RESTClient(),
			Config:    f.restConfig,
			Resource:  resource,
			Namespace: meta.Namespace,
			Name:      name,
			Remote:    *port,
		})
	if err := tunnel.ForwardPort(); err != nil {
		return nil, err
	}

	return tunnel, nil
}

func (fi *Framework) GetPod(meta metav1.ObjectMeta) (*core.Pod, error) {
	podList, err := fi.kubeClient.CoreV1().Pods(meta.Namespace).List(context.TODO(), metav1.ListOptions{})
	if err != nil {
		return nil, err
	}
	for _, pod := range podList.Items {
		if bytes.HasPrefix([]byte(pod.Name), []byte(meta.Name)) {
			return &pod, nil
		}
	}
	return nil, fmt.Errorf("no pod found for workload %v", meta.Name)
}

func (fi *Invocation) PrintDebugInfoOnFailure() {
	if CurrentGinkgoTestDescription().Failed {
		fi.PrintDebugHelpers()
		TestFailed = true
	}
}

func (f *Framework) PrintDebugHelpers() {
	sh := shell.NewSession()
	fmt.Println("\n======================================[ Describe Nodes ]===================================================")
	if err := sh.Command("/usr/bin/kubectl", "get", "nodes").Run(); err != nil {
		fmt.Println(err)
	}

	fmt.Println("\n======================================[ Describe Job ]===================================================")
	if err := sh.Command("/usr/bin/kubectl", "describe", "job", "-n", f.Namespace()).Run(); err != nil {
		fmt.Println(err)
	}

	fmt.Println("\n======================================[ Describe Pod ]===================================================")
	if err := sh.Command("/usr/bin/kubectl", "describe", "po", "-n", f.Namespace()).Run(); err != nil {
		fmt.Println(err)
	}

	fmt.Println("\n======================================[ Describe Mongo ]===================================================")
	if err := sh.Command("/usr/bin/kubectl", "describe", "mg", "-n", f.Namespace()).Run(); err != nil {
		fmt.Println(err)
	}

	fmt.Println("\n======================================[ Describe MySQL ]===================================================")
	if err := sh.Command("/usr/bin/kubectl", "describe", "my", "-n", f.Namespace()).Run(); err != nil {
		fmt.Println(err)
	}

	fmt.Println("\n======================================[ Describe OpsRequest ]===================================================")
	if err := sh.Command("/usr/bin/kubectl", "describe", "myops", "-n", f.Namespace()).Run(); err != nil {
		fmt.Println(err)
	}

	fmt.Println("\n======================================[ Describe Repository ]==========================================")
	if err := sh.Command("/usr/bin/kubectl", "describe", "repository", "-n", f.Namespace()).Run(); err != nil {
		fmt.Println(err)
	}

	fmt.Println("\n======================================[ Describe BackupConfiguration ]==========================================")
	if err := sh.Command("/usr/bin/kubectl", "describe", "backupconfiguration", "-n", f.Namespace()).Run(); err != nil {
		fmt.Println(err)
	}

	fmt.Println("\n======================================[ Describe BackupBlueprint ]==========================================")
	if err := sh.Command("/usr/bin/kubectl", "describe", "backupblueprint", "-n", f.Namespace()).Run(); err != nil {
		fmt.Println(err)
	}

	fmt.Println("\n======================================[ Describe BackupSession ]==========================================")
	if err := sh.Command("/usr/bin/kubectl", "describe", "backupsession", "-n", f.Namespace()).Run(); err != nil {
		fmt.Println(err)
	}

	fmt.Println("\n======================================[ Describe RestoreSession ]==========================================")
	if err := sh.Command("/usr/bin/kubectl", "describe", "restoresession", "-n", f.Namespace()).Run(); err != nil {
		fmt.Println(err)
	}

	fmt.Println("\n======================================[ Describe Nodes ]===================================================")
	if err := sh.Command("/usr/bin/kubectl", "describe", "nodes").Run(); err != nil {
		fmt.Println(err)
	}
}

func (fi *Invocation) IsGKE() bool {
	_, ok := os.LookupEnv("GOOGLE_SERVICE_ACCOUNT_JSON_KEY")

	return ok
}

func (fi *Invocation) AppendToCleanupList(resources ...interface{}) {
	for r := range resources {
		fi.testResources = append(fi.testResources, resources[r])
	}
}

func getGVRAndObjectMeta(obj interface{}) (schema.GroupVersionResource, metav1.ObjectMeta, error) {
	switch r := obj.(type) {
	case *api.MongoDB:
		r.GetObjectKind().SetGroupVersionKind(api.SchemeGroupVersion.WithKind(api.ResourceKindMongoDB))
		gvk := r.GroupVersionKind()
		return schema.GroupVersionResource{Group: gvk.Group, Version: gvk.Version, Resource: api.ResourcePluralMongoDB}, r.ObjectMeta, nil
	case *v1alpha1.MongoDBOpsRequest:
		r.GetObjectKind().SetGroupVersionKind(opsapi.SchemeGroupVersion.WithKind(opsapi.ResourceKindMongoDBOpsRequest))
		gvk := r.GroupVersionKind()
		return schema.GroupVersionResource{Group: gvk.Group, Version: gvk.Version, Resource: opsapi.ResourcePluralMongoDBOpsRequest}, r.ObjectMeta, nil
	case *api.MySQL:
		r.GetObjectKind().SetGroupVersionKind(api.SchemeGroupVersion.WithKind(api.ResourceKindMySQL))
		gvk := r.GroupVersionKind()
		return schema.GroupVersionResource{Group: gvk.Group, Version: gvk.Version, Resource: api.ResourcePluralMySQL}, r.ObjectMeta, nil
	case *v1alpha1.MySQLOpsRequest:
		r.GetObjectKind().SetGroupVersionKind(opsapi.SchemeGroupVersion.WithKind(opsapi.ResourceKindMySQLOpsRequest))
		gvk := r.GroupVersionKind()
		return schema.GroupVersionResource{Group: gvk.Group, Version: gvk.Version, Resource: opsapi.ResourcePluralMySQLOpsRequest}, r.ObjectMeta, nil
	case *core.Secret:
		r.GetObjectKind().SetGroupVersionKind(core.SchemeGroupVersion.WithKind("Secret"))
		gvk := r.GroupVersionKind()
		return schema.GroupVersionResource{Group: gvk.Group, Version: gvk.Version, Resource: "secrets"}, r.ObjectMeta, nil
	case *core.Service:
		r.GetObjectKind().SetGroupVersionKind(core.SchemeGroupVersion.WithKind(apis.KindService))
		gvk := r.GroupVersionKind()
		return schema.GroupVersionResource{Group: gvk.Group, Version: gvk.Version, Resource: apis.ResourcePluralService}, r.ObjectMeta, nil
	case *core.ConfigMap:
		r.GetObjectKind().SetGroupVersionKind(core.SchemeGroupVersion.WithKind("ConfigMap"))
		gvk := r.GroupVersionKind()
		return schema.GroupVersionResource{Group: gvk.Group, Version: gvk.Version, Resource: "configmaps"}, r.ObjectMeta, nil
	case *cm_api.Issuer:
		r.GetObjectKind().SetGroupVersionKind(cm_api.SchemeGroupVersion.WithKind(cm_api.IssuerKind))
		gvk := r.GroupVersionKind()
		return schema.GroupVersionResource{Group: gvk.Group, Version: gvk.Version, Resource: "issuers"}, r.ObjectMeta, nil
	case *stash_v1alpha1.Repository:
		r.GetObjectKind().SetGroupVersionKind(stash_v1alpha1.SchemeGroupVersion.WithKind(stash_v1alpha1.ResourceKindRepository))
		gvk := r.GroupVersionKind()
		return schema.GroupVersionResource{Group: gvk.Group, Version: gvk.Version, Resource: stash_v1alpha1.ResourcePluralRepository}, r.ObjectMeta, nil
	case *stash_v1beta1.BackupConfiguration:
		r.GetObjectKind().SetGroupVersionKind(stash_v1beta1.SchemeGroupVersion.WithKind(stash_v1beta1.ResourceKindBackupConfiguration))
		gvk := r.GroupVersionKind()
		return schema.GroupVersionResource{Group: gvk.Group, Version: gvk.Version, Resource: stash_v1beta1.ResourcePluralBackupConfiguration}, r.ObjectMeta, nil
	case *stash_v1beta1.BackupBlueprint:
		r.GetObjectKind().SetGroupVersionKind(stash_v1beta1.SchemeGroupVersion.WithKind(stash_v1beta1.ResourceKindBackupBlueprint))
		gvk := r.GroupVersionKind()
		return schema.GroupVersionResource{Group: gvk.Group, Version: gvk.Version, Resource: stash_v1beta1.ResourcePluralBackupConfiguration}, r.ObjectMeta, nil
	case *stash_v1beta1.BackupSession:
		r.GetObjectKind().SetGroupVersionKind(stash_v1beta1.SchemeGroupVersion.WithKind(stash_v1beta1.ResourceKindBackupSession))
		gvk := r.GroupVersionKind()
		return schema.GroupVersionResource{Group: gvk.Group, Version: gvk.Version, Resource: stash_v1beta1.ResourcePluralBackupSession}, r.ObjectMeta, nil
	case *stash_v1beta1.RestoreSession:
		r.GetObjectKind().SetGroupVersionKind(stash_v1beta1.SchemeGroupVersion.WithKind(stash_v1beta1.ResourceKindRestoreSession))
		gvk := r.GroupVersionKind()
		return schema.GroupVersionResource{Group: gvk.Group, Version: gvk.Version, Resource: stash_v1beta1.ResourcePluralRestoreSession}, r.ObjectMeta, nil
	default:
		return schema.GroupVersionResource{}, metav1.ObjectMeta{}, fmt.Errorf("failed to get GroupVersionResource. Reason: Unknown resource type")
	}
}

func (f *Framework) waitUntilResourceDeleted(gvr schema.GroupVersionResource, objMeta metav1.ObjectMeta) error {
	return wait.PollImmediate(PullInterval, WaitTimeOut, func() (done bool, err error) {
		if _, err := f.dmClient.Resource(gvr).Namespace(objMeta.Namespace).Get(context.TODO(), objMeta.Name, metav1.GetOptions{}); err != nil {
			if kerr.IsNotFound(err) {
				return true, nil
			} else {
				return true, err
			}
		}
		return false, nil
	})
}

func (fi *Invocation) CleanupTestResources() error {
	// delete all test resources
	for r := range fi.testResources {
		gvr, objMeta, err := getGVRAndObjectMeta(fi.testResources[r])
		if err != nil {
			return err
		}
		err = fi.dmClient.Resource(gvr).Namespace(objMeta.Namespace).Delete(context.TODO(), objMeta.Name, meta_util.DeleteInForeground())
		if err != nil && !kerr.IsNotFound(err) {
			return err
		}
	}

	// wait until resource has been deleted
	for r := range fi.testResources {
		gvr, objMeta, err := getGVRAndObjectMeta(fi.testResources[r])
		if err != nil {
			return err
		}
		err = fi.waitUntilResourceDeleted(gvr, objMeta)
		if err != nil {
			return err
		}
	}

	return nil
}
