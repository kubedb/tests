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
	"crypto/x509"
	"encoding/pem"
	"fmt"
	"strconv"
	"strings"
	"time"

	"kubedb.dev/apimachinery/apis/autoscaling/v1alpha1"
	api "kubedb.dev/apimachinery/apis/kubedb/v1alpha2"

	. "github.com/onsi/gomega"
	"github.com/pkg/errors"
	"go.mongodb.org/mongo-driver/bson"
	"go.mongodb.org/mongo-driver/bson/primitive"
	"go.mongodb.org/mongo-driver/mongo"
	"go.mongodb.org/mongo-driver/mongo/options"
	"gomodules.xyz/x/arrays"
	core "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/klog/v2"
	v1 "kmodules.xyz/client-go/api/v1"
	"kmodules.xyz/client-go/tools/exec"
	"kmodules.xyz/client-go/tools/portforward"
)

type KubedbTable struct {
	FirstName string
	LastName  string
	// for MySQL
	Id   int64  `xorm:"pk autoincr"`
	Name string `xorm:"varchar(25) not null 'usr_name' comment('NickName')"`
}

const (
	SampleDB       = "sampleDB"
	AnotherDB      = "anotherDB"
	SampleDocument = "sampleDoc"
)

type Collection struct {
	Name     string
	Document Document
}

type Document struct {
	Name  string
	State string
}

var SampleCollection = Collection{
	Name: "sampleCollection",
	Document: Document{
		Name:  SampleDocument,
		State: "initial",
	},
}

var AnotherCollection = Collection{
	Name: "anotherCollection",
	Document: Document{
		Name:  SampleDocument,
		State: "initial",
	},
}

var UpdatedCollection = Collection{
	Name: "sampleCollection",
	Document: Document{
		Name:  SampleDocument,
		State: "updated",
	},
}

func (f *Framework) ForwardPort(meta metav1.ObjectMeta, resource, name string, remotePort int) (*portforward.Tunnel, error) {
	tunnel := portforward.NewTunnel(portforward.TunnelOptions{
		Client:    f.kubeClient.CoreV1().RESTClient(),
		Config:    f.restConfig,
		Resource:  resource,
		Namespace: meta.Namespace,
		Name:      name,
		Remote:    remotePort,
	})

	if err := tunnel.ForwardPort(); err != nil {
		return nil, err
	}
	return tunnel, nil
}

func (f *Framework) GetMongoDBClient(meta metav1.ObjectMeta, tunnel *portforward.Tunnel, isReplSet ...bool) (*options.ClientOptions, error) {
	mongodb, err := f.GetMongoDB(meta)
	if err != nil {
		return nil, err
	}

	user := "root"
	pass, err := f.GetMongoDBRootPassword(mongodb)
	if err != nil {
		return nil, err
	}

	clientOpts := options.Client().ApplyURI(fmt.Sprintf("mongodb://%s:%s@127.0.0.1:%v", user, pass, tunnel.Local))
	if mongodb.Spec.SSLMode == api.SSLModeRequireSSL {
		if err := f.GetSSLCertificate(meta); err != nil {
			return nil, err
		}
		clientOpts = options.Client().ApplyURI(fmt.Sprintf("mongodb://%s:%s@localhost:%v/?ssl=true&sslclientcertificatekeyfile=/tmp/mongodb/%v&&sslcertificateauthorityfile=/tmp/mongodb/%v", user, pass, tunnel.Local, api.MongoPemFileName, api.TLSCACertFileName))
	}

	if (len(isReplSet) > 0 && isReplSet[0]) || IsRepSet(mongodb) {
		clientOpts.SetDirect(true)
	}
	return clientOpts, nil
}

func (f *Framework) ConnectAndPing(meta metav1.ObjectMeta, resource, name string, isReplSet ...bool) (*mongo.Client, *portforward.Tunnel, error) {
	tunnel, err := f.ForwardPort(meta, resource, name, api.MongoDBDatabasePort)
	if err != nil {
		return nil, nil, err
	}

	clientOpts, err := f.GetMongoDBClient(meta, tunnel, isReplSet...)
	if err != nil {
		return nil, nil, err
	}

	client, err := mongo.Connect(context.Background(), clientOpts)
	if err != nil {
		return nil, nil, err
	}

	err = client.Ping(context.TODO(), nil)
	if err != nil {
		return nil, nil, err
	}
	return client, tunnel, err
}

