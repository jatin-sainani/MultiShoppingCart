# Shopping Cart Service

Single Go API with interchangeable `memory`, `mysql`, and `dynamodb` cart stores.

## Local run

```powershell
cd src
go test ./...
go run .
```

The default local backend is `memory`, so the service starts without AWS resources.

## Switching backends

### MySQL

```powershell
$env:STORE_BACKEND = "mysql"
$env:MYSQL_HOST = "<rds-endpoint>"
$env:MYSQL_PORT = "3306"
$env:MYSQL_DATABASE = "shopping"
$env:MYSQL_USER = "cartadmin"
$env:MYSQL_PASSWORD = "<password>"
go run .
```

### DynamoDB

```powershell
$env:STORE_BACKEND = "dynamodb"
$env:AWS_REGION = "us-east-1"
$env:DYNAMODB_TABLE_NAME = "multi-backend-cart-carts"
$env:DYNAMODB_STRONG_READS = "false"
go run .
```
