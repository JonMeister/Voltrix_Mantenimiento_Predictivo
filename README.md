# Voltrix Energy · Equipo 6 — Mantenimiento Predictivo

Servicio backend de **Mantenimiento Predictivo** para la plataforma distribuida **Voltrix Energy S.A.S.**

- **Integrantes:** Brandon Alexis Franco, Jonathan Aristizábal, Mateo Noguera, Isabella Bermúdez, Juan Marín.
- **Stack:** Go 1.22+ · PostgreSQL 16 Alpine · Docker / Docker Compose · Kubernetes (Floci EKS) · Terraform (IaC).
- **Imagen Docker Hub:** `jonmeister/voltrix-mantenimiento-api` (controlada por `SERVICE_VERSION`, por defecto `1.0.0`).
- **Contrato OpenAPI 3.0.3:** [openapi.yaml](./openapi.yaml) (`GET /openapi.yaml` y Redoc en `GET /docs`).

---

## Estructura del Repositorio

```text
Voltrix_Mantenimiento/
├── cmd/api/                   # Punto de entrada HTTP
├── internal/
│   ├── config/                # Configuración por variables de entorno
│   ├── domain/                # DTOs OpenAPI 3.0.3 y validaciones
│   ├── client/                # Adaptadores HTTP (Equipos 1, 3, 4 y 5) + mock Avance 1
│   ├── workerpool/            # Motor de evaluación de riesgo
│   ├── repository/            # PostgreSQL 16 (pgx/v5) y migraciones
│   ├── service/               # Orquestación y generación de órdenes
│   └── handler/               # Endpoints REST, trazabilidad JSON y semáforo 429
├── migrations/                # 001_init.sql y semilla 002_seed_avance1.sql
├── k8s/                       # Manifiestos Kubernetes (ConfigMap, Secret, Postgres, Deployment, Service)
├── infra/
│   ├── floci-eks/             # Terraform para clúster EKS en Floci
│   └── ec2/                   # Terraform para AWS EC2 (t3.small, 2 GiB RAM + 1 GiB Swap)
├── Dockerfile                 # Multi-stage Go -> Alpine no-root
├── docker-compose.yml         # API + PostgreSQL en red privada (192m + 256m)
├── openapi.yaml               # Contrato oficial OpenAPI 3.0.3
├── index.html                 # Visor Redoc embebido en /docs
└── .env.example               # Plantilla de variables (NO subir .env)
```

---

## 1. Ejecución Local (Docker Compose)

```bash
cp .env.example .env
# Edita .env y asigna POSTGRES_PASSWORD

docker compose pull
docker compose up -d
```
*(Para recompilar localmente tras cambiar código Go: `docker compose up -d --build`.)*

### Verificación rápida
```bash
# Salud y base de datos
curl -s http://localhost:8080/health | python3 -m json.tool --no-ensure-ascii

# Contrato OpenAPI
curl -s http://localhost:8080/openapi.yaml | head -n 20

# Listar órdenes (ejemplo Equipo 7)
curl -s -H "X-Equipo-Consumidor: equipo-7" \
  "http://localhost:8080/mantenimiento/ordenes?plantaId=PL-001" | python3 -m json.tool --no-ensure-ascii

# Consultar una orden por ID
curl -s -H "X-Equipo-Consumidor: equipo-7" \
  http://localhost:8080/mantenimiento/ordenes/OM-20261119-001 | python3 -m json.tool --no-ensure-ascii

# Generar órdenes predictivas
curl -s -X POST -H "Content-Type: application/json" -H "X-Equipo-Consumidor: equipo-6" \
  http://localhost:8080/mantenimiento/generar \
  -d '{"plantaIds":["PL-001","PL-002"],"desde":"2026-11-01T00:00:00-05:00","hasta":"2026-11-18T23:59:59-05:00","umbralRiesgo":50}' \
  | python3 -m json.tool --no-ensure-ascii

# Logs y memoria
docker compose logs -f --tail 50 api
docker stats --no-stream voltrix-mantenimiento-api voltrix-mantenimiento-db
```

### Apagar en local
```bash
docker compose down      # conserva datos de PostgreSQL
docker compose down -v   # reinicia PostgreSQL desde cero
```

