// Copyright 2016 CodisLabs. All Rights Reserved.
// Licensed under the MIT (MIT-LICENSE.txt) license.

package topom

import (
	"time"

	"github.com/CodisLabs/codis/pkg/models"
	"github.com/CodisLabs/codis/pkg/proxy"
	"github.com/CodisLabs/codis/pkg/utils/log"
	"github.com/CodisLabs/codis/pkg/utils/redis"
	"github.com/CodisLabs/codis/pkg/utils/rpc"
	"github.com/CodisLabs/codis/pkg/utils/sync2"
)

// 存储 Redis 节点的各个状态信息，如 Redis 的 INFO 命令获取到的内容
type RedisStats struct {
	Stats map[string]string `json:"stats,omitempty"`
	Error *rpc.RemoteError  `json:"error,omitempty"`

	// 哨兵/从服务器
	Sentinel map[string]*redis.SentinelGroup `json:"sentinel,omitempty"`

	UnixTime int64 `json:"unixtime"`
	Timeout  bool  `json:"timeout,omitempty"`
}

func (s *Topom) newRedisStats(addr string, timeout time.Duration, do func(addr string) (*RedisStats, error)) *RedisStats {
	var ch = make(chan struct{})
	stats := &RedisStats{}

	go func() {
		defer close(ch)
		p, err := do(addr)
		if err != nil {
			stats.Error = rpc.NewRemoteError(err)
		} else {
			stats.Stats, stats.Sentinel = p.Stats, p.Sentinel
		}
	}()

	// golang 技巧，设置超时时间，避免协程函数返回超时
	select {
	case <-ch:
		return stats
	case <-time.After(timeout):
		return &RedisStats{Timeout: true}
	}
}

// 刷新 redis 节点信息，指定多个协程，返回协程组的句柄 future
func (s *Topom) RefreshRedisStats(timeout time.Duration) (*sync2.Future, error) {
	s.mu.Lock()
	defer s.mu.Unlock()
	ctx, err := s.newContext()
	if err != nil {
		return nil, err
	}
	var fut sync2.Future
	goStats := func(addr string, do func(addr string) (*RedisStats, error)) {
		fut.Add()
		go func() {
			stats := s.newRedisStats(addr, timeout, do)
			stats.UnixTime = time.Now().Unix()
			fut.Done(addr, stats)
		}()
	}
	//遍历ctx中的group，再遍历每个group中的Server
	for _, g := range ctx.group {
		for _, x := range g.Servers {
			goStats(x.Addr, func(addr string) (*RedisStats, error) {
				m, err := s.stats.redisp.InfoFull(addr)
				if err != nil {
					return nil, err
				}
				return &RedisStats{Stats: m}, nil
			})
		}
	}
	//通过sentinel维护codis集群中每一组的主备关系
	for _, server := range ctx.sentinel.Servers {
		goStats(server, func(addr string) (*RedisStats, error) {
			c, err := s.ha.redisp.GetClient(addr)
			if err != nil {
				return nil, err
			}
			defer s.ha.redisp.PutClient(c)
			m, err := c.Info()
			if err != nil {
				return nil, err
			}
			sentinel := redis.NewSentinel(s.config.ProductName, s.config.ProductAuth)
			p, err := sentinel.MastersAndSlavesClient(c)
			if err != nil {
				return nil, err
			}
			return &RedisStats{Stats: m, Sentinel: p}, nil
		})
	}
	//前面的所有gostats执行完之后，遍历Future的vmap，将值赋给Topom.stats.servers
	go func() {
		stats := make(map[string]*RedisStats)
		for k, v := range fut.Wait() {
			stats[k] = v.(*RedisStats)
		}
		s.mu.Lock()
		defer s.mu.Unlock()
		s.stats.servers = stats
	}()
	return &fut, nil
}

type ProxyStats struct {
	Stats *proxy.Stats     `json:"stats,omitempty"`
	Error *rpc.RemoteError `json:"error,omitempty"`

	UnixTime int64 `json:"unixtime"`
	Timeout  bool  `json:"timeout,omitempty"`
}

func (s *Topom) newProxyStats(p *models.Proxy, timeout time.Duration) *ProxyStats {
	var ch = make(chan struct{})
	stats := &ProxyStats{}

	go func() {
		defer close(ch)
		x, err := s.newProxyClient(p).StatsSimple()
		if err != nil {
			stats.Error = rpc.NewRemoteError(err)
		} else {
			stats.Stats = x
		}
	}()

	select {
	case <-ch:
		return stats
	case <-time.After(timeout):
		return &ProxyStats{Timeout: true}
	}
}

func (s *Topom) RefreshProxyStats(timeout time.Duration) (*sync2.Future, error) {
	s.mu.Lock()
	defer s.mu.Unlock()
	ctx, err := s.newContext()
	if err != nil {
		return nil, err
	}
	var fut sync2.Future
	for _, p := range ctx.proxy {
		fut.Add()
		go func(p *models.Proxy) {
			stats := s.newProxyStats(p, timeout)
			stats.UnixTime = time.Now().Unix()
			fut.Done(p.Token, stats)

			switch x := stats.Stats; {
			case x == nil:
			case x.Closed || x.Online:
			default:
				if err := s.OnlineProxy(p.AdminAddr); err != nil {
					log.WarnErrorf(err, "auto online proxy-[%s] failed", p.Token)
				}
			}
		}(p)
	}
	go func() {
		stats := make(map[string]*ProxyStats)
		for k, v := range fut.Wait() {
			stats[k] = v.(*ProxyStats)
		}
		s.mu.Lock()
		defer s.mu.Unlock()
		s.stats.proxies = stats
	}()
	return &fut, nil
}