func (f *Framework) GetMongosPodName(meta metav1.ObjectMeta) (string, error) {
	pods, err := f.kubeClient.CoreV1().Pods(meta.Namespace).List(context.TODO(), metav1.ListOptions{})
	if err != nil {
		return "", err
	}
	for _, pod := range pods.Items {
		if strings.HasPrefix(pod.Name, fmt.Sprintf("%v-mongos", meta.Name)) {
			return pod.Name, nil
		}
	}
	return "", fmt.Errorf("no pod found for mongodb: %s", meta.Name)
}

func (f *Framework) GetReplicaMasterNode(meta metav1.ObjectMeta, nodeName string, replicaNumber *int32) (string, error) {
	if replicaNumber == nil {
		return "", fmt.Errorf("replica is zero")
	}

	fn := func(clientPodName string) (bool, error) {
		client, tunnel, err := f.ConnectAndPing(meta, string(core.ResourcePods), clientPodName, true)
		if err != nil {
			return false, err
		}
		defer tunnel.Close()

		res := make(map[string]interface{})
		if err := client.Database("admin").RunCommand(context.Background(), bson.D{{Key: "isMaster", Value: "1"}}).Decode(&res); err != nil {
			return false, err
		}

		if val, ok := res["ismaster"]; ok && val == true {
			return true, nil
		}
		return false, fmt.Errorf("%v not master node", clientPodName)
	}

	// For MongoDB ReplicaSet, Find out the primary instance.
	// Extract information `IsMaster: true` from the component's status.
	for i := int32(0); i <= *replicaNumber; i++ {
		clientPodName := fmt.Sprintf("%v-%d", nodeName, i)
		var isMaster bool
		isMaster, err := fn(clientPodName)
		if err == nil && isMaster {
			return clientPodName, nil
		}
	}
	return "", fmt.Errorf("no primary node")
}

func (f *Framework) GetPrimaryService(meta metav1.ObjectMeta) string {
	// MongoDB creates primary Service with the same name as the database.
	return meta.Name
}

func (f *Framework) EventuallyPingMongo(meta metav1.ObjectMeta) GomegaAsyncAssertion {
	return Eventually(
		func() bool {
			svcName := f.GetPrimaryService(meta)
			_, tunnel, err := f.ConnectAndPing(meta, string(core.ResourceServices), svcName)
			if err != nil {
				klog.Errorln("Failed to ConnectAndPing. Reason: ", err)
				return false
			}
			defer tunnel.Close()
			return true
		},
		time.Minute*5,
		time.Second*5,
	)
}

func (f *Framework) EventuallyInsertDocument(meta metav1.ObjectMeta, dbName string, collectionCount int) GomegaAsyncAssertion {
	return Eventually(
		func() (bool, error) {
			svcName := f.GetPrimaryService(meta)
			client, tunnel, err := f.ConnectAndPing(meta, string(core.ResourceServices), svcName)
			if err != nil {
				klog.Errorln("Failed to ConnectAndPing. Reason: ", err)
				return false, err
			}
			defer tunnel.Close()

			person := &KubedbTable{
				FirstName: "kubernetes",
				LastName:  "database",
			}

			if _, err := client.Database(dbName).Collection("people").InsertOne(context.Background(), person); err != nil {
				klog.Errorln("creation error:", err)
				return false, err
			}

			// above one is 0th element
			for i := 1; i < collectionCount; i++ {

				person := &KubedbTable{
					FirstName: fmt.Sprintf("kubernetes-%03d", i),
					LastName:  fmt.Sprintf("database-%03d", i),
				}

				if _, err := client.Database(dbName).Collection(fmt.Sprintf("people-%03d", i)).InsertOne(context.Background(), person); err != nil {
					klog.Errorln("creation error:", err)
					return false, err
				}
			}

			return true, nil
		},
		time.Minute*5,
		time.Second*5,
	)
}

