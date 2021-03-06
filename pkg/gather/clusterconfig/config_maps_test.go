package clusterconfig

import (
	"context"
	"encoding/json"
	"io/ioutil"
	"os"
	"testing"

	corev1 "k8s.io/api/core/v1"
	"k8s.io/klog"

	"github.com/openshift/insights-operator/pkg/record"
	"github.com/openshift/insights-operator/pkg/utils"
)

func TestConfigMapAnonymizer(t *testing.T) {
	klog.SetOutput(utils.NewTestLog(t).Writer())

	var cases = []struct {
		testName               string
		configMapName          string
		expectedAnonymizedJSON string
	}{
		{
			"ConfigMap Non PEM data",
			"openshift-install",
			`{
				"invoker":"codeReadyContainers",
				"version":"unreleased-master-2205-g2055609f95b19322ee6cfdd0bea73399297c4a3e"
			}`,
		},
		{
			"ConfigMap PEM is anonymized",
			"initial-kube-apiserver-server-ca",
			`{
				"ca-bundle.crt": "-----BEGIN CERTIFICATE-----\nANONYMIZED\n-----END CERTIFICATE-----\n-----BEGIN CERTIFICATE-----\nANONYMIZED\n-----END CERTIFICATE-----\n-----BEGIN CERTIFICATE-----\nANONYMIZED\n-----END CERTIFICATE-----\n-----BEGIN CERTIFICATE-----\nANONYMIZED\n-----END CERTIFICATE-----\n-----BEGIN CERTIFICATE-----\nANONYMIZED\n-----END CERTIFICATE-----\n"
			}`,
		},
		{
			"ConfigMap BinaryData non anonymized",
			"test-binary",
			`{
				"ls": "z/rt/gcAAAEDAA=="
			}`,
		},
	}

	for _, tt := range cases {
		t.Run(tt.testName, func(t *testing.T) {
			f, err := os.Open("testdata/configmaps.json")
			mustNotFail(t, err, "error opening test data file. %+v")
			defer f.Close()
			bts, err := ioutil.ReadAll(f)
			mustNotFail(t, err, "error reading test data file. %+v")
			var cml *corev1.ConfigMapList
			mustNotFail(t, json.Unmarshal([]byte(bts), &cml), "error unmarshalling json %+v")
			cm := findMap(cml, tt.configMapName)
			mustNotFail(t, cm != nil, "haven't found a ConfigMap %+v")
			var res []byte
			cmdata := map[string]string{}
			addAnonymized := func(cmdata map[string]string, dn string, encodebase64 bool, d []byte) {
				m := record.Marshalable(ConfigMapAnonymizer{v: d, encodeBase64: encodebase64})

				res, err = m.Marshal(context.TODO())
				cmdata[dn] = string(res)
				mustNotFail(t, err, "serialization failed %+v")
			}
			for dn, dv := range cm.Data {
				addAnonymized(cmdata, dn, false, []byte(dv))
			}
			for dn, dv := range cm.BinaryData {
				addAnonymized(cmdata, dn, true, dv)
			}
			var md []byte
			md, err = json.Marshal(cmdata)
			mustNotFail(t, err, "marshaling failed %+v")
			d := map[string]string{}
			err = json.Unmarshal([]byte(tt.expectedAnonymizedJSON), &d)
			mustNotFail(t, err, "unmarshaling of expected failed %+v")
			exp, err := json.Marshal(d)
			mustNotFail(t, err, "marshaling of expected failed %+v")
			if string(exp) != string(md) {
				t.Fatalf("The test %s result is unexpected. Result: \n%s \nExpected \n%s", tt.testName, string(md), string(exp))
			}
		})
	}

}

func mustNotFail(t *testing.T, err interface{}, fmtstr string) {
	if e, ok := err.(error); ok && e != nil {
		t.Fatalf(fmtstr, e)
	}
	if e, ok := err.(bool); ok && !e {
		t.Fatalf(fmtstr, e)
	}
}

func findMap(cml *corev1.ConfigMapList, name string) *corev1.ConfigMap {
	for _, it := range cml.Items {
		if it.Name == name {
			return &it
		}
	}
	return nil
}
