# whatsbot [![golangci-lint](https://github.com/empiricalhq/bot/actions/workflows/golangci-lint.yml/badge.svg)](https://github.com/empiricalhq/bot/actions/workflows/golangci-lint.yml) [![CodeQL](https://github.com/empiricalhq/bot/actions/workflows/codeql.yml/badge.svg)](https://github.com/empiricalhq/bot/actions/workflows/codeql.yml)

We use golang and golangci-lint as tools. You can install them via mise with:

```bash
mise install
```

To install the dependencies, run:

```bash
mise run install
```

To start the bot, run:

```bash
mise run dev
```

You might need to use Windows Terminal or VSCode terminal to see the QR correctly.

## Conversation flow

Every build embeds the repository's `conversation.json`. The bot reads the file at `FLOW_FILE_PATH` (default `conversation.json`) when it exists, so you can edit the flow without rebuilding. When that file is missing, the bot uses the embedded copy and logs which path it did not find. A file that exists but is unreadable or invalid is an error; the bot never falls back for it.
