package doorman

import (
	"encoding/json"
	"fmt"
	"net"

	"go.uber.org/zap"
)

var (
	_ whitelistLoader = (*staticWhiteList)(nil)

	valueWhiteListListLoader = "list"
)

type IPMap map[string]bool
type Networks []*net.IPNet

type Whitelist struct {
	ips  IPMap
	nets Networks
}

type whitelistLoader interface {
	Fetch(log *zap.Logger) (*Whitelist, error)
}

type staticWhiteList []string

func parseStaticList(sw []string) (IPMap, Networks, error) {
	statics := make(IPMap)
	nets := make(Networks, 0)
	for _, we := range sw {
		iip, ipnet, err := net.ParseCIDR(we)
		if err != nil {
			ip := net.ParseIP(we)
			if ip == nil {
				return nil, nil, fmt.Errorf("cannot parse %q as ip", we)
			}
			statics[we] = true
		} else {
			// if we have an ipv4 with a 32bit mask, this is a singular ip
			if ones, bits := ipnet.Mask.Size(); ones == 32 && bits == 32 {
				statics[iip.String()] = true
				continue
			}
			nets = append(nets, ipnet)
		}
	}
	return statics, nets, nil
}

func (sw *staticWhiteList) Fetch(log *zap.Logger) (*Whitelist, error) {
	ips, nets, err := parseStaticList(*sw)
	if err != nil {
		return nil, err
	}
	return &Whitelist{
		ips:  ips,
		nets: nets,
	}, nil
}

func (w *Whitelist) IsAllowed(log *zap.Logger, clip string) bool {
	if ok := w.ips[clip]; ok {
		return true
	}
	ip := net.ParseIP(clip)
	if ip == nil {
		return false
	}

	for _, n := range w.nets {
		if n.Contains(ip) {
			log.Debug("ip is whitelisted", zap.String("net", n.String()), zap.String("ip", clip))
			return true
		}
	}
	return false
}

type whitelister struct {
	loader     []whitelistLoader
	whitelists []*Whitelist
}

func (wl *whitelister) isAllowed(log *zap.Logger, clientip string) bool {
	for _, w := range wl.whitelists {
		if w.IsAllowed(log, clientip) {
			return true
		}
	}
	return false
}

func fromWhitelistSpecs(log *zap.Logger, bks Plugins) (*whitelister, error) {
	res := whitelister{}
	for _, b := range bks {
		switch b.Type {
		case valueWhiteListListLoader:
			var w staticWhiteList
			if err := json.Unmarshal(b.Spec, &w); err != nil {
				return nil, fmt.Errorf("cannot unmarshal static whitelister: %w", err)
			}
			res.loader = append(res.loader, &w)
			ldr, err := w.Fetch(log)
			if err != nil {
				log.Error("cannot fetch loader data", zap.String("backend", b.Name), zap.Error(err))
			} else {
				res.whitelists = append(res.whitelists, ldr)
			}
		}
	}
	return &res, nil
}
