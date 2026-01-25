# Go 语言基础

## 简介

Go（又称 Golang）是 Google 开发的一种静态强类型、编译型、并发型编程语言。Go 语言于 2009 年正式对外发布，目标是在保持编译速度的同时提供高效的运行性能。

## 核心特性

### 1. 简洁的语法

Go 语言的语法设计简洁明了：

```go
package main

import "fmt"

func main() {
    fmt.Println("Hello, World!")
}
```

### 2. 并发支持

Go 内置了强大的并发原语 - Goroutine 和 Channel：

```go
// 创建 goroutine
go func() {
    // 并发执行的代码
}()

// 使用 channel 通信
ch := make(chan int)
ch <- 42      // 发送
value := <-ch // 接收
```

### 3. 接口

Go 的接口是隐式实现的，非常灵活：

```go
type Reader interface {
    Read(p []byte) (n int, err error)
}

// 任何实现了 Read 方法的类型都自动实现了 Reader 接口
```

### 4. 错误处理

Go 使用显式的错误返回而非异常：

```go
func doSomething() error {
    if problem {
        return errors.New("something went wrong")
    }
    return nil
}

if err := doSomething(); err != nil {
    log.Fatal(err)
}
```

## 常用工具

### go mod - 依赖管理

```bash
go mod init myproject    # 初始化模块
go mod tidy             # 整理依赖
go get package@version  # 添加依赖
```

### go test - 测试

```bash
go test ./...           # 运行所有测试
go test -v ./...        # 详细输出
go test -cover ./...    # 查看覆盖率
```

### go build - 编译

```bash
go build ./cmd/server   # 编译
go install ./...        # 安装到 GOPATH/bin
```

## 最佳实践

1. **保持包小而专注**：每个包应该有单一职责
2. **使用接口抽象**：面向接口编程，而非具体实现
3. **处理所有错误**：不要忽略 error 返回值
4. **使用 context**：管理请求生命周期和取消
5. **编写测试**：使用 table-driven tests
6. **使用 gofmt**：保持代码风格一致

