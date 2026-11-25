output "cluster_name" {
  description = "Nombre del clúster EKS creado en Floci"
  value       = aws_eks_cluster.mantenimiento.name
}

output "cluster_endpoint" {
  description = "Endpoint de la API de Kubernetes del clúster emulado"
  value       = aws_eks_cluster.mantenimiento.endpoint
}

output "cluster_status" {
  description = "Estado del clúster EKS"
  value       = aws_eks_cluster.mantenimiento.status
}

output "kubeconfig_command" {
  description = "Comando AWS CLI para actualizar kubeconfig apuntando a Floci"
  value       = "aws --endpoint-url=${var.floci_endpoint} eks update-kubeconfig --region ${var.aws_region} --name ${aws_eks_cluster.mantenimiento.name}"
}