---

## 2. Despliegue en AWS EC2 con Terraform (Componente B)

### Configurar AWS y Key Pair (fuera del repo)
```bash
aws configure
aws sts get-caller-identity

# Solo si no tienes un Key Pair en us-east-1:
aws ec2 create-key-pair --region us-east-1 --key-name voltrix-equipo6-key \
  --query 'KeyMaterial' --output text > ~/.ssh/voltrix-equipo6-key.pem
chmod 400 ~/.ssh/voltrix-equipo6-key.pem
```

### Aprovisionar EC2
```bash
cd infra/ec2
cp terraform.tfvars.example terraform.tfvars
# Edita key_pair_name, instance_type, ssh_allowed_cidr (tu IP /32) y postgres_password

terraform init
terraform apply
```

### Verificar despliegue público (esperar 2-3 min tras `apply`)
```bash
export API_URL=$(terraform output -raw api_base_url)
export EC2_IP=$(terraform output -raw ec2_public_ip)

curl -s "$API_URL/health" | python3 -m json.tool --no-ensure-ascii
curl -s -H "X-Equipo-Consumidor: equipo-7" \
  "$API_URL/mantenimiento/ordenes?plantaId=PL-001" | python3 -m json.tool --no-ensure-ascii

# Ver logs de peticiones por SSH (sin /health)
ssh -i ~/.ssh/voltrix-equipo6-key.pem ubuntu@"$EC2_IP" \
  "docker logs voltrix-mantenimiento-api | grep -v '\"/health\"'"
```
Registra los outputs `api_base_url`, `openapi_url` y `health_url` en `Coordinacion_Voltrix`.

### Actualizar o destruir EC2
```bash
# Actualizar imagen/código en EC2
ssh -i ~/.ssh/voltrix-equipo6-key.pem ubuntu@"$EC2_IP" \
  "cd /opt/voltrix-mantenimiento && git pull && docker compose pull && docker compose up -d"

# Destruir recursos en AWS
terraform destroy
```

---

## 3. Clúster EKS en Floci y Pods Kubernetes (Componente A)

### Levantar Floci y crear clúster con Terraform
```bash
docker start floci
# (Si no existe): docker run -d --name floci -p 4566:4566 -v /var/run/docker.sock:/var/run/docker.sock floci/floci:latest

cd infra/floci-eks
terraform init
terraform apply -auto-approve
```

### Conectar `kubectl`
```bash
cd ../..
K8S_PORT=$(docker port floci-aws-eks-voltrix-mantenimiento-eks 6443 | head -n1 | awk -F: '{print $NF}')
docker exec floci-aws-eks-voltrix-mantenimiento-eks cat /etc/rancher/k3s/k3s.yaml \
  | sed "s|https://127.0.0.1:6443|https://localhost:${K8S_PORT}|g" > kubeconfig.floci
chmod 600 kubeconfig.floci

KUBECONFIG=kubeconfig.floci kubectl get nodes -o wide
```

### Desplegar y probar Pods (`k8s/`)
```bash
cp k8s/secret.example.yaml k8s/secret.yaml
# Edita k8s/secret.yaml (archivo ignorado por Git)

KUBECONFIG=kubeconfig.floci kubectl apply \
  -f k8s/configmap.yaml -f k8s/secret.yaml -f k8s/postgres.yaml -f k8s/deployment.yaml -f k8s/service.yaml

KUBECONFIG=kubeconfig.floci kubectl rollout status deployment/mantenimiento-db
KUBECONFIG=kubeconfig.floci kubectl rollout status deployment/mantenimiento-api
KUBECONFIG=kubeconfig.floci kubectl get pods,svc,deployments -o wide
```

```bash
# Terminal 1: túnel hacia el Service
KUBECONFIG=kubeconfig.floci kubectl port-forward svc/mantenimiento-svc 8088:8080

# Terminal 2: probar API en Kubernetes
curl -s http://localhost:8088/health | python3 -m json.tool --no-ensure-ascii
KUBECONFIG=kubeconfig.floci kubectl logs -f -l app=voltrix-mantenimiento --tail=50
```
