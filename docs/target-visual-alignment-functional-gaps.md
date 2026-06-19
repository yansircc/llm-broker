# Target visual alignment functional gaps

## Scope

This pass aligns the public site and customer console to the referenced target project's visual system and copy while preserving this repo's route invariant:

- public UI remains `/`
- customer UI remains `/app/*`
- admin UI remains `/console/*`

The target site uses `/dashboard`, `/keys`, `/purchase`, and `/zh/*`. This branch maps those surfaces onto the existing `/app/*` customer routes instead of changing the runtime route contract.

## Visual-only pages or controls

| Surface | Current state | Missing functional backend |
| --- | --- | --- |
| `/app/key-test` | Visual Key testing form and result area | API key probe endpoint, model truth validation, latency/result reporting |
| `/app/images` | Visual OpenAI Images-compatible docs and pricing cards | Image provider driver, image request relay path, image-key grouping, per-image billing |
| `/app/subscriptions` | Visual subscription status and monthly plan cards | Subscription plans, daily quota ledger, renewal/cancel lifecycle, payment binding |
| `/app/redeem` | Visual redeem-code form | Redeem-code table, validation endpoint, idempotent ledger credit write |
| `/app/referrals/earnings` | Visual commission ledger empty state | Commission ledger, referred-customer list, withdrawal workflow, settlement status |
| `/app/login` | Visual forgot-password link is present but disabled | Password reset request/token/confirm flow |
| `/blog` | Static visual blog cards | CMS or post storage/rendering |
| `/partner` and `/contact` | Static visual partner and WeChat tutorial entry | Public partner application workflow and configured own WeChat contact/QR code |

## Partial visual alignment over existing data

| Surface | Existing truth reused | Target feature not yet present |
| --- | --- | --- |
| `/app/dashboard` | `/me`, `/keys`, `/billing/summary`, `/usage`, `/payments/orders`, `/referrals` | June stamp campaign state, VIP tier thresholds, detailed campaign reward state |
| `/app/keys` | Existing API key CRUD, status, budget fields | Key plaintext persistence/copyback, key groups, expiry, target-style rate-limit model |
| `/app/usage` | Existing request usage logs | First-token latency and real User-Agent display |
| `/app/billing` | Existing one-time payment order creation | Monthly subscription purchase, selectable payment method propagation, and USDT-specific payment handling |
| `/app/orders` | Existing order list and status filter | Payment-method filter and target-style method metadata |
| `/app/referrals` | Existing referral code/url/signups/reward summary | Tiered affiliate rates, customer table, withdrawal request |
| `/app/settings` | Existing account read and password change | Username update endpoint |

## Implementation boundary

Do not add these missing behaviors by storing duplicate frontend state. Each should enter through one source of truth:

- key test results should come from a probe endpoint, not client-only inference
- image generation should be a provider driver/relay/billing path, not a UI-only request path
- subscriptions should be a ledger/quota model, not a second balance boolean
- redeem codes should write billing ledger rows idempotently
- affiliate earnings should derive from payment/referral facts, not manually maintained display totals
