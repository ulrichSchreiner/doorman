package doorman

import (
	"crypto/tls"
	"fmt"
	"regexp"
	"strings"
	"sync"

	"github.com/caddyserver/caddy/v2"
	"github.com/go-ldap/ldap"
	"go.uber.org/zap"
)

var (
	onlynum                        = regexp.MustCompile(`[^\w]`)
	getConnection configInitialize = initConnection
)

type configInitialize func(*zap.Logger, *ldapConfiguration) (ldapsearcher, error)

const (
	defaultUID    = "uid"
	defaultMobile = "mobile"
	defaultEMail  = "mail"
	defaultName   = "displayName"
)

type ldapConfiguration struct {
	Address         string `json:"address"`
	User            string `json:"user"`
	Password        string `json:"password"`
	SearchBase      string `json:"search_base"`
	UIDAttribute    string `json:"uid_attributes"`
	MobileAttribute string `json:"mobile_attribute"`
	EMailAttribute  string `json:"email_attribute"`
	NameAttribute   string `json:"name_attribute"`
	TLS             bool   `json:"tls"`
	InsecureSkip    bool   `json:"insecure_skip"`

	sync.Mutex
}

type ldapsearcher interface {
	Search(searchRequest *ldap.SearchRequest) (*ldap.SearchResult, error)
	Bind(username, password string) error
	Close()
}

func setdefault(s, d string) string {
	if s == "" {
		return d
	}
	return s
}

func (cfg *ldapConfiguration) init(r *caddy.Replacer) *ldapConfiguration {
	cfg.UIDAttribute = setdefault(r.ReplaceKnown(cfg.UIDAttribute, ""), defaultUID)
	cfg.MobileAttribute = setdefault(r.ReplaceKnown(cfg.MobileAttribute, ""), defaultMobile)
	cfg.EMailAttribute = setdefault(r.ReplaceKnown(cfg.EMailAttribute, ""), defaultEMail)
	cfg.NameAttribute = setdefault(r.ReplaceKnown(cfg.NameAttribute, ""), defaultName)
	cfg.Address = r.ReplaceKnown(cfg.Address, "")
	cfg.User = r.ReplaceKnown(cfg.User, "")
	cfg.Password = r.ReplaceKnown(cfg.Password, "")
	cfg.SearchBase = r.ReplaceKnown(cfg.SearchBase, "")
	return cfg
}

func (cfg *ldapConfiguration) Search(log *zap.Logger, uid string) (*UserEntry, error) {
	con, err := getConnection(log, cfg)
	if err != nil {
		return nil, err
	}
	defer con.Close()

	return cfg.search(log, con, uid)
}

// func (cfg *ldapConfiguration) updateConnection(c *ldap.Conn) {
// 	cfg.Lock()
// 	defer cfg.Unlock()
// 	cfg.connection = c
// }

func initConnection(lg *zap.Logger, cfg *ldapConfiguration) (ldapsearcher, error) {
	var con *ldap.Conn
	if cfg.TLS {
		tlsConf := &tls.Config{InsecureSkipVerify: cfg.InsecureSkip}
		c, err := ldap.DialTLS("tcp", cfg.Address, tlsConf)
		if err != nil {
			lg.Error("cannot connect to TLS server", zap.Bool("insecure", cfg.InsecureSkip), zap.Error(err))
			return nil, fmt.Errorf("%w: cannot connect to TLS ldap server", err)
		}
		con = c
	} else {
		c, err := ldap.Dial("tcp", cfg.Address)
		if err != nil {
			lg.Error("cannot connect to server", zap.Error(err))
			return nil, fmt.Errorf("%w: cannot connect to ldap server", err)
		}
		con = c
	}
	// bind with a user to make queries
	err := con.Bind(cfg.User, cfg.Password)
	if err != nil {
		lg.Error("cannot bind to server", zap.Error(err))
		con.Close()
		return nil, fmt.Errorf("%w: cannot bind to ldap server", err)
	}
	return con, nil
}

func (cfg *ldapConfiguration) search(lg *zap.Logger, con ldapsearcher, uid string) (*UserEntry, error) {
	returnattributes := []string{"dn", cfg.UIDAttribute}
	if cfg.MobileAttribute != "" {
		returnattributes = append(returnattributes, cfg.MobileAttribute)
	}
	if cfg.EMailAttribute != "" {
		returnattributes = append(returnattributes, cfg.EMailAttribute)
	}
	if cfg.NameAttribute != "" {
		returnattributes = append(returnattributes, cfg.NameAttribute)
	}

	lg.Info("search user", zap.String("base", cfg.SearchBase), zap.String("uidattribute", cfg.UIDAttribute), zap.String("user", uid), zap.Strings("attributes", returnattributes))
	// Search for the given username
	searchRequest := ldap.NewSearchRequest(
		cfg.SearchBase,
		ldap.ScopeWholeSubtree, ldap.NeverDerefAliases, 0, 0, false,
		fmt.Sprintf("(%s=%s)", cfg.UIDAttribute, uid),
		returnattributes,
		nil,
	)
	sr, err := con.Search(searchRequest)
	if err != nil {
		lg.Error("error searching for user", zap.Error(err))
		return nil, fmt.Errorf("%w: cannot connect to search ldap server", ErrNoConnection)
	}

	if len(sr.Entries) == 0 {
		return nil, fmt.Errorf("%w: %q", ErrNoUser, uid)
	}

	// there must be exactly ONE result with the given UID
	if len(sr.Entries) != 1 {
		lg.Error("there is not one exact result", zap.String("username", uid), zap.Int("num-results", len(sr.Entries)))
		return nil, fmt.Errorf("%w: more than one user found", ErrNoUser)
	}

	lg.Info("found ldap entry", zap.Any("attributes", sr.Entries[0].Attributes))

	found := &UserEntry{
		UID: uid,
	}
	for _, a := range sr.Entries[0].Attributes {
		if a.Name == cfg.MobileAttribute {
			s := a.Values[0]
			s = strings.ReplaceAll(s, "+", "00")
			s = onlynum.ReplaceAllString(s, "")
			found.Mobile = s
		}
		if a.Name == cfg.UIDAttribute {
			s := a.Values[0]
			found.UID = s
		}
		if a.Name == cfg.EMailAttribute {
			s := a.Values[0]
			found.EMail = s
		}
		if a.Name == cfg.NameAttribute {
			s := a.Values[0]
			found.Name = s
		}
	}

	return found, nil
}
