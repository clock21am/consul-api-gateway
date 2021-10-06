package reconciler

import (
	"reflect"

	"github.com/hashicorp/consul-api-gateway/internal/k8s/gatewayclient"
	"github.com/hashicorp/consul-api-gateway/internal/k8s/utils"
	"github.com/hashicorp/consul-api-gateway/pkg/core"
	"github.com/hashicorp/go-hclog"
	gw "sigs.k8s.io/gateway-api/apis/v1alpha2"
)

type K8sGateway struct {
	consulNamespace string
	logger          hclog.Logger
	gateway         *gw.Gateway
	listeners       map[string]*K8sListener
}

var _ core.Gateway = &K8sGateway{}

type K8sGatewayConfig struct {
	ConsulNamespace string
	Logger          hclog.Logger
	Client          gatewayclient.Client
}

func NewK8sGateway(gateway *gw.Gateway, config K8sGatewayConfig) *K8sGateway {
	gatewayLogger := config.Logger.Named("gateway").With("name", gateway.Name, "namespace", gateway.Namespace)
	listeners := make(map[string]*K8sListener)
	for _, listener := range gateway.Spec.Listeners {
		k8sListener := NewK8sListener(gateway, listener, K8sListenerConfig{
			ConsulNamespace: config.ConsulNamespace,
			Logger:          gatewayLogger,
			Client:          config.Client,
		})
		listeners[k8sListener.ID()] = k8sListener
	}

	return &K8sGateway{
		consulNamespace: config.ConsulNamespace,
		logger:          gatewayLogger,
		gateway:         gateway,
		listeners:       listeners,
	}
}

func (g *K8sGateway) ID() core.GatewayID {
	return core.GatewayID{
		Service:         g.gateway.Name,
		ConsulNamespace: g.consulNamespace,
	}
}

func (g *K8sGateway) Logger() hclog.Logger {
	return g.logger
}

func (g *K8sGateway) Meta() map[string]string {
	return map[string]string{
		"managed_by":                               "consul-api-gateway",
		"consul-api-gateway/k8s/Gateway.Name":      g.gateway.Name,
		"consul-api-gateway/k8s/Gateway.Namespace": g.gateway.Namespace,
	}
}

func (g *K8sGateway) Listeners() []core.Listener {
	listeners := []core.Listener{}

	for _, listener := range g.listeners {
		listeners = append(listeners, listener)
	}

	return listeners
}

func (g *K8sGateway) Compare(other core.Gateway) core.CompareResult {
	if other == nil {
		return core.CompareResultInvalid
	}
	if g == nil {
		return core.CompareResultNotEqual
	}

	if otherGateway, ok := other.(*K8sGateway); ok {
		if g.gateway.Generation > otherGateway.gateway.Generation {
			return core.CompareResultNewer
		}
		if reflect.DeepEqual(g.gateway.Spec, otherGateway.gateway.Spec) {
			return core.CompareResultEqual
		}
		return core.CompareResultNotEqual
	}
	return core.CompareResultInvalid
}

func (g *K8sGateway) ShouldBind(route core.Route) bool {
	k8sRoute, ok := route.(*K8sRoute)
	if !ok {
		return false
	}
	for _, ref := range k8sRoute.CommonRouteSpec().ParentRefs {
		if namespacedName, isGateway := referencesGateway(k8sRoute.GetNamespace(), ref); isGateway {
			if utils.NamespacedName(g.gateway) == namespacedName {
				return true
			}
		}
	}

	return false
}
