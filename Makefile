.oneshell:
WEBAPPS := $(dir $(wildcard **/*/webapp/package.json))
BINARY := doorman

define compile_and_run_target
    rm -rf $(HOME)/tmp/modd-$(1).conf
	mkdir -p $(HOME)/tmp
	echo -e 'test/caddy.json *.go {\n  prep +onchange: $(MAKE) $(1)\n  daemon +sigterm: $(MAKE) localrun-$(1) \n}' >> $(HOME)/tmp/modd-$(1).conf
	modd -f $(HOME)/tmp/modd-$(1).conf
endef

.PHONY:
dev: ;
	# this tasks spawns nsqd, a postgres instance a dummy webserver as backend and all the
	# daemons of this repository with wathing for file changes
	tmuxp load devsession.yml


.PHONY:
testrunner:
	goconvey -port 6081 -excludedDirs="webapp,bin,test"

.PHONY:
all: $(BINARY);

# next target is used by tmux-session specified in devsession.yml
.PHONY:
dev-%: %
	+$(call compile_and_run_target,$<)

.PHONY:
dev-frontend:
	cd webapp && npm i && npm run dev

.PHONY:
build-frontend:
	cd webapp && npm i && npm run build

.PHONY:
dev-%: %
	+$(call compile_and_run_target,$<)

.PHONY:
$(BINARY):
	mkdir -p bin
	CGO_ENABLED=0 go build -o bin/$@ ./cmd

.PHONY:
localrun-%: %
	bin/$< run --config test/caddy.json

.PHONY:
docker-image:
	docker build -t  quay.io/ulrichschreiner/doorman .
.PHONY:
second-instance:
	sed 's/:2015/:22015/g' test/caddy.json >/tmp/caddy.json
	bin/$(BINARY) run --config /tmp/caddy.json

.PHONY:
testldap:
	@docker build -t doorman-testldap -f test/Dockerfile.ldap test/
	@docker run -it --rm -p 6389:389 -p 6636:636 --name doorman-testldap doorman-testldap

.PHONY:
keydb-ephemeral:
	docker run -it --rm -p 16379:6379 --name doorman-keydb eqalpha/keydb

.PHONY:
redis-cli:
	docker exec -it doorman-keydb redis-cli

.PHONY:
install-devtools-arch:
	# install https://github.com/cortesi/modd
	# install https://github.com/tmux-python/tmuxp
	#
	# this is more or less for documentation purposes ... both tools
	# are used for local development
	sudo pacman -S community/tmuxp
	yay -S aur/modd