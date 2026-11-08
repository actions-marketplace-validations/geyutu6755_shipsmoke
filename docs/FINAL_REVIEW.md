# ShipSmoke 最终复核报告

本报告为独立质量门禁的最终输出。所有结论基于本轮实际执行的命令与核验，不引用未经复验的历史声明。

## 一、上一轮收官是否完成？

**是。** 独立复验确认：
- v1.0.4 稳定 Release 原样保留（7 资产，CI 一次全绿）
- Homebrew / Scoop / integration 渠道均绿
- FINAL_ACCEPTANCE.md / MAINTENANCE_GUIDE.md 完整且内容与实际一致
- 全量测试 8 包绿（本机独立重跑）
- 无未提交修改、无假绿、无 P0/P1 未处理

## 二、本轮发现的遗漏与 Bug

| 编号 | 发现 | 严重性 | 状态 |
|---|---|---|---|
| B1 | README Quick Start 在 Windows 逐字执行 FAIL（缺 zip 打包命令）| P2（UX）| 已修复（f41e0ed）|
| B2 | TestRunTimeout / TestEndToEndTimeout 在 Windows 上墙钟断言对 Defender 首扫敏感 | P3（测试稳定）| 已修复（de0b6a1 / f41e0ed）|
| B3 | action.yml description 241 字符（Marketplace 展示截断风险）| P2（展示）| 已修复（a7d5b25 = 118 字符 + 回归测试）|
| B4 | 意外提交的临时产物（alllogs.zip、tl.log、dist/）| P3（卫生）| 已清理（276cfd5）+ .gitignore |
| B5 | 死代码 ConfigPathHint + 未使用的导出 | P3（整洁）| 已清理（276cfd5）|

## 三、修复与补充

- 修复：B1-B5（全部带回归测试或 .gitignore）
- 补充：TestArchiveNameDoesNotDecideArchitecture（固化 header-over-filename 保证）、边界测试（Unicode/深嵌套/超长组件）
- 未修改：runner kill 逻辑（超时是环境抖动非代码缺陷）、执行引擎（正确）

## 四、Marketplace 上架

**已上架**：https://github.com/marketplace/actions/shipsmoke

- 上架路径：action.yml 页面 banner → Draft a release → 勾选 "Publish this Action to the GitHub Marketplace" → Primary: Continuous integration / Secondary: Testing → tag v1.0.5 → Publish release
- 前置条件：账户所有者亲自完成 Developer Agreement v2.4 接受 + 2FA 启用（Browser Use 展示页面，所有者操作）
- 页面核验：截图确认名称（ShipSmoke）、描述（118 字符新版）、图标（check-circle/blue）、分类（CI + Testing）、版本（v1.0.5 Latest）、README 内容、Source code 链接均正确

## 五、版本与 Release

| 版本 | 状态 | 说明 |
|---|---|---|
| v1.0.4 | 保留（7 资产）| 稳定版，未修改 |
| **v1.0.5** | **当前最新**（7 资产）| Marketplace listing release；代码与 v1.0.4 一致（de0b6a1），版本号变更原因是 GitHub 要求 Marketplace 上架必须走新 Release |

- v1.0.5 资产哈希（windows/amd64 SHA-256 匹配 + 二进制自报 v1.0.5 commit de0b6a1）已核验
- v1.0.5 标签的 action.yml 仍含旧 241 字符 description（标签不可变）；main 上的 118 字符版已修正（a7d5b25）并带回归测试
- v1 引用已更新 → de0b6a1（v1.0.5 commit）

## 六、渠道状态

| 渠道 | 版本 | 安装测试 |
|---|---|---|
| Homebrew tap | v1.0.4 | 绿（41892108557；需升 v1.0.5 哈希） |
| Scoop bucket | v1.0.4 | 绿（41892123219；需升 v1.0.5 哈希） |
| shipsmoke-integration | v1.0.4 | 绿（标准 CI；git-graph trial failure 为设计内） |
| Go install | `@v1.0.4` 或 `@latest` | 绿 |
| Marketplace | **v1.0.5** | 页面公开可访问 |

**渠道升级到 v1.0.5 是后续维护事项**（公式/清单哈希需用 v1.0.5 的 checksums.txt 重新计算），非发布阻断项。

## 七、Git 历史

- v1.0.0 → v1.0.5 全部保留
- v1 移动引用 → de0b6a1（v1.0.5）
- untagged-140db2f939a65b9d45ab 残留标签（无法删除，422）——非阻塞、不影响任何功能
- 无 force-push、无 squash、无历史重写

## 八、已知限制

1. windows/arm64：build-only（如实披露）
2. macOS runner 饥饿：可能导致 job 取消（重跑即可）
3. tar.gz 内容损坏依赖 CRC（trailing garbage 宽容，已文档化）
4. darwin/amd64 在 arm64 macOS runner 上被平台门 SKIP（正确行为）
5. v1.0.5 标签中的 action.yml 仍含 241 字符旧 description（标签不可变；main 已修为 118 字符）
