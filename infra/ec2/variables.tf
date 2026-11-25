variable "aws_region" {
  description = "Región de AWS para el despliegue EC2"
  type        = string
  default     = "us-east-1"
}

variable "instance_type" {
  description = "Tipo de instancia elegible según plan/fecha de cuenta AWS (1 GiB RAM: t3.micro o t2.micro)"
  type        = string
  default     = "t3.micro"
}

variable "key_pair_name" {
  description = "Nombre del Key Pair existente en AWS para acceso SSH"
  type        = string
}

variable "ssh_allowed_cidr" {
  description = "Rango CIDR autorizado para SSH (recomendado: tu IP pública /32)"
  type        = string
  default     = "0.0.0.0/0"
}

variable "repo_url" {
  description = "URL pública del repositorio Git del Equipo 6"
  type        = string
  default     = "https://github.com/JonMeister/Voltrix_Mantenimiento_Predictivo.git"
}

variable "repo_branch" {
  description = "Rama a desplegar en EC2"
  type        = string
  default     = "main"
}

variable "api_port" {
  description = "Puerto público de la API"
  type        = number
  default     = 8080
}

variable "postgres_db" {
  type    = string
  default = "voltrix_mantenimiento"
}

variable "postgres_user" {
  type    = string
  default = "voltrix_user"
}

variable "postgres_password" {
  description = "Contraseña de PostgreSQL (inyectar vía terraform.tfvars local o TF_VAR_postgres_password)"
  type        = string
  sensitive   = true
}

variable "data_source" {
  description = "mock (Avance 1) o http (Avance 2 / Final)"
  type        = string
  default     = "mock"
}

variable "url_inventario" {
  type    = string
  default = ""
}

variable "url_alertas" {
  type    = string
  default = ""
}

variable "url_reportes" {
  type    = string
  default = ""
}

variable "url_pronosticos" {
  type    = string
  default = ""
}

