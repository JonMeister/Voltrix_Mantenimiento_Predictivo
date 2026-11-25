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
  region                      = var.aws_region
  access_key                  = var.floci_access_key
  secret_key                  = var.floci_secret_key
  skip_credentials_validation = true
  skip_metadata_api_check     = true
  skip_requesting_account_id  = true
  s3_use_path_style           = true

  endpoints {
    ec2 = var.floci_endpoint
    eks = var.floci_endpoint
    iam = var.floci_endpoint
    sts = var.floci_endpoint
    ecr = var.floci_endpoint
    s3  = var.floci_endpoint
  }
}

# --- Rol IAM de mínimo privilegio para el clúster EKS ---
data "aws_iam_policy_document" "eks_assume_role" {
  statement {
    effect  = "Allow"
    actions = ["sts:AssumeRole"]
    principals {
      type        = "Service"
      identifiers = ["eks.amazonaws.com"]
    }
  }
}

resource "aws_iam_role" "eks_cluster_role" {
  name               = "${var.cluster_name}-role"
  assume_role_policy = data.aws_iam_policy_document.eks_assume_role.json

  tags = {
    Proyecto = "VoltrixEnergy"
    Equipo   = "equipo-6"
    Servicio = "mantenimiento-predictivo"
  }
}

resource "aws_iam_role_policy_attachment" "eks_cluster_policy" {
  role       = aws_iam_role.eks_cluster_role.name
  policy_arn = "arn:aws:iam::aws:policy/AmazonEKSClusterPolicy"
}

# --- Red VPC dedicada para el clúster emulado en Floci ---
resource "aws_vpc" "eks_vpc" {
  cidr_block           = "10.60.0.0/16"
  enable_dns_hostnames = true
  enable_dns_support   = true

  tags = {
    Name     = "${var.cluster_name}-vpc"
    Proyecto = "VoltrixEnergy"
    Equipo   = "equipo-6"
  }
}

resource "aws_subnet" "eks_subnet_a" {
  vpc_id            = aws_vpc.eks_vpc.id
  cidr_block        = "10.60.1.0/24"
  availability_zone = "${var.aws_region}a"

  tags = {
    Name = "${var.cluster_name}-subnet-a"
  }
}

resource "aws_subnet" "eks_subnet_b" {
  vpc_id            = aws_vpc.eks_vpc.id
  cidr_block        = "10.60.2.0/24"
  availability_zone = "${var.aws_region}b"

  tags = {
    Name = "${var.cluster_name}-subnet-b"
  }
}

# --- Clúster EKS emulado en Floci ---
resource "aws_eks_cluster" "mantenimiento" {
  name     = var.cluster_name
  role_arn = aws_iam_role.eks_cluster_role.arn
  version  = var.kubernetes_version

  vpc_config {
    subnet_ids = [
      aws_subnet.eks_subnet_a.id,
      aws_subnet.eks_subnet_b.id,
    ]
  }

  depends_on = [
    aws_iam_role_policy_attachment.eks_cluster_policy,
  ]

  tags = {
    Proyecto = "VoltrixEnergy"
    Equipo   = "equipo-6"
    Servicio = "mantenimiento-predictivo"
  }
}

