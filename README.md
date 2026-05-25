# NFT Auction Client

NFT 拍卖后端服务，用于监听以太坊区块链上的 NFT 拍卖事件，并提供 RESTful API 供前端查询拍卖数据。

## 项目结构

```
ft_auction_client/
├── config/           # 配置管理
│   └── config.go    # 环境变量和配置加载
├── database/         # 数据库操作
│   └── database.go  # MySQL 连接和迁移
├── handlers/        # HTTP 处理器
│   └── handlers.go  # RESTful API 路由处理
├── models/          # 数据模型
│   └── models.go   # GORM 模型定义
├── service/        # 业务逻辑
│   └── event_listener.go  # 区块链事件监听
├── main.go         # 应用入口
├── go.mod          # Go 模块依赖
└── .env.example    # 环境变量示例
```

## 功能特性

- **区块链事件监听**: 实时监听 NFT 拍卖合约的 AuctionCreated、BidPlaced、AuctionEnded、AuctionCanceled 事件
- **数据持久化**: 将拍卖和出价数据存储到 MySQL 数据库
- **历史同步**: 启动时自动同步历史事件
- **RESTful API**: 提供完整的拍卖查询接口
- **数据库迁移**: 自动创建数据库和表结构

## 技术栈

- **Go 1.21+**
- **Gin** - HTTP 框架
- **GORM** - ORM 库
- **go-ethereum** - 以太坊交互
- **MySQL** - 数据库

## 快速开始

### 1. 环境要求

- Go 1.21 或更高版本
- MySQL 5.7 或更高版本
- 以太坊节点 (Infura/Alchemy/本地节点)

### 2. 配置环境变量

复制 `.env.example` 为 `.env` 并修改配置：

```bash
cp .env.example .env
```

编辑 `.env` 文件：

```env
RPC_URL=https://sepolia.infura.io/v3/your-infura-key
AUCTION_CONTRACT=0x652526a13f8f808cb6db604020a788c693ad719f
NFT_CONTRACT=0x311aae3d9327c77b6d8f4a78dc9f564359808ac0
ORACLE_CONTRACT=0xf301b2639238ae39b27291de67bdfebacb3ffe86
PROXY_ADMIN=0xe532f0471636704f0ce4ca93405bfdd0a0dc5592
DB_HOST=localhost
DB_PORT=3306
DB_USER=root
DB_PASSWORD=your_password
DB_NAME=auction_db
SERVER_PORT=8080
```

### 3. 安装依赖

```bash
go mod tidy
```

### 4. 运行服务

```bash
go run main.go
```

## API 接口

### 健康检查

```
GET /health
```

响应：
```json
{
  "status": "ok"
}
```

### 获取拍卖列表

```
GET /api/v1/auctions
GET /api/v1/auctions?page=1&size=10
GET /api/v1/auctions?active=true
```

响应：
```json
{
  "total": 100,
  "page": 1,
  "size": 10,
  "data": [
    {
      "id": 1,
      "auction_id": 123,
      "nft_address": "0x...",
      "token_id": 1,
      "seller": "0x...",
      "start_time": "2024-01-01 10:00:00",
      "end_time": "2024-01-02 10:00:00",
      "starting_price": 1.5,
      "highest_bidder": "0x...",
      "highest_bid": 2.5,
      "highest_bid_usd": 5000.00,
      "is_active": true,
      "winner": "0x0000000000000000000000000000000000000000",
      "final_price": 0
    }
  ]
}
```

### 获取单个拍卖详情

```
GET /api/v1/auctions/:id
```

### 获取拍卖出价历史

```
GET /api/v1/auctions/:auctionId/bids
GET /api/v1/auctions/:auctionId/bids?page=1&size=20
```

### 获取用户的出价记录

```
GET /api/v1/bidders/:bidder/bids
```

### 获取市场统计

```
GET /api/v1/stats
```

响应：
```json
{
  "total_auctions": 100,
  "active_auctions": 25,
  "total_bids": 500,
  "highest_bid": 10.5,
  "highest_bid_usd": 21000.00
}
```

## 数据库模型

### Auction (拍卖)

| 字段 | 类型 | 说明 |
|------|------|------|
| id | uint | 主键 |
| auction_id | uint64 | 拍卖 ID (唯一索引) |
| nft_address | address | NFT 合约地址 |
| token_id | uint64 | Token ID |
| seller | address | 卖家地址 |
| start_time | datetime | 开始时间 |
| end_time | datetime | 结束时间 |
| starting_price | float64 | 起拍价 (ETH) |
| highest_bidder | address | 最高出价者 |
| highest_bid | float64 | 最高出价 (ETH) |
| highest_bid_usd | float64 | 最高出价 (USD) |
| is_active | bool | 是否活跃 |
| winner | address | 最终赢家 |
| final_price | float64 | 最终成交价 |

### Bid (出价)

| 字段 | 类型 | 说明 |
|------|------|------|
| id | uint | 主键 |
| auction_id | uint64 | 拍卖 ID |
| bidder | address | 出价者地址 |
| amount | float64 | 出价金额 (ETH) |
| amount_usd | float64 | 出价金额 (USD) |
| is_eth | bool | 是否为 ETH |
| block_number | uint64 | 区块号 |
| tx_hash | hash | 交易哈希 (唯一索引) |
| created_at | datetime | 创建时间 |

### PriceUpdate (价格更新)

| 字段 | 类型 | 说明 |
|------|------|------|
| id | uint | 主键 |
| asset | string | 资产符号 |
| price_usd | float64 | USD 价格 |
| timestamp | datetime | 时间戳 |

## 事件监听

服务启动后会：

1. 连接到以太坊节点
2. 从数据库获取上次同步的区块位置
3. 同步历史事件（如果需要）
4. 订阅新事件并实时处理

监听的事件：
- `AuctionCreated` - 拍卖创建
- `BidPlaced` - 出价
- `AuctionEnded` - 拍卖结束
- `AuctionCanceled` - 拍卖取消

## 配置说明

| 变量名 | 说明 | 默认值 |
|--------|------|--------|
| RPC_URL | 以太坊 RPC 节点 URL | https://sepolia.infura.io/v3/... |
| AUCTION_CONTRACT | 拍卖合约地址 | - |
| NFT_CONTRACT | NFT 合约地址 | - |
| ORACLE_CONTRACT | 预言机合约地址 | - |
| PROXY_ADMIN | 代理管理员地址 | - |
| DB_HOST | 数据库主机 | localhost |
| DB_PORT | 数据库端口 | 3306 |
| DB_USER | 数据库用户名 | root |
| DB_PASSWORD | 数据库密码 | password |
| DB_NAME | 数据库名称 | auction_db |
| SERVER_PORT | HTTP 服务端口 | 8080 |

## 开发

### 项目模块说明

- **config**: 配置加载，支持 .env 文件和环境变量
- **database**: 数据库连接、初始化和迁移
- **models**: GORM 数据模型定义
- **handlers**: HTTP 请求处理和响应格式化
- **service**: 区块链事件监听和业务逻辑

### 代码规范

- 使用 GORM 的 `errors.Is()` 检查错误
- 数据库查询添加超时控制
- 事件处理使用数据库事务保证数据一致性
- 响应转换使用辅助函数减少重复代码

## 许可证

MIT License