package clusterconfig

import (
	"context"
	"fmt"
	"io/ioutil"
	"os"
	"path"
	"path/filepath"
	"regexp"
	"strings"
	"sync"
	"time"

	corev1 "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/cli-runtime/pkg/genericclioptions"
	kubescheme "k8s.io/client-go/kubernetes/scheme"
	corev1client "k8s.io/client-go/kubernetes/typed/core/v1"
	"k8s.io/klog"

	configv1 "github.com/openshift/api/config/v1"
	"github.com/openshift/client-go/config/clientset/versioned/scheme"
	configv1client "github.com/openshift/client-go/config/clientset/versioned/typed/config/v1"
	"github.com/openshift/oc/pkg/cli/admin/inspect"

	"github.com/openshift/insights-operator/pkg/record"
)

var (
	serializer     = scheme.Codecs.LegacyCodec(configv1.SchemeGroupVersion)
	kubeSerializer = kubescheme.Codecs.LegacyCodec(corev1.SchemeGroupVersion)
)

type Gatherer struct {
	client     configv1client.ConfigV1Interface
	coreClient corev1client.CoreV1Interface

	lock        sync.Mutex
	lastVersion *configv1.ClusterVersion
}

func New(client configv1client.ConfigV1Interface, coreClient corev1client.CoreV1Interface) *Gatherer {
	return &Gatherer{
		client:     client,
		coreClient: coreClient,
	}
}

var reInvalidUIDCharacter = regexp.MustCompile(`[^a-z0-9\-]`)

var cInspect = make(chan struct{}, 1)

