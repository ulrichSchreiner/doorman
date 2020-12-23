package doorman

import (
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"os"
	"os/exec"
	"sync"

	"github.com/caddyserver/caddy/v2"
	"go.uber.org/zap"
	"gopkg.in/fsnotify.v1"
)

var (
	_ userSearcher = (*userlistBackend)(nil)
	_ userSearcher = (*userSearchCommand)(nil)

	ErrNoUser       = fmt.Errorf("no user found")
	ErrNoConnection = fmt.Errorf("no connection")

	valueCommandSearcher = "command"
	valueListSearcher    = "list"
	valueLdapSearcher    = "ldap"
	valueFileSearcher    = "file"
)

type UserEntry struct {
	Name   string `json:"name,omitempty"`
	UID    string `json:"uid"`
	Mobile string `json:"mobile"`
	EMail  string `json:"email"`
}

type userSearcher interface {
	Search(log *zap.Logger, uid string) (*UserEntry, error)
}

type userlistBackend []UserEntry

func (ulb *userlistBackend) Search(log *zap.Logger, uid string) (*UserEntry, error) {
	for _, u := range *ulb {
		if u.UID == uid {
			return &u, nil
		}
	}
	return nil, fmt.Errorf("%w: %s", ErrNoUser, uid)
}

type userSearchCommand struct {
	Command  string   `json:"command"`
	Args     []string `json:"args,omitempty"`
	UseStdin bool     `json:"use_stdin"`
}

func newUserSearchCommand(cfg userSearchCommand, rpl *caddy.Replacer) *userSearchCommand {
	cfg.Command = rpl.ReplaceKnown(cfg.Command, "")
	for i, a := range cfg.Args {
		cfg.Args[i] = rpl.ReplaceKnown(a, "")
	}
	return &cfg
}

func (usc *userSearchCommand) Search(log *zap.Logger, uid string) (*UserEntry, error) {
	args := append([]string{}, usc.Args...)
	if !usc.UseStdin {
		args = append(args, uid)
	}
	c := exec.Command(usc.Command, args...)
	stdin, err := c.StdinPipe()
	if err != nil {
		return nil, err
	}

	go func() {
		defer stdin.Close()
		if usc.UseStdin {
			_, _ = io.WriteString(stdin, uid)
		}
	}()

	out, err := c.CombinedOutput()
	if err != nil {
		return nil, err
	}
	var res UserEntry
	if err = json.Unmarshal(out, &res); err != nil {
		return nil, fmt.Errorf("the command did not return the correct json format for a user entry (got: %s): %w", string(out), err)
	}
	return &res, nil
}

type userfileBackend struct {
	lock  sync.RWMutex
	Path  string `json:"path"`
	Watch bool   `json:"watch"`
	data  userlistBackend
}

func (ufb *userfileBackend) start(log *zap.Logger) error {
	if err := ufb.load(); err != nil {
		return err
	}
	if ufb.Watch {
		watcher, err := fsnotify.NewWatcher()
		if err != nil {
			return err
		}
		err = watcher.Add(ufb.Path)
		if err != nil {
			return err
		}
		go func() {
			defer watcher.Close()
			log.Info("start watcher", zap.String("path", ufb.Path))
			for {
				select {
				case event, ok := <-watcher.Events:
					if !ok {
						return
					}
					if event.Op&fsnotify.Write == fsnotify.Write {
						log.Info("user file changed", zap.String("path", ufb.Path), zap.String("op", event.Op.String()))
						err := ufb.load()
						if err != nil {
							log.Error("cannot reload user file", zap.Error(err))
						}
					}
				case err, ok := <-watcher.Errors:
					if !ok {
						return
					}
					log.Error("watch error occurred", zap.Error(err))
				}
			}
		}()
	}
	return nil
}

func (ufb *userfileBackend) Search(log *zap.Logger, uid string) (*UserEntry, error) {
	ufb.lock.RLock()
	defer ufb.lock.RUnlock()

	return ufb.data.Search(log, uid)
}

func (ufb *userfileBackend) load() error {
	f, err := os.Open(ufb.Path)
	if err != nil {
		return fmt.Errorf("cannot open file (%s) for reading: %w", ufb.Path, err)
	}
	defer f.Close()

	ufb.lock.Lock()
	defer ufb.lock.Unlock()

	return json.NewDecoder(f).Decode(&ufb.data)
}

type userBackends struct {
	searchers []userSearcher
}

func fromUserSpecs(log *zap.Logger, bks Plugins) (*userBackends, error) {
	r := caddy.NewReplacer()
	res := userBackends{}
	for _, b := range bks {
		switch b.Type {
		case valueCommandSearcher:
			var d userSearchCommand
			if err := json.Unmarshal(b.Spec, &d); err != nil {
				return nil, fmt.Errorf("cannot unmarshal search command: %w", err)
			}
			res.searchers = append(res.searchers, newUserSearchCommand(d, r))
		case valueListSearcher:
			var d userlistBackend
			if err := json.Unmarshal(b.Spec, &d); err != nil {
				return nil, fmt.Errorf("cannot unmarshal userlist: %w", err)
			}
			res.searchers = append(res.searchers, &d)
		case valueFileSearcher:
			var d userfileBackend
			if err := json.Unmarshal(b.Spec, &d); err != nil {
				return nil, fmt.Errorf("cannot unmarshal userlist: %w", err)
			}
			if err := d.start(log); err != nil {
				return nil, fmt.Errorf("cannot start userfile backend: %w", err)
			}
			res.searchers = append(res.searchers, &d)
		case valueLdapSearcher:
			var d ldapConfiguration
			if err := json.Unmarshal(b.Spec, &d); err != nil {
				return nil, fmt.Errorf("cannot unmarshal ldap config: %w", err)
			}
			res.searchers = append(res.searchers, d.init(r))
		default:
			return nil, fmt.Errorf("unknown user backend: %q", b.Type)
		}
	}
	return &res, nil
}

func (ulb *userBackends) Search(log *zap.Logger, uid string) (*UserEntry, error) {
	for _, b := range ulb.searchers {
		u, err := b.Search(log, uid)
		if err == nil {
			return u, nil
		}
		if !errors.Is(err, ErrNoUser) {
			return nil, err
		}
	}
	return nil, fmt.Errorf("%w: %s", ErrNoUser, uid)
}