func (f *Framework) EventuallyDocumentExists(meta metav1.ObjectMeta, dbName string, collectionCount int) GomegaAsyncAssertion {
	return Eventually(
		func() (bool, error) {
			svcName := f.GetPrimaryService(meta)
			client, tunnel, err := f.ConnectAndPing(meta, string(core.ResourceServices), svcName)
			if err != nil {
				klog.Errorln("Failed to ConnectAndPing. Reason: ", err)
				return false, err
			}
			defer tunnel.Close()

			expected := &KubedbTable{
				FirstName: "kubernetes",
				LastName:  "database",
			}
			person := &KubedbTable{}

			if er := client.Database(dbName).Collection("people").FindOne(context.Background(), bson.M{"firstname": expected.FirstName}).Decode(&person); er != nil || person == nil || person.LastName != expected.LastName {
				klog.Errorln("checking error:", er)
				return false, er
			}

			// above one is 0th element
			for i := 1; i < collectionCount; i++ {
				expected := &KubedbTable{
					FirstName: fmt.Sprintf("kubernetes-%03d", i),
					LastName:  fmt.Sprintf("database-%03d", i),
				}
				person := &KubedbTable{}

				if er := client.Database(dbName).Collection(fmt.Sprintf("people-%03d", i)).FindOne(context.Background(), bson.M{"firstname": expected.FirstName}).Decode(&person); er != nil || person == nil || person.LastName != expected.LastName {
					klog.Errorln("checking error:", er)
					return false, er
				}
			}
			return true, nil
		},
		time.Minute*5,
		time.Second*5,
	)
}

func (f *Framework) EventuallyInsertCollection(meta metav1.ObjectMeta, dbName string, collections ...Collection) GomegaAsyncAssertion {
	return Eventually(
		func() (bool, error) {
			svcName := f.GetPrimaryService(meta)
			client, tunnel, err := f.ConnectAndPing(meta, string(core.ResourceServices), svcName)
			if err != nil {
				klog.Errorln("Failed to ConnectAndPing. Reason: ", err)
				return false, err
			}
			defer tunnel.Close()

			for i := range collections {
				if _, err := client.Database(dbName).Collection(collections[i].Name).InsertOne(context.Background(), collections[i].Document); err != nil {
					klog.Errorln("creation error:", err)
					return false, err
				}
			}
			return true, nil
		},
		time.Minute*5,
		time.Second*5,
	)
}

func (f *Framework) EventuallyUpdateCollection(meta metav1.ObjectMeta, dbName string, collections ...Collection) GomegaAsyncAssertion {
	return Eventually(
		func() (bool, error) {
			svcName := f.GetPrimaryService(meta)
			client, tunnel, err := f.ConnectAndPing(meta, string(core.ResourceServices), svcName)
			if err != nil {
				klog.Errorln("Failed to ConnectAndPing. Reason: ", err)
				return false, err
			}
			defer tunnel.Close()

			for i := range collections {
				updateResult, err := client.Database(dbName).Collection(collections[i].Name).ReplaceOne(context.Background(), bson.M{"name": SampleDocument}, collections[i].Document)
				if err != nil {
					klog.Errorln("update error:", err)
					return false, err
				}
				if updateResult.MatchedCount == 0 {
					return false, fmt.Errorf("no matching document found for the collection: %s", collections[i].Name)
				}
			}
			return true, nil
		},
		time.Minute*5,
		time.Second*5,
	)
}

func (f *Framework) GetDocument(meta metav1.ObjectMeta, dbName, collectionName string) (*Document, error) {
	svcName := f.GetPrimaryService(meta)
	client, tunnel, err := f.ConnectAndPing(meta, string(core.ResourceServices), svcName)
	if err != nil {
		klog.Errorln("Failed to ConnectAndPing. Reason: ", err)
		return nil, err
	}
	defer tunnel.Close()

	var resp *Document
	if err := client.Database(dbName).Collection(collectionName).FindOne(context.Background(), bson.M{"name": SampleDocument}).Decode(&resp); err != nil {
		return nil, err
	}
	return resp, nil
}

