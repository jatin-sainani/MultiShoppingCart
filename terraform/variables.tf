variable "aws_region" {
  type    = string
  default = "us-east-1"
}

variable "project_name" {
  type    = string
  default = "multi-backend-cart"
}

variable "container_port" {
  type    = number
  default = 8080
}

variable "task_cpu" {
  type    = number
  default = 256
}

variable "task_memory" {
  type    = number
  default = 512
}

variable "api_desired_count" {
  type    = number
  default = 1
}

variable "alb_ingress_cidr_blocks" {
  type    = list(string)
  default = ["0.0.0.0/0"]
}

variable "store_backend" {
  type    = string
  default = "dynamodb"
}

variable "mysql_database" {
  type    = string
  default = "shopping"
}

variable "mysql_username" {
  type    = string
  default = "cartadmin"
}

variable "mysql_allocated_storage" {
  type    = number
  default = 20
}

variable "mysql_max_open_conns" {
  type    = number
  default = 10
}

variable "mysql_max_idle_conns" {
  type    = number
  default = 5
}

variable "mysql_conn_max_idle_time" {
  type    = string
  default = "5m"
}

variable "mysql_conn_max_lifetime" {
  type    = string
  default = "30m"
}

variable "log_retention_days" {
  type    = number
  default = 7
}
