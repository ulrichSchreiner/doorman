package doorman

import (
	"errors"
	"fmt"
	"testing"

	"github.com/caddyserver/caddy/v2"
	"github.com/go-ldap/ldap"
	"go.uber.org/zap"
)

type dummyconnection struct {
	result *ldap.SearchResult
	err    error
}

func (dc *dummyconnection) Search(searchRequest *ldap.SearchRequest) (*ldap.SearchResult, error) {
	return dc.result, dc.err
}

func (dc *dummyconnection) Bind(username, password string) error {
	return nil
}

func (dc *dummyconnection) Close() {}

func createInit(res *ldap.SearchResult, err error) configInitialize {
	return func(_ *zap.Logger, cfg *ldapConfiguration) error {
		cfg.connection = &dummyconnection{result: res, err: err}
		return nil
	}
}

func Test_ldapConfiguration_initWithDefaults(t *testing.T) {
	ld := (&ldapConfiguration{}).init(caddy.NewReplacer())

	if ld.UIDAttribute != defaultUID {
		t.Errorf("UIDAttribute is %q but should be %q", ld.UIDAttribute, defaultUID)
	}
	if ld.MobileAttribute != defaultMobile {
		t.Errorf("MobileAttribute is %q but should be %q", ld.MobileAttribute, defaultUID)
	}
	if ld.EMailAttribute != defaultEMail {
		t.Errorf("EMailAttribute is %q but should be %q", ld.EMailAttribute, defaultUID)
	}
	if ld.NameAttribute != defaultName {
		t.Errorf("NameAttribute is %q but should be %q", ld.NameAttribute, defaultUID)
	}
}

func createLDAPSearchResult(m map[string]string, num int) *ldap.SearchResult {
	if m == nil {
		return nil
	}

	var atts []*ldap.EntryAttribute
	for k, v := range m {
		atts = append(atts, &ldap.EntryAttribute{
			Name:   k,
			Values: []string{v},
		})
	}

	var entries []*ldap.Entry
	for i := 0; i < num; i++ {
		entries = append(entries, &ldap.Entry{
			Attributes: atts,
		})
	}
	return &ldap.SearchResult{
		Entries: entries,
	}
}

func Test_search(t *testing.T) {
	tests := []struct {
		name            string
		attributes      map[string]string
		uid             string
		username        string
		mobile          string
		mail            string
		uidattribute    string
		mobileattribute string
		nameattribute   string
		mailattribute   string
		numentries      int
		wantErr         bool
		wantNoUserErr   bool
		returnErr       bool
	}{
		{
			name:       "search for user with all attributes",
			attributes: map[string]string{defaultUID: "test", defaultEMail: "my@mail.com", defaultMobile: "123123", defaultName: "test user"},
			uid:        "test",
			mobile:     "123123",
			mail:       "my@mail.com",
			username:   "test user",
			numentries: 1,
		},
		{
			name:            "remapped user attributes",
			attributes:      map[string]string{"myuid": "test", "mymail": "my@mail.com", "mymobile": "123123", "myname": "test user"},
			uid:             "test",
			mobile:          "123123",
			mail:            "my@mail.com",
			username:        "test user",
			uidattribute:    "myuid",
			mobileattribute: "mymobile",
			nameattribute:   "myname",
			mailattribute:   "mymail",
			numentries:      1,
		},
		{
			name:          "search with empty results",
			attributes:    make(map[string]string),
			wantNoUserErr: true,
			numentries:    0,
		},
		{
			name:          "search with too many results",
			attributes:    make(map[string]string),
			wantNoUserErr: true,
			numentries:    2,
		},
		{
			name:       "search returns error",
			numentries: 0,
			wantErr:    true,
			returnErr:  true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			var reserr error
			if tt.returnErr {
				reserr = fmt.Errorf("simple error")
			}
			initConnection = createInit(createLDAPSearchResult(tt.attributes, tt.numentries), reserr)
			ld := (&ldapConfiguration{}).init(caddy.NewReplacer())
			if tt.uidattribute != "" {
				ld.UIDAttribute = tt.uidattribute
			}
			if tt.mobileattribute != "" {
				ld.MobileAttribute = tt.mobileattribute
			}
			if tt.nameattribute != "" {
				ld.NameAttribute = tt.nameattribute
			}
			if tt.mailattribute != "" {
				ld.EMailAttribute = tt.mailattribute
			}
			ue, err := ld.Search(zap.NewNop(), tt.uid)
			if err != nil {
				if errors.Is(err, ErrNoUser) != tt.wantNoUserErr {
					t.Errorf("got error: %v but wanted a ErrNoUser", err)
				} else if !(tt.wantErr || tt.wantNoUserErr) {
					t.Errorf("got error: %v", err)
				}
			} else {
				if err == nil {
					if ue.UID != tt.uid {
						t.Errorf("got userid %q, but wanted: %q", ue.UID, tt.uid)
					}
					if ue.Name != tt.username {
						t.Errorf("got username %q, but wanted: %q", ue.Name, tt.username)
					}
					if ue.Mobile != tt.mobile {
						t.Errorf("got mobile %q, but wanted: %q", ue.Mobile, tt.mobile)
					}
					if ue.EMail != tt.mail {
						t.Errorf("got mail %q, but wanted: %q", ue.EMail, tt.mail)
					}
				}
			}
		})
	}

}