func (i *Gatherer) Gather(ctx context.Context, recorder record.Interface) error {
	select {
	case cInspect <- struct{}{}:
		klog.Infoln("[INSPECT] -------- BEGIN --------")
		setupStartTime := time.Now()

		ioStreams := genericclioptions.IOStreams{In: os.Stdin, Out: os.Stdout, ErrOut: os.Stderr}
		inspectOptions := inspect.NewInspectOptions(ioStreams)

		klog.Infoln("[INSPECT] OK -- inspect.NewInspectOptions")

		err := inspectOptions.Complete(nil, []string{"clusteroperators"})
		if err != nil {
			klog.Errorln(err)
			return err
		}

		klog.Infoln("[INSPECT] OK -- inspectOptions.Complete")

		err = inspectOptions.Validate()
		if err != nil {
			klog.Errorln(err)
			return err
		}

		klog.Infoln("[INSPECT] OK -- inspectOptions.Validate")

		go func() {
			klog.Infoln("[INSPECT] GOROUTINE ALIVE")

			runStartTime := time.Now()
			err = inspectOptions.Run()

			if err != nil {
				klog.Errorln("[INSPECT] ERROR -- inspectOptions.Run --", err)
			} else {
				klog.Infoln("[INSPECT] OK -- inspectOptions.Run")
			}

			<-cInspect

			klog.Infoln("[INSPECT] -------- END --", time.Since(runStartTime).Seconds(), "seconds", "--------")
		}()

		klog.Infoln("[INSPECT] -------- GOROUTINE SPAWNED --", time.Since(setupStartTime).Seconds(), "seconds", "--------")

	default:
		klog.Infoln("[INSPECT] -------- GOROUTINE RUNNING -- RECORD STAGE BEGIN --------")

		const baseDirPath = "."
		files, err := ioutil.ReadDir(baseDirPath)

		if err != nil {
			klog.Errorln(err)
			return err
		}

		// Find most recent inspect directory.
		var lastInspectDir os.FileInfo = nil

		for _, f := range files {
			if f.IsDir() && strings.HasPrefix(f.Name(), "inspect.local.") {
				if lastInspectDir == nil || f.ModTime().Sub(lastInspectDir.ModTime()) > 0 {
					lastInspectDir = f
				}
			}
		}

		recordCounter := 0

		if lastInspectDir == nil {
			klog.Infoln("[INSPECT] WARNING: No inspect directory found")
		} else {
			// Find all YAML files in the inspect directory.
			inspectDirPath := path.Join(baseDirPath, lastInspectDir.Name())
			re := regexp.MustCompile(`^.*inspect\.local\.\d+`)

			err = filepath.Walk(inspectDirPath, func(path string, info os.FileInfo, err error) error {
				if info.Mode().IsRegular() && filepath.Ext(info.Name()) == ".yaml" {
					recorder.Record(record.Record{Name: re.ReplaceAllLiteralString(path, "config/inspect"), Captured: time.Now(), LocalPath: path})
					recordCounter++
				}

				if err != nil {
					klog.Errorln(err)
				}

				return err
			})

			if err != nil {
				klog.Errorln(err)
				return err
			}
		}

		klog.Infoln("[INSPECT] -------- RECORD STAGE END --", recordCounter, "YAML files recorded --------")
	}

	return record.Collect(ctx, recorder,
		func() ([]record.Record, []error) {
			config, err := i.client.ClusterOperators().List(metav1.ListOptions{})
			if errors.IsNotFound(err) {
				return nil, nil
			}
			if err != nil {
				return nil, []error{err}
			}
			records := make([]record.Record, 0, len(config.Items))
			for i := range config.Items {
				records = append(records, record.Record{Name: fmt.Sprintf("config/clusteroperator/%s", config.Items[i].Name), Item: ClusterOperatorAnonymizer{&config.Items[i]}})
			}

			for _, item := range config.Items {
				if isHealthyOperator(&item) {
					continue
				}
				for _, namespace := range namespacesForOperator(&item) {
					pods, err := i.coreClient.Pods(namespace).List(metav1.ListOptions{})
					if err != nil {
						klog.V(2).Infof("Unable to find pods in namespace %s for failing operator %s", namespace, item.Name)
					}
					for i := range pods.Items {
						if isHealthyPod(&pods.Items[i]) {
							continue
						}
						records = append(records, record.Record{Name: fmt.Sprintf("config/pod/%s/%s", pods.Items[i].Namespace, pods.Items[i].Name), Item: PodAnonymizer{&pods.Items[i]}})
					}
				}
			}

			return records, nil
		},
		func() ([]record.Record, []error) {
			nodes, err := i.coreClient.Nodes().List(metav1.ListOptions{})
			if err != nil {
				return nil, []error{err}
			}
			records := make([]record.Record, 0, len(nodes.Items))
			for i := range nodes.Items {
				if isHealthyNode(&nodes.Items[i]) {
					continue
				}
				records = append(records, record.Record{Name: fmt.Sprintf("config/node/%s", nodes.Items[i].Name), Item: NodeAnonymizer{&nodes.Items[i]}})
			}

			return records, nil
		},
		func() ([]record.Record, []error) {
			config, err := i.client.ClusterVersions().Get("version", metav1.GetOptions{})
			if errors.IsNotFound(err) {
				return nil, nil
			}
			if err != nil {
				return nil, []error{err}
			}
			i.setClusterVersion(config)
			return []record.Record{{Name: "config/version", Item: ClusterVersionAnonymizer{config}}}, nil
		},
		func() ([]record.Record, []error) {
			version := i.ClusterVersion()
			if version == nil {
				return nil, nil
			}
			return []record.Record{{Name: "config/id", Item: Raw{string(version.Spec.ClusterID)}}}, nil
		},
		func() ([]record.Record, []error) {
			config, err := i.client.Infrastructures().Get("cluster", metav1.GetOptions{})
			if errors.IsNotFound(err) {
				return nil, nil
			}
			if err != nil {
				return nil, []error{err}
			}
			return []record.Record{{Name: "config/infrastructure", Item: InfrastructureAnonymizer{config}}}, nil
		},
		func() ([]record.Record, []error) {
			config, err := i.client.Networks().Get("cluster", metav1.GetOptions{})
			if errors.IsNotFound(err) {
				return nil, nil
			}
			if err != nil {
				return nil, []error{err}
			}
			return []record.Record{{Name: "config/network", Item: Anonymizer{config}}}, nil
		},
		func() ([]record.Record, []error) {
			config, err := i.client.Authentications().Get("cluster", metav1.GetOptions{})
			if errors.IsNotFound(err) {
				return nil, nil
			}
			if err != nil {
				return nil, []error{err}
			}
			return []record.Record{{Name: "config/authentication", Item: Anonymizer{config}}}, nil
		},
		func() ([]record.Record, []error) {
			config, err := i.client.FeatureGates().Get("cluster", metav1.GetOptions{})
			if errors.IsNotFound(err) {
				return nil, nil
			}
			if err != nil {
				return nil, []error{err}
			}
			return []record.Record{{Name: "config/featuregate", Item: FeatureGateAnonymizer{config}}}, nil
		},
		func() ([]record.Record, []error) {
			config, err := i.client.OAuths().Get("cluster", metav1.GetOptions{})
			if errors.IsNotFound(err) {
				return nil, nil
			}
			if err != nil {
				return nil, []error{err}
			}
			return []record.Record{{Name: "config/oauth", Item: Anonymizer{config}}}, nil
		},
		func() ([]record.Record, []error) {
			config, err := i.client.Ingresses().Get("cluster", metav1.GetOptions{})
			if errors.IsNotFound(err) {
				return nil, nil
			}
			if err != nil {
				return nil, []error{err}
			}
			return []record.Record{{Name: "config/ingress", Item: IngressAnonymizer{config}}}, nil
		},
	)
}

