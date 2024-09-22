package infra

import (
	"github.com/cryptoniumX/mpcium/pkg/constant"
	"github.com/cryptoniumX/mpcium/pkg/logger"
	"github.com/hashicorp/consul/api"
	"github.com/spf13/viper"
)

type ConsulKV interface {
	Put(kv *api.KVPair, options *api.WriteOptions) (*api.WriteMeta, error)
	Get(key string, options *api.QueryOptions) (*api.KVPair, *api.QueryMeta, error)
	Delete(key string, options *api.WriteOptions) (*api.WriteMeta, error)
	List(prefix string, options *api.QueryOptions) (api.KVPairs, *api.QueryMeta, error)
}

func GetConsulClient(environment string) *api.Client {
	config := api.DefaultConfig()
	if environment != constant.EnvProduction {
		config.Scheme = "http"
	} else {
		config.Scheme = "https"
		config.Token = viper.GetString("consul.token")
		config.HttpAuth = &api.HttpBasicAuth{
			Username: viper.GetString("consul.username"),
			Password: viper.GetString("consul.password"),
		}
	}

	config.Address = viper.GetString("consul.address")
	client, err := api.NewClient(config)
	if err != nil {
		logger.Fatal("Failed to create consul client", err)
	}

	return client
}
