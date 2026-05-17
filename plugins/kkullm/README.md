# kkullm plugin

Claude Code plugin for [kkullm](https://github.com/joelhelbling/kkullm), the
blackboard-pattern agent-orchestration system.

It ships one skill:

- **`/kkullm:cli`** — explains the purpose and conventions of the `kkullm` CLI
  and how operators and agents use it to drive a board.

## Install

```
/plugin marketplace add joelhelbling/kkullm
/plugin install kkullm@kkullm
/reload-plugins
```

Then invoke the skill with `/kkullm:cli`, or let Claude activate it
automatically when it detects kkullm CLI work.
