# Demo script

Scripted walkthrough for a recording (asciinema or GIF). Run each block from the repo root. Narration lines are comments — say them or caption them, don't type them.

```bash
# 1. Build
go build -o bin/heimdall .

# 2. Validate the config before starting anything — catches bad YAML,
#    missing fields, and duplicate credentials before they become a runtime problem.
./bin/heimdall validate --config examples/basic/heimdall.yaml

# 3. Start the gateway (leave this running in this pane)
./bin/heimdall serve --config examples/basic/heimdall.yaml
```

Second pane:

```bash
# 4. Call it as `claude-code` (user: alice) — the API key identifies the client credential,
#    which resolves to alice's identity
curl -sS -X POST http://127.0.0.1:9090/mcp \
  -H "X-API-KEY: my-secret-key-alice" \
  -H "Content-Type: application/json" \
  -H "Accept: application/json, text/event-stream" \
  -d '{"jsonrpc":"2.0","id":1,"method":"initialize","params":{"protocolVersion":"2024-11-05","capabilities":{},"clientInfo":{"name":"curl-demo","version":"1"}}}'

# 5. Call it with no key at all — rejected before it ever reaches policy
curl -sS -X POST http://127.0.0.1:9090/mcp \
  -H "Content-Type: application/json" \
  -d '{"jsonrpc":"2.0","id":1,"method":"initialize","params":{}}'

# 6. Ask policy directly what it would decide for alice vs. an unknown user,
#    without touching the running gateway at all
./bin/heimdall policy check \
  --policy examples/basic/policy.yaml \
  --user alice --action tools/call --resource-kind tool --resource-name everything.list_files

./bin/heimdall policy check \
  --policy examples/basic/policy.yaml \
  --user mallory --action tools/call --resource-kind tool --resource-name everything.list_files
```

Step 6's second call should print `DENY` — default-deny, no rule grants `mallory` anything. That contrast (step 4's success vs. step 6's explicit deny) is the actual point of the demo: identity determines what's visible and callable, not just whether the server responds.

## Recording checklist

- [ ] Record with `asciinema rec docs/demo.cast` or a GIF tool — not committed here yet
- [ ] Trim dead time between commands
- [ ] Link the recording from the README once it exists
