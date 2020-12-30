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
	"crypto/x509"
	"fmt"
	"strings"
	"time"

	api "kubedb.dev/apimachinery/apis/kubedb/v1alpha2"

	_ "github.com/go-sql-driver/mysql"
	sql_driver "github.com/go-sql-driver/mysql"
	. "github.com/onsi/gomega"
	core "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"kmodules.xyz/client-go/tools/portforward"
	"xorm.io/xorm"
)

const (
	TLSSkibVerify              = "skip-verify"
	TLSTrue                    = "true"
	TLSFalse                   = "false"
	RequiredSecureTransportON  = "ON"
	RequiredSecureTransportOFF = "OFF"
	TLSCustomConfig            = "custom"
	DBMySQL                    = "mysql"
	MySQLRootUser              = "root"
	MySQLRequiredSSLUser       = "ssl-User"
	MySQLRequiredSSLPassword   = "not@secret"
)

//type KubedbMySQLTable struct {
//	Id   int64
//	Name string `xorm:"varchar(25) not null unique 'usr_name' comment('NickName')"`
//}

type DatabaseConnectionInfo struct {
	StatefulSetOrdinal int
	ClientPodIndex     int
	DatabaseName       string
	User               string
	Param              string
}

func (f *Framework) forwardPort(meta metav1.ObjectMeta, stsOrdinal, clientPodIndex int) (*portforward.Tunnel, error) {
	var clientPodName string
	if stsOrdinal == 0 {
		clientPodName = fmt.Sprintf("%v-%d", meta.Name, clientPodIndex)
	} else {
		clientPodName = fmt.Sprintf("%v-%d-%d", meta.Name, stsOrdinal, clientPodIndex)
	}

	tunnel := portforward.NewTunnel(
		portforward.TunnelOptions{
			Client:    f.kubeClient.CoreV1().RESTClient(),
			Config:    f.restConfig,
			Resource:  string(core.ResourcePods),
			Namespace: meta.Namespace,
			Name:      clientPodName,
			Remote:    3306,
		})

	if err := tunnel.ForwardPort(); err != nil {
		return nil, err
	}
	return tunnel, nil
}

func (f *Framework) getMySQLClient(meta metav1.ObjectMeta, tunnel *portforward.Tunnel, dbInfo DatabaseConnectionInfo) (*xorm.Engine, error) {
	mysql, err := f.GetMySQL(meta)
	if err != nil {
		return nil, err
	}

	pass, err := f.GetMySQLRootPassword(mysql)
	if err != nil {
		return nil, err
	}

	if strings.Contains(dbInfo.Param, "tls") {
		serverSecret, err := f.kubeClient.CoreV1().Secrets(f.Namespace()).Get(context.TODO(), mysql.MustCertSecretName(api.MySQLServerCert), metav1.GetOptions{})
		if err != nil {
			return nil, err
		}
		cacrt := serverSecret.Data["ca.crt"]
		certPool := x509.NewCertPool()
		certPool.AppendCertsFromPEM(cacrt)
		// get client-secret
		clientSecret, err := f.kubeClient.CoreV1().Secrets(f.Namespace()).Get(context.TODO(), mysql.MustCertSecretName(api.MySQLClientCert), metav1.GetOptions{})
		if err != nil {
			return nil, err
		}
		ccrt := clientSecret.Data["tls.crt"]
		ckey := clientSecret.Data["tls.key"]
		cert, err := tls.X509KeyPair(ccrt, ckey)
		if err != nil {
			return nil, err
		}
		var clientCert []tls.Certificate
		clientCert = append(clientCert, cert)
		// tls custom setup
		if dbInfo.User == MySQLRootUser {
			err = sql_driver.RegisterTLSConfig(TLSCustomConfig, &tls.Config{
				RootCAs: certPool,
			})
			if err != nil {
				return nil, err
			}
		}

		if dbInfo.User == MySQLRequiredSSLUser {
			pass = MySQLRequiredSSLPassword
			// tls custom setup
			err = sql_driver.RegisterTLSConfig(TLSCustomConfig, &tls.Config{
				RootCAs:      certPool,
				Certificates: clientCert,
			})
			if err != nil {
				return nil, err
			}
		}
	}

	cnnstr := fmt.Sprintf("%v:%v@tcp(127.0.0.1:%v)/%s?%s", dbInfo.User, pass, tunnel.Local, dbInfo.DatabaseName, dbInfo.Param)
	en, err := xorm.NewEngine("mysql", cnnstr)
	en.ShowSQL(true)
	return en, err

}

