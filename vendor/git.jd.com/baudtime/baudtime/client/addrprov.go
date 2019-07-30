package client

import (
	"context"
	"git.jd.com/baudtime/baudtime/util"
	"net"
	"sort"
	"strconv"
	"sync"
	"sync/atomic"
	"time"
)

type ServiceAddrProvider interface {
	GetServiceAddr() (string, error)
	ServiceDown(addr string)
	ServiceRecover(addr string)
	Watch()
	StopWatch()
}

type StaticServiceAddrProvider struct {
	mtx            sync.RWMutex
	healthyAddrs   []string
	unhealthyAddrs map[string]struct{}
	iter           uint32
	stopC          chan struct{}
}

func NewStaticAddrProvider(addrs ...string) *StaticServiceAddrProvider {
	return &StaticServiceAddrProvider{
		healthyAddrs:   addrs,
		unhealthyAddrs: make(map[string]struct{}, 0),
		stopC:          make(chan struct{}),
	}
}

func (provider *StaticServiceAddrProvider) GetServiceAddr() (string, error) {
	provider.mtx.RLock()
	defer provider.mtx.RUnlock()

	addrNum := len(provider.healthyAddrs)
	if addrNum <= 0 {
		return "", nil
	}

	iter := atomic.LoadUint32(&provider.iter)
	iter = (iter + 1) % uint32(addrNum)
	atomic.StoreUint32(&provider.iter, iter)

	addr := provider.healthyAddrs[iter]
	return addr, nil
}

func (provider *StaticServiceAddrProvider) ServiceDown(addr string) {
	var healthyAddrs []string

	provider.mtx.RLock()
	for _, address := range provider.healthyAddrs {
		if address != addr {
			healthyAddrs = append(healthyAddrs, address)
		}
	}
	provider.mtx.RUnlock()

	provider.mtx.Lock()
	provider.healthyAddrs = healthyAddrs
	provider.unhealthyAddrs[addr] = struct{}{}
	provider.mtx.Unlock()
}

func (provider *StaticServiceAddrProvider) ServiceRecover(addr string) {
	found := false

	provider.mtx.RLock()
	for _, address := range provider.healthyAddrs {
		if address == addr {
			found = true
		}
	}
	provider.mtx.RUnlock()

	if found {
		return
	}

	provider.mtx.Lock()
	provider.healthyAddrs = append(provider.healthyAddrs, addr)
	sort.Strings(provider.healthyAddrs)
	delete(provider.unhealthyAddrs, addr)
	provider.mtx.Unlock()
}

func (provider *StaticServiceAddrProvider) Watch() {
	go func() {
		ticker := time.NewTicker(45 * time.Second)
		defer ticker.Stop()

		for {
			select {
			case <-ticker.C:
				var addrsRecovered []string
				provider.mtx.RLock()
				for unhealthyAddr := range provider.unhealthyAddrs {
					if util.Ping(unhealthyAddr) {
						addrsRecovered = append(addrsRecovered, unhealthyAddr)
					}
				}
				provider.mtx.RUnlock()

				for _, addrR := range addrsRecovered {
					provider.ServiceRecover(addrR)
				}
			case <-provider.stopC:
				return
			}
		}
	}()
}

func (provider *StaticServiceAddrProvider) StopWatch() {
	provider.stopC <- struct{}{}
}

type DnsServiceAddrProvider struct {
	StaticServiceAddrProvider
	serviceDomain string
	servicePort   string
	resolver      *net.Resolver
}

func NewDnsAddrProvider(serviceDomain string, servicePort int) *DnsServiceAddrProvider {
	return &DnsServiceAddrProvider{
		serviceDomain: serviceDomain,
		servicePort:   strconv.Itoa(servicePort),
		resolver:      &net.Resolver{PreferGo: true},
	}
}

func (provider *DnsServiceAddrProvider) Watch() {
	go func() {
		ticker := time.NewTicker(45 * time.Second)
		defer ticker.Stop()

		for {
			select {
			case <-ticker.C:
				hosts, err := provider.resolver.LookupHost(context.Background(), provider.serviceDomain)
				if err == nil {
					var addrs []string
					for _, host := range hosts {
						addrs = append(addrs, host+":"+provider.servicePort)
					}
					sort.Strings(addrs)

					if !equal(addrs, provider.healthyAddrs) {
						provider.mtx.Lock()
						provider.healthyAddrs = addrs
						provider.mtx.Unlock()
					}
				}

				var addrsRecovered []string
				provider.mtx.RLock()
				for unhealthyAddr := range provider.unhealthyAddrs {
					if util.Ping(unhealthyAddr) {
						addrsRecovered = append(addrsRecovered, unhealthyAddr)
					}
				}
				provider.mtx.RUnlock()

				for _, addrR := range addrsRecovered {
					provider.ServiceRecover(addrR)
				}
			case <-provider.stopC:
				return
			}
		}
	}()
}

func equal(left []string, right []string) bool {
	if len(left) != len(right) {
		return false
	}

	for i := 0; i < len(right); i++ {
		if left[i] != right[i] {
			return false
		}
	}

	return true
}