func (f *Framework) EventuallyDropDatabase(meta metav1.ObjectMeta, dbName string) GomegaAsyncAssertion {
	return Eventually(
		func() (bool, error) {
			svcName := f.GetPrimaryService(meta)
			client, tunnel, err := f.ConnectAndPing(meta, string(core.ResourceServices), svcName)
			if err != nil {
				klog.Errorln("Failed to ConnectAndPing. Reason: ", err)
				return false, err
			}
			defer tunnel.Close()

			if err := client.Database(dbName).Drop(context.Background()); err != nil {
				klog.Errorln("creation error:", err)
				return false, err
			}
			return true, nil
		},
		time.Minute*5,
		time.Second*5,
	)
}

func (f *Framework) DatabaseExists(meta metav1.ObjectMeta, dbName string) (bool, error) {
	svcName := f.GetPrimaryService(meta)
	client, tunnel, err := f.ConnectAndPing(meta, string(core.ResourceServices), svcName)
	if err != nil {
		return false, err
	}
	defer tunnel.Close()

	databases, err := client.ListDatabaseNames(context.Background(), bson.D{})
	if err != nil {
		return false, err
	}

	if exist, _ := arrays.Contains(databases, dbName); exist {
		return true, nil
	}
	return false, nil
}

// EventuallyEnableSharding enables sharding of a database. Call this only when spec.shardTopology is set.
func (f *Framework) EventuallyEnableSharding(meta metav1.ObjectMeta, dbName string) GomegaAsyncAssertion {
	return Eventually(
		func() (bool, error) {
			svcName := f.GetPrimaryService(meta)
			client, tunnel, err := f.ConnectAndPing(meta, string(core.ResourceServices), svcName, false)
			if err != nil {
				klog.Errorln("Failed to ConnectAndPing. Reason: ", err)
				return false, err
			}
			defer tunnel.Close()

			singleRes := client.Database("admin").RunCommand(context.Background(), bson.D{{Key: "enableSharding", Value: dbName}})
			if singleRes.Err() != nil {
				klog.Errorln("RunCommand enableSharding error:", err)
				return false, err
			}

			// Now shard collection
			singleRes = client.Database("admin").RunCommand(context.Background(), bson.D{{Key: "shardCollection", Value: fmt.Sprintf("%v.public", dbName)}, {Key: "key", Value: bson.M{"firstname": 1}}})
			if singleRes.Err() != nil {
				klog.Errorln("RunCommand shardCollection error:", err)
				return false, err
			}

			return true, nil
		},
		time.Minute*5,
		time.Second*5,
	)
}

// EventuallyCollectionPartitioned checks if a database is partitioned or not. Call this only when spec.shardTopology is set.
func (f *Framework) EventuallyCollectionPartitioned(meta metav1.ObjectMeta, dbName string) GomegaAsyncAssertion {
	return Eventually(
		func() (bool, error) {
			svcName := f.GetPrimaryService(meta)
			client, tunnel, err := f.ConnectAndPing(meta, string(core.ResourceServices), svcName, false)
			if err != nil {
				klog.Errorln("Failed to ConnectAndPing. Reason: ", err)
				return false, err
			}
			defer tunnel.Close()

			res := make(map[string]interface{})
			err = client.Database("config").Collection("databases").FindOne(context.TODO(), bson.D{{Key: "_id", Value: dbName}}).Decode(&res)
			if err != nil {
				if err == mongo.ErrNoDocuments {
					klog.Infoln("No document in config.databases:", err)
					return false, nil
				}
				klog.Errorln("Query error:", err)
				return false, err
			}

			val, ok := res["partitioned"]
			if ok && val == true {
				return true, nil
			}
			klog.Errorln("db", dbName, "is not partitioned. Got partitioned:", val)
			return false, nil
		},
		time.Minute*5,
		time.Second*5,
	)
}

