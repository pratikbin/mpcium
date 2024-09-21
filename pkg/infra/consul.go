package infra

import (
	"github.com/cryptoniumX/mpcium/pkg/config"
	"github.com/cryptoniumX/mpcium/pkg/constant"
	"github.com/cryptoniumX/mpcium/pkg/logger"
	"github.com/hashicorp/consul/api"
)

type ConsulKV interface {
	Put(kv *api.KVPair, options *api.WriteOptions) (*api.WriteMeta, error)
	Get(key string, options *api.QueryOptions) (*api.KVPair, *api.QueryMeta, error)
	Delete(key string, options *api.WriteOptions) (*api.WriteMeta, error)
	List(prefix string, options *api.QueryOptions) (api.KVPairs, *api.QueryMeta, error)
}

func GetConsulClient(environment string, cfg *config.AppConfig) *api.Client {
	config := api.DefaultConfig()
	if environment != constant.EnvProduction {
		config.Scheme = "http"
	} else {
		config.Scheme = "https"
		config.Token = cfg.Consul.Token
		config.HttpAuth = &api.HttpBasicAuth{
			Username: cfg.Consul.Username,
			Password: cfg.Consul.Password,
		}
	}

	config.Address = cfg.Consul.Address
	client, err := api.NewClient(config)
	if err != nil {
		logger.Fatal("Failed to create consul client", err)
	}

	return client
}
