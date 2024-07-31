package apiserver

import (
	"net/http"
	"time"

	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime/schema"
	"k8s.io/apimachinery/pkg/util/sets"
	genericapiserver "k8s.io/apiserver/pkg/server"
	serverstorage "k8s.io/apiserver/pkg/server/storage"

	"github.com/grafana/grafana/pkg/aggregator/apis/aggregation/v0alpha1"
	v0alpha1helper "github.com/grafana/grafana/pkg/aggregator/apis/aggregation/v0alpha1/helper"
	aggregatorscheme "github.com/grafana/grafana/pkg/aggregator/apiserver/scheme"
	clientset "github.com/grafana/grafana/pkg/aggregator/generated/clientset/versioned"
	informers "github.com/grafana/grafana/pkg/aggregator/generated/informers/externalversions"
	dataplaneservicerest "github.com/grafana/grafana/pkg/aggregator/registry/dataplaneservice/rest"
	"github.com/grafana/grafana/pkg/plugins"
)

func init() {
	metav1.AddToGroupVersion(aggregatorscheme.Scheme, schema.GroupVersion{Group: "", Version: "v1"})

	unversioned := schema.GroupVersion{Group: "", Version: "v1"}
	aggregatorscheme.Scheme.AddUnversionedTypes(unversioned,
		&metav1.Status{},
		&metav1.APIVersions{},
		&metav1.APIGroupList{},
		&metav1.APIGroup{},
		&metav1.APIResourceList{},
	)
}

type ExtraConfig struct {
	PluginClient          plugins.Client
	PluginContextProvider PluginContextProvider
	ProxyClientCertFile   string
	ProxyClientKeyFile    string
}

type Config struct {
	GenericConfig *genericapiserver.RecommendedConfig
	ExtraConfig   ExtraConfig
}

type completedConfig struct {
	GenericConfig genericapiserver.CompletedConfig
	ExtraConfig   *ExtraConfig
}

// CompletedConfig same as Config, just to swap private object.
type CompletedConfig struct {
	// Embed a private pointer that cannot be instantiated outside of this package.
	*completedConfig
}

type runnable interface {
	Run(stopCh <-chan struct{}) error
}

// preparedGenericAPIServer is a private wrapper that enforces a call of PrepareRun() before Run can be invoked.
type preparedGrafanaAggregator struct {
	*GrafanaAggregator
	runnable runnable
}

// GrafanaAggregator contains state for a Kubernetes cluster master/api server.
type GrafanaAggregator struct {
	GenericAPIServer      *genericapiserver.GenericAPIServer
	RegistrationInformers informers.SharedInformerFactory
	delegateHandler       http.Handler
	proxyHandlers         map[string]*proxyHandler
	handledGroupVersions  map[string]sets.Set[string]
	PluginClient          plugins.Client
	PluginContextProvider PluginContextProvider
}

// Complete fills in any fields not set that are required to have valid data. It's mutating the receiver.
func (cfg *Config) Complete() CompletedConfig {
	c := completedConfig{
		cfg.GenericConfig.Complete(),
		&cfg.ExtraConfig,
	}

	// the aggregator wires its own discovery mechanism
	// TODO eventually collapse this by extracting all of the discovery out
	c.GenericConfig.EnableDiscovery = false

	return CompletedConfig{&c}
}