func (f *Framework) getMaxIncomingConnections(meta metav1.ObjectMeta, resource, name string, isRepSet bool) (int32, error) {
	client, tunnel, err := f.ConnectAndPing(meta, resource, name, isRepSet)
	if err != nil {
		return 0, fmt.Errorf("failed to ConnectAndPing. Reason: %v", err)
	}
	defer tunnel.Close()

	res := make(map[string]interface{})
	err = client.Database("admin").RunCommand(context.Background(), bson.D{{Key: "getCmdLineOpts", Value: 1}}).Decode(&res)
	if err != nil {
		klog.Errorln("RunCommand getCmdLineOpts error:", err)
		return 0, err
	}

	res, ok := res["parsed"].(map[string]interface{})
	if !ok {
		return 0, fmt.Errorf("can't get 'parsed' value")
	}

	res, ok = res["net"].(map[string]interface{})
	if !ok {
		return 0, fmt.Errorf("can't get 'parsed.net' value")
	}

	val, ok := res["maxIncomingConnections"].(int32)
	if ok {
		return val, nil
	}

	return 0, fmt.Errorf("unable to get maxIncomingConnections")
}

func (f *Framework) EventuallyMaxIncomingConnections(meta metav1.ObjectMeta) GomegaAsyncAssertion {
	return Eventually(
		func() (int32, error) {
			mongodb, err := f.GetMongoDB(meta)
			if err != nil {
				return 0, err
			}
			if mongodb.Spec.ShardTopology == nil {
				// Sharding isn't enabled. We can use the Service to verify incoming connection
				svcName := f.GetPrimaryService(meta)
				val, err := f.getMaxIncomingConnections(meta, string(core.ResourceServices), svcName, IsRepSet(mongodb))
				return val, err
			} else {
				value := int32(-1)
				// Sharding is enabled. We have to verify the MaxIncomingConnections on each shards, mongos nodes,
				// and config server nodes.
				for i := int32(0); i < mongodb.Spec.ShardTopology.Shard.Shards; i++ {
					nodeName := mongodb.ShardNodeName(i)
					podName, err := f.GetReplicaMasterNode(meta, nodeName, &mongodb.Spec.ShardTopology.Shard.Replicas)
					if err != nil {
						return 0, err
					}
					val, err := f.getMaxIncomingConnections(meta, string(core.ResourcePods), podName, true)
					if err != nil {
						return 0, err
					}
					if value != -1 && val != value {
						return 0, fmt.Errorf("different maxIncomingConnections in different nodes. %v & %v ", val, value)
					}
					value = val
				}

				// config server nodes
				nodeName := mongodb.ConfigSvrNodeName()
				podName, err := f.GetReplicaMasterNode(meta, nodeName, &mongodb.Spec.ShardTopology.ConfigServer.Replicas)
				if err != nil {
					return 0, err
				}
				val, err := f.getMaxIncomingConnections(meta, string(core.ResourcePods), podName, true)
				if err != nil {
					return 0, err
				}

				if value != -1 && val != value {
					return 0, fmt.Errorf("different maxIncomingConnections in different nodes. %v & %v ", val, value)
				}
				value = val

				// mongos nodes
				podName, err = f.GetMongosPodName(meta)
				if err != nil {
					return 0, err
				}
				val, err = f.getMaxIncomingConnections(meta, string(core.ResourcePods), podName, true)
				if err != nil {
					return 0, err
				}

				if value != -1 && val != value {
					return 0, fmt.Errorf("different maxIncomingConnections in different nodes. %v & %v ", val, value)
				}
				value = val
				return value, nil
			}
		},
		time.Minute*5,
		time.Second*5,
	)
}

