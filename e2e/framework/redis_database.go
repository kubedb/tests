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
	"fmt"
	api "kubedb.dev/apimachinery/apis/kubedb/v1alpha1"
	"strings"
	"time"

	. "github.com/onsi/gomega"
)

//func (f *Framework) GetDatabasePod(meta metav1.ObjectMeta) *core.Pod {
//	var pod *core.Pod
//	var err error
//
//	Eventually(func() error {
//		pod, err = f.kubeClient.CoreV1().Pods(meta.Namespace).Get(context.TODO(), meta.Name+"-0", metav1.GetOptions{})
//		return err
//	}).Should(BeNil())
//
//	return pod
//}

//func (f *Framework) GetRedisClient(meta metav1.ObjectMeta) (*redis.Client, error) {
//	pod := f.GetDatabasePod(meta)
//
//	f.tunnel = portforward.NewTunnel(
//		f.kubeClient.CoreV1().RESTClient(),
//		f.restConfig,
//		meta.Namespace,
//		pod.Name,
//		6379,
//	)
//
//	if err := f.tunnel.ForwardPort(); err != nil {
//		return nil, err
//	}
//
//	return redis.NewClient(&redis.Options{
//		Addr:     fmt.Sprintf("localhost:%v", f.tunnel.Local),
//		Password: "", // no password set
//		DB:       0,  // use default DB
//	}), nil
//}

func (f *Framework) EventuallyRedisConfig(redis *api.Redis, config string) GomegaAsyncAssertion {
	configPair := strings.Split(config, " ")

	return Eventually(
		func() string {

			//client, err := f.GetRedisClient(meta)
			//Expect(err).NotTo(HaveOccurred())
			//
			//defer f.tunnel.Close()

			// ping database to check if it is ready
			pong, err := f.testConfig.GetPingResult(redis)
			if err != nil {
				return ""
			}

			if !strings.Contains(pong, "PONG") {
				return ""
			}

			// get configuration
			//response := client.ConfigGet(configPair[0])
			//result := response.Val()
			//ret := make([]string, 0)
			//for _, r := range result {
			//	ret = append(ret, r.(string))
			//}
			result, err := f.testConfig.GetRedisConfig(redis, configPair[0])
			if err != nil {
				return ""
			}
			return strings.Join(result, " ")
		},
		time.Minute*5,
		time.Second*5,
	)
}

func (f *Framework) EventuallySetItem(redis *api.Redis, key, value string) GomegaAsyncAssertion {
	return Eventually(
		func() bool {
			//client, err := f.GetRedisClient(meta)
			//Expect(err).NotTo(HaveOccurred())
			//
			//defer f.tunnel.Close()
			//
			//cmd := client.Set(key, value, 0)
			//return cmd.Err() == nil

			_, err := f.testConfig.SetItem(redis, key, value)
			fmt.Println("redis_database.go:line-112   ", err)
		    return err == nil
		},
		time.Minute*5,
		time.Second*5,
	)
}

func (f *Framework) EventuallyGetItem(redis *api.Redis, key string) GomegaAsyncAssertion {
	return Eventually(
		func() string {
			//client, err := f.GetRedisClient(meta)
			//Expect(err).NotTo(HaveOccurred())
			//
			//defer f.tunnel.Close()
			//
			//cmd := client.Get(key)
			//val, err := cmd.Result()
			//if err != nil {
			//	fmt.Printf("got error while looking for key-value %v:%v. Error: %v. Full response: %v\n", key, val, err, cmd)
			//	return ""
			//}
			//return string(val)
			res, err := f.testConfig.GetItem(redis, key)
			if err != nil {
				return ""
			}
			return res
		},
		time.Minute*5,
		time.Second*5,
	)
}