// NewWithDelegate returns a new instance of GrafanaAggregator from the given config.
func (c completedConfig) NewWithDelegate(delegationTarget genericapiserver.DelegationTarget) (*GrafanaAggregator, error) {
	genericServer, err := c.GenericConfig.New("grafana-aggregator", delegationTarget)
	if err != nil {
		return nil, err
	}

	dataplaneServiceRegistrationControllerInitiated := make(chan struct{})
	if err := genericServer.RegisterMuxAndDiscoveryCompleteSignal("DataPlaneServiceRegistrationControllerInitiated", dataplaneServiceRegistrationControllerInitiated); err != nil {
		return nil, err
	}

	dataplaneServiceClient, err := clientset.NewForConfig(c.GenericConfig.LoopbackClientConfig)
	if err != nil {
		return nil, err
	}
	informerFactory := informers.NewSharedInformerFactory(
		dataplaneServiceClient,
		5*time.Minute,
	)

	s := &GrafanaAggregator{
		GenericAPIServer:      genericServer,
		RegistrationInformers: informerFactory,
		delegateHandler:       delegationTarget.UnprotectedHandler(),
		proxyHandlers:         map[string]*proxyHandler{},
		PluginClient:          c.ExtraConfig.PluginClient,
		PluginContextProvider: c.ExtraConfig.PluginContextProvider,
	}

	apiserviceRegistrationController := NewDataPlaneServiceRegistrationController(informerFactory.Aggregation().V0alpha1().DataPlaneServices(), s)

	apiGroupInfo := dataplaneservicerest.NewRESTStorage(c.GenericConfig.MergedResourceConfig, c.GenericConfig.RESTOptionsGetter, true)
	if err := s.GenericAPIServer.InstallAPIGroup(&apiGroupInfo); err != nil {
		return nil, err
	}

	s.GenericAPIServer.AddPostStartHookOrDie("start-dataplane-aggregator-informers", func(context genericapiserver.PostStartHookContext) error {
		informerFactory.Start(context.StopCh)
		return nil
	})

	s.GenericAPIServer.AddPostStartHookOrDie("dataplaneservice-registration-controller", func(context genericapiserver.PostStartHookContext) error {
		go apiserviceRegistrationController.Run(context.StopCh, dataplaneServiceRegistrationControllerInitiated)
		select {
		case <-context.StopCh:
		case <-dataplaneServiceRegistrationControllerInitiated:
		}

		return nil
	})

	return s, nil
}

// PrepareRun prepares the aggregator to run
func (s *GrafanaAggregator) PrepareRun() (preparedGrafanaAggregator, error) {
	prepared := s.GenericAPIServer.PrepareRun()
	return preparedGrafanaAggregator{GrafanaAggregator: s, runnable: prepared}, nil
}

func (s preparedGrafanaAggregator) Run(stopCh <-chan struct{}) error {
	return s.runnable.Run(stopCh)
}

func (s *GrafanaAggregator) AddDataPlaneService(dataplaneService *v0alpha1.DataPlaneService) error {
	if proxyHandler, exists := s.proxyHandlers[dataplaneService.Name]; exists {
		proxyHandler.updateDataPlaneService(dataplaneService)
		return nil
	}

	proxyPath := "/apis/dataplane/" + dataplaneService.Spec.Group + "/" + dataplaneService.Spec.Version
	proxyHandler := &proxyHandler{
		localDelegate:         s.delegateHandler,
		client:                s.PluginClient,
		pluginContextProvider: s.PluginContextProvider,
	}
	proxyHandler.updateDataPlaneService(dataplaneService)
	s.proxyHandlers[dataplaneService.Name] = proxyHandler
	s.GenericAPIServer.Handler.NonGoRestfulMux.Handle(proxyPath, proxyHandler)
	s.GenericAPIServer.Handler.NonGoRestfulMux.UnlistedHandlePrefix(proxyPath+"/", proxyHandler)

	versions, exist := s.handledGroupVersions[dataplaneService.Spec.Group]
	if exist {
		versions.Insert(dataplaneService.Spec.Version)
		return nil
	}

	s.handledGroupVersions[dataplaneService.Spec.Group] = sets.New[string](dataplaneService.Spec.Version)
	return nil
}

func (s *GrafanaAggregator) RemoveDataPlaneService(apiServiceName string) {
	version := v0alpha1helper.DataPlaneServiceNameToGroupVersion(apiServiceName)

	proxyPath := "/apis/dataplane/" + version.Group + "/" + version.Version
	s.GenericAPIServer.Handler.NonGoRestfulMux.Unregister(proxyPath)
	s.GenericAPIServer.Handler.NonGoRestfulMux.Unregister(proxyPath + "/")
	delete(s.proxyHandlers, apiServiceName)

	versions, exist := s.handledGroupVersions[version.Group]
	if !exist {
		return
	}
	versions.Delete(version.Version)
	if versions.Len() > 0 {
		return
	}
	delete(s.handledGroupVersions, version.Group)
	groupPath := "/apis/dataplane/" + version.Group
	s.GenericAPIServer.Handler.NonGoRestfulMux.Unregister(groupPath)
	s.GenericAPIServer.Handler.NonGoRestfulMux.Unregister(groupPath + "/")
}

// DefaultAPIResourceConfigSource returns default configuration for an APIResource.
func DefaultAPIResourceConfigSource() *serverstorage.ResourceConfig {
	ret := serverstorage.NewResourceConfig()
	ret.EnableVersions(
		v0alpha1.SchemeGroupVersion,
	)
	return ret
}
