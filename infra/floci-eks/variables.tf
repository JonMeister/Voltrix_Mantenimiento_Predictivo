variable "aws_region" {
  description = "Región AWS emulada en Floci"
  type        = string
  default     = "us-east-1"
}

variable "floci_endpoint" {
  description = "Endpoint local de Floci"
  type        = string
  default     = "http://localhost:4566"
}

variable "floci_access_key" {
  description = "Credencial ficticia para emulador local Floci (inyectada por tfvars o TF_VAR)"
  type        = string
  default     = "test"
  sensitive   = true
}

variable "floci_secret_key" {
  description = "Credencial ficticia para emulador local Floci (inyectada por tfvars o TF_VAR)"
  type        = string
  default     = "test"
  sensitive   = true
}

variable "cluster_name" {
  description = "Nombre del clúster EKS emulado"
  type        = string
  default     = "voltrix-mantenimiento-eks"
}

variable "kubernetes_version" {
  description = "Versión de Kubernetes del clúster EKS"
  type        = string
  default     = "1.31"
}