type Raw struct{ string }

func (r Raw) Marshal(_ context.Context) ([]byte, error) {
	return []byte(r.string), nil
}

type Anonymizer struct{ runtime.Object }

func (a Anonymizer) Marshal(_ context.Context) ([]byte, error) {
	return runtime.Encode(serializer, a.Object)
}

type InfrastructureAnonymizer struct{ *configv1.Infrastructure }

func (a InfrastructureAnonymizer) Marshal(_ context.Context) ([]byte, error) {
	return runtime.Encode(serializer, anonymizeInfrastructure(a.Infrastructure))
}

func anonymizeInfrastructure(config *configv1.Infrastructure) *configv1.Infrastructure {
	config.Status.APIServerURL = anonymizeURL(config.Status.APIServerURL)
	config.Status.EtcdDiscoveryDomain = anonymizeURL(config.Status.EtcdDiscoveryDomain)
	config.Status.InfrastructureName = anonymizeURL(config.Status.InfrastructureName)
	config.Status.APIServerInternalURL = anonymizeURL(config.Status.APIServerInternalURL)
	return config
}

type ClusterVersionAnonymizer struct{ *configv1.ClusterVersion }

func (a ClusterVersionAnonymizer) Marshal(_ context.Context) ([]byte, error) {
	a.ClusterVersion.Spec.Upstream = configv1.URL(anonymizeURL(string(a.ClusterVersion.Spec.Upstream)))
	return runtime.Encode(serializer, a.ClusterVersion)
}

type FeatureGateAnonymizer struct{ *configv1.FeatureGate }

func (a FeatureGateAnonymizer) Marshal(_ context.Context) ([]byte, error) {
	return runtime.Encode(serializer, a.FeatureGate)
}

type IngressAnonymizer struct{ *configv1.Ingress }

func (a IngressAnonymizer) Marshal(_ context.Context) ([]byte, error) {
	a.Ingress.Spec.Domain = anonymizeURL(a.Ingress.Spec.Domain)
	return runtime.Encode(serializer, a.Ingress)
}

var reURL = regexp.MustCompile(`[^\.\-/\:]`)

func anonymizeURL(s string) string { return reURL.ReplaceAllString(s, "x") }

type ClusterOperatorAnonymizer struct{ *configv1.ClusterOperator }

func (a ClusterOperatorAnonymizer) Marshal(_ context.Context) ([]byte, error) {
	return runtime.Encode(serializer, a.ClusterOperator)
}

