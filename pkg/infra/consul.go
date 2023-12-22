package infra

import (
	"github.com/hashicorp/consul/api"
)

type ConsulKV interface {
	Put(kv *api.KVPair, options *api.WriteOptions) (*api.WriteMeta, error)
	Get(key string, options *api.QueryOptions) (*api.KVPair, *api.QueryMeta, error)
	Delete(key string, options *api.WriteOptions) (*api.WriteMeta, error)
	List(prefix string, options *api.QueryOptions) (api.KVPairs, *api.QueryMeta, error)
}
