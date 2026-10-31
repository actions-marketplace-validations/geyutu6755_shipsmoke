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

shipsmoke v1.0.0 — verification report
host: ubuntu-24.04/amd64   config: .shipsmoke.yml   duration: 912ms

  mytool-linux-amd64  format=tar.gz (detected: linux/amd64)
    [PASS] check #1: mytool --version (118ms)
    [PASS] check #2: mytool --help (104ms)
  [SKIP] mytool-windows-amd64 — artifact targets [windows/amd64] but host is linux/amd64

  summary: 2 passed, 0 failed, 1 skipped, 0 errored  →  PASS

json report: shipsmoke-report.json
```

*（取自自验证 CI 运行的真实终端输出。）*

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
# Linux / macOS 示例
curl -fsSL -o shipsmoke.tar.gz \
  https://github.com/geyutu6755/shipsmoke/releases/latest/download/shipsmoke_linux_amd64.tar.gz
tar -xzf shipsmoke.tar.gz && ./shipsmoke version
```

**使用 Go 安装：**

```bash
go install github.com/geyutu6755/shipsmoke/cmd/shipsmoke@latest
```

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

最小配方 —— 在与发行包匹配的 runner 上验证：

```yaml
jobs:
  verify:
    strategy:
      matrix:
        include:
          - { os: ubuntu-24.04, target: linux/amd64 }
          - { os: macos-15,     target: darwin/arm64 }
          - { os: windows-2025, target: windows/amd64 }
    runs-on: ${{ matrix.os }}
    permissions:
      contents: read
    steps:
      - uses: actions/checkout@v4
      - name: 在此构建（或下载）你的发行包
        run: ./build.sh
      - uses: geyutu6755/shipsmoke@v1
        with:
          config: .shipsmoke.yml
          report: shipsmoke-report.json
      - name: 仅在验证通过后发布
        if: success()
        run: ./publish.sh
```

任何检查失败都会使该步骤失败（退出码 1），从而阻断发布步骤。Action 还会
写入 Job Summary 表格并上传 JSON 报告。

两种接入模式：

1. **构建后、公开发布前验证**（V1.0 完整支持）—— 构建打包后运行
   ShipSmoke，通过才发布。
2. **发布后重新下载验证**（文档提供配方）—— 发布完成后下载已发布的资产
   再跑一遍。见
   [docs/CONFIGURATION.md](docs/CONFIGURATION.md#post-release-verification)。

[examples/](examples/) 里有完整且经过 CI 验证的 Go、Rust、C 三个示例项目。

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

- [配置参考](docs/CONFIGURATION.md) —— 完整 Schema、匹配器、CI 配方
- [系统设计](docs/SYSTEM_DESIGN.md)、[威胁模型](docs/THREAT_MODEL.md)
- [已知限制](docs/LIMITATIONS.md)、[故障排查](docs/TROUBLESHOOTING.md)
- [验证报告](docs/VALIDATION_REPORT.md) —— 每项声明均附 CI 运行链接

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
