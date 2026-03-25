data "aws_availability_zones" "available" {
  state = "available"
}

locals {
  azs = slice(data.aws_availability_zones.available.names, 0, 2)

  api_environment = {
    AWS_REGION               = var.aws_region
    DYNAMODB_STRONG_READS    = "false"
    DYNAMODB_TABLE_NAME      = aws_dynamodb_table.shopping_carts.name
    MYSQL_CONN_MAX_IDLE_TIME = var.mysql_conn_max_idle_time
    MYSQL_CONN_MAX_LIFETIME  = var.mysql_conn_max_lifetime
    MYSQL_DATABASE           = var.mysql_database
    MYSQL_HOST               = aws_db_instance.mysql.address
    MYSQL_MAX_IDLE_CONNS     = tostring(var.mysql_max_idle_conns)
    MYSQL_MAX_OPEN_CONNS     = tostring(var.mysql_max_open_conns)
    MYSQL_PASSWORD           = random_password.mysql.result
    MYSQL_PORT               = tostring(aws_db_instance.mysql.port)
    MYSQL_USER               = var.mysql_username
    PORT                     = tostring(var.container_port)
    STORE_BACKEND            = var.store_backend
  }
}

resource "aws_vpc" "this" {
  cidr_block           = "10.8.0.0/16"
  enable_dns_hostnames = true
  enable_dns_support   = true

  tags = {
    Name = "${var.project_name}-vpc"
  }
}

resource "aws_internet_gateway" "this" {
  vpc_id = aws_vpc.this.id
}

resource "aws_subnet" "public" {
  for_each = {
    a = { cidr = "10.8.1.0/24", az = local.azs[0] }
    b = { cidr = "10.8.2.0/24", az = local.azs[1] }
  }

  vpc_id                  = aws_vpc.this.id
  cidr_block              = each.value.cidr
  availability_zone       = each.value.az
  map_public_ip_on_launch = true

  tags = {
    Name = "${var.project_name}-public-${each.key}"
  }
}

resource "aws_subnet" "private" {
  for_each = {
    a = { cidr = "10.8.10.0/24", az = local.azs[0] }
    b = { cidr = "10.8.11.0/24", az = local.azs[1] }
  }

  vpc_id            = aws_vpc.this.id
  cidr_block        = each.value.cidr
  availability_zone = each.value.az

  tags = {
    Name = "${var.project_name}-private-${each.key}"
  }
}

resource "aws_eip" "nat" {
  domain = "vpc"
}

resource "aws_nat_gateway" "this" {
  allocation_id = aws_eip.nat.id
  subnet_id     = aws_subnet.public["a"].id

  depends_on = [aws_internet_gateway.this]
}

resource "aws_route_table" "public" {
  vpc_id = aws_vpc.this.id

  route {
    cidr_block = "0.0.0.0/0"
    gateway_id = aws_internet_gateway.this.id
  }
}

resource "aws_route_table_association" "public" {
  for_each       = aws_subnet.public
  subnet_id      = each.value.id
  route_table_id = aws_route_table.public.id
}

resource "aws_route_table" "private" {
  vpc_id = aws_vpc.this.id

  route {
    cidr_block     = "0.0.0.0/0"
    nat_gateway_id = aws_nat_gateway.this.id
  }
}

resource "aws_route_table_association" "private" {
  for_each       = aws_subnet.private
  subnet_id      = each.value.id
  route_table_id = aws_route_table.private.id
}

resource "aws_security_group" "alb" {
  name        = "${var.project_name}-alb-sg"
  description = "Public ALB ingress"
  vpc_id      = aws_vpc.this.id

  ingress {
    from_port   = 80
    to_port     = 80
    protocol    = "tcp"
    cidr_blocks = var.alb_ingress_cidr_blocks
  }

  egress {
    from_port   = 0
    to_port     = 0
    protocol    = "-1"
    cidr_blocks = ["0.0.0.0/0"]
  }
}

resource "aws_security_group" "api" {
  name        = "${var.project_name}-api-sg"
  description = "API ECS tasks"
  vpc_id      = aws_vpc.this.id

  ingress {
    from_port       = var.container_port
    to_port         = var.container_port
    protocol        = "tcp"
    security_groups = [aws_security_group.alb.id]
  }

  egress {
    from_port   = 0
    to_port     = 0
    protocol    = "-1"
    cidr_blocks = ["0.0.0.0/0"]
  }
}

resource "aws_security_group" "mysql" {
  name        = "${var.project_name}-mysql-sg"
  description = "RDS ingress from ECS tasks only"
  vpc_id      = aws_vpc.this.id

  ingress {
    from_port       = 3306
    to_port         = 3306
    protocol        = "tcp"
    security_groups = [aws_security_group.api.id]
  }

  egress {
    from_port   = 0
    to_port     = 0
    protocol    = "-1"
    cidr_blocks = ["0.0.0.0/0"]
  }
}

