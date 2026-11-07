# ShipSmoke

**验证用户实际下载的软件。**

[![CI](https://github.com/geyutu6755/shipsmoke/actions/workflows/ci.yml/badge.svg)](https://github.com/geyutu6755/shipsmoke/actions/workflows/ci.yml)
[![Release](https://img.shields.io/github/v/release/geyutu6755/shipsmoke)](https://github.com/geyutu6755/shipsmoke/releases)
[![License: MIT](https://img.shields.io/badge/License-MIT-blue.svg)](LICENSE)

文档语言：[English](README.md)

ShipSmoke 是一个免费、开源、跨平台的命令行工具，用于验证软件发行包 ——
**ZIP**、**tar.gz** 和独立可执行文件 —— 能否在目标操作系统上真正解压、启动
并完成基本功能。它是一个发布质量门禁：损坏的发行包会在 CI 中被拦截，而不是
被用户发现。

```console
$ shipsmoke check

shipsmoke v1.0.3 — verification report
host: windows/amd64   config: .shipsmoke.yml   duration: 822ms

  [SKIP] gokit-unix — artifact targets [linux/amd64, linux/arm64, darwin/amd64, darwin/arm64] but host is windows/amd64
  gokit-windows  format=zip (detected: windows/amd64)
    [PASS] check #1: gokit.exe --version (475ms)

  summary: 1 passed, 0 failed, 1 skipped, 0 errored  →  PASS

json report: dist/shipsmoke-report.json
```

*（已发布的 v1.0.3 二进制在 Windows 机器上验证
[`examples/go-cli`](examples/go-cli) 的真实输出。SKIP 正是重点：不面向当前操作
系统的产物会被如实报告为「未验证」，而不是被算作通过。在 Linux 与 macOS 运行器
上，同一份配置会运行 tar.gz 并跳过 zip。）*

## 为什么选择 ShipSmoke

校验和只能证明上传过程没有损坏，不能证明压缩包里的程序能运行。ShipSmoke
与 GoReleaser（其 `verify` 为 Pro 付费功能，且只运行用户自定义命令、不运行
被测二进制）、cosign（签名）、GitHub artifact attestation（来源证明）互补：

- **在真实操作系统上真实执行** —— 解压发行包、定位入口程序、以子进程运行
  可配置的检查（退出码、stdout/stderr 匹配、超时）。不伪造平台名称：每项
  检查都在操作系统/架构真正匹配的 GitHub 托管 runner 上执行。
- **基于二进制元数据而非文件名猜测** —— 通过解析真实的 ELF / PE / Mach-O
  文件头（Go 标准库）识别入口程序。
- **默认安全** —— Zip Slip 路径穿越双重防护、压缩包条目数/单文件/总体积/
  压缩比流式限制、拒绝符号链接/硬链接/设备节点、参数数组直接启动（绝不
  经过 Shell）、超时进程树击杀、报告中敏感值自动脱敏。
- **独立且免费** —— 不绑定构建工具，无付费 API、无云端、无账户。单一静态
  二进制；唯一第三方依赖是 YAML 解析器。
- **CI 原生** —— 清晰的退出码规范、机器可读 JSON 报告、GitHub Actions
  Job Summary 开箱即用。

## 安装

**下载安装（推荐）：** 从
[Releases](https://github.com/geyutu6755/shipsmoke/releases) 下载对应平台
的压缩包并解压。

```bash
# Linux x86-64
curl -fsSL -o shipsmoke.tar.gz \
  https://github.com/geyutu6755/shipsmoke/releases/latest/download/shipsmoke_linux_amd64.tar.gz
tar -xzf shipsmoke.tar.gz && ./shipsmoke version

# Linux ARM64
curl -fsSL -o shipsmoke.tar.gz \
  https://github.com/geyutu6755/shipsmoke/releases/latest/download/shipsmoke_linux_arm64.tar.gz
tar -xzf shipsmoke.tar.gz && ./shipsmoke version

# macOS Apple Silicon
curl -fsSL -o shipsmoke.tar.gz \
  https://github.com/geyutu6755/shipsmoke/releases/latest/download/shipsmoke_darwin_arm64.tar.gz
tar -xzf shipsmoke.tar.gz && ./shipsmoke version

# macOS Intel
curl -fsSL -o shipsmoke.tar.gz \
  https://github.com/geyutu6755/shipsmoke/releases/latest/download/shipsmoke_darwin_amd64.tar.gz
tar -xzf shipsmoke.tar.gz && ./shipsmoke version
```

```powershell
# Windows x86-64（PowerShell）
curl.exe -fsSL -o shipsmoke.zip `
  https://github.com/geyutu6755/shipsmoke/releases/latest/download/shipsmoke_windows_amd64.zip
Expand-Archive shipsmoke.zip -DestinationPath . && .\shipsmoke.exe version
```

**包管理器：**

```bash
# Homebrew（macOS/Linux）
brew install geyutu6755/tap/shipsmoke

# Scoop（Windows）
scoop bucket add geyutu6755 https://github.com/geyutu6755/scoop-bucket
scoop install shipsmoke

# Go
go install github.com/geyutu6755/shipsmoke/cmd/shipsmoke@latest
```

tap 与 bucket 在每次推送时都会于 CI 中真实安装并测试
（[tap 运行记录](https://github.com/geyutu6755/homebrew-tap/actions)、
[bucket 运行记录](https://github.com/geyutu6755/scoop-bucket/actions)）。

每个发布压缩包在发布前都由 ShipSmoke 本身在真实 runner 上验证过（见
release 工作流）。

## 快速开始

```bash
cd your-project
shipsmoke init        # 生成带注释的 .shipsmoke.yml
shipsmoke validate    # 只校验配置，不执行任何程序
shipsmoke check       # 质量门禁；退出码驱动 CI 决策
```

完整的 `.shipsmoke.yml` 示例：

```yaml
version: 1

report:
  json: shipsmoke-report.json        # 机器可读报告（可选）

artifacts:
  - name: mytool-linux-amd64
    path: dist/mytool-linux-amd64.tar.gz
    format: auto                     # auto | zip | tar.gz | binary
    binary: mytool                   # 入口程序，相对压缩包根目录
    runs_on: [linux/amd64]           # 只在真正能运行的平台上验证
    checks:
      - args: ["--version"]
        expect:
          exit_code: 0
          stdout_contains: ["mytool version"]
        timeout: 30s

  - name: mytool-windows-amd64
    path: dist/mytool-windows-amd64.zip
    binary: mytool.exe
    runs_on: [windows/amd64]
    checks:
      - args: ["--version"]
        expect: { exit_code: 0 }
```

在 `linux/amd64` runner 上：第一个发行包被解压并执行；第二个被标记为
**SKIP** 并给出原因。没有执行过的检查永远不会被报告为通过。

完整 Schema 与全部匹配器（`stdout_contains`、`stdout_not_contains`、
`equals`、`regex`、`stderr_*`）：[docs/CONFIGURATION.md](docs/CONFIGURATION.md)。

## 在 GitHub Actions 中使用

为你的项目生成一份完整、经过预验证的配置：

```console
$ shipsmoke init --ci --stack go --name mytool
generated files for the go stack (tool "mytool"):
  .shipsmoke.yml          — 验证配置
  .github/workflows/shipsmoke.yml — 三平台 CI：构建、打包、shipsmoke 门禁
```

支持的栈明确为 `go`、`rust`、`c` 三种（ShipSmoke 不推断构建系统）。生成的
workflow 与 [shipsmoke-integration](https://github.com/geyutu6755/shipsmoke-integration)
中验证过的形状一致：构建 → 打包（Linux/macOS 用 `tar.gz`，Windows 用
`zip`）→ 在 ubuntu-24.04、macos-15、windows-2025 上 `shipsmoke` 验证。

Go CLI 的手写等价版本——可直接复制，构建与验证步骤内联（无辅助脚本）：

```yaml
name: shipsmoke
on:
  push:
    branches: [main]
  pull_request:
permissions:
  contents: read
jobs:
  verify:
    name: Verify release artifact (${{ matrix.os }})
    strategy:
      fail-fast: false
      matrix:
        os: [ubuntu-24.04, macos-15, windows-2025]
    runs-on: ${{ matrix.os }}
    steps:
      - uses: actions/checkout@11d5960a326750d5838078e36cf38b85af677262 # v4
      - uses: actions/setup-go@40f1582b2485089dde7abd97c1529aa768e1baff # v5
        with:
          go-version: 'stable'
      - name: Build and package
        shell: bash
        run: |
          set -euo pipefail
          EXE=""
          [ "$RUNNER_OS" = "Windows" ] && EXE=".exe"
          mkdir -p dist
          go build -o "dist/mytool$EXE" .
          if [ "$RUNNER_OS" = "Windows" ]; then
            powershell -NoProfile -Command "Compress-Archive -Path dist/mytool.exe -DestinationPath dist/mytool.zip -Force"
          else
            tar -czf dist/mytool.tar.gz -C dist mytool
          fi
      - name: Verify artifact with shipsmoke
        uses: geyutu6755/shipsmoke@v1
        with:
          config: .shipsmoke.yml
          report: dist/shipsmoke-report.json
          version: v1.0.4
      - name: 仅在验证通过后发布
        if: success()
        run: echo "全部产物验证通过 —— 在此把 dist/ 附加到你的 release"
```

任何检查失败都会使 verify 步骤失败（退出码 1），从而阻断依赖它的步骤。
Action 会写入 Job Summary 表格并上传 JSON 报告。完整教程：
[docs/TUTORIAL_GO.md](docs/TUTORIAL_GO.md)、
[docs/TUTORIAL_RUST.md](docs/TUTORIAL_RUST.md)、
[docs/TUTORIAL_C.md](docs/TUTORIAL_C.md)。

**Action 安全模型（v1.0.2 起）：**

- 用户输入仅通过环境变量进入 Shell 代码 —— 所有 `run:` 块内不存在任何
  `${{ inputs.* }}` 插值，恶意输入值无法以脚本注入方式执行（三平台均有
  自动化注入回归测试覆盖）。
- Action 下载发行包时会**先对照该版本的 `checksums.txt` 校验 SHA-256，再
  解压**；下载失败、清单缺失条目或哈希不匹配都会在二进制被执行之前终止
  任务。`verified: checksum` / `verified: user-provided` 作为 step 输出暴露。
- `version:` 接受 `latest` 或完整标签 `vX.Y.Z`；`v1` 这类可移动引用在
  Action 内会被拒绝（`uses:` 中可用其选择 Action 本身，工具版本另行固定）。
  两者都固定 —— 例如 `uses: geyutu6755/shipsmoke@v1.0.2` 加
  `version: v1.0.2` —— 可获得完全可复现的门禁。

两种接入模式（对应两个不同的问题，不能互相替代）：

1. **构建后、公开发布前验证** —— 构建打包后运行 ShipSmoke，通过才发布：
   `shipsmoke init --ci --stack go|rust|c --name TOOL`。它**拦住**一次坏发布。
2. **发布后重新下载验证** —— 发布完成后下载用户真正会拿到的资产再跑一遍：
   `shipsmoke init --ci --published --name TOOL --asset-linux … --asset-darwin …
   --asset-windows …`。它**发现**一次已经发生的坏发布。

已有自建发布流水线的第三方项目，只有第 2 种模式对他们有意义——他们不会为了接入
本工具重写成第 1 种。配方与字段说明见
[docs/CONFIGURATION.md](docs/CONFIGURATION.md#post-release-verification)。

[examples/](examples/) 里有完整且经过 CI 验证的 Go、Rust、C 三个示例项目。

Action 还在**另一个仓库**
[shipsmoke-integration](https://github.com/geyutu6755/shipsmoke-integration)
中以「外部使用者」的身份被端到端验证：它消费已发布的 `v1` 标签而非本仓库源码，
覆盖三个操作系统，并包含故意破坏产物、要求门禁失败的作业。需要注意：该仓库仍由
同一维护者所有，按 [docs/ADOPTION.md](docs/ADOPTION.md) 的分层属于 **Tier 2，
不算第三方采用**。ShipSmoke 目前没有 Tier 3 采用者，这里如实说明而不加以暗示。

## 退出码

| 退出码 | 含义 |
|---|---|
| 0 | 所有已执行的检查全部通过 |
| 1 | 至少一项检查失败（或某个发行包无法验证） |
| 2 | 配置或用法错误（YAML 非法、未知字段、配置缺失） |
| 3 | 没有任何检查被执行 —— 全部被跳过；CI 不应视为通过（`--allow-empty` 可覆盖） |

状态：**PASS**（已执行且符合预期）、**FAIL**（已执行但不符合预期）、
**SKIP**（未执行 —— 发行包面向其他平台，始终标注原因）、**ERROR**
（基础设施问题：文件缺失、压缩包不安全、无法启动）。

## 安全性

ShipSmoke 会执行程序。它对可控的一切做了加固 —— Zip Slip 路径穿越双重
防护、流式压缩包限制（条目数/单文件/总体积/压缩比）、显式拒绝符号链接/
硬链接/设备节点、参数数组启动（无 Shell、无注入）、超时进程树击杀、输出
捕获上限、报告中敏感环境变量脱敏 —— 但被验证的二进制以你的工作流权限运行。
**它不是操作系统级沙箱。** 请验证你自己构建的发行包；对不可信的第三方
发行包请使用没有部署密钥的临时 runner。详见
[docs/THREAT_MODEL.md](docs/THREAT_MODEL.md) 与 [SECURITY.md](SECURITY.md)。

## 文档

- [一分钟快速入门](#在-github-actions-中使用) 与教程（[Go](docs/TUTORIAL_GO.md) / [Rust](docs/TUTORIAL_RUST.md) / [C](docs/TUTORIAL_C.md)）
- [配置参考](docs/CONFIGURATION.md) —— 完整 Schema、匹配器、CLI 参数、CI 配方
- [三层验证模型](docs/VERIFICATION_LAYERS.md)、[供应链边界](docs/SUPPLY_CHAIN.md)
- [案例研究](docs/CASE_STUDIES/git-graph-v0.8.3-macos-arch-mismatch.md) —— 真实 macos-amd64/arch 失配
- [第三方接入模板](docs/THIRD_PARTY_TEMPLATE.md)、[试用计划](docs/TRIAL_PLAN.md)
- [系统设计](docs/SYSTEM_DESIGN.md)、[威胁模型](docs/THREAT_MODEL.md)
- [已知限制](docs/LIMITATIONS.md)、[故障排查](docs/TROUBLESHOOTING.md)
- [验证报告](docs/VALIDATION_REPORT.md) —— 每项声明均附 CI 运行链接
- [最终验收](docs/FINAL_ACCEPTANCE.md)、[维护指南](docs/MAINTENANCE_GUIDE.md)
- [维护者介绍](docs/INTRODUCTION.md)、[候选项目](docs/CANDIDATES.md)、[采用跟踪](docs/ADOPTION.md)

## 开发

```bash
go test ./...           # 单元 + 集成（测试夹具在测试内交叉编译）
go vet ./...
gofmt -l .
```

依赖仅 `gopkg.in/yaml.v3`。CI 在 `ubuntu-24.04`、`macos-15`（arm64）、
`windows-2025` 上运行完整测试套件。

## 许可证

[MIT](LICENSE)
