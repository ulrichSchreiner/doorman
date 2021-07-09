package doorman

import (
	"context"
	"encoding/json"
	"fmt"
	"strings"
	"sync"
	"time"

	"github.com/benbjohnson/clock"
	"github.com/go-redis/redis/v8"
	"github.com/mailgun/groupcache/v2"
	"go.uber.org/zap"
)

const (
	megabyte = 1024 * 1024
)

var (
	_ ttlstore = (*memstore)(nil)
	_ kvstore  = (*memstore)(nil)
	_ ttlstore = (*redisStore)(nil)
	_ kvstore  = (*redisStore)(nil)

	ErrNoKey    = fmt.Errorf("no key found")
	ErrTimedOut = fmt.Errorf("key timed out")
)

type ttlValue struct {
	Value string `json:"value"`
	Until int64  `json:"until"`
}

type yesno string

func (yn yesno) Yes() bool {
	return strings.ToLower(string(yn)) == "yes"
}

type yesNoWaiter struct {
	waitc chan yesno
}

func newYesNo() *yesNoWaiter {
	return &yesNoWaiter{
		waitc: make(chan yesno),
	}
}

func (ynw *yesNoWaiter) Say(yn yesno) {
	ynw.waitc <- yn
	ynw.Close()
}

func (ynw *yesNoWaiter) Yes() {
	ynw.Say(yesno("yes"))
}

func (ynw *yesNoWaiter) No() {
	ynw.Say(yesno("no"))
}

func (ynw *yesNoWaiter) Close() {
	close(ynw.waitc)
}

func (ynw *yesNoWaiter) WaitFor() (*yesno, error) {
	yn, ok := <-ynw.waitc
	if ok {
		return &yn, nil
	}
	return nil, fmt.Errorf("no legal value for waitfor")
}

type ttlstore interface {
	PutTTL(log *zap.Logger, key string, value string, ttl time.Duration) error
	GetTTL(log *zap.Logger, key string) (string, error)
}

type kvstore interface {
	PutTTL(log *zap.Logger, key string, value string, ttl time.Duration) error
	GetTTL(log *zap.Logger, key string) (string, error)
	Put(log *zap.Logger, key string, value string) error
	Get(log *zap.Logger, key string) (string, error)
	Has(log *zap.Logger, key string) bool
	Del(log *zap.Logger, key string)
	Block(log *zap.Logger, key string, ttl time.Duration) (*yesNoWaiter, error)
	Unblock(log *zap.Logger, key string, val yesno, ttl time.Duration) error
}

type memstore struct {
	sync.RWMutex
	cl       clock.Clock
	data     map[string]ttlValue
	rawdata  map[string]string
	locks    map[string]*yesNoWaiter
	settings StoreSettings
}

func newMemstore(cl clock.Clock, sst StoreSettings) *memstore {
	return &memstore{
		cl:       cl,
		data:     make(map[string]ttlValue),
		rawdata:  make(map[string]string),
		settings: sst,
	}
}

func (ms *memstore) PutTTL(log *zap.Logger, key, value string, ttl time.Duration) error {
	ms.Lock()
	defer ms.Unlock()

	until := ms.cl.Now().Add(ttl)
	if value == "" {
		value = until.UTC().Format(time.RFC3339)
	}
	ms.data[key] = ttlValue{Value: value, Until: until.UTC().Unix()}
	return nil
}

func (ms *memstore) Put(log *zap.Logger, key, value string) error {
	ms.Lock()
	defer ms.Unlock()

	ms.rawdata[key] = value
	return nil
}

func (ms *memstore) GetTTL(log *zap.Logger, key string) (string, error) {
	delkey := ""
	ms.RLock()
	defer func() {
		ms.RUnlock()
		if delkey != "" {
			ms.delKey(delkey)
		}
	}()

	v, ok := ms.data[key]
	if !ok {
		return "", fmt.Errorf("%w: %s", ErrNoKey, key)
	}
	if ms.cl.Now().UTC().Unix() < v.Until {
		// still valid
		return v.Value, nil
	}
	delkey = key
	return "", fmt.Errorf("%w: %s", ErrTimedOut, key)
}

func (ms *memstore) Get(log *zap.Logger, key string) (string, error) {
	ms.Lock()
	defer ms.Unlock()

	val, ok := ms.rawdata[key]
	if !ok {
		return "", fmt.Errorf("%w: %s", ErrNoKey, key)
	}

	return val, nil
}

func (ms *memstore) Has(log *zap.Logger, key string) bool {
	ms.Lock()
	defer ms.Unlock()

	_, ok := ms.rawdata[key]
	return ok
}

func (ms *memstore) Del(log *zap.Logger, key string) {
	ms.Lock()
	defer ms.Unlock()

	delete(ms.rawdata, key)
}

func (ms *memstore) delKey(key string) {
	ms.Lock()
	defer ms.Unlock()

	delete(ms.data, key)
}