func (f *Framework) getClusterAuthModeFromDB(meta metav1.ObjectMeta) (string, error) {
	svcName := f.GetPrimaryService(meta)
	client, tunnel, err := f.ConnectAndPing(meta, string(core.ResourceServices), svcName, true)
	if err != nil {
		return "", fmt.Errorf("failed to ConnectAndPing. Reason: %v", err)
	}
	defer tunnel.Close()

	res := make(map[string]interface{})
	err = client.Database("admin").
		RunCommand(context.Background(), bson.D{
			primitive.E{Key: "getParameter", Value: 1},
			primitive.E{Key: "clusterAuthMode", Value: 1},
		}).Decode(&res)
	if err != nil {
		klog.Errorln("RunCommand getCmdLineOpts error:", err)
		return "", err
	}

	val, ok := res["clusterAuthMode"]
	if !ok {
		return "", fmt.Errorf("clusterAuthMode not found")
	}

	return val.(string), nil
}

func (f *Framework) getSSLModeFromDB(meta metav1.ObjectMeta) (string, error) {
	svcName := f.GetPrimaryService(meta)

	client, tunnel, err := f.ConnectAndPing(meta, string(core.ResourceServices), svcName, true)
	if err != nil {
		return "", fmt.Errorf("failed to ConnectAndPing. Reason: %v", err)
	}
	defer tunnel.Close()

	res := make(map[string]interface{})
	err = client.Database("admin").
		RunCommand(context.Background(), bson.D{
			primitive.E{Key: "getParameter", Value: 1},
			primitive.E{Key: "sslMode", Value: 1},
		}).Decode(&res)
	if err != nil {
		klog.Errorln("RunCommand getCmdLineOpts error:", err)
		return "", err
	}

	val, ok := res["sslMode"]
	if !ok {
		return "", fmt.Errorf("sslMode not found")
	}

	return val.(string), nil
}

func (f *Framework) EventuallyUserSSLSettings(meta metav1.ObjectMeta, clusterAuthMode *api.ClusterAuthMode, sslMode *api.SSLMode) GomegaAsyncAssertion {
	return Eventually(
		func() (bool, error) {
			mongodb, err := f.GetMongoDB(meta)
			if err != nil {
				return false, err
			}

			clusterAuth := mongodb.Spec.ClusterAuthMode
			if clusterAuthMode != nil {
				clusterAuth = *clusterAuthMode
			}
			if clusterAuth != "" {
				val, err := f.getClusterAuthModeFromDB(meta)
				if err != nil {
					return false, err
				}
				if val != string(clusterAuth) {
					return false, fmt.Errorf("expected clusterAuthMode %v, but got %v", clusterAuth, val)
				}
			}

			sm := mongodb.Spec.SSLMode
			if sslMode != nil {
				sm = *sslMode
			}
			if sm != "" {
				val, err := f.getSSLModeFromDB(meta)
				if err != nil {
					//log.Infoln(err) //TODO: Fixed some cert-manager problems here. Delete later
					return false, err
				}
				if val != string(sm) {
					return false, fmt.Errorf("expected SSLMode %v, but got %v", sm, val)
				}
			}
			return true, nil
		},
		time.Minute*1,
		time.Second*1,
	)
}

func (f *Framework) getStorageEngine(meta metav1.ObjectMeta, resource, name string) (string, error) {
	client, tunnel, err := f.ConnectAndPing(meta, resource, name, true)
	if err != nil {
		return "", fmt.Errorf("failed to ConnectAndPing. Reason: %v", err)
	}
	defer tunnel.Close()

	res := make(map[string]interface{})
	err = client.Database("admin").
		RunCommand(context.Background(), bson.D{
			primitive.E{Key: "serverStatus", Value: 1},
		}).Decode(&res)
	if err != nil {
		klog.Errorln("RunCommand serverStatus error:", err)
		return "", err
	}

	if val, ok := res["storageEngine"]; !ok {
		return "", fmt.Errorf("storageEngine Not found")
	} else {
		if se, ok := val.(map[string]interface{}); ok {
			return se["name"].(string), nil
		}
	}

	return "", nil
}

