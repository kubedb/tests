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
	"strings"
	"time"

	api "kubedb.dev/apimachinery/apis/kubedb/v1alpha2"

	"github.com/aws/aws-sdk-go/aws"
	promClient "github.com/prometheus/client_model/go"
	"github.com/prometheus/prom2json"
	core "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/util/wait"
	"k8s.io/klog/v2"
	kutil "kmodules.xyz/client-go"
	mona "kmodules.xyz/monitoring-agent-api/api/v1"
)

const (
	mysqlUpMetric            = "mysql_up"
	mySQLMetricsMatchedCount = 2
	mysqlVersionMetric       = "mysql_version_info"
)

func (f *Framework) AddMySQLMonitor(obj *api.MySQL) {
	obj.Spec.Monitor = &mona.AgentSpec{
		Prometheus: &mona.PrometheusSpec{
			Exporter: mona.PrometheusExporterSpec{
				Port:            mona.PrometheusExporterPortNumber,
				Resources:       core.ResourceRequirements{},
				SecurityContext: nil,
			},
		},
		Agent: mona.AgentPrometheus,
	}
}

//VerifyMySQLExporter uses metrics from given URL
//and check against known key and value
//to verify the connection is functioning as intended
func (f *Framework) VerifyMySQLExporter(my *api.MySQL) error {
	tunnel, err := f.ForwardToPort(my.ObjectMeta, string(core.ResourceServices), my.StatsService().ServiceName(), aws.Int(mona.PrometheusExporterPortNumber))
	if err != nil {
		klog.Infoln(err)
		return err
	}

	return wait.PollImmediate(time.Second, kutil.ReadinessTimeout, func() (bool, error) {
		metricsURL := fmt.Sprintf("http://127.0.0.1:%d/metrics", tunnel.Local)
		mfChan := make(chan *promClient.MetricFamily, 1024)
		transport := makeTransport()

		err := prom2json.FetchMetricFamilies(metricsURL, mfChan, transport)
		if err != nil {
			klog.Infoln(err)
			return false, nil
		}

		var count = 0
		for mf := range mfChan {
			if mf.Metric != nil && mf.Metric[0].Gauge != nil && mf.Metric[0].Gauge.Value != nil {
				if *mf.Name == mysqlVersionMetric && strings.Contains(my.Spec.Version, *mf.Metric[0].Label[0].Value) {
					count++
				} else if *mf.Name == mysqlUpMetric && int(*mf.Metric[0].Gauge.Value) > 0 {
					count++
				}
			}
		}

		if count != mySQLMetricsMatchedCount {
			return false, nil
		}
		klog.Infoln("Found ", count, " metrics out of ", mySQLMetricsMatchedCount)
		return true, nil
	})
}
