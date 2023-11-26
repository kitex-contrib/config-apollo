package apollo

import (
	"sync"
	"testing"
	"time"

	"github.com/cloudwego/kitex/pkg/klog"
	"github.com/shima-park/agollo"
	"gopkg.in/go-playground/assert.v1"
)

type fakeApollo struct {
	len  int
	resp chan *agollo.ApolloResponse
	sync.Mutex
}

func (fa *fakeApollo) Start() <-chan *agollo.LongPollerError {
	return make(<-chan *agollo.LongPollerError)
}

func (fa *fakeApollo) Stop() {
}

func (fa *fakeApollo) Get(key string, opts ...agollo.GetOption) string {
	return ""
}

func (fa *fakeApollo) GetNameSpace(namespace string) agollo.Configurations {
	return make(agollo.Configurations)
}

func (fa *fakeApollo) Watch() <-chan *agollo.ApolloResponse {
	return fa.resp
}

func (fa *fakeApollo) WatchNamespace(namespace string, stop chan bool) <-chan *agollo.ApolloResponse {
	return fa.resp
}

func (fa *fakeApollo) Options() agollo.Options {
	return agollo.Options{}
}

func NewFakeApollo() *fakeApollo {
	return &fakeApollo{
		resp: make(chan *agollo.ApolloResponse),
	}
}

// update config-info
func (fa *fakeApollo) change(cfg configParamKey, data string) {
	fa.Lock()
	defer fa.Unlock()
	klog.Infof("change data : %s", data)
	for idx := 0; idx < fa.len; idx++ {
		fa.resp <- &agollo.ApolloResponse{
			NewValue: agollo.Configurations{cfg.Key: data},
		}
	}
}

func TestRegisterAndDeregister(t *testing.T) {
	fake := NewFakeApollo()

	cli := &client{
		acli:     fake,
		stop:     make(chan bool),
		handlers: make(map[configParamKey]map[int64]callbackHandler),
	}

	var gotlock sync.Mutex
	gots := make(map[configParamKey]map[int64]string)
	configkey := configParamKey{
		NameSpace: "n1",
		Key:       "k1",
		Cluster:   "c1",
	}

	id1 := GetUniqueID()

	fake.Lock()
	fake.len++
	fake.Unlock()

	var wg sync.WaitGroup
	wg.Add(1)
	go func() {
		defer wg.Done()
		cli.RegisterConfigCallback(ConfigParam{
			Key:       "k1",
			nameSpace: "n1",
			Cluster:   "c1",
		}, func(s string, cp ConfigParser) {
			gotlock.Lock()
			defer gotlock.Unlock()
			ids, ok := gots[configkey]
			klog.Info("onchange callback1:", s)
			if !ok {
				ids = map[int64]string{}
				gots[configkey] = ids
			}
			ids[id1] = s
		}, id1)
	}()

	id2 := GetUniqueID()

	fake.Lock()
	fake.len++
	fake.Unlock()

	wg.Add(1)
	go func() {
		defer wg.Done()
		cli.RegisterConfigCallback(ConfigParam{
			Key:       "k1",
			nameSpace: "n1",
			Cluster:   "c1",
		}, func(s string, cp ConfigParser) {
			gotlock.Lock()
			defer gotlock.Unlock()
			klog.Info("onchange callback2:", s)
			ids, ok := gots[configkey]
			if !ok {
				ids = map[int64]string{}
				gots[configkey] = ids
			}
			ids[id2] = s
		}, id2)
	}()
	wg.Wait()
	// wait the goroutine init
	time.Sleep(2 * time.Second)
	// first change
	fake.change(configParamKey{
		Key:       "k1",
		NameSpace: "n1",
		Cluster:   "c1",
	}, "first change")

	// wait goroutine deal
	time.Sleep(2 * time.Second)

	assert.Equal(t, map[configParamKey]map[int64]string{
		{
			Key:       "k1",
			NameSpace: "n1",
			Cluster:   "c1",
		}: {
			id1: "first change",
			id2: "first change",
		},
	}, gots)

	cli.DeregisterConfig(ConfigParam{
		Key:       "k1",
		nameSpace: "n1",
		Cluster:   "c1",
	}, id2)

	fake.Lock()
	fake.len--
	fake.Unlock()

	fake.change(configParamKey{
		Key:       "k1",
		NameSpace: "n1",
		Cluster:   "c1",
	}, "second change")

	// wait goroutine deal
	time.Sleep(2 * time.Second)

	cli.DeregisterConfig(ConfigParam{
		Key:       "k1",
		nameSpace: "n1",
		Cluster:   "c1",
	}, id1)

	fake.Lock()
	fake.len--
	fake.Unlock()

	assert.Equal(t, map[configParamKey]map[int64]string{
		{
			Key:       "k1",
			NameSpace: "n1",
			Cluster:   "c1",
		}: {
			id1: "second change",
			id2: "first change",
		},
	}, gots)

	fake.change(configParamKey{
		Key:       "k1",
		NameSpace: "n1",
		Cluster:   "c1",
	}, "third change")
	time.Sleep(2 * time.Second)
	assert.Equal(t, map[configParamKey]map[int64]string{
		{
			Key:       "k1",
			NameSpace: "n1",
			Cluster:   "c1",
		}: {
			id1: "second change",
			id2: "first change",
		},
	}, gots)
}
