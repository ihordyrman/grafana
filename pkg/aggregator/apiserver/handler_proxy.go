package apiserver

import (
	"context"
	"fmt"
	"net/http"
	"sync/atomic"

	data "github.com/grafana/grafana-plugin-sdk-go/experimental/apis/data/v0alpha1"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/apiserver/pkg/endpoints/handlers/responsewriters"
	endpointmetrics "k8s.io/apiserver/pkg/endpoints/metrics"
	genericapirequest "k8s.io/apiserver/pkg/endpoints/request"
	"k8s.io/klog/v2"

	"github.com/grafana/grafana-plugin-sdk-go/backend"
	aggregationv0alpha1 "github.com/grafana/grafana/pkg/aggregator/apis/aggregation/v0alpha1"
	query "github.com/grafana/grafana/pkg/apis/query/v0alpha1"
	"github.com/grafana/grafana/pkg/plugins"
	"github.com/grafana/grafana/pkg/web"
)

// proxyHandler provides a http.Handler which will proxy traffic to a plugin client.
type proxyHandler struct {
	localDelegate         http.Handler
	client                plugins.Client
	pluginContextProvider PluginContextProvider
	handlingInfo          atomic.Value
}

type proxyHandlingInfo struct {
	name    string
	handler *pluginHandler
}

func proxyError(w http.ResponseWriter, req *http.Request, error string, code int) {
	http.Error(w, error, code)

	ctx := req.Context()
	info, ok := genericapirequest.RequestInfoFrom(ctx)
	if !ok {
		klog.Warning("no RequestInfo found in the context")
		return
	}
	endpointmetrics.RecordRequestTermination(req, info, "grafana-aggregator", code)
}

func (r *proxyHandler) ServeHTTP(w http.ResponseWriter, req *http.Request) {
	value := r.handlingInfo.Load()
	if value == nil {
		r.localDelegate.ServeHTTP(w, req)
		return
	}
	handlingInfo := value.(proxyHandlingInfo)

	_, ok := genericapirequest.UserFrom(req.Context())
	if !ok {
		proxyError(w, req, "missing user", http.StatusInternalServerError)
		return
	}

	handlingInfo.handler.ServeHTTP(w, req)
}

// these methods provide locked access to fields
func (r *proxyHandler) updateDataPlaneService(dataplaneService *aggregationv0alpha1.DataPlaneService) {
	newInfo := proxyHandlingInfo{
		name: dataplaneService.Name,
	}

	proxyPath := fmt.Sprintf("/apis/dataplane/%s/%s", dataplaneService.Spec.Group, dataplaneService.Spec.Version)

	newInfo.handler = newPluginHandler(
		r.client,
		r.pluginContextProvider,
		proxyPath,
		dataplaneService.Spec.Services,
		r.localDelegate,
	)

	r.handlingInfo.Store(newInfo)
}

// responder implements rest.Responder for assisting a connector in writing objects or errors.
type responder struct {
	w http.ResponseWriter
}

// TODO this should properly handle content type negotiation
// if the caller asked for protobuf and you write JSON bad things happen.
func (r *responder) Object(statusCode int, obj runtime.Object) {
	responsewriters.WriteRawJSON(statusCode, obj, r.w)
}

func (r *responder) Error(_ http.ResponseWriter, _ *http.Request, err error) {
	http.Error(r.w, err.Error(), http.StatusServiceUnavailable)
}

type pluginHandler struct {
	client                plugins.Client
	mux                   *http.ServeMux
	pluginContextProvider PluginContextProvider
	availableServices     []aggregationv0alpha1.Service
	delegate              http.Handler
}

func newPluginHandler(
	client plugins.Client,
	pluginContextProvider PluginContextProvider,
	proxyPath string,
	availableServices []aggregationv0alpha1.Service,
	delegate http.Handler,
) *pluginHandler {
	mux := http.NewServeMux()
	h := &pluginHandler{
		client:                client,
		mux:                   mux,
		pluginContextProvider: pluginContextProvider,
		delegate:              delegate,
		availableServices:     availableServices,
	}

	for _, service := range availableServices {
		switch service.Type {
		case aggregationv0alpha1.DataServiceType:
			mux.Handle("POST "+proxyPath+"/namespaces/{namespace}/query", h.QueryDataHandler())
		}
	}

	// fallback to the delegate
	mux.Handle("/", delegate)

	return h
}

func (h *pluginHandler) ServeHTTP(w http.ResponseWriter, req *http.Request) {
	h.mux.ServeHTTP(w, req)
}

func (h *pluginHandler) QueryDataHandler() http.HandlerFunc {
	return func(w http.ResponseWriter, req *http.Request) {
		responder := &responder{w: w}
		ctx := req.Context()
		dqr := data.QueryDataRequest{}
		err := web.Bind(req, &dqr)
		if err != nil {
			responder.Error(w, req, err)
			return
		}

		queries, dsRef, err := data.ToDataSourceQueries(dqr)
		if err != nil {
			responder.Error(w, req, err)
			return
		}

		pluginContext, err := h.pluginContextProvider.GetPluginContext(ctx, dsRef.Type, dsRef.UID)
		if err != nil {
			responder.Error(w, req, fmt.Errorf("unable to get plugin context: %w", err))
			return
		}

		if dsRef != nil && pluginContext.DataSourceInstanceSettings != nil && dsRef.UID != pluginContext.DataSourceInstanceSettings.UID {
			responder.Error(w, req, fmt.Errorf("expected query body datasource and request to match"))
			return
		}

		ctx = backend.WithGrafanaConfig(ctx, pluginContext.GrafanaConfig)
		rsp, err := h.client.QueryData(ctx, &backend.QueryDataRequest{
			Queries:       queries,
			PluginContext: pluginContext,
		})
		if err != nil {
			responder.Error(w, req, err)
			return
		}
		responder.Object(query.GetResponseCode(rsp),
			&query.QueryDataResponse{QueryDataResponse: *rsp},
		)
	}
}

type PluginContextProvider interface {
	GetPluginContext(ctx context.Context, pluginID, uid string) (backend.PluginContext, error)
}
