package cluster

import (
	"github.com/tilt-dev/ctlptl/pkg/api"
	"k8s.io/client-go/tools/clientcmd"
	clientcmdapi "k8s.io/client-go/tools/clientcmd/api"

	// Client auth plugins! They will auto-init if we import them.
	_ "k8s.io/client-go/plugin/pkg/client/auth"
)

type Controller struct {
	config clientcmdapi.Config
}

func ControllerWithConfig(config clientcmdapi.Config) *Controller {
	return &Controller{config: config}
}

func DefaultController() (*Controller, error) {
	rules := clientcmd.NewDefaultClientConfigLoadingRules()
	rules.DefaultClientConfig = &clientcmd.DefaultClientConfig

	overrides := &clientcmd.ConfigOverrides{}
	loader := clientcmd.NewNonInteractiveDeferredLoadingClientConfig(rules, overrides)
	rawConfig, err := loader.RawConfig()
	if err != nil {
		return nil, err
	}
	return &Controller{
		config: rawConfig,
	}, nil
}

func (c *Controller) List() ([]*api.Cluster, error) {
	result := []*api.Cluster{}
	for name, ct := range c.config.Contexts {
		result = append(result, &api.Cluster{
			TypeMeta: api.TypeMeta{APIVersion: "ctlptl.dev/v1alpha1", Kind: "Cluster"},
			Name:     name,
			Product:  productFromContext(ct).String(),
		})
	}
	return result, nil
}
