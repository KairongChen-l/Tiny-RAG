# Database Package - GORM 工具包

本包提供了统一的 GORM 数据库连接和事务管理工具，确保代码的一致性和最佳实践。

## 功能特性

- ✅ **优化的连接池配置**：自动配置连接池参数
- ✅ **统一的事务管理**：简化事务处理，自动处理回滚
- ✅ **上下文支持**：支持 context.Context 传递
- ✅ **Prepared Statement 缓存**：提升性能
- ✅ **错误处理**：统一的错误处理和 panic 恢复

## 使用方法

### 创建数据库连接

```go
import "github.com/krc/rag/pkg/database"

db, err := database.NewGORM(database.Config{
    DSN: "user:password@tcp(localhost:3306)/dbname?charset=utf8mb4&parseTime=True&loc=Local",
    MaxOpenConns: 25,        // 可选，默认 25
    MaxIdleConns: 5,         // 可选，默认 5
    ConnMaxLifetime: 5 * time.Minute,  // 可选，默认 5 分钟
    ConnMaxIdleTime: 10 * time.Minute, // 可选，默认 10 分钟
})
if err != nil {
    log.Fatal(err)
}
```

### 使用事务

#### 简单事务（无 context）

```go
err := database.Transaction(db, func(tx *gorm.DB) error {
    // 执行数据库操作
    if err := tx.Create(&user).Error; err != nil {
        return err // 自动回滚
    }
    return nil // 自动提交
})
```

#### 带 context 的事务

```go
err := database.TransactionWithContext(db, ctx, func(tx *gorm.DB) error {
    // 执行数据库操作
    if err := tx.WithContext(ctx).Create(&user).Error; err != nil {
        return err // 自动回滚
    }
    return nil // 自动提交
})
```

## 配置说明

### 连接池参数

- **MaxOpenConns**: 最大打开连接数（默认：25）
  - 建议值：`(最大并发请求数) / (每个请求的平均数据库操作数)`
  
- **MaxIdleConns**: 最大空闲连接数（默认：5）
  - 建议值：`MaxOpenConns / 5`

- **ConnMaxLifetime**: 连接最大生存时间（默认：5 分钟）
  - 防止连接长时间占用，建议设置为应用重启间隔的一半

- **ConnMaxIdleTime**: 连接最大空闲时间（默认：10 分钟）
  - 自动关闭长时间未使用的连接

### GORM 配置

- **PrepareStmt**: 启用 prepared statement 缓存（默认：true）
  - 提升重复查询的性能

- **Logger**: 日志级别（默认：Silent）
  - 使用 zap logger 进行日志记录，而不是 GORM 内置日志

## 最佳实践

1. **使用事务处理多个操作**
   ```go
   err := database.Transaction(db, func(tx *gorm.DB) error {
       if err := tx.Create(&doc).Error; err != nil {
           return err
       }
       if err := tx.Create(&chunks).Error; err != nil {
           return err
       }
       return nil
   })
   ```

2. **始终传递 context**
   ```go
   err := database.TransactionWithContext(db, ctx, func(tx *gorm.DB) error {
       return tx.WithContext(ctx).Create(&doc).Error
   })
   ```

3. **错误处理**
   - 事务函数返回错误时自动回滚
   - 发生 panic 时自动回滚并重新 panic

4. **连接池调优**
   - 根据实际负载调整连接池参数
   - 监控数据库连接数，避免连接耗尽

## 示例

完整示例请参考 `internal/index/mysql/store.go`。

