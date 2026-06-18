# Deploy Targets

Each `*.env` file in this directory is a deploy target.

Use an explicit target for every deploy or rollback:

```bash
DEPLOY_TARGET=cdx bash .agents/skills/deploy/scripts/deploy.sh
DEPLOY_TARGET=cdx bash .agents/skills/deploy/scripts/restore.sh latest
```

List known targets:

```bash
bash .agents/skills/deploy/scripts/deploy.sh targets
```

Target files should contain only non-secret deploy routing values such as:

```bash
REMOTE=root@example-host
SITE=https://example.com
SERVICE=llm-broker
DEPLOY_STRATEGY=legacy
```

Runtime secrets still belong on the server in `/etc/llm-broker.env`.