func (f *Framework) MovePrimary(meta metav1.ObjectMeta, dbName string) error {
	svcName := f.GetPrimaryService(meta)
	client, tunnel, err := f.ConnectAndPing(meta, string(core.ResourceServices), svcName, true)
	if err != nil {
		return err
	}
	defer tunnel.Close()

	buildInfo := make(map[string]interface{})
	err = client.Database("admin").RunCommand(context.Background(), bson.D{{Key: "movePrimary", Value: dbName}, {Key: "to", Value: "shard2"}}).Decode(&buildInfo)
	if err != nil {
		return err
	}

	if val, ok := buildInfo["ok"]; ok {
		if val == 1.0 {
			return nil
		} else {
			return errors.New(buildInfo["errmsg"].(string))
		}
	}

	return nil
}

func (f *Framework) getDiskSizeMongoDB(db *api.MongoDB, storage *v1alpha1.MongoDBStorageAutoscalerSpec) (float64, error) {
	var (
		podName string
		pod     *core.Pod
		err     error
	)

	if storage.ReplicaSet != nil || storage.Standalone != nil {
		podName = fmt.Sprintf("%v-0", db.OffshootName())
	} else if storage.Shard != nil {
		podName = fmt.Sprintf("%v-0", db.ShardNodeName(0))
	} else {
		podName = fmt.Sprintf("%v-0", db.ConfigSvrNodeName())
	}

	pod, err = f.kubeClient.CoreV1().Pods(f.namespace).Get(context.TODO(), podName, metav1.GetOptions{})
	if err != nil {
		return -1, err
	}
	command := []string{"df", "/data/db", "--output=size"}
	res, err := exec.ExecIntoPod(f.restConfig, pod, exec.Command(command...))
	if err != nil {
		return -1, err
	}

	s := strings.Split(res, "\n")
	if len(s) >= 2 {
		return strconv.ParseFloat(strings.Trim(s[1], " "), 64)
	} else {
		return -1, errors.New("bad df output")
	}
}

func (f *Framework) FillDiskMongoDB(db *api.MongoDB, storage *v1alpha1.MongoDBStorageAutoscalerSpec) error {
	var (
		podName string
		pod     *core.Pod
		err     error
	)

	if storage.ReplicaSet != nil || storage.Standalone != nil {
		podName = fmt.Sprintf("%v-0", db.OffshootName())
	} else if storage.Shard != nil {
		podName = fmt.Sprintf("%v-0", db.ShardNodeName(0))
	} else {
		podName = fmt.Sprintf("%v-0", db.ConfigSvrNodeName())
	}

	pod, err = f.kubeClient.CoreV1().Pods(f.namespace).Get(context.TODO(), podName, metav1.GetOptions{})
	if err != nil {
		return err
	}
	command := []string{"dd", "if=/dev/zero", "of=/data/db/file.txt", "count=1024", "bs=548576", "status=none"}

	_, err = exec.ExecIntoPod(f.restConfig, pod, exec.Command(command...))
	if err != nil {
		return err
	}

	return nil
}

func (f *Framework) getCertificateFromShell(db *api.MongoDB, certType string) (string, error) {
	var (
		podName string
		pod     *core.Pod
		err     error
	)

	if db.Spec.ShardTopology == nil {
		podName = fmt.Sprintf("%v-0", db.OffshootName())
	} else {
		podName = fmt.Sprintf("%v-0", db.ConfigSvrNodeName())
	}

	pod, err = f.kubeClient.CoreV1().Pods(f.namespace).Get(context.TODO(), podName, metav1.GetOptions{})
	if err != nil {
		return "", err
	}

	path := "/var/run/mongodb/tls/client.pem"
	if certType == "ca" {
		path = "/var/run/mongodb/tls/ca.crt"
	}

	command := []string{"cat", path}
	return exec.ExecIntoPod(f.restConfig, pod, exec.Command(command...))
}

func (f *Framework) GetCertificate(db *api.MongoDB, certType string) (*x509.Certificate, error) {
	certContent, err := f.getCertificateFromShell(db, certType)
	if err != nil {
		return nil, err
	}

	blk, _ := pem.Decode([]byte(certContent))

	return x509.ParseCertificate(blk.Bytes)
}

