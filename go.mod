module github.com/openshift/insights-operator

go 1.12

require (
	github.com/coreos/bbolt v1.3.3 // indirect
	github.com/getsentry/raven-go v0.2.1-0.20190513200303-c977f96e1095 // indirect
	github.com/grpc-ecosystem/grpc-gateway v1.11.3 // indirect
	github.com/onsi/ginkgo v1.10.2 // indirect
	github.com/openshift/api v3.9.1-0.20190718162300-9525304a0adb+incompatible
	github.com/openshift/client-go v0.0.0-20190813201236-5a5508328169
	github.com/openshift/library-go v0.0.0-20191003152030-97c62d8a2901
	github.com/openshift/oc v0.0.0-alpha.0.0.20191108214724-8c30708e5cbb
	github.com/prometheus/client_golang v1.1.0
	github.com/spf13/cobra v0.0.5
	github.com/spf13/pflag v1.0.5
	go.etcd.io/bbolt v1.3.3 // indirect
	golang.org/x/time v0.0.0-20190308202827-9d24e82272b4
	google.golang.org/appengine v1.6.1 // indirect
	k8s.io/api v0.0.0
	k8s.io/apimachinery v0.0.0
	k8s.io/cli-runtime v0.0.0
	k8s.io/client-go v11.0.0+incompatible
	k8s.io/component-base v0.0.0
	k8s.io/klog v1.0.0
	k8s.io/kubectl v0.0.0
)

replace (
	bitbucket.org/ww/goautoneg => github.com/munnerz/goautoneg v0.0.0-20190414153302-2ae31c8b6b30
	github.com/apcera/gssapi => github.com/openshift/gssapi v0.0.0-20161010215902-5fb4217df13b
	github.com/aws/aws-sdk-go => github.com/aws/aws-sdk-go v0.0.0-20190125224522-81f3829f5a9d
	github.com/containers/image => github.com/openshift/containers-image v0.0.0-20190130162827-4bc6d24282b1
	github.com/docker/distribution => github.com/openshift/docker-distribution v0.0.0-20180925154709-d4c35485a70d
	github.com/docker/docker => github.com/docker/docker v0.0.0-20180612054059-a9fbbdc8dd87
	github.com/ghodss/yaml => github.com/ghodss/yaml v0.0.0-20170327235444-0ca9ea5df545
	github.com/golang/glog => github.com/openshift/golang-glog v0.0.0-20190322123450-3c92600d7533
	github.com/onsi/ginkgo => github.com/openshift/onsi-ginkgo v0.0.0-20190125161613-53ca7dc85f60
	github.com/openshift/api => github.com/openshift/api v0.0.0-20191101174058-d428681bd3ce
	github.com/openshift/client-go => github.com/openshift/client-go v0.0.0-20191022152013-2823239d2298
	github.com/prometheus/client_golang => github.com/prometheus/client_golang v0.0.0-20181207105117-505eaef01726
	golang.org/x/time => github.com/golang/time v0.0.0-20181108054448-85acf8d2951c
	k8s.io/api => github.com/openshift/kubernetes-api v0.0.0-20191104140539-95bf9aaf1163
	k8s.io/apiextensions-apiserver => k8s.io/apiextensions-apiserver v0.0.0-20191016113550-5357c4baaf65
	k8s.io/apimachinery => github.com/openshift/kubernetes-apimachinery v0.0.0-20191104122935-b3ae09682aa5
	k8s.io/apiserver => k8s.io/apiserver v0.0.0-20191016112112-5190913f932d
	k8s.io/cli-runtime => github.com/openshift/kubernetes-cli-runtime v0.0.0-20191104123654-c7adf34ec214
	k8s.io/client-go => github.com/openshift/kubernetes-client-go v0.0.0-20191104123213-2d372fee3921
	k8s.io/cloud-provider => k8s.io/cloud-provider v0.0.0-20191016115326-20453efc2458
	k8s.io/cluster-bootstrap => k8s.io/cluster-bootstrap v0.0.0-20191016115129-c07a134afb42
	k8s.io/code-generator => k8s.io/code-generator v0.0.0-20191004115455-8e001e5d1894
	k8s.io/component-base => k8s.io/component-base v0.0.0-20191016111319-039242c015a9
	k8s.io/cri-api => k8s.io/cri-api v0.0.0-20190828162817-608eb1dad4ac
	k8s.io/csi-translation-lib => k8s.io/csi-translation-lib v0.0.0-20191016115521-756ffa5af0bd
	k8s.io/kube-aggregator => k8s.io/kube-aggregator v0.0.0-20191016112429-9587704a8ad4
	k8s.io/kube-controller-manager => k8s.io/kube-controller-manager v0.0.0-20191016114939-2b2b218dc1df
	k8s.io/kube-proxy => k8s.io/kube-proxy v0.0.0-20191016114407-2e83b6f20229
	k8s.io/kube-scheduler => k8s.io/kube-scheduler v0.0.0-20191016114748-65049c67a58b
	k8s.io/kubectl => github.com/openshift/kubernetes-kubectl v0.0.0-20191104125842-899bec6923b1
	k8s.io/kubelet => k8s.io/kubelet v0.0.0-20191016114556-7841ed97f1b2
	k8s.io/kubernetes => github.com/openshift/kubernetes v0.0.0-20191104135101-e3568aa898f3
	k8s.io/legacy-cloud-providers => k8s.io/legacy-cloud-providers v0.0.0-20191016115753-cf0698c3a16b
	k8s.io/metrics => k8s.io/metrics v0.0.0-20191016113814-3b1a734dba6e
	k8s.io/node-api => k8s.io/node-api v0.0.0-20191016115955-b0b11a2622b0
	k8s.io/sample-apiserver => k8s.io/sample-apiserver v0.0.0-20191016112829-06bb3c9d77c9
	k8s.io/sample-cli-plugin => k8s.io/sample-cli-plugin v0.0.0-20191016114214-d25a4244b17f
	k8s.io/sample-controller => k8s.io/sample-controller v0.0.0-20191016113152-0c2dd40eec0c
)

replace github.com/openshift/library-go => github.com/openshift/library-go v0.0.0-20191104114208-f1a64d6c1a3a

replace github.com/openshift/source-to-image => github.com/openshift/source-to-image v0.0.0-20191031172932-56e8595e83fb
