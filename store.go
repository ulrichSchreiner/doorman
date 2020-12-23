package doorman

import (
	"encoding/json"
	"fmt"
	"time"

	"github.com/benbjohnson/clock"
	"go.uber.org/zap"
)

const (
	storageMemory = "memory"
	storageRedis  = "redis"
)

type persistentStore struct {
	whs      *whitelister
	users    ttlstore
	tokensrv *tokenservice
	log      *zap.Logger
	kvs      kvstore
}

type blockinfo struct {
	User string `json:"user"`
	IP   string `json:"ip"`
}

func newStore(log *zap.Logger, cl clock.Clock, sst StoreSettings, whs *whitelister) (*persistentStore, error) {
	var kvs kvstore
	switch sst.PersistentType {
	case storageMemory:
		kvs = newMemstore(cl, sst)
	case storageRedis:
		kvs = newRedisStore(log, cl, sst.Redis.client, sst)
	default:
		return nil, fmt.Errorf("illegal persistent_type: %s", sst.PersistentType)
	}
	return &persistentStore{
		whs:      whs,
		users:    kvs,
		tokensrv: newOTP(kvs, sst),
		log:      log,
		kvs:      kvs,
	}, nil
}

func (s *persistentStore) isAllowed(clientip string) bool {
	if !s.whs.isAllowed(s.log, clientip) {
		return s.isIPAllowed(s.log, clientip)
	}
	return true
}

func (s *persistentStore) isIPAllowed(log *zap.Logger, clientip string) bool {
	_, err := s.users.GetTTL(log, userKey("user", clientip))
	return err == nil
}

func (s *persistentStore) allowUserIP(log *zap.Logger, clip string, ttl time.Duration) error {
	return s.users.PutTTL(log, userKey("user", clip), "", ttl)
}

func (s *persistentStore) blockinfo(log *zap.Logger, key string) (string, string, error) {
	var bi blockinfo
	data, err := s.kvs.GetTTL(log, "blockinfo:"+key)
	if err != nil {
		return "", "", fmt.Errorf("no blockinfo for key: %w", err)
	}
	if err = json.Unmarshal([]byte(data), &bi); err != nil {
		return "", "", fmt.Errorf("cannot unmarshal blockinfo: %w", err)
	}
	return bi.User, bi.IP, nil
}

func (s *persistentStore) block(log *zap.Logger, user, ip, key string, ttl time.Duration) (*yesNoWaiter, error) {
	bi := blockinfo{User: user, IP: ip}
	data, err := json.Marshal(bi)
	if err != nil {
		return nil, fmt.Errorf("cannot marshal blockinfo: %w", err)
	}
	if err := s.kvs.PutTTL(log, "blockinfo:"+key, string(data), ttl); err != nil {
		return nil, fmt.Errorf("cannot put blockinfo: %w", err)
	}
	return s.kvs.Block(log, "block:"+key, ttl)
}

func (s *persistentStore) unblock(log *zap.Logger, key string, val yesno, ttl time.Duration) error {
	return s.kvs.Unblock(log, "block:"+key, val, ttl)
}

func userKey(user, clip string) string {
	return fmt.Sprintf("allow:%s:%s", user, clip)
}