func (f *Framework) EventuallyCreateUserWithRequiredSSL(meta metav1.ObjectMeta, dbInfo DatabaseConnectionInfo) GomegaAsyncAssertion {
	sql := fmt.Sprintf("CREATE USER '%s'@'%s' IDENTIFIED BY '%s' REQUIRE SSL;", MySQLRequiredSSLUser, "%", MySQLRequiredSSLPassword)
	privilege := fmt.Sprintf("GRANT ALL ON mysql.* TO '%s'@'%s';", MySQLRequiredSSLUser, "%")
	flush := "FLUSH PRIVILEGES;"
	return Eventually(
		func() bool {
			tunnel, err := f.forwardPort(meta, dbInfo.StatefulSetOrdinal, dbInfo.ClientPodIndex)
			if err != nil {
				return false
			}
			defer tunnel.Close()

			en, err := f.getMySQLClient(meta, tunnel, dbInfo)
			if err != nil {
				return false
			}
			defer en.Close()

			if err := en.Ping(); err != nil {
				return false
			}
			// create new User
			if _, err = en.Query(sql); err != nil {
				return false
			}
			// grand all permission for the new User
			if _, err = en.Query(privilege); err != nil {
				return false
			}

			// flush privilege
			if _, err = en.Query(flush); err != nil {
				return false
			}

			return true
		},
		Timeout,
		RetryInterval,
	)
}

func (f *Framework) EventuallyCheckSSLSettings(meta metav1.ObjectMeta, dbInfo DatabaseConnectionInfo, config string) GomegaAsyncAssertion {
	sslConfigVarPair := strings.Split(config, "=")
	sql := fmt.Sprintf("SHOW VARIABLES LIKE '%s';", sslConfigVarPair[0])
	return Eventually(
		func() []map[string][]byte {
			tunnel, err := f.forwardPort(meta, dbInfo.StatefulSetOrdinal, dbInfo.ClientPodIndex)
			if err != nil {
				return nil
			}
			defer tunnel.Close()

			en, err := f.getMySQLClient(meta, tunnel, dbInfo)
			if err != nil {
				return nil
			}
			defer en.Close()

			results, err := en.Query(sql)
			if err != nil {
				return nil
			}
			return results
		},
		Timeout,
		RetryInterval,
	)
}

func (f *Framework) EventuallyDBConnection(meta metav1.ObjectMeta, dbInfo DatabaseConnectionInfo) GomegaAsyncAssertion {
	return Eventually(
		func() bool {
			tunnel, err := f.forwardPort(meta, dbInfo.StatefulSetOrdinal, dbInfo.ClientPodIndex)
			if err != nil {
				return false
			}
			defer tunnel.Close()

			en, err := f.getMySQLClient(meta, tunnel, dbInfo)
			if err != nil {
				return false
			}
			defer en.Close()

			if err = en.Ping(); err != nil {
				return false
			}
			return true
		},
		Timeout,
		RetryInterval,
	)
}

func (f *Framework) EventuallyCreateTable(meta metav1.ObjectMeta, dbInfo DatabaseConnectionInfo) GomegaAsyncAssertion {
	return Eventually(
		func() bool {
			tunnel, err := f.forwardPort(meta, dbInfo.StatefulSetOrdinal, dbInfo.ClientPodIndex)
			if err != nil {
				return false
			}
			defer tunnel.Close()

			en, err := f.getMySQLClient(meta, tunnel, dbInfo)
			if err != nil {
				return false
			}
			defer en.Close()

			if err := en.Ping(); err != nil {
				return false
			}
			return en.Charset("utf8mb4").StoreEngine("InnoDB").Sync2(new(KubedbTable)) == nil
		},
		Timeout,
		RetryInterval,
	)
}

