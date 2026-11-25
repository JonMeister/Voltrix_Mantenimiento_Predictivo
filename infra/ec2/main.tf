terraform {
  required_version = ">= 1.5.0"

  required_providers {
    aws = {
      source  = "hashicorp/aws"
      version = "~> 5.60"
    }
  }
}

provider "aws" {
  region = var.aws_region
  # Las credenciales AWS se suministran por variables de entorno (AWS_ACCESS_KEY_ID,
  # AWS_SECRET_ACCESS_KEY) o perfil AWS CLI; NUNCA se escriben en este archivo.
}

data "aws_ami" "ubuntu_2404" {
  most_recent = true
  owners      = ["099720109477"] # Canonical

  filter {
    name   = "name"
    values = ["ubuntu/images/hvm-ssd-gp3/ubuntu-noble-24.04-amd64-server-*"]
  }

  filter {
    name   = "virtualization-type"
    values = ["hvm"]
  }
}

resource "aws_security_group" "mantenimiento_sg" {
  name        = "voltrix-mantenimiento-ec2-sg"
  description = "Expone solo el puerto de la API (8080) y SSH administrativo. PostgreSQL nunca se expone."

  ingress {
    description = "API HTTP Mantenimiento Predictivo"
    from_port   = var.api_port
    to_port     = var.api_port
    protocol    = "tcp"
    cidr_blocks = ["0.0.0.0/0"]
  }

  ingress {
    description = "SSH administrativo"
    from_port   = 22
    to_port     = 22
    protocol    = "tcp"
    cidr_blocks = [var.ssh_allowed_cidr]
  }

  egress {
    description = "Salida para consumo de otras celulas y descarga de paquetes"
    from_port   = 0
    to_port     = 0
    protocol    = "-1"
    cidr_blocks = ["0.0.0.0/0"]
  }

  tags = {
    Name     = "voltrix-mantenimiento-ec2-sg"
    Proyecto = "VoltrixEnergy"
    Equipo   = "equipo-6"
  }
}

resource "aws_instance" "mantenimiento_ec2" {
  ami                         = data.aws_ami.ubuntu_2404.id
  instance_type               = var.instance_type
  key_name                    = var.key_pair_name
  vpc_security_group_ids      = [aws_security_group.mantenimiento_sg.id]
  associate_public_ip_address = true

  root_block_device {
    volume_size = 16
    volume_type = "gp3"
    encrypted   = true
  }

  user_data = templatefile("${path.module}/user_data.sh.tpl", {
    repo_url          = var.repo_url
    repo_branch       = var.repo_branch
    api_port          = var.api_port
    postgres_db       = var.postgres_db
    postgres_user     = var.postgres_user
    postgres_password = var.postgres_password
    data_source       = var.data_source
    url_inventario    = var.url_inventario
    url_alertas       = var.url_alertas
    url_reportes      = var.url_reportes
    url_pronosticos   = var.url_pronosticos
  })

  tags = {
    Name     = "voltrix-equipo6-mantenimiento"
    Proyecto = "VoltrixEnergy"
    Equipo   = "equipo-6"
    Servicio = "mantenimiento-predictivo"
  }
}

