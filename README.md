# SerialTerminalForWindowsTerminal

[English](#english) | [中文](#chinese)

---

## English

A cross-platform serial terminal with TUI, charset conversion, TCP/UDP forwarding, Lua plugins, and file transfer support.

### Features

- **Serial communication** with full port configuration (baud, data bits, stop bits, parity)
- **Hex mode** for binary protocol inspection with configurable frame size and timestamps
- **Charset conversion** — e.g., read GBK device output as UTF-8 in your terminal
- **TCP/UDP forwarding** — broadcast serial data to multiple servers, receive from any
- **Lua plugin system** — transform input/output data or intercept commands with Lua scripts
- **File transfer** via trzsz / lrzsz protocols
- **TUI mode** (`-g`) with Bubble Tea interface: viewport, input bar, modal panels
- **Console mode** (default) with dot-command prefix (`.` at line start)
- **Interactive setup wizard** when no port is specified

### Quick Start

```bash
go build -o sterm ./cmd/serialterminal

# Connect to serial port
./sterm -p COM8 -b 115200

# With charset conversion (device outputs GBK, terminal shows UTF-8)
./sterm -p COM8 -b 115200 -o GBK

# Hex mode
./sterm -p COM8 -b 115200 -i hex

# TUI mode
./sterm -p COM8 -b 115200 -g

# With TCP forwarding
./sterm -p COM8 -f 1 -a 127.0.0.1:12345

# Interactive (no port specified)
./sterm
```

### CLI Flags

| Short | Long | Type | Default | Description |
|---|---|---|---|---|
| `-p` | `--port` | string | `""` | Serial port (`/dev/ttyUSB0`, `COMx`) |
| `-b` | `--baud` | int | `115200` | Baud rate |
| `-d` | `--data` | int | `8` | Data bits (5/6/7/8) |
| `-s` | `--stop` | int | `0` | Stop bits (0:1, 1:1.5, 2:2) |
| `-v` | `--verify` | int | `0` | Parity (0:none, 1:odd, 2:even, 3:mark, 4:space) |
| `-o` | `--out` | string | `UTF-8` | Output charset |
| `-i` | `--in` | string | `UTF-8` | Input charset (use `hex` for hex mode) |
| `-e` | `--end` | string | `\n` | Line ending sent to device |
| `-F` | `--Frame` | int | `16` | Hex frame size |
| `-g` | `--gui` | bool | `false` | Enable TUI mode |
| `-k` | `--hotkey-mod` | string | `ctrl+alt` | Hotkey modifier (`ctrl+alt` or `ctrl+shift`) |
| `-f` | `--forward` | []int | `nil` | Forward mode (1:TCP, 2:UDP, repeatable) |
| `-a` | `--address` | []string | `nil` | Forward address (repeatable) |
| `-l` | `--log` | string | `""` | Log file path |
| `-t` | `--time` | string | `""` | Timestamp format |

### Dot Commands

In console mode, type `.` at line start to enter command mode:

| Command | Description |
|---|---|
| `.help` | Show command help |
| `.exit` | Exit the terminal |
| `.hex <data>` | Send raw hex bytes |
| `.forward list\|add\|remove\|enable\|disable\|update` | Manage forwarding |
| `.plugin list\|load\|unload\|enable\|disable\|reload` | Manage Lua plugins |
| `.mode show\|set <field> <value>` | View or change runtime settings |

### Plugin System

Create `.lua` files and load them with `.plugin load <path>`:

```lua
-- Transform outgoing data (append marker)
function OnInput(payload)
  return payload .. "\r\n"
end

-- Transform incoming data (add prefix)
function OnOutput(payload)
  return "[DEV] " .. payload
end

-- Intercept or modify commands (return false to block)
function OnCommand(line)
  return line, true
end
```

Plugins chain: each enabled plugin sees the output of the previous one. Return `nil` to drop data.

### Architecture

```
cmd/serialterminal/           # Entry point
internal/
  termapp/                    # Core application (App, TUI, console, commands)
  config/                     # Configuration types
  session/                    # Serial port + trzsz lifecycle
  event/                      # UI event types
  flag/                       # CLI flag parsing + interactive wizard
pkg/
  charset/                    # Charset conversion utilities
  forward/                    # TCP/UDP forwarding manager
  luaplugin/                  # Lua plugin engine
```

---

## 中文

一款跨平台串口终端，支持 TUI 界面、编码转换、TCP/UDP 转发、Lua 插件和文件传输。

### 功能特性

- **串口通信** — 完整端口配置（波特率、数据位、停止位、校验位）
- **Hex 模式** — 二进制协议调试，可配置帧大小和时间戳
- **双向编码转换** — 如设备输出 GBK，终端显示 UTF-8
- **TCP/UDP 数据转发** — 串口数据广播至多台服务器，任一台可回传
- **Lua 插件系统** — 使用 Lua 脚本转换输入/输出数据或拦截命令
- **文件传输** — 支持 trzsz / lrzsz 协议
- **TUI 界面** (`-g`) — 基于 Bubble Tea，带视口、输入栏、模态面板
- **控制台模式** — 行首 `.` 进入命令模式，支持 Tab 补全
- **交互配置向导** — 不带端口参数时自动启动

### 快速开始

```bash
go build -o sterm ./cmd/serialterminal

# 连接串口
./sterm -p COM8 -b 115200

# 编码转换（设备输出 GBK，终端显示 UTF-8）
./sterm -p COM8 -b 115200 -o GBK

# Hex 模式
./sterm -p COM8 -b 115200 -i hex

# TUI 模式
./sterm -p COM8 -b 115200 -g

# TCP 转发
./sterm -p COM8 -f 1 -a 127.0.0.1:12345

# 交互式（不指定端口）
./sterm
```

### CLI 参数

| 短参 | 长参 | 类型 | 默认值 | 说明 |
|---|---|---|---|---|
| `-p` | `--port` | string | `""` | 串口设备 (`/dev/ttyUSB0`、`COMx`) |
| `-b` | `--baud` | int | `115200` | 波特率 |
| `-d` | `--data` | int | `8` | 数据位 |
| `-s` | `--stop` | int | `0` | 停止位 (0:1, 1:1.5, 2:2) |
| `-v` | `--verify` | int | `0` | 校验 (0:无, 1:奇, 2:偶, 3:1, 4:0) |
| `-o` | `--out` | string | `UTF-8` | 输出编码 |
| `-i` | `--in` | string | `UTF-8` | 输入编码 (`hex` 开启 Hex 模式) |
| `-e` | `--end` | string | `\n` | 发送到设备的换行符 |
| `-F` | `--Frame` | int | `16` | Hex 帧大小 |
| `-g` | `--gui` | bool | `false` | 启用 TUI 界面 |
| `-k` | `--hotkey-mod` | string | `ctrl+alt` | 快捷键修饰 (`ctrl+alt` 或 `ctrl+shift`) |
| `-f` | `--forward` | []int | `nil` | 转发模式 (1:TCP, 2:UDP, 可多次传入) |
| `-a` | `--address` | []string | `nil` | 转发地址 (可多次传入) |
| `-l` | `--log` | string | `""` | 日志文件路径 |
| `-t` | `--time` | string | `""` | 时间戳格式 |

### 点命令

控制台模式下，行首输入 `.` 进入命令模式：

| 命令 | 说明 |
|---|---|
| `.help` | 显示帮助 |
| `.exit` | 退出终端 |
| `.hex <数据>` | 发送原始 Hex 字节 |
| `.forward list\|add\|remove\|enable\|disable\|update` | 管理转发 |
| `.plugin list\|load\|unload\|enable\|disable\|reload` | 管理 Lua 插件 |
| `.mode show\|set <字段> <值>` | 查看或修改运行时设置 |

### 插件系统

编写 `.lua` 文件，通过 `.plugin load <路径>` 加载：

```lua
-- 转换输出数据（追加换行）
function OnInput(payload)
  return payload .. "\r\n"
end

-- 转换输入数据（添加前缀）
function OnOutput(payload)
  return "[DEV] " .. payload
end

-- 拦截命令（返回 false 阻止执行）
function OnCommand(line)
  return line, true
end
```

插件链式执行，每个启用的插件接收上一个插件的输出。返回 `nil` 可丢弃数据。

### 架构说明

```
cmd/serialterminal/           # 入口点
internal/
  termapp/                    # 核心应用（App、TUI、控制台、命令）
  config/                     # 配置类型
  session/                    # 串口 + trzsz 生命周期
  event/                      # UI 事件类型
  flag/                       # CLI 参数解析 + 交互向导
pkg/
  charset/                    # 编码转换工具
  forward/                    # TCP/UDP 转发管理
  luaplugin/                  # Lua 插件引擎
```
