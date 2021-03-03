package clusterconfig

import (
	"context"
	"fmt"
	"reflect"
	"runtime"
	"sort"
	"strings"
	"time"

	"k8s.io/client-go/rest"
	"k8s.io/klog/v2"

	"github.com/openshift/insights-operator/pkg/record"
)

type gatherStatusReport struct {
	Name         string        `json:"name"`
	Duration     time.Duration `json:"duration_in_ms"`
	RecordsCount int           `json:"records_count"`
	Errors       []string      `json:"errors"`
}

// Gatherer is a driving instance invoking collection of data
type Gatherer struct {
	ctx                     context.Context
	gatherKubeConfig        *rest.Config
	gatherProtoKubeConfig   *rest.Config
	metricsGatherKubeConfig *rest.Config
}

type gatherResult struct {
	records []record.Record
	errors  []error
}

type gatherFunction func(g *Gatherer, c chan<- gatherResult)
type gathering struct {
	function gatherFunction
	canFail  bool
}

func important(function gatherFunction) gathering {
	return gathering{function, false}
}

func failable(function gatherFunction) gathering {
	return gathering{function, true}
}

const gatherAll = "ALL"

var gatherFunctions = map[string]gathering{
	"pdbs":                              important(GatherPodDisruptionBudgets),
	"metrics":                           important(GatherMostRecentMetrics),
	"operators":                         important(GatherClusterOperators),
	"container_images":                  important(GatherContainerImages),
	"nodes":                             important(GatherNodes),
	"config_maps":                       failable(GatherConfigMaps),
	"version":                           important(GatherClusterVersion),
	"id":                                important(GatherClusterID),
	"infrastructures":                   important(GatherClusterInfrastructure),
	"networks":                          important(GatherClusterNetwork),
	"authentication":                    important(GatherClusterAuthentication),
	"image_registries":                  important(GatherClusterImageRegistry),
	"image_pruners":                     important(GatherClusterImagePruner),
	"feature_gates":                     important(GatherClusterFeatureGates),
	"oauths":                            important(GatherClusterOAuth),
	"ingress":                           important(GatherClusterIngress),
	"proxies":                           important(GatherClusterProxy),
	"certificate_signing_requests":      important(GatherCertificateSigningRequests),
	"crds":                              important(GatherCRD),
	"host_subnets":                      important(GatherHostSubnet),
	"machine_sets":                      important(GatherMachineSet),
	"install_plans":                     important(GatherInstallPlans),
	"service_accounts":                  important(GatherServiceAccounts),
	"machine_config_pools":              important(GatherMachineConfigPool),
	"container_runtime_configs":         important(GatherContainerRuntimeConfig),
	"netnamespaces":                     important(GatherNetNamespace),
	"openshift_apiserver_operator_logs": failable(GatherOpenShiftAPIServerOperatorLogs),
	"openshift_sdn_logs":                failable(GatherOpenshiftSDNLogs),
	"openshift_sdn_controller_logs":     failable(GatherOpenshiftSDNControllerLogs),
	"openshift_authentication_logs":     failable(GatherOpenshiftAuthenticationLogs),
	"sap_config":                        failable(GatherSAPConfig),
	"sap_pods":                          failable(GatherSAPPods),
	"olm_operators":                     failable(GatherOLMOperators),
}

// New creates new Gatherer
func New(gatherKubeConfig *rest.Config, gatherProtoKubeConfig *rest.Config, metricsGatherKubeConfig *rest.Config) *Gatherer {
	return &Gatherer{
		gatherKubeConfig:        gatherKubeConfig,
		gatherProtoKubeConfig:   gatherProtoKubeConfig,
		metricsGatherKubeConfig: metricsGatherKubeConfig,
	}
}

