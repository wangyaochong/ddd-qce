# AI Generation Rules

Domain 代码生成规则见 [docs/ai-domain-generation-rules.md](docs/ai-domain-generation-rules.md)

## 引用方式

使用 ddd-qce 框架的项目，将以下内容添加到项目根目录的 AI 规则文件中：

- **opencode**: 复制本文件内容到项目的 `AGENTS.md`
- **Claude Code**: 复制本文件内容到项目的 `CLAUDE.md`
- **其他 AI 工具**: 将 `docs/ai-domain-generation-rules.md` 的内容添加到对应规则文件中

## Git Tag 约定

- **格式**: `v{YYYYMMDD}.v{N}`，如 `v20260529.v1`
- **递增规则**: 同一天多次发布递增 N（`v20260529.v1` → `v20260529.v2`）
- **范围**: 只为 core 打 Tag，exampleapp 不打
- **脚本**: 使用 `./scripts/tag.sh` 自动生成 Tag
  - `./scripts/tag.sh` — dry-run 预览
  - `./scripts/tag.sh --push` — 打 Tag 并推送到 origin
