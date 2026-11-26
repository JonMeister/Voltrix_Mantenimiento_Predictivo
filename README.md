# Voltrix Energy · Equipo 6 — Mantenimiento Predictivo

Servicio backend de **Mantenimiento Predictivo** para la plataforma distribuida de monitoreo y control de microplantas renovables **Voltrix Energy S.A.S.**

- **Integrantes:** Brandon Alexis Franco, Jonathan Aristizábal, Mateo Noguera, Isabella Bermúdez, Juan Marín.
- **Stack tecnológico:** Go 1.22+ · PostgreSQL 16 Alpine · Docker / Docker Compose · Kubernetes (Floci EKS) · Terraform (IaC).
- **Contrato OpenAPI 3.0.3:** [openapi.yaml](./openapi.yaml) (servido en vivo en `GET /openapi.yaml` y documentación visual Redoc en `GET /docs`).

---

## Estructura del Repositorio

```text
Voltrix_Mantenimiento/
├── cmd/api/                   # Punto de entrada HTTP y ciclo de vida del servicio
├── internal/
│   ├── config/                # Carga estricta de configuración por variables de entorno
│   ├── domain/                # DTOs OpenAPI 3.0.3, validaciones y máquina de estados
│   ├── client/                # Adaptadores HTTP (Equipos 1, 3, 4 y 5) + proveedor mock Avance 1
│   ├── workerpool/            # Motor secuencial (W=1) vs. Worker Pool paralelo por lotes (Go)
│   ├── repository/            # Persistencia PostgreSQL 16 (pgx/v5) y migraciones
│   ├── service/               # Orquestador concurrente (errgroup) y generación de órdenes
│   └── handler/               # Endpoints REST, trazabilidad JSON y semáforo 429
├── migrations/                # Esquema SQL (001_init.sql) y semilla Avance 1 (002_seed_avance1.sql)
├── k8s/                       # Manifiestos Kubernetes (Deployment 2 réplicas, Service, ConfigMap, Secret)
├── infra/
│   ├── floci-eks/             # IaC Terraform para clúster EKS emulado en Floci + IAM mínimo privilegio
│   └── ec2/                   # IaC Terraform para despliegue real en AWS EC2 (1 GiB RAM + Swap 1 GiB)
├── Dockerfile                 # Multi-stage Go -> Alpine no-root (< 25 MB)
├── docker-compose.yml         # API + PostgreSQL privado con límites de memoria (192m + 256m)
├── openapi.yaml               # Contrato oficial OpenAPI 3.0.3
├── index.html                 # Visor Redoc estático embebido en /docs
└── .env.example               # Plantilla de variables (NO subir .env real)
```

---

## 1. Ejecución Local con Docker Compose (Componente B)

```bash
cp .env.example .env
# Edita .env y asigna una contraseña segura en POSTGRES_PASSWORD

docker compose up -d --build
```

### Verificación rápida de endpoints

```bash
# Salud del servicio y PostgreSQL
curl -s http://localhost:8086/health

# Contrato OpenAPI
curl -s http://localhost:8080/openapi.yaml | head -n 20

# Listar órdenes de ejemplo (datos semilla compatibles con Equipo 7)
curl -s -H "X-Equipo-Consumidor: equipo-7" \
  "http://localhost:8086/mantenimiento/ordenes?plantaId=PL-001"

# Consultar una orden específica
curl -s -H "X-Equipo-Consumidor: equipo-7" \
  http://localhost:8080/mantenimiento/ordenes/OM-20261119-001

# Generar órdenes en modo paralelo (Worker Pool)
curl -s -X POST -H "Content-Type: application/json" -H "X-Equipo-Consumidor: equipo-6" \
  "http://localhost:8080/mantenimiento/generar?modo=paralelo&workers=4" \
  -d '{"plantaIds":["PL-001","PL-002"],"desde":"2026-11-01T00:00:00-05:00","hasta":"2026-11-18T23:59:59-05:00","umbralRiesgo":50,"tamanoLote":2}'

# Verificar uso de memoria en contenedores
docker stats --no-stream voltrix-mantenimiento-api voltrix-mantenimiento-db
```

---

## 2. Despliegue Real en AWS EC2 con Terraform (Componente B)

1. Configura tus credenciales de AWS fuera del repositorio (`aws configure` o variables `AWS_ACCESS_KEY_ID` / `AWS_SECRET_ACCESS_KEY`).
2. Prepara tus variables locales:
   ```bash
   cd infra/ec2
   cp terraform.tfvars.example terraform.tfvars
   # Edita key_pair_name, instance_type (verificar plan de cuenta AWS) y postgres_password
   ```
3. Aprovisiona la instancia (configura automáticamente 1 GiB de Swap, Docker, Security Group solo `:8080` y levanta `docker compose`):
   ```bash
   terraform init
   terraform apply
   ```
4. Copia los outputs (`api_base_url`, `openapi_url`, `health_url`) en la hoja compartida `Coordinacion_Voltrix`.

---

## 3. Clúster EKS Emulado en Floci (Componente A)

1. Inicia Floci (requiere Docker en modo root):
   ```bash
   docker start floci   # o docker run -d --name floci -p 4566:4566 -v /var/run/docker.sock:/var/run/docker.sock floci/floci:latest
   ```
2. Crea el clúster EKS y el rol IAM de mínimo privilegio con Terraform (o mediante el contenedor oficial `hashicorp/terraform`):
   ```bash
   cd infra/floci-eks
   cp terraform.tfvars.example terraform.tfvars
   terraform init
   terraform apply
   ```
3. Extrae el `kubeconfig` del clúster emulado (archivo `kubeconfig.floci` ignorado por `.gitignore`) y verifica los nodos:
   ```bash
   docker exec floci-aws-eks-voltrix-mantenimiento-eks cat /etc/rancher/k3s/k3s.yaml \
     | sed 's|https://127.0.0.1:6443|https://localhost:6500|g' > kubeconfig.floci
   chmod 600 kubeconfig.floci
   KUBECONFIG=kubeconfig.floci kubectl get nodes -o wide
   ```
4. Despliega los manifiestos (a partir de Avance 1/2):
   ```bash
   cp k8s/secret.example.yaml k8s/secret.yaml  # edita contraseña (archivo ignorado por Git)
   kubectl apply -f k8s/configmap.yaml -f k8s/secret.yaml -f k8s/postgres.yaml -f k8s/deployment.yaml -f k8s/service.yaml
   kubectl get pods,svc,deployments
   ```