// Gather is hosting and calling all the recording functions
func (g *Gatherer) Gather(ctx context.Context, gatherList []string, recorder record.Interface) error {
	g.ctx = ctx
	var errors []string
	var gatherReport []gatherStatusReport

	if len(gatherList) == 0 {
		errors = append(errors, "no gather functions are specified to run")
	}

	if contains(gatherList, gatherAll) {
		gatherList = fullGatherList()
	}

	// Starts the gathers in Go routines
	cases, starts, err := g.startGathering(gatherList, &errors)
	if err != nil {
		return err
	}

	// Gets the info from the Go routines
	remaining := len(cases)
	for remaining > 0 {
		chosen, value, _ := reflect.Select(cases)
		// The chosen channel has been closed, so zero out the channel to disable the case
		cases[chosen].Chan = reflect.ValueOf(nil)
		remaining -= 1

		elapsed := time.Since(starts[chosen]).Truncate(time.Millisecond)

		gatherResults, _ := value.Interface().(gatherResult)
		gatherFunc := gatherFunctions[gatherList[chosen]].function
		gatherCanFail := gatherFunctions[gatherList[chosen]].canFail
		gatherName := runtime.FuncForPC(reflect.ValueOf(gatherFunc).Pointer()).Name()
		klog.V(4).Infof("Gather %s took %s to process %d records", gatherName, elapsed, len(gatherResults.records))
		gatherReport = append(gatherReport, gatherStatusReport{gatherName, time.Duration(elapsed.Milliseconds()), len(gatherResults.records), extractErrors(gatherResults.errors)})

		if gatherCanFail {
			for _, err := range gatherResults.errors {
				klog.V(5).Infof("Couldn't gather %s' received following error: %s\n", gatherName, err.Error())
			}
		} else {
			errors = append(errors, extractErrors(gatherResults.errors)...)
		}
		for _, record := range gatherResults.records {
			if err := recorder.Record(record); err != nil {
				errors = append(errors, fmt.Sprintf("unable to record %s: %v", record.Name, err))
				continue
			}
		}
		klog.V(5).Infof("Read from %s's channel and received %s\n", gatherName, value.String())
	}

	// Creates the gathering performance report
	if err := recordGatherReport(recorder, gatherReport); err != nil {
		errors = append(errors, fmt.Sprintf("unable to record io status reports: %v", err))
	}

	if len(errors) > 0 {
		return sumErrors(errors)
	}
	return nil
}

// Runs each gather functions in a goroutine.
// Every gather function is given its own channel to send back the results.
// 1. return value: `cases` list, used for dynamically reading from the channels.
// 2. return value: `starts` list, contains that start time of each gather function.
func (g *Gatherer) startGathering(gatherList []string, errors *[]string) ([]reflect.SelectCase, []time.Time, error) {
	var cases []reflect.SelectCase
	var starts []time.Time
	// Starts the gathers in Go routines
	for _, gatherId := range gatherList {
		gather, ok := gatherFunctions[gatherId]
		gFn := gather.function
		if !ok {
			*errors = append(*errors, fmt.Sprintf("unknown gatherId in config: %s", gatherId))
			continue
		}
		channel := make(chan gatherResult)
		cases = append(cases, reflect.SelectCase{Dir: reflect.SelectRecv, Chan: reflect.ValueOf(channel)})
		gatherName := runtime.FuncForPC(reflect.ValueOf(gFn).Pointer()).Name()

		klog.V(5).Infof("Gathering %s", gatherName)
		starts = append(starts, time.Now())
		go gFn(g, channel)

		if err := g.ctx.Err(); err != nil {
			return nil, nil, err
		}
	}
	return cases, starts, nil
}

func recordGatherReport(recorder record.Interface, report []gatherStatusReport) error {
	r := record.Record{Name: "insights-operator/gathers", Item: record.JSONMarshaller{Object: report}}
	return recorder.Record(r)
}

func extractErrors(errors []error) []string {
	var errStrings []string
	for _, err := range errors {
		errStrings = append(errStrings, err.Error())
	}
	return errStrings
}

func sumErrors(errors []string) error {
	sort.Strings(errors)
	errors = uniqueStrings(errors)
	return fmt.Errorf("%s", strings.Join(errors, ", "))
}

func fullGatherList() []string {
	gatherList := make([]string, 0, len(gatherFunctions))
	for k := range gatherFunctions {
		gatherList = append(gatherList, k)
	}
	return gatherList
}

func uniqueStrings(list []string) []string {
	if len(list) < 2 {
		return list
	}
	keys := make(map[string]bool)
	set := []string{}
	for _, entry := range list {
		if _, value := keys[entry]; !value {
			keys[entry] = true
			set = append(set, entry)
		}
	}
	return set
}

func contains(s []string, e string) bool {
	for _, a := range s {
		if a == e {
			return true
		}
	}
	return false
}