func isHealthyOperator(operator *configv1.ClusterOperator) bool {
	for _, condition := range operator.Status.Conditions {
		switch {
		case condition.Type == configv1.OperatorDegraded && condition.Status == configv1.ConditionTrue,
			condition.Type == configv1.OperatorAvailable && condition.Status == configv1.ConditionFalse:
			return false
		}
	}
	return true
}

func namespacesForOperator(operator *configv1.ClusterOperator) []string {
	var ns []string
	for _, ref := range operator.Status.RelatedObjects {
		if ref.Resource == "namespaces" {
			ns = append(ns, ref.Name)
		}
	}
	return ns
}

type NodeAnonymizer struct{ *corev1.Node }

func (a NodeAnonymizer) Marshal(_ context.Context) ([]byte, error) {
	return runtime.Encode(kubeSerializer, anonymizeNode(a.Node))
}

func anonymizeNode(node *corev1.Node) *corev1.Node {
	for k := range node.Annotations {
		if isProductNamespacedKey(k) {
			continue
		}
		node.Annotations[k] = ""
	}
	for k, v := range node.Labels {
		if isProductNamespacedKey(k) {
			continue
		}
		node.Labels[k] = anonymizeString(v)
	}
	for i := range node.Status.Addresses {
		node.Status.Addresses[i].Address = anonymizeURL(node.Status.Addresses[i].Address)
	}
	node.Status.NodeInfo.BootID = anonymizeString(node.Status.NodeInfo.BootID)
	node.Status.NodeInfo.SystemUUID = anonymizeString(node.Status.NodeInfo.SystemUUID)
	node.Status.NodeInfo.MachineID = anonymizeString(node.Status.NodeInfo.MachineID)
	node.Status.Images = nil
	return node
}

func anonymizeString(s string) string {
	return strings.Repeat("x", len(s))
}

func isProductNamespacedKey(key string) bool {
	return strings.Contains(key, "openshift.io/") || strings.Contains(key, "k8s.io/") || strings.Contains(key, "kubernetes.io/")
}

func isHealthyNode(node *corev1.Node) bool {
	for _, condition := range node.Status.Conditions {
		if condition.Type == corev1.NodeReady && condition.Status != corev1.ConditionTrue {
			return false
		}
	}
	return true
}

type PodAnonymizer struct{ *corev1.Pod }

func (a PodAnonymizer) Marshal(_ context.Context) ([]byte, error) {
	return runtime.Encode(kubeSerializer, anonymizePod(a.Pod))
}

func anonymizePod(pod *corev1.Pod) *corev1.Pod {
	// pods gathered from openshift namespaces and cluster operators are expected to be under our control and contain
	// no sensitive information
	return pod
}

func isHealthyPod(pod *corev1.Pod) bool {
	// pending pods may be unable to schedule or start due to failures, and the info they provide in status is important
	// for identifying why scheduling hass not happened
	if pod.Status.Phase == corev1.PodPending {
		return false
	}
	// pods that have containers that have terminated with non-zero exit codes are considered failure
	for _, status := range pod.Status.InitContainerStatuses {
		if status.LastTerminationState.Terminated != nil && status.LastTerminationState.Terminated.ExitCode != 0 {
			return false
		}
		if status.State.Terminated != nil && status.State.Terminated.ExitCode != 0 {
			return false
		}
	}
	for _, status := range pod.Status.ContainerStatuses {
		if status.LastTerminationState.Terminated != nil && status.LastTerminationState.Terminated.ExitCode != 0 {
			return false
		}
		if status.State.Terminated != nil && status.State.Terminated.ExitCode != 0 {
			return false
		}
	}
	return true
}

func (i *Gatherer) setClusterVersion(version *configv1.ClusterVersion) {
	i.lock.Lock()
	defer i.lock.Unlock()
	if i.lastVersion != nil && i.lastVersion.ResourceVersion == version.ResourceVersion {
		return
	}
	i.lastVersion = version.DeepCopy()
}

func (i *Gatherer) ClusterVersion() *configv1.ClusterVersion {
	i.lock.Lock()
	defer i.lock.Unlock()
	return i.lastVersion
}
