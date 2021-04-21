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
	"strconv"
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

type MariaDBInfo struct {
	DatabaseName string
	User         string
	Param        string
}

type kubedbTable struct {
	Id   int64  `xorm:"pk autoincr"`
	Name string `xorm:"varchar(25) not null 'usr_name' comment('NickName')"`
}

func (fi *Invocation) GetMariaDBClient(meta metav1.ObjectMeta, tunnel *portforward.Tunnel, dbInfo MariaDBInfo) (*xorm.Engine, error) {
	md, err := fi.GetMariaDB(meta)
	if err != nil {
		return nil, err
	}

	pass, err := fi.GetMariaDBRootPassword(md)
	if err != nil {
		return nil, err
	}

	if SslEnabledMariaDB(md) {
		serverSecret, err := fi.kubeClient.CoreV1().Secrets(fi.Namespace()).Get(context.TODO(), md.GetCertSecretName(api.MariaDBServerCert), metav1.GetOptions{})
		if err != nil {
			return nil, err
		}
		cacrt := serverSecret.Data["ca.crt"]
		certPool := x509.NewCertPool()
		certPool.AppendCertsFromPEM(cacrt)
		// get client-secret
		clientSecret, err := fi.kubeClient.CoreV1().Secrets(fi.Namespace()).Get(context.TODO(), md.GetCertSecretName(api.MariaDBArchiverCert), metav1.GetOptions{})
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

func (fi *Invocation) ForwardPortMD(meta metav1.ObjectMeta) (*portforward.Tunnel, error) {
	db, err := fi.GetMariaDB(meta)
	if err != nil {
		return nil, err
	}
	tunnel := portforward.NewTunnel(portforward.TunnelOptions{
		Client:    fi.kubeClient.CoreV1().RESTClient(),
		Config:    fi.restConfig,
		Resource:  string(core.ResourceServices),
		Namespace: meta.Namespace,
		Name:      db.ServiceName(),
		Remote:    3306,
	})

	if err := tunnel.ForwardPort(); err != nil {
		return nil, err
	}
	return tunnel, nil
}

func (fi *Invocation) EventuallyCreateUserWithRequiredSSLMD(meta metav1.ObjectMeta, dbInfo MariaDBInfo) GomegaAsyncAssertion {
	sql := fmt.Sprintf("CREATE USER '%s'@'%s' IDENTIFIED BY '%s' REQUIRE SSL;", MySQLRequiredSSLUser, "%", MySQLRequiredSSLPassword)
	privilege := fmt.Sprintf("GRANT ALL ON *.* TO '%s'@'%s';", MySQLRequiredSSLUser, "%")
	flush := "FLUSH PRIVILEGES;"
	return Eventually(
		func() bool {
			tunnel, err := fi.ForwardPortMD(meta)
			if err != nil {
				return false
			}
			defer tunnel.Close()

			en, err := fi.GetMariaDBClient(meta, tunnel, dbInfo)
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

func (fi *Invocation) EventuallyCheckSSLSettingsMD(meta metav1.ObjectMeta, dbInfo MariaDBInfo, config string) GomegaAsyncAssertion {
	sslConfigVarPair := strings.Split(config, "=")
	sql := fmt.Sprintf("SHOW VARIABLES LIKE '%s';", sslConfigVarPair[0])
	return Eventually(
		func() []map[string][]byte {
			tunnel, err := fi.ForwardPortMD(meta)
			if err != nil {
				return nil
			}
			defer tunnel.Close()

			en, err := fi.GetMariaDBClient(meta, tunnel, dbInfo)
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

func (fi *Invocation) EventuallyDBConnectionMD(meta metav1.ObjectMeta, dbInfo MariaDBInfo) GomegaAsyncAssertion {
	return Eventually(
		func() bool {
			tunnel, err := fi.ForwardPortMD(meta)
			if err != nil {
				return false
			}
			defer tunnel.Close()

			en, err := fi.GetMariaDBClient(meta, tunnel, dbInfo)
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

func (fi *Invocation) EventuallyCreateTableMD(meta metav1.ObjectMeta, dbInfo MariaDBInfo) GomegaAsyncAssertion {
	return Eventually(
		func() bool {
			tunnel, err := fi.ForwardPortMD(meta)
			if err != nil {
				return false
			}
			defer tunnel.Close()

			en, err := fi.GetMariaDBClient(meta, tunnel, dbInfo)
			if err != nil {
				return false
			}
			defer en.Close()

			if err := en.Ping(); err != nil {
				return false
			}
			return en.Charset("utf8mb4").StoreEngine("InnoDB").Sync2(new(kubedbTable)) == nil
		},
		Timeout,
		RetryInterval,
	)
}

func (fi *Invocation) EventuallyDropDatabaseMD(meta metav1.ObjectMeta, dbInfo MariaDBInfo) GomegaAsyncAssertion {
	queries := []string{
		fmt.Sprintf("DROP DATABASE %s;", dbInfo.DatabaseName),
	}
	return Eventually(
		func() bool {
			tunnel, err := fi.ForwardPortMD(meta)
			if err != nil {
				return false
			}
			defer tunnel.Close()

			en, err := fi.GetMariaDBClient(meta, tunnel, dbInfo)
			if err != nil {
				return false
			}
			defer en.Close()

			if err := en.Ping(); err != nil {
				return false
			}
			for _, query := range queries {
				if _, err = en.Query(query); err != nil {
					return false
				}
			}
			return true
		},
		Timeout,
		RetryInterval,
	)
}

func (fi *Invocation) EventuallyCreateTestDBMD(meta metav1.ObjectMeta, dbInfo MariaDBInfo) GomegaAsyncAssertion {
	queries := []string{
		fmt.Sprintf("CREATE DATABASE %s;", dbInfo.DatabaseName),
	}
	dbInfo.DatabaseName = DBMySQL
	return Eventually(
		func() bool {
			tunnel, err := fi.ForwardPortMD(meta)
			if err != nil {
				return false
			}
			defer tunnel.Close()

			en, err := fi.GetMariaDBClient(meta, tunnel, dbInfo)

			if err != nil {
				return false
			}

			if err := en.Ping(); err != nil {
				return false
			}
			for _, query := range queries {
				if _, err := en.Query(query); err != nil {
					return false
				}
			}
			return true
		},
		Timeout,
		RetryInterval,
	)
}

func (fi *Invocation) EventuallyExistsDBMD(meta metav1.ObjectMeta, dbInfo MariaDBInfo) GomegaAsyncAssertion {
	getDatabasesQuery := `SHOW DATABASES;`
	return Eventually(
		func() int {
			tunnel, err := fi.ForwardPortMD(meta)
			if err != nil {
				return -1
			}
			defer tunnel.Close()
			en, err := fi.GetMariaDBClient(meta, tunnel, MariaDBInfo{
				DatabaseName: DBMySQL,
				User:         dbInfo.User,
				Param:        dbInfo.Param,
			})
			if err != nil {
				return -1
			}
			defer en.Close()

			if err := en.Ping(); err != nil {
				return -1
			}
			result, err := en.Query(getDatabasesQuery)
			if err != nil {
				return -1
			}
			for _, value := range result {
				if strings.Compare(string(value["Database"]), dbInfo.DatabaseName) == 0 {
					return 1
				}
			}
			return 0
		},
		Timeout,
		RetryInterval,
	)
}

func (fi *Invocation) EventuallyInsertRowMD(meta metav1.ObjectMeta, dbInfo MariaDBInfo, total int) GomegaAsyncAssertion {
	count := 0
	return Eventually(
		func() bool {
			tunnel, err := fi.ForwardPortMD(meta)
			if err != nil {
				fmt.Println(err)
				return false
			}
			defer tunnel.Close()

			en, err := fi.GetMariaDBClient(meta, tunnel, dbInfo)
			if err != nil {
				fmt.Println(err)
				return false
			}
			defer en.Close()

			if err := en.Ping(); err != nil {
				fmt.Println(err)
				return false
			}

			for i := count; i < total; i++ {
				if _, err := en.Insert(&kubedbTable{
					Name: fmt.Sprintf("KubedbName-%v", i),
				}); err != nil {
					fmt.Println(err)
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

func (fi *Invocation) EventuallyCountRowMD(meta metav1.ObjectMeta, dbInfo MariaDBInfo) GomegaAsyncAssertion {
	return Eventually(
		func() int {
			tunnel, err := fi.ForwardPortMD(meta)
			if err != nil {
				return -1
			}
			defer tunnel.Close()

			en, err := fi.GetMariaDBClient(meta, tunnel, dbInfo)
			if err != nil {
				return -1
			}
			defer en.Close()

			if err := en.Ping(); err != nil {
				return -1
			}

			kubedb := new(kubedbTable)
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

func (fi *Invocation) EventuallyMariaDBVariable(meta metav1.ObjectMeta, dbInfo MariaDBInfo, config string) GomegaAsyncAssertion {
	configPair := strings.Split(config, "=")
	sql := fmt.Sprintf("SHOW VARIABLES LIKE '%s';", configPair[0])
	return Eventually(
		func() []map[string][]byte {
			tunnel, err := fi.ForwardPortMD(meta)
			if err != nil {
				return nil
			}
			defer tunnel.Close()

			en, err := fi.GetMariaDBClient(meta, tunnel, dbInfo)
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

func (fi *Invocation) EventuallyDatabaseVersionUpdatedMD(meta metav1.ObjectMeta, dbInfo MariaDBInfo, targetedVersion string) GomegaAsyncAssertion {
	query := `SHOW VARIABLES LIKE "version";`
	return Eventually(
		func() bool {
			tunnel, err := fi.ForwardPortMD(meta)
			if err != nil {
				return false
			}
			defer tunnel.Close()

			en, err := fi.GetMariaDBClient(meta, tunnel, dbInfo)
			if err != nil {
				return false
			}
			defer en.Close()

			if err := en.Ping(); err != nil {
				return false
			}

			result, err := en.QueryString(query)
			if err != nil {
				return false
			}

			if strings.Contains(result[0]["Value"], targetedVersion) {
				return true
			}
			return false
		},
		Timeout,
		RetryInterval,
	)
}

func (fi *Invocation) EventuallyONLINEMembersCountMD(meta metav1.ObjectMeta, dbInfo MariaDBInfo) GomegaAsyncAssertion {
	query := `show status like "wsrep_cluster_size";`
	return Eventually(
		func() int {
			tunnel, err := fi.ForwardPortMD(meta)
			if err != nil {
				return -1
			}
			defer tunnel.Close()

			en, err := fi.GetMariaDBClient(meta, tunnel, dbInfo)
			if err != nil {
				return -1
			}
			defer en.Close()

			if err := en.Ping(); err != nil {
				return -1
			}

			result, err := en.QueryString(query)
			if err != nil {
				return -1
			}

			clusterSize, err := strconv.Atoi(result[0]["Value"])
			if err != nil {
				return -1
			}
			return clusterSize
		},
		Timeout,
		RetryInterval,
	)
}

func GetMariaDBInfo(dbName string, dbUser string, dbParam string) MariaDBInfo {
	return MariaDBInfo{
		DatabaseName: dbName,
		User:         dbUser,
		Param:        dbParam,
	}
}
