output "ec2_public_ip" {
  description = "IP pública de la instancia EC2"
  value       = aws_instance.mantenimiento_ec2.public_ip
}

output "api_base_url" {
  description = "URL base del despliegue en EC2 (para hoja Coordinacion_Voltrix)"
  value       = "http://${aws_instance.mantenimiento_ec2.public_ip}:${var.api_port}"
}

output "openapi_url" {
  description = "URL del contrato OpenAPI (para hoja Coordinacion_Voltrix)"
  value       = "http://${aws_instance.mantenimiento_ec2.public_ip}:${var.api_port}/openapi.yaml"
}

output "health_url" {
  description = "URL del endpoint /health (para hoja Coordinacion_Voltrix)"
  value       = "http://${aws_instance.mantenimiento_ec2.public_ip}:${var.api_port}/health"
}

