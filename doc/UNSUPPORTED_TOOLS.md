# 不支持的工具处理规范

## 概述

本文档描述了kiro2api如何处理上游CodeWhisperer不支持的工具调用。

## 当前不支持的工具

- `web_search` - 网络搜索工具
- `websearch` - 网络搜索工具（变体名称）

## 实现逻辑

### 1. 工具定义过滤（OpenAI → Anthropic）

**位置**: `converter/tools.go` -> `validateAndProcessTools()`

**行为**:
- 在将OpenAI格式的工具定义转换为Anthropic格式时，静默过滤掉`web_search`和`websearch`工具
- 过滤是透明的，不返回错误，不影响其他有效工具的处理

**代码示例**:
```go
// 过滤不支持的工具：web_search (静默过滤，不发送到上游)
if tool.Function.Name == "web_search" || tool.Function.Name == "websearch" {
    continue
}
```

### 1.1 工具定义过滤（Anthropic → CodeWhisperer）

**位置**: `converter/codewhisperer.go` -> `BuildCodeWhispererRequest()`

**行为**:
- 在将Anthropic格式的工具定义转换为CodeWhisperer格式时，静默过滤掉`web_search`和`websearch`工具
- 确保`userInputMessageContext.tools`数组中不包含不支持的工具

**代码示例**:
```go
// 过滤不支持的工具：web_search (静默过滤，不发送到上游)
if tool.Name == "web_search" || tool.Name == "websearch" {
    continue
}
```

### 2. 消息历史处理

**位置**: `converter/tools.go` -> `convertContentBlock()`

**行为**:
- 静默过滤消息内容中名称为`web_search`或`websearch`的`tool_use`块
- 对应的`tool_result`块目前保留（因为无法直接判断tool_use_id对应的工具名称）
- 过滤通过返回`nil, nil`实现，上层`convertOpenAIContentToAnthropic`检测到nil会跳过该块
- 不产生任何错误或警告

**代码示例**:
```go
case "tool_use":
    // 过滤不支持的web_search工具调用（静默过滤，返回nil表示跳过）
    if name, ok := block["name"].(string); ok {
        if name == "web_search" || name == "websearch" {
            return nil, nil
        }
    }
    return block, nil
```

### 3. 历史消息中的工具调用过滤

**位置**: `converter/codewhisperer.go` -> `extractToolUsesFromMessage()`

**行为**:
- 在从助手消息中提取工具使用记录时，也过滤掉`web_search`和`websearch`工具
- 确保历史消息中不包含不支持的工具调用记录

### 4. 错误处理机制

**现有机制**: `parser/tool_lifecycle_manager.go` -> `HandleToolCallError()`

虽然目前不会主动生成web_search错误（因为被过滤的工具不会被调用），但系统已具备工具错误处理能力：

```go
type ToolCallError struct {
    ToolCallID string `json:"tool_call_id"`
    Error      string `json:"error"`
}
```

## 响应规范

### 正常流程

当用户定义包含web_search的工具列表时：

**请求**:
```json
{
  "model": "claude-sonnet-4-20250514",
  "messages": [...],
  "tools": [
    {"type": "function", "function": {"name": "get_weather", ...}},
    {"type": "function", "function": {"name": "web_search", ...}},
    {"type": "function", "function": {"name": "calculator", ...}}
  ]
}
```

**发送到上游**（仅包含支持的工具）:
```json
{
  "model": "...",
  "messages": [...],
  "tools": [
    {"name": "get_weather", ...},
    {"name": "calculator", ...}
  ]
}
```

### 历史消息处理

**请求**（包含web_search的历史消息）:
```json
{
  "model": "claude-sonnet-4-20250514",
  "messages": [
    {
      "role": "user",
      "content": "Search for information about Go"
    },
    {
      "role": "assistant",
      "content": [
        {"type": "text", "text": "Let me search for that."},
        {"type": "tool_use", "id": "call_123", "name": "web_search", "input": {...}},
        {"type": "tool_use", "id": "call_456", "name": "calculator", "input": {...}}
      ]
    }
  ]
}
```

**发送到上游**（过滤掉web_search相关块）:
```json
{
  "messages": [
    {
      "role": "user",
      "content": "Search for information about Go"
    },
    {
      "role": "assistant",
      "content": [
        {"type": "text", "text": "Let me search for that."},
        {"type": "tool_use", "id": "call_456", "name": "calculator", "input": {...}}
      ]
    }
  ]
}
```

### 客户端行为

客户端应该理解：
1. 如果AI需要web_search功能，它无法通过kiro2api的CodeWhisperer后端使用
2. AI可能会用其他方式（如纯文本）回应，或使用其他可用工具
3. 客户端可以在本地实现web_search工具，并通过消息循环执行
4. 历史消息中的web_search工具调用会被自动过滤，不会发送到上游

## 扩展性

要添加更多不支持的工具：

1. 在`validateAndProcessTools`函数中添加过滤条件：
```go
if tool.Function.Name == "web_search" || 
   tool.Function.Name == "websearch" ||
   tool.Function.Name == "new_unsupported_tool" {
    continue
}
```

2. 可选：添加相应的测试用例

## 测试

相关测试位于 `converter/tools_test.go`:

**工具定义过滤测试**:
- `TestValidateAndProcessTools_FilterWebSearch` - 测试基本过滤功能
- `TestValidateAndProcessTools_FilterWebSearchVariant` - 测试变体名称过滤

**消息历史过滤测试**:
- `TestConvertOpenAIContentToAnthropic_FilterWebSearchInHistory` - 测试消息中web_search过滤
- `TestConvertOpenAIContentToAnthropic_FilterWebSearchVariantInHistory` - 测试消息中websearch变体过滤

运行测试：
```bash
go test ./converter -v -run "Filter"
```

## 相关文件

- `converter/tools.go` - OpenAI到Anthropic的工具转换和过滤逻辑
- `converter/codewhisperer.go` - Anthropic到CodeWhisperer的工具转换和过滤逻辑
- `converter/tools_test.go` - 测试用例
- `parser/tool_lifecycle_manager.go` - 工具生命周期管理（包含错误处理）
- `parser/event_stream_types.go` - 事件类型定义（包含`ToolCallError`）

## 注意事项

1. **静默过滤**: 当前实现不通知客户端哪些工具被过滤，这是设计决策，避免增加客户端复杂度
2. **全面过滤**: 工具定义和消息历史中的web_search都会被过滤，确保不发送不支持的工具到上游
3. **tool_result处理**: 目前tool_result块不会被过滤（因为无法直接判断其对应的工具名），但由于tool_use已被过滤，实际不会产生新的web_search相关的tool_result
4. **未来改进**: 可以考虑在响应中添加警告信息，告知客户端某些工具不可用

## 更新日志

- 2025-01-XX: 初始实现，添加web_search过滤功能
