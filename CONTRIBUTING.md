# Contributing

感谢你愿意改进 Branchopus。

## 提交问题

请先搜索现有 Issue。新 Issue 尽量包含：

- Windows 版本与 Branchopus 版本。
- 可复现的目录规模和操作步骤。
- 实际表现与预期表现。
- 必要时附截图；不要上传包含敏感路径或文件名的截图。

## 提交代码

1. Fork 仓库并创建独立分支。
2. 保持改动聚焦，避免同时重构无关代码。
3. 后端改动需要补充或更新 Go 测试。
4. 前端改动需要通过 TypeScript 与生产构建检查。
5. 提交 Pull Request 前运行：

```powershell
go test ./...
cd frontend
npm ci
npm run build
```

提交信息建议使用简短的祈使句，例如 `Fix search branch expansion`。
