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
	core "k8s.io/api/core/v1"
	v1 "k8s.io/api/core/v1"
	kerr "k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/labels"
	"k8s.io/apimachinery/pkg/runtime/schema"
	"k8s.io/apimachinery/pkg/util/wait"
	"k8s.io/client-go/kubernetes"
	"k8s.io/client-go/rest"
	kutil "kmodules.xyz/client-go"
	meta_util "kmodules.xyz/client-go/meta"
	"kmodules.xyz/client-go/tools/portforward"
	mona "kmodules.xyz/monitoring-agent-api/api/v1"
	"stash.appscode.dev/apimachinery/apis"
)

const (
	updateRetryInterval  = 10 * 1000 * 1000 * time.Nanosecond
	maxAttempts          = 5
	mongodbUpMetric      = "mongodb_up"
	metricsMatchedCount  = 2
	mongodbVersionMetric = "mongodb_version_info"
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

func (f *Framework) CleanWorkloadLeftOvers() {
	// delete statefulset
	if err := f.kubeClient.AppsV1().StatefulSets(f.namespace).DeleteCollection(context.TODO(), meta_util.DeleteInForeground(), metav1.ListOptions{
		LabelSelector: labels.SelectorFromSet(map[string]string{
			api.LabelDatabaseKind: api.ResourceKindMongoDB,
		}).String(),
	}); err != nil && !kerr.IsNotFound(err) {
		fmt.Printf("error in deletion of Statefulset. Error: %v", err)
	}

	// delete pvc
	if err := f.kubeClient.CoreV1().PersistentVolumeClaims(f.namespace).DeleteCollection(context.TODO(), meta_util.DeleteInForeground(), metav1.ListOptions{
		LabelSelector: labels.SelectorFromSet(map[string]string{
			api.LabelDatabaseKind: api.ResourceKindMongoDB,
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
	err = f.VerifyExporter(newMeta)
	if err != nil {
		log.Infoln(err)
		return err
	}
	// for shards
	newMeta.Name = mongoDB.ShardNodeName(int32(0))
	err = f.VerifyExporter(newMeta)
	if err != nil {
		log.Infoln(err)
		return err
	}
	// for mongos
	newMeta.Name = mongoDB.MongosNodeName()
	err = f.VerifyExporter(newMeta)
	if err != nil {
		log.Infoln(err)
		return err
	}

	return nil
}

func (f *Framework) VerifyInMemory(meta metav1.ObjectMeta) error {
	mongoDB, err := f.dbClient.KubedbV1alpha2().MongoDBs(meta.Namespace).Get(context.TODO(), meta.Name, metav1.GetOptions{})
	if err != nil {
		log.Infoln(err)
		return err
	}

	if mongoDB.Spec.ShardTopology == nil {
		podName := fmt.Sprintf("%s-0", mongoDB.OffshootName())
		storageEngine, err := f.getStorageEngine(mongoDB.ObjectMeta, podName)
		if err != nil {
			log.Infoln(err)
			return err
		}

		if storageEngine != string(api.StorageEngineInMemory) {
			return fmt.Errorf("storageEngine is not inMemory")
		}

		return nil
	}
	// for shards
	podName := fmt.Sprintf("%s-0", mongoDB.ShardNodeName(int32(0)))
	storageEngine, err := f.getStorageEngine(mongoDB.ObjectMeta, podName)
	if err != nil {
		log.Infoln(err)
		return err
	}

	if storageEngine != string(api.StorageEngineInMemory) {
		return fmt.Errorf("storageEngine is not inMemory")
	}

	return nil
}

//VerifyExporter uses metrics from given URL
//and check against known key and value
//to verify the connection is functioning as intended
func (f *Framework) VerifyExporter(meta metav1.ObjectMeta) error {
	tunnel, err := f.ForwardToPort(meta, fmt.Sprintf("%v-0", meta.Name), aws.Int(mona.PrometheusExporterPortNumber))
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

func (f *Framework) ForwardToPort(meta metav1.ObjectMeta, clientPodName string, port *int) (*portforward.Tunnel, error) {
	var defaultPort = mona.PrometheusExporterPortNumber
	if port != nil {
		defaultPort = *port
	}

	tunnel := portforward.NewTunnel(
		f.kubeClient.CoreV1().RESTClient(),
		f.restConfig,
		meta.Namespace,
		clientPodName,
		defaultPort,
	)
	if err := tunnel.ForwardPort(); err != nil {
		return nil, err
	}

	return tunnel, nil
}

func ForwardPort(
	kubeClient kubernetes.Interface,
	restConfig *rest.Config,
	meta metav1.ObjectMeta, clientPodName string, port int) (*portforward.Tunnel, error) {
	tunnel := portforward.NewTunnel(
		kubeClient.CoreV1().RESTClient(),
		restConfig,
		meta.Namespace,
		clientPodName,
		port,
	)

	if err := tunnel.ForwardPort(); err != nil {
		return nil, err
	}
	return tunnel, nil
}

func FowardedPodsIPWithTunnel(
	kubeClient kubernetes.Interface, restConfig *rest.Config,
	redis *api.Redis) ([][]string, [][]*portforward.Tunnel, error) {

	var (
		rdAddresses [][]string
		tunnels     [][]*portforward.Tunnel
		err         error
		podName     string
	)
	rdAddresses = make([][]string, int(*redis.Spec.Cluster.Master))
	tunnels = make([][]*portforward.Tunnel, int(*redis.Spec.Cluster.Master))
	for i := 0; i < int(*redis.Spec.Cluster.Master); i++ {
		rdAddresses[i] = make([]string, int(*redis.Spec.Cluster.Replicas)+1)
		tunnels[i] = make([]*portforward.Tunnel, int(*redis.Spec.Cluster.Replicas)+1)
		for j := 0; j <= int(*redis.Spec.Cluster.Replicas); j++ {
			podName = fmt.Sprintf("%s-shard%d-%d", redis.Name, i, j)
			if tunnels[i][j], err = ForwardPort(kubeClient, restConfig, redis.ObjectMeta, podName, 6379); err != nil {
				return nil, nil, err
			}
			rdAddresses[i][j] = fmt.Sprintf("%d", tunnels[i][j].Local)
		}
	}

	return rdAddresses, tunnels, nil
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

func (f *Invocation) IsGKE() bool {
	_, ok := os.LookupEnv("GOOGLE_SERVICE_ACCOUNT_JSON_KEY")

	return ok
}

func (fi *Invocation) AppendToCleanupList(resources ...interface{}) {
	for r := range resources {
		fi.testResources = append(fi.testResources, resources[r])
	}
}

func getGVRAndObjectMeta(obj interface{}) (schema.GroupVersionResource, metav1.ObjectMeta, error) {
	switch w := obj.(type) {
	case *api.MongoDB:
		w.GetObjectKind().SetGroupVersionKind(api.SchemeGroupVersion.WithKind(api.ResourceKindMongoDB))
		gvk := w.GroupVersionKind()
		return schema.GroupVersionResource{Group: gvk.Group, Version: gvk.Version, Resource: api.ResourcePluralMongoDB}, w.ObjectMeta, nil
	case *v1alpha1.MongoDBOpsRequest:
		w.GetObjectKind().SetGroupVersionKind(opsapi.SchemeGroupVersion.WithKind(opsapi.ResourceKindMongoDBOpsRequest))
		gvk := w.GroupVersionKind()
		return schema.GroupVersionResource{Group: gvk.Group, Version: gvk.Version, Resource: opsapi.ResourcePluralMongoDBOpsRequest}, w.ObjectMeta, nil
	case *api.MySQL:
		w.GetObjectKind().SetGroupVersionKind(api.SchemeGroupVersion.WithKind(api.ResourceKindMySQL))
		gvk := w.GroupVersionKind()
		return schema.GroupVersionResource{Group: gvk.Group, Version: gvk.Version, Resource: api.ResourcePluralMySQL}, w.ObjectMeta, nil
	case *v1alpha1.MySQLOpsRequest:
		w.GetObjectKind().SetGroupVersionKind(opsapi.SchemeGroupVersion.WithKind(opsapi.ResourceKindMySQLOpsRequest))
		gvk := w.GroupVersionKind()
		return schema.GroupVersionResource{Group: gvk.Group, Version: gvk.Version, Resource: opsapi.ResourcePluralMySQLOpsRequest}, w.ObjectMeta, nil
	case *core.Secret:
		w.GetObjectKind().SetGroupVersionKind(core.SchemeGroupVersion.WithKind("Secret"))
		gvk := w.GroupVersionKind()
		return schema.GroupVersionResource{Group: gvk.Group, Version: gvk.Version, Resource: "secrets"}, w.ObjectMeta, nil
	case *core.Service:
		w.GetObjectKind().SetGroupVersionKind(core.SchemeGroupVersion.WithKind(apis.KindService))
		gvk := w.GroupVersionKind()
		return schema.GroupVersionResource{Group: gvk.Group, Version: gvk.Version, Resource: apis.ResourcePluralService}, w.ObjectMeta, nil
	case *core.ConfigMap:
		w.GetObjectKind().SetGroupVersionKind(core.SchemeGroupVersion.WithKind("ConfigMap"))
		gvk := w.GroupVersionKind()
		return schema.GroupVersionResource{Group: gvk.Group, Version: gvk.Version, Resource: "configmaps"}, w.ObjectMeta, nil
	case *cm_api.Issuer:
		w.GetObjectKind().SetGroupVersionKind(cm_api.SchemeGroupVersion.WithKind(cm_api.IssuerKind))
		gvk := w.GroupVersionKind()
		return schema.GroupVersionResource{Group: gvk.Group, Version: gvk.Version, Resource: "issuers"}, w.ObjectMeta, nil
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
