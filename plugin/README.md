# Fusionaly plugin

Ask your analytics a question in the AI client you already use. "Where did my traffic come from this week?" "Why did signups drop on Tuesday?" Your agent asks your Fusionaly server, reads the numbers, and answers in plain English.

It bundles two things. The **MCP server** gives your agent four read-only tools over your analytics. The **skill** teaches it how to turn a question into the right queries and a straight answer.

Your data stays on your server. Your agent asks your instance, and only the answer reaches your AI provider. Fusionaly never talks to an AI provider itself.

Needs Fusionaly **v2.3.0 or later**. Run `sudo fusionaly update` if you are behind.

## What you get

| Tool | What it does |
|------|--------------|
| `list_websites` | Your websites and their ids |
| `whats_new` | What changed: traffic spikes and drops against the usual day, new referrers, goal spikes, trending pages |
| `get_schema` | The analytics tables and how to query them |
| `query` | One read-only SQL query, up to 1000 rows |

| Skill | What it does |
|-------|--------------|
| `fusionaly` | Answer analytics questions: pick the queries, read the results, give a verdict with numbers |
| `connect` | Connect this machine to your server: asks for the URL and key, verifies them, writes your client's config |

There is no tool that writes. The agent can look, never touch. Accounts, settings, and keys are not readable.

## First, two values

1. **Your URL**, for example `https://data.yoursite.com`.
2. **An agent key** from **Administration > Agents** in Fusionaly. It is read-only.

## Install

One plugin, the same skills everywhere. Pick your client.

### Claude Code

Three commands, **typed one at a time**, each followed by Enter:

```
/plugin marketplace add karloscodes/fusionaly-oss
```

```
/plugin install fusionaly
```

```
/fusionaly:connect
```

`/fusionaly:connect` asks for the URL and the key and checks them against your server. It saves the URL in Claude Code's settings. The key stays an environment variable, `FUSIONALY_AGENT_KEY`, so it never lands in a config file you might commit. Restart, then `/mcp` shows the server.

Without the plugin:

```bash
claude mcp add --transport http fusionaly https://data.yoursite.com/mcp \
  --header "Authorization: Bearer <agent-key>"
```

### Codex

```bash
codex plugin marketplace add karloscodes/fusionaly-oss
codex plugin add fusionaly@fusionaly
```

Then ask Codex to "connect Fusionaly". It verifies your URL and key and runs:

```bash
codex mcp add fusionaly --url https://data.yoursite.com/mcp --bearer-token-env-var FUSIONALY_AGENT_KEY
```

Codex reads the key from the `FUSIONALY_AGENT_KEY` environment variable.

### Gemini CLI

```bash
gemini extensions install https://github.com/karloscodes/fusionaly-oss
```

The install asks for your URL and agent key. Change them later with `gemini extensions config fusionaly`.

### Cursor

Import the plugin from *Dashboard > Plugins > Add Marketplace > Import from Repo* with `karloscodes/fusionaly-oss`, then ask the agent to "connect Fusionaly". Or add the server by hand in `~/.cursor/mcp.json`:

```json
{
  "mcpServers": {
    "fusionaly": {
      "url": "https://data.yoursite.com/mcp",
      "headers": { "Authorization": "Bearer ${env:FUSIONALY_AGENT_KEY}" }
    }
  }
}
```

### Claude Desktop

Claude Desktop runs local MCP servers, so bridge it with `mcp-remote`. Open **Settings > Developer > Edit Config** and add:

```json
{
  "mcpServers": {
    "fusionaly": {
      "command": "npx",
      "args": [
        "-y", "mcp-remote",
        "https://data.yoursite.com/mcp",
        "--header", "Authorization: Bearer <agent-key>"
      ]
    }
  }
}
```

Restart Claude Desktop. You need [Node.js](https://nodejs.org) for `npx`.

### VS Code, with Copilot

```bash
code --add-mcp '{"name":"fusionaly","type":"http","url":"https://data.yoursite.com/mcp","headers":{"Authorization":"Bearer <agent-key>"}}'
```

### Any other client

Every client takes either a URL with an `Authorization: Bearer <key>` header, or a stdio command:

```bash
npx -y mcp-remote https://data.yoursite.com/mcp --header "Authorization: Bearer <agent-key>"
```

### No MCP at all

Any tool that can make an HTTP request can use the SQL API with the same key. See [the skill reference](skills/fusionaly/reference.md#sql-api-without-mcp).

## Then ask

- "How is my traffic this week compared to last week?"
- "Where do my visitors come from?"
- "Which pages drive signups?"
- "Why did visitors drop yesterday?"

## If it does not connect

- **404 on `/mcp`**: the server predates v2.3.0. Run `sudo fusionaly update`.
- **401**: wrong key, or no key generated yet. Get it from Administration > Agents.
- **Nothing in the tool list**: most clients read MCP config only at startup. Restart the client.
- **The server shows up but has no tools**: the URL or the key never resolved. Run the `connect` skill (`/fusionaly:connect` in Claude Code), which verifies both before saving.

**Keep the key out of config files.** Every setup above reads it from the `FUSIONALY_AGENT_KEY` environment variable where the client allows it. Where a client needs the key in plain text (Claude Desktop, VS Code), don't commit that file, and don't keep it in a dotfiles repo.