resource "aws_cloudwatch_log_group" "api" {
  name              = "/ecs/${var.project_name}"
  retention_in_days = var.log_retention_days
}

resource "aws_ecr_repository" "api" {
  name = "${var.project_name}-api"
}

resource "aws_ecs_cluster" "this" {
  name = "${var.project_name}-cluster"
}

resource "aws_lb" "api" {
  name               = substr(replace("${var.project_name}-alb", "_", "-"), 0, 32)
  internal           = false
  load_balancer_type = "application"
  security_groups    = [aws_security_group.alb.id]
  subnets            = [for subnet in aws_subnet.public : subnet.id]
}

resource "aws_lb_target_group" "api" {
  name        = substr(replace("${var.project_name}-tg", "_", "-"), 0, 32)
  port        = var.container_port
  protocol    = "HTTP"
  target_type = "ip"
  vpc_id      = aws_vpc.this.id

  health_check {
    path    = "/health"
    matcher = "200"
  }
}

resource "aws_lb_listener" "http" {
  load_balancer_arn = aws_lb.api.arn
  port              = 80
  protocol          = "HTTP"

  default_action {
    type             = "forward"
    target_group_arn = aws_lb_target_group.api.arn
  }
}

resource "aws_db_subnet_group" "mysql" {
  name       = "${var.project_name}-db-subnets"
  subnet_ids = [for subnet in aws_subnet.private : subnet.id]
}

resource "random_password" "mysql" {
  length           = 20
  special          = true
  override_special = "_%-+!#"
}

resource "aws_db_instance" "mysql" {
  identifier             = "${var.project_name}-mysql"
  allocated_storage      = var.mysql_allocated_storage
  engine                 = "mysql"
  engine_version         = "8.0"
  instance_class         = "db.t3.micro"
  db_name                = var.mysql_database
  username               = var.mysql_username
  password               = random_password.mysql.result
  db_subnet_group_name   = aws_db_subnet_group.mysql.name
  vpc_security_group_ids = [aws_security_group.mysql.id]
  publicly_accessible    = false
  skip_final_snapshot    = true
  deletion_protection    = false
  storage_encrypted      = false
  multi_az               = false
}

resource "aws_dynamodb_table" "shopping_carts" {
  name         = "${var.project_name}-carts"
  billing_mode = "PAY_PER_REQUEST"
  hash_key     = "cart_id"

  attribute {
    name = "cart_id"
    type = "S"
  }

  attribute {
    name = "customer_id"
    type = "N"
  }

  attribute {
    name = "created_at"
    type = "S"
  }

  global_secondary_index {
    name            = "customer_id-created_at-index"
    hash_key        = "customer_id"
    range_key       = "created_at"
    projection_type = "ALL"
  }
}

data "aws_iam_role" "lab_role" {
  name = "LabRole"
}

resource "docker_image" "api" {
  name = "${aws_ecr_repository.api.repository_url}:latest"

  build {
    context = "../src"
  }
}

resource "docker_registry_image" "api" {
  name = docker_image.api.name
}

resource "aws_ecs_task_definition" "api" {
  family                   = "${var.project_name}-task"
  network_mode             = "awsvpc"
  requires_compatibilities = ["FARGATE"]
  cpu                      = var.task_cpu
  memory                   = var.task_memory
  execution_role_arn       = data.aws_iam_role.lab_role.arn
  task_role_arn            = data.aws_iam_role.lab_role.arn

  container_definitions = jsonencode([
    {
      name      = "${var.project_name}-container"
      image     = docker_image.api.name
      essential = true
      environment = [
        for key in sort(keys(local.api_environment)) : {
          name  = key
          value = local.api_environment[key]
        }
      ]
      portMappings = [
        {
          containerPort = var.container_port
        }
      ]
      logConfiguration = {
        logDriver = "awslogs"
        options = {
          "awslogs-group"         = aws_cloudwatch_log_group.api.name
          "awslogs-region"        = var.aws_region
          "awslogs-stream-prefix" = "ecs"
        }
      }
    }
  ])
}

resource "aws_ecs_service" "api" {
  name            = "${var.project_name}-service"
  cluster         = aws_ecs_cluster.this.id
  task_definition = aws_ecs_task_definition.api.arn
  desired_count   = var.api_desired_count
  launch_type     = "FARGATE"

  network_configuration {
    subnets          = [for subnet in aws_subnet.private : subnet.id]
    security_groups  = [aws_security_group.api.id]
    assign_public_ip = false
  }

  load_balancer {
    target_group_arn = aws_lb_target_group.api.arn
    container_name   = "${var.project_name}-container"
    container_port   = var.container_port
  }

  depends_on = [
    aws_lb_listener.http,
    docker_registry_image.api
  ]
}
