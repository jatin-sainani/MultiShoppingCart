output "alb_dns_name" {
  value       = aws_lb.api.dns_name
  description = "Public ALB DNS name for the shopping cart API"
}

output "mysql_endpoint" {
  value       = aws_db_instance.mysql.address
  description = "RDS MySQL endpoint"
}

output "mysql_database" {
  value       = var.mysql_database
  description = "RDS database name"
}

output "mysql_username" {
  value       = var.mysql_username
  description = "RDS database username"
}

output "dynamodb_table_name" {
  value       = aws_dynamodb_table.shopping_carts.name
  description = "DynamoDB table used for shopping carts"
}