func (f *Framework) EventuallyInsertRow(meta metav1.ObjectMeta, dbInfo DatabaseConnectionInfo, total int) GomegaAsyncAssertion {
	count := 0
	return Eventually(
		func() bool {
			tunnel, err := f.forwardPort(meta, dbInfo.StatefulSetOrdinal, dbInfo.ClientPodIndex)
			if err != nil {
				return false
			}
			defer tunnel.Close()

			en, err := f.getMySQLClient(meta, tunnel, dbInfo)
			if err != nil {
				return false
			}
			defer en.Close()

			if err := en.Ping(); err != nil {
				return false
			}

			for i := count; i < total; i++ {
				if _, err := en.Insert(&KubedbTable{
					//Id:   int64(i),
					Name: fmt.Sprintf("KubedbName-%v", i),
				}); err != nil {
					return false
				}
				count++
			}
			return true
		},
		Timeout,
		RetryInterval,
	)
}

func (f *Framework) EventuallyCountRow(meta metav1.ObjectMeta, dbInfo DatabaseConnectionInfo) GomegaAsyncAssertion {
	return Eventually(
		func() int {
			tunnel, err := f.forwardPort(meta, dbInfo.StatefulSetOrdinal, dbInfo.ClientPodIndex)
			if err != nil {
				return -1
			}
			defer tunnel.Close()

			en, err := f.getMySQLClient(meta, tunnel, dbInfo)
			if err != nil {
				return -1
			}
			defer en.Close()

			if err := en.Ping(); err != nil {
				return -1
			}

			kubedb := new(KubedbTable)
			total, err := en.Count(kubedb)
			if err != nil {
				return -1
			}
			return int(total)
		},
		Timeout,
		RetryInterval,
	)
}

func (f *Framework) EventuallyONLINEMembersCount(meta metav1.ObjectMeta, dbInfo DatabaseConnectionInfo) GomegaAsyncAssertion {
	return Eventually(
		func() int {
			tunnel, err := f.forwardPort(meta, dbInfo.StatefulSetOrdinal, dbInfo.ClientPodIndex)
			if err != nil {
				return -1
			}
			defer tunnel.Close()

			en, err := f.getMySQLClient(meta, tunnel, dbInfo)
			if err != nil {
				return -1
			}
			defer en.Close()

			if err := en.Ping(); err != nil {
				return -1
			}

			var cnt int
			_, err = en.SQL("select count(MEMBER_STATE) from performance_schema.replication_group_members where MEMBER_STATE = ?", "ONLINE").Get(&cnt)
			if err != nil {
				return -1
			}
			return cnt
		},
		Timeout,
		RetryInterval,
	)
}

func (f *Framework) EventuallyDatabaseVersionUpdated(meta metav1.ObjectMeta, dbInfo DatabaseConnectionInfo, targetedVersion string) GomegaAsyncAssertion {
	query := `SHOW VARIABLES LIKE "version";`
	return Eventually(
		func() bool {
			tunnel, err := f.forwardPort(meta, dbInfo.StatefulSetOrdinal, dbInfo.ClientPodIndex)
			if err != nil {
				return false
			}
			defer tunnel.Close()

			en, err := f.getMySQLClient(meta, tunnel, dbInfo)
			if err != nil {
				return false
			}
			defer en.Close()

			if err := en.Ping(); err != nil {
				return false
			}

			r, err := en.QueryString(query)
			if err != nil {
				return false
			}

			if strings.Contains(string(r[0]["Value"]), targetedVersion) {
				return true
			}
			return false
		},
		Timeout,
		RetryInterval,
	)
}

func (f *Framework) EventuallyMySQLVariable(meta metav1.ObjectMeta, dbInfo DatabaseConnectionInfo, config string) GomegaAsyncAssertion {
	configPair := strings.Split(config, "=")
	sql := fmt.Sprintf("SHOW VARIABLES LIKE '%s';", configPair[0])
	return Eventually(
		func() []map[string][]byte {
			tunnel, err := f.forwardPort(meta, dbInfo.StatefulSetOrdinal, dbInfo.ClientPodIndex)
			if err != nil {
				return nil
			}
			defer tunnel.Close()

			en, err := f.getMySQLClient(meta, tunnel, dbInfo)
			if err != nil {
				return nil
			}
			defer en.Close()

			if err := en.Ping(); err != nil {
				return nil
			}

			results, err := en.Query(sql)
			if err != nil {
				return nil
			}
			return results
		},
		time.Minute*5,
		time.Second*5,
	)
}