func (ms *memstore) Block(log *zap.Logger, key string, ttl time.Duration) (*yesNoWaiter, error) {
	ms.Lock()
	defer ms.Unlock()

	ynw := newYesNo()
	ms.locks[key] = ynw

	go func() {
		ms.cl.Sleep(ttl)
		ms.Lock()
		if _, exists := ms.locks[key]; exists {
			ynw.Close()
			delete(ms.locks, key)
		}
		ms.Unlock()
	}()
	return ynw, nil
}

func (ms *memstore) Unblock(log *zap.Logger, key string, val yesno, ttl time.Duration) error {
	ms.Lock()
	defer ms.Unlock()

	ynw, has := ms.locks[key]
	if !has {
		return fmt.Errorf("no such lock: %s", key)
	}
	ynw.Say(val)
	delete(ms.locks, key)
	return nil
}

type redisStore struct {
	cl       clock.Clock
	rc       *redis.Client
	cache    *groupcache.Group
	settings StoreSettings
	log      *zap.Logger
}

func newRedisStore(log *zap.Logger, cl clock.Clock, rc *redis.Client, sst StoreSettings) *redisStore {
	rs := &redisStore{
		cl:       cl,
		rc:       rc,
		settings: sst,
		log:      log,
	}
	size := 30 * megabyte
	if sst.MemCacheMB != 0 {
		size = sst.MemCacheMB * megabyte
	}
	rs.cache = groupcache.NewGroup("cache", int64(size), groupcache.GetterFunc(rs.loaderFunc))
	return rs
}

func (rs *redisStore) loaderFunc(ctx context.Context, id string, dest groupcache.Sink) error {
	val, until, err := rs.getTTL(ctx, rs.log, id)
	if err != nil {
		return err
	}
	return dest.SetBytes([]byte(val), *until)
}

func (rs *redisStore) PutTTL(log *zap.Logger, key string, value string, ttl time.Duration) error {
	until := rs.cl.Now().Add(ttl)
	if value == "" {
		value = until.UTC().Format(time.RFC3339)
	}
	v := ttlValue{Value: value, Until: until.UTC().Unix()}
	jsval, err := json.Marshal(v)
	if err != nil {
		return err
	}
	_, err = rs.rc.SetNX(context.Background(), key, jsval, ttl).Result()
	if err != nil {
		return fmt.Errorf("cannot set %q: %w", key, err)
	}
	return nil
}

func (rs *redisStore) Put(log *zap.Logger, key string, value string) error {
	_, err := rs.rc.SetNX(context.Background(), key, value, 0).Result()
	if err != nil {
		return fmt.Errorf("cannot set %q: %w", key, err)
	}
	return nil
}

func (rs *redisStore) GetTTL(log *zap.Logger, key string) (string, error) {
	var res string
	dest := groupcache.StringSink(&res)
	err := rs.cache.Get(context.Background(), key, dest)
	log.Info("get from cache", zap.String("key", key), zap.String("res", res), zap.Error(err))
	if err != nil {
		return res, err
	}
	return res, nil
}

func (rs *redisStore) Get(log *zap.Logger, key string) (string, error) {
	val, err := rs.rc.Get(context.Background(), key).Result()
	if err != nil {
		if err == redis.Nil {
			return "", ErrNoKey
		}
		return "", err
	}
	return val, nil
}

func (rs *redisStore) Has(log *zap.Logger, key string) bool {
	_, err := rs.rc.Get(context.Background(), key).Result()
	return err == nil
}

func (rs *redisStore) Del(log *zap.Logger, key string) {
	_, _ = rs.rc.Del(context.Background(), key).Result()
}

func (rs *redisStore) getTTL(ctx context.Context, log *zap.Logger, key string) (string, *time.Time, error) {
	val, err := rs.rc.Get(ctx, key).Result()
	if err != nil {
		if err == redis.Nil {
			return "", nil, ErrNoKey
		}
		return "", nil, err
	}
	var ttv ttlValue
	if err := json.Unmarshal([]byte(val), &ttv); err != nil {
		return "", nil, err
	}
	ttl := time.Unix(ttv.Until, 0)
	if ttl.After(rs.cl.Now()) {
		return ttv.Value, &ttl, nil
	}
	return "", &ttl, ErrTimedOut
}

func (rs *redisStore) Block(log *zap.Logger, key string, ttl time.Duration) (*yesNoWaiter, error) {
	ynw := newYesNo()

	go func() {
		res := rs.rc.BLPop(context.Background(), ttl, key)
		if res.Err() != nil {
			ynw.Close()
			return
		}
		vals := res.Val()
		if len(vals) != 2 {
			log.Error("result array has not expected size", zap.Strings("vals", vals))
			ynw.Close()
			return
		}
		rs.Del(log, key) // make sure key is removed
		ynw.Say(yesno(vals[1]))
	}()
	return ynw, nil
}

func (rs *redisStore) Unblock(log *zap.Logger, key string, val yesno, ttl time.Duration) error {
	err := rs.rc.LPush(context.Background(), key, string(val)).Err()
	_ = rs.rc.Expire(context.Background(), key, ttl).Err()
	return err
}
