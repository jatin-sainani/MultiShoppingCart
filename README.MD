# Multi Backend

Multi Backend is a Go shopping cart service with swappable persistence layers. The same API can run against in-memory storage for local development, MySQL for relational consistency, or DynamoDB for managed key-value storage.

## Highlights

- Single API surface across `memory`, `mysql`, and `dynamodb` backends
- Local-first development with no cloud dependency in memory mode
- Terraform deployment for ECS, ALB, RDS MySQL, DynamoDB, ECR, and networking
- Result-generation scripts for backend comparison and consistency probes
- OpenAPI definition in `src/api.yaml`

## Repository Layout

- `src/` Go service, tests, Docker build, and OpenAPI spec
- `terraform/` AWS infrastructure for app and data stores
- `scripts/` repeatable comparison and consistency helpers

## Local Development

```powershell
cd src
go test ./...
go run .
```

The default backend is `memory`, so the service starts without AWS resources.

## Switching Backends

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

## Deployment

```powershell
cd terraform
terraform init
terraform apply
```

Use `store_backend=mysql` or `store_backend=dynamodb` to select the backing store for the ECS task definition. Generated benchmark JSON and Terraform state are intentionally excluded from version control.