func (f *Framework) EventuallyVolumeExpandedMongoDB(db *api.MongoDB, storage *v1alpha1.MongoDBStorageAutoscalerSpec) GomegaAsyncAssertion {
	return Eventually(
		func() (bool, error) {
			size, err := f.getDiskSizeMongoDB(db, storage)
			if err != nil {
				return false, err
			}
			if size > 1048576 { //1GB
				return true, nil
			}
			return false, nil
		},
		time.Minute*10,
		time.Second*5,
	)
}

func (f *Framework) getCurrentCPUMongoDB(prevDB *api.MongoDB, compute *v1alpha1.MongoDBComputeAutoscalerSpec) (bool, error) {
	db, err := f.dbClient.KubedbV1alpha2().MongoDBs(prevDB.Namespace).Get(context.TODO(), prevDB.Name, metav1.GetOptions{})
	if err != nil {
		return false, err
	}
	if compute.ReplicaSet != nil || compute.Standalone != nil {
		return db.Spec.PodTemplate.Spec.Resources.Requests.Cpu().MilliValue() > prevDB.Spec.PodTemplate.Spec.Resources.Requests.Cpu().MilliValue(), nil
	} else if compute.Shard != nil {
		return db.Spec.ShardTopology.Shard.PodTemplate.Spec.Resources.Requests.Cpu().MilliValue() > prevDB.Spec.ShardTopology.Shard.PodTemplate.Spec.Resources.Requests.Cpu().MilliValue(), nil
	} else if compute.ConfigServer != nil {
		return db.Spec.ShardTopology.ConfigServer.PodTemplate.Spec.Resources.Requests.Cpu().MilliValue() > prevDB.Spec.ShardTopology.ConfigServer.PodTemplate.Spec.Resources.Requests.Cpu().MilliValue(), nil
	} else {
		return db.Spec.ShardTopology.Mongos.PodTemplate.Spec.Resources.Requests.Cpu().MilliValue() > prevDB.Spec.ShardTopology.Mongos.PodTemplate.Spec.Resources.Requests.Cpu().MilliValue(), nil
	}
}

func (f *Framework) EventuallyVerticallyScaledMongoDB(meta metav1.ObjectMeta, compute *v1alpha1.MongoDBComputeAutoscalerSpec) GomegaAsyncAssertion {
	db, err := f.dbClient.KubedbV1alpha2().MongoDBs(meta.Namespace).Get(context.TODO(), meta.Name, metav1.GetOptions{})
	Expect(err).NotTo(HaveOccurred())

	return Eventually(
		func() (bool, error) {
			return f.getCurrentCPUMongoDB(db, compute)
		},
		time.Minute*30,
		time.Second*5,
	)
}

func (f *Framework) EventuallyDatabaseResumed(db *api.MongoDB) GomegaAsyncAssertion {
	return Eventually(
		func() (bool, error) {
			db, err := f.dbClient.KubedbV1alpha2().MongoDBs(db.Namespace).Get(context.TODO(), db.Name, metav1.GetOptions{})
			if err != nil {
				return false, err
			}

			if v1.IsConditionTrue(db.Status.Conditions, api.DatabasePaused) {
				return false, nil
			}
			return true, nil
		},
		time.Minute*5,
		time.Second*5,
	)
}

func (f *Framework) EventuallyTLSUserCreated(db *api.MongoDB) GomegaAsyncAssertion {
	return Eventually(
		func() (bool, error) {
			svcName := f.GetPrimaryService(db.ObjectMeta)
			client, tunnel, err := f.ConnectAndPing(db.ObjectMeta, string(core.ResourceServices), svcName, true)
			if err != nil {
				return false, err
			}
			defer tunnel.Close()

			res := make(map[string]interface{})
			err = client.Database("$external").RunCommand(context.Background(), bson.D{{Key: "usersInfo", Value: "CN=root,OU=client,O=kubedb"}}).Decode(&res)
			if err != nil {
				klog.Error("Failed to run command. error: ", err)
				return false, err
			}
			users, ok := res["users"].(primitive.A)
			if ok && len(users) == 0 {
				return false, nil
			}

			return true, nil
		},
		time.Minute*5,
		time.Second*5,
	)
}
