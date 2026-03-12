---
name: cells
description: |
  Provision and register broker egress cells without exposing root-level infra steps in the frontend.
  Triggers: add cell, create cell, provision cell, 开 cell, 新增 cell, local cell, France cell, danted, wg-backed proxy.
---

## Cells

Treat cell creation as an ops workflow:

1. Provision the proxy endpoint on the broker host or a remote WG-backed host
2. Register the endpoint as an egress cell in broker admin API
3. Verify egress from the broker host before binding any account

### Naming

Use display names, not IDs, for the human naming convention.

- Format: `COUNTRY Provider NN`
- Local broker cells: `COUNTRY Provider NN(local)`
- Keep IDs stable and technical even if an old ID is already in production

Current naming baseline:

- `UK Linode 01(local)` for the existing broker-host local cell
- `FR Linode 01` for the existing France Linode cell

So the next cells should be:

- `UK Linode 02(local)`
- `UK Linode 03(local)`
- `FR Linode 02`
- `FR Linode 03`

Use the wrapper for the normal flow:

```bash
bash .claude/skills/cells/scripts/add_cell.sh local ...
bash .claude/skills/cells/scripts/add_cell.sh remote ...
```

What the wrapper does:

1. Provision Dante on the target host
2. Register the cell as `disabled`
3. Verify the proxy from the broker host using `https://api64.ipify.org`
4. Flip the cell to `active` unless `--leave-disabled` is set

### Local broker cell

```bash
bash .claude/skills/cells/scripts/add_cell.sh local \
  --id cell-uk-linode-02-local \
  --name "UK Linode 02(local)" \
  --listen-port 11082 \
  --ipv6 2600:3c13:e001:ae::101 \
  --label site=core-local \
  --label ipv6=2600:3c13:e001:ae::101
```

### Remote WG-backed cell

```bash
bash .claude/skills/cells/scripts/add_cell.sh remote \
  --id cell-fr-linode-02 \
  --name "FR Linode 02" \
  --target root@172.239.9.209 \
  --listen-port 12081 \
  --proxy-host 10.77.0.2 \
  --wg-bind-ip 10.77.0.2 \
  --allow-from 10.77.0.1/32 \
  --ipv6 2600:3c1a::101 \
  --install-package \
  --label country=FR \
  --label city=Paris \
  --label transport=wg-direct \
  --label ipv6=2600:3c1a::101
```

### Underlying scripts

- `provision_local_danted.sh`
- `provision_remote_danted.sh`
- `register_cell.sh`
- `verify_cell.sh`

Use the lower-level scripts when you need to reprovision or debug a single step.

Do not bind unrelated accounts during validation. Provision and verify the cell first, then use the existing migration UI or `/admin/accounts/{id}/cell` afterwards.
