package remoteconfig

import (
	"context"
	"fmt"
	"sync"

	"github.com/nacos-group/nacos-sdk-go/v2/clients"
	"github.com/nacos-group/nacos-sdk-go/v2/clients/config_client"
	"github.com/nacos-group/nacos-sdk-go/v2/vo"
	"github.com/sevoniva-labs/velora/server/internal/platform/config"
	"github.com/sevoniva-labs/velora/server/internal/platform/nacosx"
)

type Source interface {
	Load(context.Context) ([]byte, error)
	Watch(context.Context, func([]byte)) error
	Provider() string
}

func New(cfg config.RemoteConfig) (Source, error) {
	switch cfg.Provider {
	case "", "disabled":
		return disabled{}, nil
	case "nacos":
		cc, servers, err := nacosx.Build(nacosx.ClientSettings{
			Servers: cfg.Servers, Namespace: cfg.Namespace,
			Username: cfg.Username, Password: cfg.Password, LogLevel: "warn",
			TLSRequired: cfg.TLSRequired, TLSCAFile: cfg.TLSCAFile,
			TLSCertFile: cfg.TLSCertFile, TLSKeyFile: cfg.TLSKeyFile, TLSServerName: cfg.TLSServerName,
		})
		if err != nil {
			return nil, err
		}
		client, err := clients.NewConfigClient(vo.NacosClientParam{ClientConfig: &cc, ServerConfigs: servers})
		if err != nil {
			return nil, err
		}
		return &nacosSource{client: client, group: cfg.Group, dataID: cfg.DataID}, nil
	default:
		return nil, fmt.Errorf("unsupported remote config provider %q", cfg.Provider)
	}
}

type disabled struct{}

func (disabled) Load(context.Context) ([]byte, error)      { return nil, nil }
func (disabled) Watch(context.Context, func([]byte)) error { return nil }
func (disabled) Provider() string                          { return "disabled" }

type nacosSource struct {
	client config_client.IConfigClient
	group  string
	dataID string
	mu     sync.Mutex
}

func (n *nacosSource) Load(_ context.Context) ([]byte, error) {
	content, err := n.client.GetConfig(vo.ConfigParam{DataId: n.dataID, Group: n.group})
	if err != nil {
		return nil, err
	}
	return []byte(content), nil
}
func (n *nacosSource) Watch(ctx context.Context, fn func([]byte)) error {
	n.mu.Lock()
	defer n.mu.Unlock()
	err := n.client.ListenConfig(vo.ConfigParam{
		DataId: n.dataID, Group: n.group,
		OnChange: func(_, _, _, data string) { fn([]byte(data)) },
	})
	if err != nil {
		return err
	}
	<-ctx.Done()
	_ = n.client.CancelListenConfig(vo.ConfigParam{DataId: n.dataID, Group: n.group})
	return ctx.Err()
}
func (n *nacosSource) Provider() string { return "nacos" }
