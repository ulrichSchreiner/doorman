package doorman

import (
	"bytes"
	"encoding/base64"
	"encoding/json"
	"errors"
	"fmt"
	"image/png"
	"math/rand"
	"time"

	"github.com/pquerna/otp"
	"github.com/pquerna/otp/totp"
	"go.uber.org/zap"
)

const (
	toplevelKey      = "otp:"
	toplevelTmpKey   = "tmpotp:"
	toplevelTmpToken = "tmptoken:"
	randKeyLen       = 64
)

type tokenservice struct {
	store    kvstore
	settings StoreSettings
}

type otpRegistration struct {
	PrivateKey string `json:"private_key"`
	URL        string `json:"url"`
}

func newOTP(kv kvstore, sst StoreSettings) *tokenservice {
	return &tokenservice{
		store:    kv,
		settings: sst,
	}
}

func (otps *tokenservice) validateTempRegistration(log *zap.Logger, issuer, uid, key, token string) error {
	val, err := otps.store.GetTTL(log, genKey(toplevelTmpKey, uid, key))
	if err != nil {
		return err
	}
	var res otpRegistration
	err = json.Unmarshal([]byte(val), &res)
	if err != nil {
		return err
	}
	if !totp.Validate(token, res.PrivateKey) {
		return fmt.Errorf("illegal token")
	}

	otps.store.Del(log, genKey(toplevelTmpKey, uid, key))
	return otps.store.Put(log, genKey(toplevelKey, uid, issuer), val)
}

func (otps *tokenservice) checkTempRegistration(log *zap.Logger, issuer, uid string) error {
	_, err := otps.store.GetTTL(log, genKey(toplevelTmpKey, uid, issuer))
	return err
}

func (otps *tokenservice) checkTempToken(log *zap.Logger, issuer, uid string) (string, error) {
	return otps.store.GetTTL(log, genKey(toplevelTmpToken, uid, issuer))
}

func (otps *tokenservice) removeTempToken(log *zap.Logger, issuer, uid string) {
	otps.store.Del(log, genKey(toplevelTmpToken, uid, issuer))
}

func (otps *tokenservice) newTempToken(log *zap.Logger, issuer, uid, val string, dur time.Duration) error {
	return otps.store.PutTTL(log, genKey(toplevelTmpToken, uid, issuer), val, dur)
}

func (otps *tokenservice) hasUser(log *zap.Logger, issuer, uid string) (bool, error) {
	_, err := otps.store.Get(log, genKey(toplevelKey, uid, issuer))
	if err != nil {
		if errors.Is(err, ErrNoKey) {
			return false, nil
		}
		return false, err
	}
	return true, nil
}

func (otps *tokenservice) validateUser(log *zap.Logger, issuer, uid, token string) (bool, error) {
	val, err := otps.store.Get(log, genKey(toplevelKey, uid, issuer))
	if err != nil {
		if errors.Is(err, ErrNoKey) {
			return false, fmt.Errorf("no registration for user: %q", uid)
		}
		return false, err
	}
	var res otpRegistration
	err = json.Unmarshal([]byte(val), &res)
	if err != nil {
		return false, err
	}
	if !totp.Validate(token, res.PrivateKey) {
		return false, nil
	}
	return true, nil
}

func (otps *tokenservice) qrImage(log *zap.Logger, uid, regkey string) (string, error) {
	val, err := otps.store.GetTTL(log, genKey(toplevelTmpKey, uid, regkey))
	if err != nil {
		return "", err
	}
	var res otpRegistration
	err = json.Unmarshal([]byte(val), &res)
	if err != nil {
		return "", err
	}

	var buf bytes.Buffer
	key, err := otp.NewKeyFromURL(res.URL)
	if err != nil {
		return "", fmt.Errorf("cannot load key URL: %w", err)
	}
	img, err := key.Image(200, 200)
	if err != nil {
		return "", fmt.Errorf("cannot create QR image: %w", err)
	}
	err = png.Encode(&buf, img)
	if err != nil {
		return "", fmt.Errorf("cannot encode QR image: %w", err)
	}
	return base64.StdEncoding.EncodeToString(buf.Bytes()), nil
}

func (otps *tokenservice) newTempRegistration(log *zap.Logger, issuer, uid string) (string, error) {
	key := randomKey(randKeyLen)

	seckey, err := totp.Generate(totp.GenerateOpts{
		Issuer:      issuer,
		AccountName: uid,
	})
	if err != nil {
		return "", fmt.Errorf("cannot create secret key: %w", err)
	}

	reg := otpRegistration{
		PrivateKey: seckey.Secret(),
		URL:        seckey.URL(),
	}
	val, err := json.Marshal(reg)
	if err != nil {
		return "", fmt.Errorf("cannot marshal to json: %w", err)
	}

	// store a TTL value only with uid/issuer, so we can detect if this user already has created a tmp-registration
	_ = otps.store.PutTTL(log, genKey(toplevelTmpKey, uid, issuer), string(val), time.Duration(otps.settings.OTP.Timeout))

	return key, otps.store.PutTTL(log, genKey(toplevelTmpKey, uid, key), string(val), time.Duration(otps.settings.OTP.Timeout))
}

func genKey(prefix, uid, key string) string {
	return prefix + uid + ":" + key
}

func randomKey(ln int) string {
	buf := make([]byte, ln)
	_, _ = rand.Read(buf)
	return base64.StdEncoding.EncodeToString(buf)
}
