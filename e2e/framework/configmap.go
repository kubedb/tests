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
	"strings"

	"github.com/appscode/go/crypto/rand"
	shell "github.com/codeskyblue/go-sh"
	. "github.com/onsi/ginkgo"
	core "k8s.io/api/core/v1"
	kerr "k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	meta_util "kmodules.xyz/client-go/meta"
)

func (f *Framework) ConfigMapForMyInitScript() (*core.ConfigMap, error) {
	sh := shell.NewSession()

	var execOut bytes.Buffer
	sh.Stdout = &execOut
	if err := sh.Command("which", "curl").Run(); err != nil {
		return nil, err
	}

	curlLoc := strings.TrimSpace(execOut.String())
	execOut.Reset()

	sh.ShowCMD = true
	if err := sh.Command(curlLoc, "-fsSL", "https://github.com/kubedb/mysql-init-scripts/raw/master/init.sql").Run(); err != nil {
		return nil, err
	}

	return &core.ConfigMap{
		ObjectMeta: metav1.ObjectMeta{
			Name:      rand.WithUniqSuffix("my-init-script"),
			Namespace: f.namespace,
		},
		Data: map[string]string{
			"init.sql": execOut.String(),
		},
	}, nil
}

func (fi *Invocation) ConfigMapForInitialization() *core.ConfigMap {
	return &core.ConfigMap{
		ObjectMeta: metav1.ObjectMeta{
			Name:      rand.WithUniqSuffix(fi.app + "-local"),
			Namespace: fi.namespace,
		},
		Data: map[string]string{
			"init.js": `db = db.getSiblingDB('kubedb');
db.people.insert({"firstname" : "kubernetes", "lastname" : "database" }); `,
			"init-in-existing-db.js": `db.people.insert({"firstname" : "kubernetes", "lastname" : "database" });`,
		},
	}
}

func (fi *Invocation) GetCustomConfig(configs []string, name string) *core.Secret {
	configs = append([]string{"net:"}, configs...)
	return &core.Secret{
		ObjectMeta: metav1.ObjectMeta{
			Name:      name,
			Namespace: fi.namespace,
		},
		StringData: map[string]string{
			"mongod.conf": strings.Join(configs, "\n"),
		},
	}
}

func (fi *Invocation) GetCustomConfigForMySQL(configs []string) *core.Secret {
	configs = append([]string{"[mysqld]"}, configs...)
	return &core.Secret{
		ObjectMeta: metav1.ObjectMeta{
			Name:      fi.app,
			Namespace: fi.namespace,
		},
		StringData: map[string]string{
			"my-custom.cnf": strings.Join(configs, "\n"),
		},
	}
}

func (fi *Invocation) GetCustomConfigForMariaDB(configs []string) *core.Secret {
	configs = append([]string{"[mysqld]"}, configs...)
	return &core.Secret{
		ObjectMeta: metav1.ObjectMeta{
			Name:      fi.app,
			Namespace: fi.namespace,
		},
		StringData: map[string]string{
			"md-custom.cnf": strings.Join(configs, "\n"),
		},
	}
}

func (fi *Invocation) CreateConfigMap(obj *core.ConfigMap) error {
	_, err := fi.kubeClient.CoreV1().ConfigMaps(obj.Namespace).Create(context.TODO(), obj, metav1.CreateOptions{})
	return err
}

func (fi *Invocation) GetConfigMap(objMeta metav1.ObjectMeta) (*core.ConfigMap, error) {
	return fi.kubeClient.CoreV1().ConfigMaps(objMeta.Namespace).Get(context.TODO(), objMeta.Name, metav1.GetOptions{})
}

func (f *Framework) DeleteConfigMap(meta metav1.ObjectMeta) error {
	err := f.kubeClient.CoreV1().ConfigMaps(meta.Namespace).Delete(context.TODO(), meta.Name, meta_util.DeleteInForeground())
	if !kerr.IsNotFound(err) {
		return err
	}
	return nil
}

func (fi *Invocation) EnsureMyInitScriptConfigMap() (*core.ConfigMap, error) {
	cm, err := fi.ConfigMapForMyInitScript()
	if err != nil {
		return nil, err
	}
	By("Create init Script ConfigMap: " + cm.Name + "\n" + cm.Data["init.sql"])
	err = fi.CreateConfigMap(cm)
	if err != nil {
		return cm, err
	}
	cm, err = fi.GetConfigMap(cm.ObjectMeta)
	fi.AppendToCleanupList(cm)

	return cm, err
}
